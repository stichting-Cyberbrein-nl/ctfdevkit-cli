// Package update handles self-updating the devkit CLI binary.
package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/platform"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/releases"
)

// SelfUpdate downloads and atomically replaces the current binary with the latest version.
func SelfUpdate(ctx context.Context, manifest *releases.Manifest, currentVersion string, plat platform.Platform) error {
	newer, err := manifest.IsNewerCLI(currentVersion)
	if err != nil {
		return fmt.Errorf("version comparison failed: %w", err)
	}
	if !newer {
		output.Successf("Already on latest CLI version (%s)", currentVersion)
		return nil
	}

	assetKey := plat.AssetKey()
	asset, ok := manifest.CLI.Assets[assetKey]
	if !ok {
		return fmt.Errorf("no binary available for platform %s", assetKey)
	}

	output.Infof("Updating devkit CLI: %s → %s", currentVersion, manifest.CLI.Version)
	output.Infof("Downloading from: %s", asset.URL)

	// Determine where the current binary lives.
	selfPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine current binary path: %w", err)
	}
	selfPath, err = filepath.EvalSymlinks(selfPath)
	if err != nil {
		return fmt.Errorf("cannot resolve binary symlink: %w", err)
	}

	// Download the archive to a temp file.
	archiveTmp, err := os.CreateTemp("", "devkit-archive-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	archivePath := archiveTmp.Name()
	archiveTmp.Close()
	defer os.Remove(archivePath)

	if err := releases.DownloadVerified(ctx, asset.URL, archivePath, asset.SHA256); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	// Extract the binary from the archive.
	binaryName := "devkit"
	if runtime.GOOS == "windows" {
		binaryName = "devkit.exe"
	}

	dir := filepath.Dir(selfPath)
	tmpPath := selfPath + ".new"

	if strings.HasSuffix(asset.URL, ".zip") {
		if err := extractFromZip(archivePath, binaryName, tmpPath); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("extracting binary from zip: %w", err)
		}
	} else {
		if err := extractFromTarGz(archivePath, binaryName, tmpPath); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("extracting binary from tar.gz: %w", err)
		}
	}

	// Make executable on Unix.
	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmpPath, 0755); err != nil {
			os.Remove(tmpPath)
			return err
		}
	}

	if runtime.GOOS == "windows" {
		return windowsReplace(selfPath, tmpPath, dir)
	}

	// Atomic rename on Unix.
	if err := os.Rename(tmpPath, selfPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("replacing binary failed: %w", err)
	}

	output.Successf("devkit CLI updated to v%s", manifest.CLI.Version)
	output.Hint("Restart your terminal or re-run devkit to use the new version.")
	return nil
}

// extractFromTarGz finds binaryName inside a .tar.gz archive and writes it to destPath.
func extractFromTarGz(archivePath, binaryName, destPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("opening gzip stream: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar: %w", err)
		}

		// Match the binary by base name (ignoring any directory prefix).
		if filepath.Base(hdr.Name) != binaryName {
			continue
		}

		out, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
		if err != nil {
			return err
		}

		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return fmt.Errorf("writing binary: %w", err)
		}
		out.Close()
		return nil
	}

	return fmt.Errorf("binary %q not found in archive", binaryName)
}

// extractFromZip finds binaryName inside a .zip archive and writes it to destPath.
func extractFromZip(archivePath, binaryName, destPath string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer zr.Close()

	for _, f := range zr.File {
		if filepath.Base(f.Name) != binaryName {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		out, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
		if err != nil {
			rc.Close()
			return err
		}

		if _, err := io.Copy(out, rc); err != nil {
			rc.Close()
			out.Close()
			return fmt.Errorf("writing binary: %w", err)
		}
		rc.Close()
		out.Close()
		return nil
	}

	return fmt.Errorf("binary %q not found in archive", binaryName)
}

// windowsReplace handles the Windows file-lock issue by using a helper batch script
// that runs after the current process exits.
func windowsReplace(selfPath, newPath, dir string) error {
	batPath := filepath.Join(dir, "devkit-update.bat")

	// Escape % in paths to prevent batch variable expansion.
	esc := func(s string) string { return strings.ReplaceAll(s, "%", "%%") }
	batContent := fmt.Sprintf(`@echo off
ping -n 2 127.0.0.1 > nul
move /Y "%s" "%s"
del "%s"
`, esc(newPath), esc(selfPath), esc(batPath))

	if err := os.WriteFile(batPath, []byte(batContent), 0644); err != nil {
		os.Remove(newPath)
		return err
	}

	// Empty string before /min is the window title — required by start.exe when
	// the path contains spaces, otherwise start treats the path as the title.
	cmd := exec.Command("cmd", "/c", "start", "", "/min", batPath)
	if err := cmd.Start(); err != nil {
		os.Remove(newPath)
		os.Remove(batPath)
		return err
	}

	output.Successf("Update staged — binary will be replaced when this process exits.")
	return nil
}
