package geoip

import (
	"net"
	"testing"
)

func TestParseIP(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"1.2.3.4", "1.2.3.4"},
		{"1.2.3.4:6881", "1.2.3.4"},
		{"[2001:db8::1]:6881", "2001:db8::1"},
		{"2001:db8::1", "2001:db8::1"},
		{"", ""},
		{"not-an-ip", ""},
	}
	for _, tc := range cases {
		got := parseIP(tc.in)
		if tc.want == "" {
			if got != nil {
				t.Fatalf("%q: got %v", tc.in, got)
			}
			continue
		}
		want := net.ParseIP(tc.want)
		if got == nil || !got.Equal(want) {
			t.Fatalf("%q: got %v want %v", tc.in, got, want)
		}
	}
}

func TestNilDB(t *testing.T) {
	var d *DB
	if d.Enabled() || d.CountryISO("8.8.8.8") != "" || d.Path() != "" {
		t.Fatal("nil DB should be inert")
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(""); err != nil {
		t.Fatal(err)
	}
}
