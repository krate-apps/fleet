package netutil

import (
	"net"
	"testing"
)

func TestBlockedIP(t *testing.T) {
	cases := []struct {
		ip      string
		blocked bool
	}{
		{"127.0.0.1", true},
		{"10.0.0.1", true},
		{"192.168.1.1", true},
		{"169.254.169.254", true},
		{"100.64.1.1", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
	}
	for _, tc := range cases {
		if got := BlockedIP(net.ParseIP(tc.ip)); got != tc.blocked {
			t.Fatalf("%s: got %v want %v", tc.ip, got, tc.blocked)
		}
	}
}

func TestValidatePublicURL(t *testing.T) {
	if _, err := ValidatePublicURL("https://example.com/a"); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidatePublicURL("http://127.0.0.1/x"); err == nil {
		t.Fatal("loopback literal should fail")
	}
	if _, err := ValidatePublicURL("http://localhost/x"); err == nil {
		t.Fatal("localhost should fail")
	}
	if _, err := ValidatePublicURL("file:///etc/passwd"); err == nil {
		t.Fatal("file scheme should fail")
	}
}
