package rtorrent

import (
	"context"
	"fmt"
)

// Torrent is a snapshot row synced from rTorrent (not stored as authority in SQLite).
type Torrent struct {
	Hash           string  `json:"hash"`
	Name           string  `json:"name"`
	SizeBytes      int64   `json:"sizeBytes"`
	BytesDone      int64   `json:"bytesDone"`
	LeftBytes      int64   `json:"leftBytes"`
	Complete       bool    `json:"complete"`
	Active         bool    `json:"active"`
	Open           bool    `json:"open"`
	Hashing        bool    `json:"hashing"`
	DownRate       int64   `json:"downRate"`
	UpRate         int64   `json:"upRate"`
	DownTotal      int64   `json:"downTotal"`
	UpTotal        int64   `json:"upTotal"`
	Ratio          float64 `json:"ratio"`
	Priority       int64   `json:"priority"`
	Directory      string  `json:"directory"`
	Label          string  `json:"label,omitempty"`
	State          string  `json:"state"`
	Progress       float64 `json:"progress"`
	ETASeconds       int64   `json:"etaSeconds"`
	UploadETASeconds int64   `json:"uploadEtaSeconds,omitempty"`
	StartedAt        int64   `json:"startedAt,omitempty"`
	FinishedAt       int64   `json:"finishedAt,omitempty"`
	SeedingSeconds   int64   `json:"seedingSeconds,omitempty"`
	ThrottleName   string  `json:"throttleName,omitempty"`
	Private        bool    `json:"private,omitempty"`
	// SeedsComplete is peers known complete (all-seeders style column).
	SeedsComplete int64 `json:"seedsComplete,omitempty"`
	PeersConnected int64 `json:"peersConnected,omitempty"`
	// ID is the list row key (hash@clientId). Empty means hash-only (legacy).
	ID string `json:"id,omitempty"`
	// ClientID / ClientKind / ClientName identify which BitTorrent client holds this row.
	ClientID   string `json:"clientId,omitempty"`
	ClientKind string `json:"clientKind,omitempty"`
	ClientName string `json:"clientName,omitempty"`
}

var torrentFields = []string{
	"d.hash=",
	"d.name=",
	"d.size_bytes=",
	"d.bytes_done=",
	"d.left_bytes=",
	"d.complete=",
	"d.is_active=",
	"d.is_open=",
	"d.is_hash_checking=",
	"d.down.rate=",
	"d.up.rate=",
	"d.down.total=",
	"d.up.total=",
	"d.ratio=",
	"d.priority=",
	"d.directory=",
	"d.custom1=",
	"d.timestamp.started=",
	"d.timestamp.finished=",
	"d.throttle_name=",
	"d.is_private=",
	"d.peers_complete=",
	"d.peers_connected=",
}

// ListTorrents returns all torrents in the main view via d.multicall2 when available.
func (c *Client) ListTorrents(ctx context.Context) ([]Torrent, error) {
	params := make([]any, 0, 2+len(torrentFields))
	params = append(params, "", "main")
	for _, f := range torrentFields {
		params = append(params, f)
	}

	v, err := c.Call(ctx, "d.multicall2", params...)
	if err != nil {
		v, err = c.Call(ctx, "d.multicall", append([]any{"main"}, castStrings(torrentFields)...)...)
		if err != nil {
			return nil, err
		}
	}
	if v.Kind != "array" {
		return nil, fmt.Errorf("unexpected multicall result kind %q", v.Kind)
	}

	out := make([]Torrent, 0, len(v.Array))
	for _, row := range v.Array {
		t, err := parseTorrentRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

func castStrings(in []string) []any {
	out := make([]any, len(in))
	for i, s := range in {
		out[i] = s
	}
	return out
}

func parseTorrentRow(row Value) (Torrent, error) {
	if row.Kind != "array" {
		return Torrent{}, fmt.Errorf("torrent row is %q, want array", row.Kind)
	}
	cols := row.Array
	get := func(i int) Value {
		if i < len(cols) {
			return cols[i]
		}
		return Value{Kind: "nil"}
	}
	asInt := func(i int) int64 {
		n, _ := get(i).AsInt()
		return n
	}
	asBool := func(i int) bool {
		n, _ := get(i).AsInt()
		return n != 0
	}

	size := asInt(2)
	done := asInt(3)
	left := asInt(4)
	downRate := asInt(9)
	ratioRaw := asInt(13)
	started := asInt(17)
	finished := asInt(18)

	t := Torrent{
		Hash:         get(0).AsString(),
		Name:         get(1).AsString(),
		SizeBytes:    size,
		BytesDone:    done,
		LeftBytes:    left,
		Complete:     asBool(5),
		Active:       asBool(6),
		Open:         asBool(7),
		Hashing:      asBool(8),
		DownRate:     downRate,
		UpRate:       asInt(10),
		DownTotal:    asInt(11),
		UpTotal:      asInt(12),
		Ratio:        float64(ratioRaw) / 1000.0,
		Priority:     asInt(14),
		Directory:    get(15).AsString(),
		Label:        get(16).AsString(),
		StartedAt:    started,
		FinishedAt:   finished,
		ThrottleName:   get(19).AsString(),
		Private:        asBool(20),
		SeedsComplete:  asInt(21),
		PeersConnected: asInt(22),
	}
	if size > 0 {
		t.Progress = float64(done) / float64(size)
	} else if t.Complete {
		t.Progress = 1
	}
	if !t.Complete && downRate > 0 && left > 0 {
		t.ETASeconds = left / downRate
	}
	// uploadeta: time to upload one full copy (size) at current up rate when still behind.
	if t.UpRate > 0 && size > 0 && t.UpTotal < size {
		t.UploadETASeconds = (size - t.UpTotal) / t.UpRate
	}
	t.State = deriveState(t)
	return t, nil
}

func deriveState(t Torrent) string {
	switch {
	case t.Hashing:
		return "hashing"
	case !t.Open:
		return "closed"
	case t.Complete && t.Active:
		return "seeding"
	case t.Complete && !t.Active:
		return "stopped"
	case !t.Complete && t.Active:
		return "downloading"
	case !t.Complete && !t.Active:
		return "paused"
	default:
		return "unknown"
	}
}
