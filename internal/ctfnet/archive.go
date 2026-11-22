package ctfnet

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// MaxArchiveBytes is the largest zip the platform accepts.
const MaxArchiveBytes = 20 << 20

var nameRule = regexp.MustCompile(`^\[\d{1,4}\] [^/\\<>:"|?*#%+]{1,60}$`)

// ValidName reports whether name is an acceptable assignment folder name, e.g. "[12] Cookie stealing".
func ValidName(name string) bool {
	return nameRule.MatchString(name) && !strings.HasSuffix(name, ".") && !strings.HasSuffix(name, " ")
}

// ZipAssignment zips dir with every entry prefixed by the folder name.
func ZipAssignment(dir string) ([]byte, error) {
	base := filepath.Base(dir)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == dir {
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%s is a symlink; the platform does not accept symlinks", path)
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == ".DS_Store" || d.Name() == "Thumbs.db" {
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("%s is not a regular file", path)
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		w, err := zw.CreateHeader(&zip.FileHeader{Name: base + "/" + filepath.ToSlash(rel), Method: zip.Deflate})
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(w, f)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("zipping %s: %w", dir, err)
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("zipping %s: %w", dir, err)
	}
	if buf.Len() > MaxArchiveBytes {
		return nil, fmt.Errorf("archive is %.1f MB; the platform accepts at most 20 MB", float64(buf.Len())/(1<<20))
	}
	return buf.Bytes(), nil
}
