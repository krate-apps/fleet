package buckets

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxArchiveBytes = 256 << 20 // 256 MiB compressed
	maxFiles        = 10_000
	maxFileBytes    = 128 << 20 // 128 MiB single file
)

// VerifySHA256 checks a file against an expected lowercase/uppercase hex digest.
func VerifySHA256(path, expected string) error {
	expected = strings.TrimSpace(strings.ToLower(expected))
	if len(expected) != 64 {
		return fmt.Errorf("sha256 must be 64 hex characters")
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, io.LimitReader(f, maxArchiveBytes+1)); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if got != expected {
		return fmt.Errorf("sha256 mismatch: got %s want %s", got, expected)
	}
	return nil
}

// ExtractTarGz safely unpacks a .tar.gz into destDir (BKT-010).
// Rejects absolute paths, .. traversal, symlinks/hardlinks, and oversized archives.
func ExtractTarGz(archivePath, destDir string) error {
	if err := os.MkdirAll(destDir, 0o700); err != nil {
		return err
	}
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return err
	}
	if st.Size() > maxArchiveBytes {
		return fmt.Errorf("archive exceeds %d bytes", maxArchiveBytes)
	}

	gz, err := gzip.NewReader(io.LimitReader(f, maxArchiveBytes))
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	var files int
	destAbs, err := filepath.Abs(destDir)
	if err != nil {
		return err
	}

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}
		files++
		if files > maxFiles {
			return fmt.Errorf("archive contains too many entries (>%d)", maxFiles)
		}
		name := filepath.Clean(hdr.Name)
		if name == "." || name == "" {
			continue
		}
		if filepath.IsAbs(name) || strings.HasPrefix(name, ".."+string(os.PathSeparator)) || name == ".." || strings.Contains(name, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("unsafe path in archive: %q", hdr.Name)
		}
		// Also reject Windows-style absolutes and drive letters if present.
		if strings.Contains(name, ":") {
			return fmt.Errorf("unsafe path in archive: %q", hdr.Name)
		}
		target := filepath.Join(destAbs, name)
		rel, err := filepath.Rel(destAbs, target)
		if err != nil || strings.HasPrefix(rel, "..") {
			return fmt.Errorf("path escapes destination: %q", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o700); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if hdr.Size > maxFileBytes {
				return fmt.Errorf("file too large in archive: %q", hdr.Name)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return err
			}
			mode := os.FileMode(hdr.Mode).Perm()
			if mode&0o111 != 0 {
				mode = 0o700
			} else {
				mode = 0o600
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
			if err != nil {
				return err
			}
			n, err := io.Copy(out, io.LimitReader(tr, maxFileBytes+1))
			_ = out.Close()
			if err != nil {
				return err
			}
			if n > maxFileBytes {
				_ = os.Remove(target)
				return fmt.Errorf("file too large in archive: %q", hdr.Name)
			}
		case tar.TypeSymlink, tar.TypeLink:
			return fmt.Errorf("links are not allowed in archives: %q", hdr.Name)
		default:
			return fmt.Errorf("unsupported tar entry type %v for %q", hdr.Typeflag, hdr.Name)
		}
	}
	return nil
}

// FindPackageRoot locates the directory containing fleet.ext.yaml under root
// (archive may wrap a single top-level folder).
func FindPackageRoot(root string) (string, error) {
	candidates := []string{root}
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	if len(entries) == 1 && entries[0].IsDir() {
		candidates = append(candidates, filepath.Join(root, entries[0].Name()))
	}
	for _, dir := range candidates {
		for _, name := range []string{"fleet.ext.yaml", "extension.yaml", "manifest.yaml"} {
			if st, err := os.Stat(filepath.Join(dir, name)); err == nil && !st.IsDir() {
				return dir, nil
			}
		}
	}
	return "", fmt.Errorf("no fleet.ext.yaml found in archive")
}
