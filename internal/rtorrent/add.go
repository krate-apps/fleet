package rtorrent

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
)

// AddMagnet loads one or more magnet links. Duplicates surface as RPC errors.
func (c *Client) AddMagnet(ctx context.Context, magnets []string, start bool) error {
	method := "load.start"
	if !start {
		method = "load.normal"
	}
	for _, m := range magnets {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		if !strings.HasPrefix(strings.ToLower(m), "magnet:") {
			return fmt.Errorf("not a magnet URI: %q", truncate(m, 64))
		}
		if _, err := c.Call(ctx, method, "", m); err != nil {
			return fmt.Errorf("load magnet: %w", err)
		}
	}
	return nil
}

// AddTorrentFile loads a raw .torrent payload (bytes) via load.raw_start / load.raw.
func (c *Client) AddTorrentFile(ctx context.Context, data []byte, start bool) error {
	if len(data) == 0 {
		return fmt.Errorf("empty torrent file")
	}
	method := "load.raw_start"
	if !start {
		method = "load.raw"
	}
	// xmlrpc-c / rTorrent expect binary as base64 in <base64> — our encoder uses string;
	// pass as base64 string and use load.raw_start with binary via i8? Actually rTorrent
	// load.raw* takes binary data. We extend writeValue for []byte as base64.
	if _, err := c.Call(ctx, method, "", data); err != nil {
		return fmt.Errorf("load torrent: %w", err)
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// EncodeBase64 is exported for tests around torrent payloads.
func EncodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
