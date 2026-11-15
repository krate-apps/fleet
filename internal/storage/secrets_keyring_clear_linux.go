//go:build linux

package storage

import "golang.org/x/sys/unix"

// clearSecretKeyring removes the Fleet master-key entry (tests only).
func clearSecretKeyring() {
	id, err := unix.KeyctlSearch(unix.KEY_SPEC_USER_KEYRING, "user", secretKeyringDesc, 0)
	if err != nil {
		return
	}
	_, _ = unix.KeyctlInt(unix.KEYCTL_UNLINK, id, unix.KEY_SPEC_USER_KEYRING, 0, 0)
}
