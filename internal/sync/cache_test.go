package sync

import (
	"testing"

	"github.com/krate-apps/fleet/internal/rtorrent"
)

func TestCacheApplyAddUpdateRemove(t *testing.T) {
	c := newCache()
	a := rtorrent.Torrent{Hash: "A", Name: "one", Active: true, State: "downloading"}
	events := c.applyTorrents([]rtorrent.Torrent{a})
	if len(events) != 1 || events[0].Type != EventAdd {
		t.Fatalf("add events=%+v", events)
	}

	a2 := a
	a2.BytesDone = 10
	events = c.applyTorrents([]rtorrent.Torrent{a2})
	if len(events) != 1 || events[0].Type != EventUpdate {
		t.Fatalf("update events=%+v", events)
	}

	events = c.applyTorrents(nil)
	if len(events) != 1 || events[0].Type != EventRemove || events[0].Hash != "A" {
		t.Fatalf("remove events=%+v", events)
	}
}
