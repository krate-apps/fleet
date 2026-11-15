package unpack

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Rar extracts archivePath into destDir using the unrar CLI.
// Both paths must resolve under allowedRoots. Symlink archives are refused.
// unrarBin defaults to "unrar" when empty (must be on PATH).
func Rar(archivePath, destDir string, allowedRoots []string, unrarBin string) (extracted int, err error) {
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
	ext := strings.ToLower(filepath.Ext(archivePath))
	if ext != ".rar" {
		return 0, fmt.Errorf("expected .rar archive, got %q", ext)
	}
	if err := os.MkdirAll(destDir, 0o750); err != nil {
		return 0, err
	}

	bin := strings.TrimSpace(unrarBin)
	if bin == "" {
		bin = "unrar"
	}
	resolved, err := exec.LookPath(bin)
	if err != nil {
		return 0, fmt.Errorf("unrar binary not found (%q): %w", bin, err)
	}

	// List members first so we can refuse absolute / traversal paths before extract.
	listOut, err := runUnrar(resolved, "lb", archivePath)
	if err != nil {
		return 0, fmt.Errorf("unrar list: %w", err)
	}
	names := strings.Split(strings.ReplaceAll(string(listOut), "\r\n", "\n"), "\n")
	count := 0
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		clean := filepath.Clean(name)
		if filepath.IsAbs(clean) || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) || clean == ".." {
			return 0, fmt.Errorf("refusing unsafe member path %q", name)
		}
		target := filepath.Join(destDir, clean)
		if err := assertContained(destDir, target); err != nil {
			return 0, err
		}
		count++
		if count > maxZipEntries {
			return 0, fmt.Errorf("archive has too many entries")
		}
	}
	if count == 0 {
		return 0, fmt.Errorf("archive has no extractable members")
	}

	// x = extract with paths; -o+ overwrite; -idq quieter; -p- empty password prompt skip.
	if _, err := runUnrar(resolved, "x", "-o+", "-idq", "-p-", archivePath, destDir+string(os.PathSeparator)); err != nil {
		return 0, fmt.Errorf("unrar extract: %w", err)
	}
	return count, nil
}

func runUnrar(bin string, args ...string) ([]byte, error) {
	cmd := exec.Command(bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("%s", msg)
	}
	return stdout.Bytes(), nil
}
