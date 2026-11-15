package backend

import (
	"encoding/json"
	"testing"
)

func TestDecodeHandshake(t *testing.T) {
	raw, _ := json.Marshal(HandshakeParams{
		ProtocolVersion:  1,
		ExtensionID:      "org.fleet.rssfeeds",
		ExtensionVersion: "1.0.0",
	})
	h, err := DecodeHandshake(raw)
	if err != nil {
		t.Fatalf("DecodeHandshake: %v", err)
	}
	if h.ExtensionID != "org.fleet.rssfeeds" {
		t.Fatalf("id = %q", h.ExtensionID)
	}
}

func TestDecodeHandshake_badVersion(t *testing.T) {
	raw, _ := json.Marshal(HandshakeParams{
		ProtocolVersion:  99,
		ExtensionID:      "org.fleet.rssfeeds",
		ExtensionVersion: "1.0.0",
	})
	if _, err := DecodeHandshake(raw); err == nil {
		t.Fatal("expected error")
	}
}
