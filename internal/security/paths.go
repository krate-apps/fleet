package security

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CheckPathPermissions validates that path exists and is not world-writable (SEC-007).
// It never attempts chmod 777 or other loosening.
func CheckPathPermissions(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("path required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	st, err := os.Lstat(abs)
	if err != nil {
		return fmt.Errorf("path %s: %w", abs, err)
	}
	if st.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("path %s: symlink not allowed for permission check root", abs)
	}
	perm := st.Mode().Perm()
	if perm&0o002 != 0 {
		return fmt.Errorf("path %s is world-writable (mode %#o); refusing to continue", abs, perm)
	}
	return nil
}

// CheckSocketAccess verifies a Unix socket path is usable without rewriting permissions.
func CheckSocketAccess(address string) error {
	address = strings.TrimSpace(address)
	if address == "" {
		return nil
	}
	path := address
	switch {
	case strings.HasPrefix(address, "unix://"):
		path = strings.TrimPrefix(address, "unix://")
	case strings.HasPrefix(address, "scgi://"):
		return nil // TCP SCGI — no local socket file
	case strings.Contains(address, "://"):
		return nil
	}
	st, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("rtorrent socket %s: %w", path, err)
	}
	if st.Mode()&os.ModeSocket == 0 && !st.Mode().IsRegular() {
		// Some systems expose sockets oddly; still reject world-writable.
	}
	if st.Mode().Perm()&0o002 != 0 {
		return fmt.Errorf("rtorrent socket %s is world-writable; refusing automatic chmod", path)
	}
	// Dial the socket — os.OpenFile on AF_UNIX often returns ENODEV/"no such device
	// or address" even when the listener is healthy.
	conn, err := net.DialTimeout("unix", path, 2*time.Second)
	if err != nil {
		return fmt.Errorf("rtorrent socket %s not accessible: %w", path, err)
	}
	_ = conn.Close()
	return nil
}

// EnsurePrivateDir creates dir with 0700 if missing; fails if existing dir is world-writable.
func EnsurePrivateDir(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	return CheckPathPermissions(path)
}
