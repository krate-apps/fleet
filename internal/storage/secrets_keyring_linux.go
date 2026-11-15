//go:build linux

package storage

import (
	"encoding/base64"
	"fmt"

	"golang.org/x/sys/unix"
)

func loadKeyFromKeyring() ([]byte, bool, error) {
	id, err := unix.KeyctlSearch(unix.KEY_SPEC_USER_KEYRING, "user", secretKeyringDesc, 0)
	if err != nil {
		// ENOKEY / EKEYREVOKED / permission — treat as miss (fall through to file).
		return nil, false, nil
	}
	var buf []byte
	for {
		n, err := unix.KeyctlBuffer(unix.KEYCTL_READ, id, buf, 0)
		if err != nil {
			return nil, false, nil
		}
		if n <= len(buf) {
			key, err := decodeSecretKeyMaterial(buf[:n])
			if err != nil {
				return nil, false, nil
			}
			return key, true, nil
		}
		buf = make([]byte, n)
	}
}

func storeKeyInKeyring(key []byte) error {
	if len(key) != secretKeyBytes {
		return fmt.Errorf("keyring payload must be %d bytes", secretKeyBytes)
	}
	// Store base64 so material matches the credential/file format.
	payload := []byte(base64.StdEncoding.EncodeToString(key))
	_, err := unix.AddKey("user", secretKeyringDesc, payload, unix.KEY_SPEC_USER_KEYRING)
	return err
}
