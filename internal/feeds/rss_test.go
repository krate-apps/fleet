package feeds

import (
	"strings"
	"testing"

	"github.com/krate-apps/fleet/internal/rtorrent"
)

func TestFilterModes(t *testing.T) {
	list := []rtorrent.Torrent{
		{Hash: "A", Name: "down", Complete: false, DownRate: 2048},
		{Hash: "B", Name: "seed", Complete: true, UpRate: 2048},
		{Hash: "C", Name: "idle", Complete: true, UpRate: 0, DownRate: 0},
		{Hash: "D", Name: "err", State: "error", Complete: false},
	}
	if n := len(Filter(list, "downloading")); n != 2 { // down + err
		t.Fatalf("downloading=%d", n)
	}
	if n := len(Filter(list, "completed")); n != 2 {
		t.Fatalf("completed=%d", n)
	}
	if n := len(Filter(list, "active")); n != 2 {
		t.Fatalf("active=%d", n)
	}
	if n := len(Filter(list, "inactive")); n != 2 {
		t.Fatalf("inactive=%d", n)
	}
	if n := len(Filter(list, "error")); n != 1 {
		t.Fatalf("error=%d", n)
	}
}

func TestBuildRSS(t *testing.T) {
	list := []rtorrent.Torrent{{Hash: "ABC", Name: "Demo <Torrent>", Label: "tv", FinishedAt: 1700000000, SizeBytes: 1024}}
	raw, err := BuildRSS(list, BuildOptions{
		Mode:        "all",
		SelfLink:    "http://127.0.0.1/api/v1/feeds/all.xml",
		ServerName:  "lab",
		IncludeDesc: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, want := range []string{
		`<?xml version="1.0"`,
		`<title>lab: All torrents</title>`,
		`<guid isPermaLink="false">ABC</guid>`,
		`Demo &lt;Torrent&gt;`,
		`<category>tv</category>`,
		`<![CDATA[`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in %s", want, s)
		}
	}
	if _, err := BuildRSS(nil, BuildOptions{Mode: "nope"}); err == nil {
		t.Fatal("expected bad mode")
	}
}
