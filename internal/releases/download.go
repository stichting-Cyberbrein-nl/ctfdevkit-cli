package releases

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// DownloadVerified downloads a file from url to destPath, verifying the SHA256 checksum.
func DownloadVerified(ctx context.Context, url, destPath, expectedSHA256 string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("User-Agent", "devkit-cli/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download server returned %d for %s", resp.StatusCode, url)
	}

	tmp, err := os.CreateTemp("", "devkit-download-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	h := sha256.New()
	w := io.MultiWriter(tmp, h)

	if _, err := io.Copy(w, resp.Body); err != nil {
		tmp.Close()
		return fmt.Errorf("writing download: %w", err)
	}
	tmp.Close()

	// Verify checksum.
	if expectedSHA256 != "" {
		actual := hex.EncodeToString(h.Sum(nil))
		if actual != expectedSHA256 {
			return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedSHA256, actual)
		}
	}

	// Atomic rename to destination.
	if err := os.Rename(tmpName, destPath); err != nil {
		// Cross-device rename fallback.
		return copyFile(tmpName, destPath)
	}

	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
