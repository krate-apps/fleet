package sync

import (
	"testing"

	"github.com/krate-apps/fleet/internal/gateways"
	"github.com/krate-apps/fleet/internal/rtorrent"
)

func TestComposeClientsStatus(t *testing.T) {
	e := NewEngine(nil, nil, Options{})
	st := e.composeClientsStatus(EngineStatus{}, false, gatewayPollResult{
		Total: 2,
		OK:    1,
		Problems: []ClientProblem{{
			Name:    "Deluge",
			Kind:    "deluge",
			Message: "daemon not connected",
		}},
	})
	if st.ClientsTotal != 2 || st.ClientsConnected != 1 || !st.Connected || len(st.Problems) != 1 {
		t.Fatalf("%+v", st)
	}
}

func TestMergeTorrentListsOneRowPerClient(t *testing.T) {
	primary := []rtorrent.Torrent{{
		Hash: "ABC",
		Name: "Same",
	}}
	remote := []remoteBatch{{
		ID:   "gw1",
		Kind: gateways.KindDeluge,
		Name: "Deluge",
		Torrents: []gateways.ListedTorrent{{
			Hash: "abc",
			Name: "Same",
		}},
	}, {
		ID:   "gw2",
		Kind: gateways.KindQBittorrent,
		Name: "qBit",
		Torrents: []gateways.ListedTorrent{{
			Hash: "def",
			Name: "OnlyRemote",
		}},
	}}

	out := mergeTorrentLists(primary, remote)
	if len(out) != 3 {
		t.Fatalf("len=%d want 3", len(out))
	}
	if out[0].ClientKind != "rtorrent" || out[0].ID != "ABC@local-rtorrent" {
		t.Fatalf("primary=%+v", out[0])
	}
	if out[1].Hash != "ABC" || out[1].ClientKind != "deluge" {
		t.Fatalf("deluge=%+v", out[1])
	}
	if out[2].Hash != "DEF" || out[2].ClientKind != "qbittorrent" {
		t.Fatalf("qbit=%+v", out[2])
	}
}
