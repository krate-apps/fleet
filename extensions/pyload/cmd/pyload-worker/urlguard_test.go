package main

import "testing"

func TestValidatePrivateBaseURL_ok(t *testing.T) {
	cases := []string{
		"http://127.0.0.1:8000",
		"http://localhost:8000/pyload",
		"http://192.168.1.10:8000",
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
		"http://0.0.0.0:8000",
	}
	for _, c := range cases {
		if _, err := validatePrivateBaseURL(c); err == nil {
			t.Fatalf("expected reject for %s", c)
		}
	}
}

func TestNormalizePyStatus_list(t *testing.T) {
	raw := []byte(`[{"id":1,"name":"a.zip","statusmsg":"downloading","size":1000,"bleft":400,"speed":50,"eta":12}]`)
	items, err := normalizePyStatus(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Name != "a.zip" || items[0].BytesDone != 600 || items[0].Speed != 50 {
		t.Fatalf("%+v", items)
	}
}

func TestNormalizePyStatus_map(t *testing.T) {
	raw := []byte(`{"9":{"name":"b.bin","status":"queued","size":10,"bleft":10,"speed":0}}`)
	items, err := normalizePyStatus(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != "9" {
		t.Fatalf("%+v", items)
	}
}

func TestNormalizePyStatus_invalid(t *testing.T) {
	_, err := normalizePyStatus([]byte(`"nope"`))
	if err == nil {
		t.Fatal("expected error")
	}
}
