package buckets

import "testing"

func TestRecognizeGitHubPackageURL(t *testing.T) {
	ok := "https://github.com/krate-apps/fleet/releases/download/v1.0.0/rss-linux-amd64.tar.gz"
	if err := RecognizeGitHubPackageURL(ok); err != nil {
		t.Fatal(err)
	}
	if !IsGitHubReleaseURL(ok) {
		t.Fatal("expected true")
	}
	bad := []string{
		"https://github.com/krate-apps/fleet/archive/refs/heads/main.tar.gz",
		"https://github.com/krate-apps/fleet.git",
		"http://github.com/krate-apps/fleet/releases/download/v1.0.0/x.tar.gz",
		"https://example.com/pkg.tar.gz",
	}
	for _, u := range bad {
		if err := RecognizeGitHubPackageURL(u); err == nil {
			t.Fatalf("expected error for %s", u)
		}
	}
	if DescribePackageURL(ok) != "github-release" {
		t.Fatalf("label=%s", DescribePackageURL(ok))
	}
}
