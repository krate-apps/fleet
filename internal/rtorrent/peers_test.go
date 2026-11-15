package rtorrent

import "testing"

func TestPeerHost(t *testing.T) {
	cases := map[string]string{
		"203.0.113.10":        "203.0.113.10",
		"203.0.113.10:51413":  "203.0.113.10",
		"[2001:db8::1]:6881":  "2001:db8::1",
		"":                    "",
	}
	for in, want := range cases {
		if got := peerHost(in); got != want {
			t.Fatalf("peerHost(%q)=%q want %q", in, got, want)
		}
	}
}
