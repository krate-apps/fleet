package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2Params are versioned password hashing parameters (SEC-003).
type Argon2Params struct {
	Memory      uint32 `json:"memory"`
	Iterations  uint32 `json:"iterations"`
	Parallelism uint8  `json:"parallelism"`
	SaltLength  uint32 `json:"saltLength"`
	KeyLength   uint32 `json:"keyLength"`
}

// DefaultArgon2Params returns the Fleet V1 defaults.
func DefaultArgon2Params() Argon2Params {
	return Argon2Params{
		Memory:      64 * 1024,
		Iterations:  3,
		Parallelism: 2,
		SaltLength:  16,
		KeyLength:   32,
	}
}

// HashPassword returns an encoded Argon2id hash.
func HashPassword(password string, p Argon2Params) (string, error) {
	if strings.TrimSpace(password) == "" {
		return "", fmt.Errorf("password required")
	}
	salt := make([]byte, p.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, p.Iterations, p.Memory, p.Parallelism, p.KeyLength)
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.Memory, p.Iterations, p.Parallelism, b64Salt, b64Hash), nil
}

// VerifyPassword checks a password against an encoded Argon2id hash.
func VerifyPassword(encoded, password string) (bool, error) {
	p, salt, hash, err := decodeHash(encoded)
	if err != nil {
		return false, err
	}
	other := argon2.IDKey([]byte(password), salt, p.Iterations, p.Memory, p.Parallelism, p.KeyLength)
	if subtle.ConstantTimeCompare(hash, other) == 1 {
		return true, nil
	}
	return false, nil
}

func decodeHash(encoded string) (Argon2Params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return Argon2Params{}, nil, nil, fmt.Errorf("unsupported hash format")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return Argon2Params{}, nil, nil, err
	}
	if version != argon2.Version {
		return Argon2Params{}, nil, nil, fmt.Errorf("unsupported argon2 version")
	}
	var p Argon2Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.Memory, &p.Iterations, &p.Parallelism); err != nil {
		return Argon2Params{}, nil, nil, err
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return Argon2Params{}, nil, nil, err
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return Argon2Params{}, nil, nil, err
	}
	p.SaltLength = uint32(len(salt))
	p.KeyLength = uint32(len(hash))
	return p, salt, hash, nil
}
