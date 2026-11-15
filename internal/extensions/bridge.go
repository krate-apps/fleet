package extensions

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// HostBridge handles reverse JSON-RPC calls from workers into Fleet public APIs.
type HostBridge interface {
	Handle(ctx context.Context, extensionID string, caps []string, method string, params json.RawMessage) (any, error)
}

// StorageBridge persists per-extension key/value JSON documents under the Fleet data dir.
type StorageBridge struct {
	DataDir string
}

func (s *StorageBridge) path(extensionID, key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" || strings.Contains(key, "..") || strings.ContainsAny(key, `/\`) {
		return "", fmt.Errorf("invalid storage key")
	}
	dir := DataDir(s.DataDir, extensionID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, key+".json"), nil
}

// Get reads a JSON document.
func (s *StorageBridge) Get(extensionID, key string) (json.RawMessage, error) {
	p, err := s.path(extensionID, key)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return json.RawMessage("null"), nil
		}
		return nil, err
	}
	return json.RawMessage(data), nil
}

// Put writes a JSON document.
func (s *StorageBridge) Put(extensionID, key string, value json.RawMessage) error {
	if len(value) == 0 {
		value = json.RawMessage("null")
	}
	if !json.Valid(value) {
		return fmt.Errorf("value must be valid JSON")
	}
	p, err := s.path(extensionID, key)
	if err != nil {
		return err
	}
	return os.WriteFile(p, value, 0o600)
}

// HasCap reports whether a capability is granted.
func HasCap(caps []string, want string) bool {
	for _, c := range caps {
		if c == want {
			return true
		}
	}
	return false
}
