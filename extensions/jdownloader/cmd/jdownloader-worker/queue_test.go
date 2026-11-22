package main

import (
	"encoding/json"
	"testing"
)

func TestNormalizeMyJDPackages_success(t *testing.T) {
	raw := json.RawMessage(`[
		{"uuid": 42, "name": "Show.S01", "status": "Downloading", "bytesLoaded": 100, "bytesTotal": 400, "speed": 50, "eta": 6, "enabled": true, "running": true},
		{"uuid": "99", "name": "Done", "finished": true, "bytesLoaded": 10, "bytesTotal": 10, "enabled": true}
	]`)
	items, err := normalizeMyJDPackages(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items", len(items))
	}
	if items[0].ID != "42" || items[0].Name != "Show.S01" || items[0].Speed != 50 || items[0].EtaSec != 6 {
		t.Fatalf("item0 %+v", items[0])
	}
	if items[1].Status != "finished" || items[1].Kind != "package" {
		t.Fatalf("item1 %+v", items[1])
	}
}

func TestNormalizeMyJDPackages_empty(t *testing.T) {
	items, err := normalizeMyJDPackages(json.RawMessage("null"))
	if err != nil || len(items) != 0 {
		t.Fatalf("empty: %v %#v", err, items)
	}
}

func TestNormalizeMyJDPackages_invalid(t *testing.T) {
	_, err := normalizeMyJDPackages(json.RawMessage(`{"no":"list"}`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseIDList(t *testing.T) {
	got := parseIDList([]any{"123", 45.0})
	if len(got) != 2 {
		t.Fatalf("%#v", got)
	}
	if got[0].(int64) != 123 || got[1].(int64) != 45 {
		t.Fatalf("%#v", got)
	}
	if len(parseIDList("")) != 0 {
		t.Fatal("empty string")
	}
}

func TestPKCS7RoundTrip(t *testing.T) {
	secret := make([]byte, 32)
	for i := range secret {
		secret[i] = byte(i + 1)
	}
	enc, err := myjdEncrypt(secret, []byte(`{"rid":1}`))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := myjdDecrypt(secret, enc)
	if err != nil {
		t.Fatal(err)
	}
	if string(plain) != `{"rid":1}` {
		t.Fatalf("got %q", plain)
	}
}

func TestMyjdSecretStable(t *testing.T) {
	a := myjdSecret("User@Example.com", "secret", "server")
	b := myjdSecret("user@example.com", "secret", "server")
	if len(a) != 32 || string(a) != string(b) {
		t.Fatal("email must be lowercased for secret")
	}
}
