package storage

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/krate-apps/fleet/internal/gateways"
)

func TestEncryptDecryptSecretRoundTrip(t *testing.T) {
	t.Setenv("CREDENTIALS_DIRECTORY", "")
	clearSecretKeyring()
	key, err := loadOrCreateSecretKey(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	enc, err := encryptSecret(key, "s3cret")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(enc, secretEncPrefix) {
		t.Fatalf("want %s prefix, got %q", secretEncPrefix, enc)
	}
	plain, err := decryptSecret(key, enc)
	if err != nil || plain != "s3cret" {
		t.Fatalf("plain=%q err=%v", plain, err)
	}
}

func TestDecryptSecretLegacyPlaintext(t *testing.T) {
	t.Setenv("CREDENTIALS_DIRECTORY", "")
	clearSecretKeyring()
	key, err := loadOrCreateSecretKey(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	plain, err := decryptSecret(key, "legacy-pass")
	if err != nil || plain != "legacy-pass" {
		t.Fatalf("plain=%q err=%v", plain, err)
	}
}

func TestLoadOrCreateSecretKeyPersists(t *testing.T) {
	t.Setenv("CREDENTIALS_DIRECTORY", "")
	clearSecretKeyring()
	dir := t.TempDir()
	k1, err := loadOrCreateSecretKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	k2, err := loadOrCreateSecretKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	if string(k1) != string(k2) {
		t.Fatal("key should be stable across loads")
	}
	info, err := os.Stat(filepath.Join(dir, secretKeyFile))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("key mode=%o want 0600", info.Mode().Perm())
	}
}

func TestLoadSecretKeyFromCredentialsDirectory(t *testing.T) {
	clearSecretKeyring()
	credDir := t.TempDir()
	dataDir := t.TempDir()
	key := make([]byte, secretKeyBytes)
	for i := range key {
		key[i] = byte(i + 1)
	}
	encoded := base64.StdEncoding.EncodeToString(key) + "\n"
	if err := os.WriteFile(filepath.Join(credDir, secretCredentialName), []byte(encoded), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CREDENTIALS_DIRECTORY", credDir)

	got, err := loadOrCreateSecretKey(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(key) {
		t.Fatalf("credential key not used")
	}
	// Must not fall through to creating a data_dir key when credentials win.
	if _, err := os.Stat(filepath.Join(dataDir, secretKeyFile)); !os.IsNotExist(err) {
		t.Fatalf("unexpected data_dir key file, err=%v", err)
	}
}

func TestGatewayPasswordEncryptedAtRest(t *testing.T) {
	t.Setenv("CREDENTIALS_DIRECTORY", "")
	clearSecretKeyring()
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	_, err = store.UpsertGatewayClient(GatewayClient{
		ID: "gw1", Kind: gateways.KindQBittorrent, Name: "Q", URL: "http://q", Username: "u", Password: "p@ss", Enabled: true,
	}, false)
	if err != nil {
		t.Fatal(err)
	}

	var stored string
	if err := store.db.QueryRow(`SELECT password FROM gateway_clients WHERE id = ?`, "gw1").Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stored, secretEncPrefix) {
		t.Fatalf("stored password not encrypted: %q", stored)
	}
	if stored == "p@ss" {
		t.Fatal("plaintext leaked into sqlite")
	}

	got, err := store.GetGatewayClient("gw1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Password != "p@ss" {
		t.Fatalf("decrypted=%q", got.Password)
	}
}

func TestGatewayPasswordLegacyMigrationOnSave(t *testing.T) {
	t.Setenv("CREDENTIALS_DIRECTORY", "")
	clearSecretKeyring()
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	_, err = store.db.Exec(`
INSERT INTO gateway_clients(id, kind, name, url, username, password, enabled, is_default, journal_unit, log_path, created_at, updated_at)
VALUES('gw_legacy', 'deluge', 'L', 'http://l', 'u', 'plain-old', 1, 0, '', '', 't', 't')`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.GetGatewayClient("gw_legacy")
	if err != nil || got.Password != "plain-old" {
		t.Fatalf("legacy read: %+v err=%v", got, err)
	}
	_, err = store.UpsertGatewayClient(GatewayClient{
		ID: "gw_legacy", Kind: gateways.KindDeluge, Name: "L", URL: "http://l", Username: "u", Enabled: true,
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	var stored string
	if err := store.db.QueryRow(`SELECT password FROM gateway_clients WHERE id = ?`, "gw_legacy").Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stored, secretEncPrefix) {
		t.Fatalf("expected migration to enc:v1, got %q", stored)
	}
	got, err = store.GetGatewayClient("gw_legacy")
	if err != nil || got.Password != "plain-old" {
		t.Fatalf("after migrate: %+v err=%v", got, err)
	}
}
