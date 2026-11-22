package main

import "testing"

func TestValidatePrivateBaseURL_ok(t *testing.T) {
	cases := []string{
		"http://127.0.0.1:3128",
		"http://localhost:3128/",
		"http://192.168.1.10:3128",
		"http://10.0.0.5/",
		"https://172.16.0.2:443",
	}
	for _, c := range cases {
		if _, err := validatePrivateBaseURL(c); err != nil {
			t.Fatalf("%s: %v", c, err)
		}
	}
}

func TestValidatePrivateBaseURL_reject(t *testing.T) {
	cases := []string{
		"http://8.8.8.8/",
		"http://169.254.169.254/latest",
		"http://metadata.google.internal/",
		"ftp://127.0.0.1/",
		"http://0.0.0.0:3128",
	}
	for _, c := range cases {
		if _, err := validatePrivateBaseURL(c); err == nil {
			t.Fatalf("expected reject for %s", c)
		}
	}
}
