// Package unpack extracts archives under configured download roots (path-guarded).
package unpack

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxZipEntries   = 10_000
	maxZipTotalSize = 8 << 30 // 8 GiB uncompressed budget
	maxZipFileSize  = 2 << 30 // 2 GiB per member
)

// Zip extracts archivePath into destDir. Both paths must resolve under allowedRoots.
// Symlink members, absolute paths, and ".." traversal are refused.
func Zip(archivePath, destDir string, allowedRoots []string) (extracted int, err error) {
	archivePath, err = resolveUnderRoots(archivePath, allowedRoots)
	if err != nil {
		return 0, fmt.Errorf("archive: %w", err)
	}
	destDir, err = resolveUnderRoots(destDir, allowedRoots)
	if err != nil {
		return 0, fmt.Errorf("dest: %w", err)
	}
	st, err := os.Lstat(archivePath)
	if err != nil {
		return 0, err
	}
	if st.Mode()&os.ModeSymlink != 0 || !st.Mode().IsRegular() {
		return 0, fmt.Errorf("archive must be a regular file")
	}
	if err := os.MkdirAll(destDir, 0o750); err != nil {
		return 0, err
	}

	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return 0, err
	}
	defer zr.Close()

	if len(zr.File) > maxZipEntries {
		return 0, fmt.Errorf("archive has too many entries (%d)", len(zr.File))
	}

	var total int64
	for _, f := range zr.File {
		if f.FileInfo().Mode()&os.ModeSymlink != 0 {
			return extracted, fmt.Errorf("refusing symlink member %q", f.Name)
		}
		name := filepath.Clean(f.Name)
		if name == "." || name == "" {
			continue
		}
		if filepath.IsAbs(name) || strings.HasPrefix(name, ".."+string(os.PathSeparator)) || name == ".." {
			return extracted, fmt.Errorf("refusing unsafe member path %q", f.Name)
		}
		target := filepath.Join(destDir, name)
		if err := assertContained(destDir, target); err != nil {
			return extracted, err
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o750); err != nil {
				return extracted, err
			}
			continue
		}
		if f.UncompressedSize64 > maxZipFileSize {
			return extracted, fmt.Errorf("member %q exceeds size limit", f.Name)
		}
		total += int64(f.UncompressedSize64)
		if total > maxZipTotalSize {
			return extracted, fmt.Errorf("archive uncompressed size exceeds budget")
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return extracted, err
		}
		if err := writeZipMember(f, target); err != nil {
			return extracted, err
		}
		extracted++
	}
	return extracted, nil
}

func writeZipMember(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return err
	}
	defer out.Close()
	n, err := io.Copy(out, io.LimitReader(rc, maxZipFileSize+1))
	if err != nil {
		return err
	}
	if n > maxZipFileSize {
		_ = os.Remove(target)
		return fmt.Errorf("member %q exceeded size limit while reading", f.Name)
	}
	return nil
}

func resolveUnderRoots(path string, roots []string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("empty path")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)
	if len(roots) == 0 {
		return "", fmt.Errorf("no download_roots configured; refuse path access")
	}
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		absRoot, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		absRoot = filepath.Clean(absRoot)
		rel, err := filepath.Rel(absRoot, abs)
		if err != nil {
			continue
		}
		if rel == "." || (!strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel)) {
			return abs, nil
		}
	}
	return "", fmt.Errorf("path %q is outside allowed download_roots", abs)
}

func assertContained(root, target string) error {
	root = filepath.Clean(root)
	target = filepath.Clean(target)
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("path %q escapes destination root", target)
	}
	return nil
}

// DefaultDest returns dest directory beside the archive (same parent, basename without .zip).
func DefaultDest(archivePath string) string {
	base := filepath.Base(archivePath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	if name == "" {
		name = "unpacked"
	}
	return filepath.Join(filepath.Dir(archivePath), name)
}
