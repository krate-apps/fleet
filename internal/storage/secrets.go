package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	secretKeyFile         = ".fleet-secrets.key"
	secretCredentialName  = "fleet-secrets.key"
	secretKeyringDesc     = "fleet-secrets"
	secretEncPrefix       = "enc:v1:"
	secretKeyBytes        = 32
)

// loadOrCreateSecretKey resolves the AES master key in order:
// 1. systemd CREDENTIALS_DIRECTORY / fleet-secrets.key
// 2. Linux user keyring description "fleet-secrets"
// 3. dataDir/.fleet-secrets.key (create 0600 if missing)
func loadOrCreateSecretKey(dataDir string) ([]byte, error) {
	if key, ok, err := loadKeyFromCredentialsDir(); err != nil {
		return nil, err
	} else if ok {
		return key, nil
	}
	if key, ok, err := loadKeyFromKeyring(); err != nil {
		return nil, err
	} else if ok {
		return key, nil
	}
	return loadOrCreateSecretKeyFile(dataDir)
}

func loadKeyFromCredentialsDir() ([]byte, bool, error) {
	dir := strings.TrimSpace(os.Getenv("CREDENTIALS_DIRECTORY"))
	if dir == "" {
		return nil, false, nil
	}
	path := filepath.Join(dir, secretCredentialName)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read credential %s: %w", secretCredentialName, err)
	}
	key, err := decodeSecretKeyMaterial(raw)
	if err != nil {
		return nil, false, fmt.Errorf("credential %s: %w", secretCredentialName, err)
	}
	return key, true, nil
}

func loadOrCreateSecretKeyFile(dataDir string) ([]byte, error) {
	if strings.TrimSpace(dataDir) == "" {
		return nil, fmt.Errorf("data dir required for secret key")
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	path := filepath.Join(dataDir, secretKeyFile)
	raw, err := os.ReadFile(path)
	if err == nil {
		key, err := decodeSecretKeyMaterial(raw)
		if err != nil {
			return nil, err
		}
		return key, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read secret key: %w", err)
	}
	key := make([]byte, secretKeyBytes)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("generate secret key: %w", err)
	}
	encoded := base64.StdEncoding.EncodeToString(key) + "\n"
	if err := os.WriteFile(path, []byte(encoded), 0o600); err != nil {
		return nil, fmt.Errorf("write secret key: %w", err)
	}
	// Best-effort: also publish into the user keyring when available.
	_ = storeKeyInKeyring(key)
	return key, nil
}

func decodeSecretKeyMaterial(raw []byte) ([]byte, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return nil, fmt.Errorf("empty secret key material")
	}
	key, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil {
		return nil, fmt.Errorf("decode secret key: %w", err)
	}
	if len(key) != secretKeyBytes {
		return nil, fmt.Errorf("secret key must be %d bytes, got %d", secretKeyBytes, len(key))
	}
	return key, nil
}

func encryptSecret(key []byte, plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if strings.HasPrefix(plaintext, secretEncPrefix) {
		return plaintext, nil
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	out := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return secretEncPrefix + base64.StdEncoding.EncodeToString(out), nil
}

func decryptSecret(key []byte, stored string) (string, error) {
	if stored == "" {
		return "", nil
	}
	if !strings.HasPrefix(stored, secretEncPrefix) {
		// Legacy plaintext — still usable until next save migrates it.
		return stored, nil
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(stored, secretEncPrefix))
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt secret: %w", err)
	}
	return string(plain), nil
}
