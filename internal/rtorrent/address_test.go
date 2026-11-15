package rtorrent

import "testing"

func TestParseAddress(t *testing.T) {
	cases := []struct {
		in      string
		network string
		addr    string
		ok      bool
	}{
		{"unix:///tmp/rt.sock", "unix", "/tmp/rt.sock", true},
		{"/run/krate/user/alice.rtorrent.sock", "unix", "/run/krate/user/alice.rtorrent.sock", true},
		{"scgi://127.0.0.1:5000", "tcp", "127.0.0.1:5000", true},
		{"", "", "", false},
		{"http://example", "", "", false},
	}
	for _, tc := range cases {
		ep, err := ParseAddress(tc.in)
		if tc.ok && err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("%q: expected error", tc.in)
		}
		if tc.ok {
			if ep.Network != tc.network || ep.Address != tc.addr {
				t.Fatalf("%q => %+v", tc.in, ep)
			}
		}
	}
}
