package gateways

import (
	"context"
	"testing"
)

func TestCapabilitiesFor(t *testing.T) {
	q := CapabilitiesFor(KindQBittorrent)
	if !q.List || !q.StartStop || !q.Files || !q.Label || q.Chunks {
		t.Fatalf("qbittorrent caps: %+v", q)
	}
	d := CapabilitiesFor(KindDeluge)
	if !d.StartStop || !d.Label || d.Peers || d.Chunks {
		t.Fatalf("deluge caps: %+v", d)
	}
	tr := CapabilitiesFor(KindTransmission)
	if !tr.Label || !tr.Reannounce || !tr.Peers {
		t.Fatalf("transmission caps: %+v", tr)
	}
	if !SupportsFeature(KindQBittorrent, "reannounce") {
		t.Fatal("qbit reannounce")
	}
	if !SupportsFeature(KindTransmission, "label") {
		t.Fatal("transmission should support label")
	}
	if SupportsFeature(KindDeluge, "peers") {
		t.Fatal("deluge peers unsupported")
	}
}

func TestApplyTorrentActionRejectsEmpty(t *testing.T) {
	res := ApplyTorrentAction(context.Background(), ControlInput{Kind: KindQBittorrent, URL: "http://x", Action: "start"})
	if res.OK || res.Message == "" {
		t.Fatalf("%+v", res)
	}
}
