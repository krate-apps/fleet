package gateways

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/krate-apps/fleet/internal/rtorrent"
)

// RatesInput sets per-torrent upload/download limits (bytes/sec; 0 = unlimited).
type RatesInput struct {
	Kind     Kind
	URL      string
	Username string
	Password string
	Hash     string
	UpBps    int64
	DownBps  int64
}

// SetTorrentRates applies bandwidth caps on a remote client.
func SetTorrentRates(ctx context.Context, in RatesInput) rtorrent.ActionResult {
	in.Kind = Kind(normalizeKind(string(in.Kind)))
	in.URL = strings.TrimRight(strings.TrimSpace(in.URL), "/")
	in.Hash = strings.ToUpper(strings.TrimSpace(in.Hash))
	if in.UpBps < 0 {
		in.UpBps = 0
	}
	if in.DownBps < 0 {
		in.DownBps = 0
	}
	if in.URL == "" {
		return rtorrent.ActionResult{Hash: in.Hash, OK: false, Message: "url required"}
	}
	if in.Hash == "" {
		return rtorrent.ActionResult{OK: false, Message: "hash required"}
	}
	if !SupportsFeature(in.Kind, "torrentRates") {
		return rtorrent.ActionResult{Hash: in.Hash, OK: false, Message: "unsupported on this client"}
	}
	switch in.Kind {
	case KindQBittorrent:
		return setQBittorrentRates(ctx, in)
	case KindDeluge:
		return setDelugeRates(ctx, in)
	case KindTransmission:
		return setTransmissionRates(ctx, in)
	case KindRTorrent:
		return rtorrent.ActionResult{Hash: in.Hash, OK: false, Message: "use local rtorrent setRates"}
	default:
		return rtorrent.ActionResult{Hash: in.Hash, OK: false, Message: "unsupported kind"}
	}
}

func setQBittorrentRates(ctx context.Context, in RatesInput) rtorrent.ActionResult {
	client := addHTTPClient()
	if err := qbitLogin(ctx, client, in.URL, in.Username, in.Password); err != nil {
		return rtorrent.ActionResult{Hash: in.Hash, OK: false, Message: err.Error()}
	}
	upLimit := in.UpBps
	downLimit := in.DownBps
	if upLimit == 0 {
		upLimit = -1
	}
	if downLimit == 0 {
		downLimit = -1
	}
	for _, step := range []struct {
		path  string
		limit int64
	}{
		{"/api/v2/torrents/setUploadLimit", upLimit},
		{"/api/v2/torrents/setDownloadLimit", downLimit},
	} {
		form := url.Values{}
		form.Set("hashes", in.Hash)
		form.Set("limit", fmt.Sprintf("%d", step.limit))
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, in.URL+step.path, strings.NewReader(form.Encode()))
		if err != nil {
			return rtorrent.ActionResult{Hash: in.Hash, OK: false, Message: err.Error()}
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		res, err := client.Do(req)
		if err != nil {
			return rtorrent.ActionResult{Hash: in.Hash, OK: false, Message: err.Error()}
		}
		body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		_ = res.Body.Close()
		if res.StatusCode >= 400 {
			return rtorrent.ActionResult{
				Hash:    in.Hash,
				OK:      false,
				Message: fmt.Sprintf("HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(body))),
			}
		}
	}
	return rtorrent.ActionResult{Hash: in.Hash, OK: true}
}

func setDelugeRates(ctx context.Context, in RatesInput) rtorrent.ActionResult {
	rpc := newDelugeCaller(in.URL, addHTTPClient().Timeout)
	if err := rpc.login(ctx, delugePassword(in.Username, in.Password)); err != nil {
		return rtorrent.ActionResult{Hash: in.Hash, OK: false, Message: err.Error()}
	}
	if _, err := rpc.ensureDaemon(ctx); err != nil {
		return rtorrent.ActionResult{Hash: in.Hash, OK: false, Message: err.Error()}
	}
	// Deluge expects KiB/s; -1 = unlimited.
	upKiB := int64(-1)
	downKiB := int64(-1)
	if in.UpBps > 0 {
		upKiB = (in.UpBps + 1023) / 1024
		if upKiB < 1 {
			upKiB = 1
		}
	}
	if in.DownBps > 0 {
		downKiB = (in.DownBps + 1023) / 1024
		if downKiB < 1 {
			downKiB = 1
		}
	}
	hash := strings.ToLower(in.Hash)
	opts := map[string]any{
		"max_upload_speed":   upKiB,
		"max_download_speed": downKiB,
	}
	if _, err := rpc.call(ctx, "core.set_torrent_options", []any{[]string{hash}, opts}, 40); err != nil {
		return rtorrent.ActionResult{Hash: in.Hash, OK: false, Message: err.Error()}
	}
	return rtorrent.ActionResult{Hash: in.Hash, OK: true}
}

func setTransmissionRates(ctx context.Context, in RatesInput) rtorrent.ActionResult {
	client := addHTTPClient()
	upLimited := in.UpBps > 0
	downLimited := in.DownBps > 0
	upKB := int64(0)
	downKB := int64(0)
	if upLimited {
		upKB = (in.UpBps + 999) / 1000
		if upKB < 1 {
			upKB = 1
		}
	}
	if downLimited {
		downKB = (in.DownBps + 999) / 1000
		if downKB < 1 {
			downKB = 1
		}
	}
	payload, _ := json.Marshal(map[string]any{
		"method": "torrent-set",
		"arguments": map[string]any{
			"ids":             []string{in.Hash},
			"uploadLimited":   upLimited,
			"downloadLimited": downLimited,
			"uploadLimit":     upKB,
			"downloadLimit":   downKB,
		},
	})
	for _, endpoint := range transmissionRPCCandidates(in.URL) {
		res := transmissionRPCCall(ctx, client, endpoint, in.Username, in.Password, payload)
		if res.OK {
			return rtorrent.ActionResult{Hash: in.Hash, OK: true}
		}
		if strings.Contains(strings.ToLower(res.Message), "409") {
			continue
		}
		return rtorrent.ActionResult{Hash: in.Hash, OK: false, Message: res.Message}
	}
	return rtorrent.ActionResult{Hash: in.Hash, OK: false, Message: "transmission session failed"}
}

// SyntheticThrottleName builds a shared rTorrent named throttle for a rate pair.
func SyntheticThrottleName(upBps, downBps int64) string {
	if upBps < 0 {
		upBps = 0
	}
	if downBps < 0 {
		downBps = 0
	}
	if upBps == 0 && downBps == 0 {
		return ""
	}
	return fmt.Sprintf("fleet-bw-u%d-d%d", upBps, downBps)
}
