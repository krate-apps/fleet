package rtorrent

import "testing"

func TestValidateTrackerURL(t *testing.T) {
	cases := []struct {
		in string
		ok bool
	}{
		{"https://tracker.example/announce", true},
		{"udp://tracker.example:6969", true},
		{"http://127.0.0.1:8080/a", true},
		{"ftp://nope", false},
		{"", false},
		{"not-a-url", false},
	}
	for _, tc := range cases {
		err := validateTrackerURL(tc.in)
		if tc.ok && err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("%q: expected error", tc.in)
		}
	}
}
