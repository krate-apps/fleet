//go:build !linux

package storage

func loadKeyFromKeyring() ([]byte, bool, error) {
	return nil, false, nil
}

func storeKeyInKeyring(key []byte) error {
	return nil
}
