package buckets

import "testing"

func TestParseBucketJSON(t *testing.T) {
	m, err := ParseBucketJSON([]byte(`{
  "schema": 1,
  "id": "org.fleet.official",
  "name": "Official",
  "official": true
}`))
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "org.fleet.official" || !m.Official {
		t.Fatalf("%+v", m)
	}
}

func TestParseBucketJSON_rejectsBadID(t *testing.T) {
	_, err := ParseBucketJSON([]byte(`{"schema":1,"id":"bad","name":"X"}`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseExtensionListingJSON(t *testing.T) {
	m, err := ParseExtensionListingJSON([]byte(`{
  "schema": 1,
  "kind": "extension",
  "id": "org.fleet.rssfeeds",
  "name": "RSS",
  "version": "1.0.0",
  "api": ">=1.0.0",
  "architecture": {
    "linux-amd64": {
      "url": "https://example.test/pkg.tar.gz",
      "hash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    }
  },
  "source": { "path": "packages/rss" }
}`))
	if err != nil {
		t.Fatal(err)
	}
	if m.Source == nil || m.Source.Path != "packages/rss" {
		t.Fatalf("%+v", m)
	}
	if m.PackageURL() == "" || m.PackageSHA256() != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("url=%q sha=%q", m.PackageURL(), m.PackageSHA256())
	}
}

func TestNormalizeHash(t *testing.T) {
	hex := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if got := NormalizeHash("sha256:" + hex); got != hex {
		t.Fatalf("got %q", got)
	}
	if got := NormalizeHash(hex); got != hex {
		t.Fatalf("got %q", got)
	}
}
