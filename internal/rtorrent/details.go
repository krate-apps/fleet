package rtorrent

import (
	"context"
	"fmt"
	"net"
	"strings"
)

// FileEntry is one file inside a torrent.
type FileEntry struct {
	Index           int64  `json:"index"`
	Path            string `json:"path"`
	SizeBytes       int64  `json:"sizeBytes"`
	Priority        int64  `json:"priority"`
	CompletedChunks int64  `json:"completedChunks"`
	RangeFirst      int64  `json:"rangeFirst"`
	RangeLast       int64  `json:"rangeLast"`
}

// TrackerEntry describes a tracker row.
type TrackerEntry struct {
	Index             int64  `json:"index"`
	ID                string `json:"id"`
	URL               string `json:"url"`
	Enabled           bool   `json:"enabled"`
	Open              bool   `json:"open"`
	Seeders           int64  `json:"seeders"`
	Leechers          int64  `json:"leechers"`
	ScrapedComplete   int64  `json:"scrapedComplete"`
	ScrapedIncomplete int64  `json:"scrapedIncomplete"`
	LastScrapeTime    int64  `json:"lastScrapeTime"`
}

// PeerEntry describes a connected peer.
type PeerEntry struct {
	ID               string  `json:"id,omitempty"`
	Address          string  `json:"address"`
	ClientVersion    string  `json:"clientVersion"`
	DownRate         int64   `json:"downRate"`
	UpRate           int64   `json:"upRate"`
	CompletedPercent float64 `json:"completedPercent"`
	Encrypted        bool    `json:"encrypted"`
	Incoming         bool    `json:"incoming"`
	// Country is an optional ISO 3166-1 alpha-2 code from GeoIP enrichment.
	Country string `json:"country,omitempty"`
	// Comment is an optional user annotation from Fleet SQLite (geoip comments parity).
	Comment string `json:"comment,omitempty"`
}

// ListFiles returns the file tree rows for a torrent hash.
func (c *Client) ListFiles(ctx context.Context, hash string) ([]FileEntry, error) {
	fields := []string{
		"f.path=",
		"f.size_bytes=",
		"f.priority=",
		"f.completed_chunks=",
		"f.range_first=",
		"f.range_second=",
	}
	params := []any{hash, ""}
	for _, f := range fields {
		params = append(params, f)
	}
	v, err := c.Call(ctx, "f.multicall", params...)
	if err != nil {
		return nil, err
	}
	if v.Kind != "array" {
		return nil, fmt.Errorf("unexpected f.multicall kind %q", v.Kind)
	}
	out := make([]FileEntry, 0, len(v.Array))
	for i, row := range v.Array {
		if row.Kind != "array" {
			continue
		}
		get := func(i int) Value {
			if i < len(row.Array) {
				return row.Array[i]
			}
			return Value{Kind: "nil"}
		}
		asInt := func(i int) int64 { n, _ := get(i).AsInt(); return n }
		out = append(out, FileEntry{
			Index:           int64(i),
			Path:            get(0).AsString(),
			SizeBytes:       asInt(1),
			Priority:        asInt(2),
			CompletedChunks: asInt(3),
			RangeFirst:      asInt(4),
			RangeLast:       asInt(5),
		})
	}
	return out, nil
}

// ListTrackers returns trackers for a torrent hash.
func (c *Client) ListTrackers(ctx context.Context, hash string) ([]TrackerEntry, error) {
	fields := []string{
		"t.url=",
		"t.is_enabled=",
		"t.is_open=",
		"t.scrape_complete=",
		"t.scrape_incomplete=",
		"t.scrape_time_last=",
	}
	params := []any{hash, ""}
	for _, f := range fields {
		params = append(params, f)
	}
	v, err := c.Call(ctx, "t.multicall", params...)
	if err != nil {
		return nil, err
	}
	if v.Kind != "array" {
		return nil, fmt.Errorf("unexpected t.multicall kind %q", v.Kind)
	}
	out := make([]TrackerEntry, 0, len(v.Array))
	for i, row := range v.Array {
		if row.Kind != "array" {
			continue
		}
		get := func(i int) Value {
			if i < len(row.Array) {
				return row.Array[i]
			}
			return Value{Kind: "nil"}
		}
		asInt := func(i int) int64 { n, _ := get(i).AsInt(); return n }
		asBool := func(i int) bool { return asInt(i) != 0 }
		complete := asInt(3)
		incomplete := asInt(4)
		idx := int64(i)
		out = append(out, TrackerEntry{
			Index:             idx,
			ID:                fmt.Sprintf("%s:t%d", hash, idx),
			URL:               get(0).AsString(),
			Enabled:           asBool(1),
			Open:              asBool(2),
			Seeders:           complete,
			Leechers:          incomplete,
			ScrapedComplete:   complete,
			ScrapedIncomplete: incomplete,
			LastScrapeTime:    asInt(5),
		})
	}
	return out, nil
}

// ListPeers returns connected peers for a torrent hash.
func (c *Client) ListPeers(ctx context.Context, hash string) ([]PeerEntry, error) {
	fields := []string{
		"p.id=",
		"p.address=",
		"p.client_version=",
		"p.down_rate=",
		"p.up_rate=",
		"p.completed_percent=",
		"p.is_encrypted=",
		"p.is_incoming=",
	}
	params := []any{hash, ""}
	for _, f := range fields {
		params = append(params, f)
	}
	v, err := c.Call(ctx, "p.multicall", params...)
	if err != nil {
		return nil, err
	}
	if v.Kind != "array" {
		return nil, fmt.Errorf("unexpected p.multicall kind %q", v.Kind)
	}
	out := make([]PeerEntry, 0, len(v.Array))
	for _, row := range v.Array {
		if row.Kind != "array" {
			continue
		}
		get := func(i int) Value {
			if i < len(row.Array) {
				return row.Array[i]
			}
			return Value{Kind: "nil"}
		}
		asInt := func(i int) int64 { n, _ := get(i).AsInt(); return n }
		asBool := func(i int) bool { return asInt(i) != 0 }
		out = append(out, PeerEntry{
			ID:               get(0).AsString(),
			Address:          get(1).AsString(),
			ClientVersion:    get(2).AsString(),
			DownRate:         asInt(3),
			UpRate:           asInt(4),
			CompletedPercent: float64(asInt(5)),
			Encrypted:        asBool(6),
			Incoming:         asBool(7),
		})
	}
	return out, nil
}

// DisconnectPeer disconnects a peer matched by address (or peer id) on a torrent.
func (c *Client) DisconnectPeer(ctx context.Context, hash, address string) error {
	hash = strings.ToUpper(strings.TrimSpace(hash))
	want := strings.TrimSpace(address)
	if hash == "" || want == "" {
		return fmt.Errorf("hash and address required")
	}
	peers, err := c.ListPeers(ctx, hash)
	if err != nil {
		return err
	}
	wantHost := peerHost(want)
	var peerID string
	for _, p := range peers {
		if p.Address == want || peerHost(p.Address) == wantHost || p.ID == want {
			peerID = strings.TrimSpace(p.ID)
			if peerID == "" {
				peerID = p.Address
			}
			break
		}
	}
	if peerID == "" {
		return fmt.Errorf("peer %q not found on %s", want, hash)
	}
	_, err = c.Call(ctx, "p.disconnect", peerID)
	return err
}

func peerHost(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	// IPv6 without brackets, or bare IP / hostname.
	if i := strings.LastIndex(addr, ":"); i > 0 && strings.Count(addr, ":") == 1 {
		return addr[:i]
	}
	return addr
}
