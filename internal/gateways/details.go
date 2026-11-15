package gateways

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/krate-apps/fleet/internal/rtorrent"
)

// ErrUnsupported is returned when a detail feature is not available on this client.
var ErrUnsupported = errors.New("unsupported on this client")

// DetailsInput is connection material for torrent detail reads.
type DetailsInput struct {
	Kind     Kind
	URL      string
	Username string
	Password string
	Hash     string
}

// UnsupportedError carries which feature is missing (PD-GW-02).
type UnsupportedError struct {
	Kind    Kind
	Feature string
}

func (e *UnsupportedError) Error() string {
	return fmt.Sprintf("%s: %s not supported on %s", ErrUnsupported.Error(), e.Feature, e.Kind)
}

func (e *UnsupportedError) Is(target error) bool {
	return target == ErrUnsupported
}

// ListFiles returns file rows when the client API supports it.
func ListFiles(ctx context.Context, in DetailsInput) ([]rtorrent.FileEntry, error) {
	in = normalizeDetails(in)
	if !SupportsFeature(in.Kind, "files") {
		return nil, &UnsupportedError{Kind: in.Kind, Feature: "files"}
	}
	switch in.Kind {
	case KindQBittorrent:
		return qbitFiles(ctx, in)
	case KindDeluge:
		return delugeFiles(ctx, in)
	case KindTransmission:
		return transmissionFiles(ctx, in)
	default:
		return nil, &UnsupportedError{Kind: in.Kind, Feature: "files"}
	}
}

// ListTrackers returns tracker rows when supported.
func ListTrackers(ctx context.Context, in DetailsInput) ([]rtorrent.TrackerEntry, error) {
	in = normalizeDetails(in)
	if !SupportsFeature(in.Kind, "trackers") {
		return nil, &UnsupportedError{Kind: in.Kind, Feature: "trackers"}
	}
	switch in.Kind {
	case KindQBittorrent:
		return qbitTrackers(ctx, in)
	case KindDeluge:
		return delugeTrackers(ctx, in)
	case KindTransmission:
		return transmissionTrackers(ctx, in)
	default:
		return nil, &UnsupportedError{Kind: in.Kind, Feature: "trackers"}
	}
}

// ListPeers returns peer rows when supported.
func ListPeers(ctx context.Context, in DetailsInput) ([]rtorrent.PeerEntry, error) {
	in = normalizeDetails(in)
	if !SupportsFeature(in.Kind, "peers") {
		return nil, &UnsupportedError{Kind: in.Kind, Feature: "peers"}
	}
	switch in.Kind {
	case KindQBittorrent:
		return qbitPeers(ctx, in)
	case KindTransmission:
		return transmissionPeers(ctx, in)
	default:
		return nil, &UnsupportedError{Kind: in.Kind, Feature: "peers"}
	}
}

func normalizeDetails(in DetailsInput) DetailsInput {
	in.Kind = Kind(normalizeKind(string(in.Kind)))
	in.URL = strings.TrimRight(strings.TrimSpace(in.URL), "/")
	in.Hash = strings.ToUpper(strings.TrimSpace(in.Hash))
	return in
}

func qbitFiles(ctx context.Context, in DetailsInput) ([]rtorrent.FileEntry, error) {
	client := addHTTPClient()
	if err := qbitLogin(ctx, client, in.URL, in.Username, in.Password); err != nil {
		return nil, err
	}
	u := in.URL + "/api/v2/torrents/files?hash=" + url.QueryEscape(in.Hash)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d", res.StatusCode)
	}
	var rows []struct {
		Name     string  `json:"name"`
		Size     int64   `json:"size"`
		Progress float64 `json:"progress"`
		Priority int64   `json:"priority"`
		Index    int64   `json:"index"`
	}
	if err := json.Unmarshal(payload, &rows); err != nil {
		return nil, fmt.Errorf("invalid JSON response")
	}
	out := make([]rtorrent.FileEntry, 0, len(rows))
	for i, r := range rows {
		idx := r.Index
		if idx == 0 && i > 0 {
			idx = int64(i)
		}
		out = append(out, rtorrent.FileEntry{
			Index:     idx,
			Path:      r.Name,
			SizeBytes: r.Size,
			Priority:  r.Priority,
		})
	}
	return out, nil
}

func qbitTrackers(ctx context.Context, in DetailsInput) ([]rtorrent.TrackerEntry, error) {
	client := addHTTPClient()
	if err := qbitLogin(ctx, client, in.URL, in.Username, in.Password); err != nil {
		return nil, err
	}
	u := in.URL + "/api/v2/torrents/trackers?hash=" + url.QueryEscape(in.Hash)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d", res.StatusCode)
	}
	var rows []struct {
		URL       string `json:"url"`
		Status    int    `json:"status"`
		NumPeers  int64  `json:"num_peers"`
		NumSeeds  int64  `json:"num_seeds"`
		NumLeeches int64 `json:"num_leeches"`
	}
	if err := json.Unmarshal(payload, &rows); err != nil {
		return nil, fmt.Errorf("invalid JSON response")
	}
	out := make([]rtorrent.TrackerEntry, 0, len(rows))
	for i, r := range rows {
		out = append(out, rtorrent.TrackerEntry{
			Index:    int64(i),
			ID:       fmt.Sprintf("%s:%d", in.Hash, i),
			URL:      r.URL,
			Enabled:  r.Status != 0,
			Open:     r.Status == 2,
			Seeders:  r.NumSeeds,
			Leechers: r.NumLeeches,
		})
	}
	return out, nil
}

func qbitPeers(ctx context.Context, in DetailsInput) ([]rtorrent.PeerEntry, error) {
	client := addHTTPClient()
	if err := qbitLogin(ctx, client, in.URL, in.Username, in.Password); err != nil {
		return nil, err
	}
	u := in.URL + "/api/v2/sync/torrentPeers?hash=" + url.QueryEscape(in.Hash)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d", res.StatusCode)
	}
	var parsed struct {
		Peers map[string]struct {
			IP        string  `json:"ip"`
			Client    string  `json:"client"`
			Progress  float64 `json:"progress"`
			DlSpeed   int64   `json:"dl_speed"`
			UpSpeed   int64   `json:"up_speed"`
			Country   string  `json:"country_code"`
		} `json:"peers"`
	}
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return nil, fmt.Errorf("invalid JSON response")
	}
	out := make([]rtorrent.PeerEntry, 0, len(parsed.Peers))
	for _, p := range parsed.Peers {
		out = append(out, rtorrent.PeerEntry{
			Address:          p.IP,
			ClientVersion:    p.Client,
			DownRate:         p.DlSpeed,
			UpRate:           p.UpSpeed,
			CompletedPercent: p.Progress * 100,
			Country:          p.Country,
		})
	}
	return out, nil
}

func delugeFiles(ctx context.Context, in DetailsInput) ([]rtorrent.FileEntry, error) {
	rpc := newDelugeCaller(in.URL, addHTTPClient().Timeout)
	if err := rpc.login(ctx, delugePassword(in.Username, in.Password)); err != nil {
		return nil, err
	}
	if _, err := rpc.ensureDaemon(ctx); err != nil {
		return nil, err
	}
	hash := strings.ToLower(in.Hash)
	raw, err := rpc.call(ctx, "core.get_torrent_status", []any{hash, []string{"files", "file_progress", "file_priorities"}}, 40)
	if err != nil {
		return nil, err
	}
	var st struct {
		Files []struct {
			Index  int64  `json:"index"`
			Path   string `json:"path"`
			Size   int64  `json:"size"`
			Offset int64  `json:"offset"`
		} `json:"files"`
		Priorities []int64 `json:"file_priorities"`
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, fmt.Errorf("invalid deluge files")
	}
	out := make([]rtorrent.FileEntry, 0, len(st.Files))
	for i, f := range st.Files {
		prio := int64(1)
		if i < len(st.Priorities) {
			prio = st.Priorities[i]
		}
		out = append(out, rtorrent.FileEntry{
			Index:     f.Index,
			Path:      f.Path,
			SizeBytes: f.Size,
			Priority:  prio,
		})
	}
	return out, nil
}

func delugeTrackers(ctx context.Context, in DetailsInput) ([]rtorrent.TrackerEntry, error) {
	rpc := newDelugeCaller(in.URL, addHTTPClient().Timeout)
	if err := rpc.login(ctx, delugePassword(in.Username, in.Password)); err != nil {
		return nil, err
	}
	if _, err := rpc.ensureDaemon(ctx); err != nil {
		return nil, err
	}
	hash := strings.ToLower(in.Hash)
	raw, err := rpc.call(ctx, "core.get_torrent_status", []any{hash, []string{"trackers"}}, 41)
	if err != nil {
		return nil, err
	}
	var st struct {
		Trackers []struct {
			URL  string `json:"url"`
			Tier int    `json:"tier"`
		} `json:"trackers"`
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, fmt.Errorf("invalid deluge trackers")
	}
	out := make([]rtorrent.TrackerEntry, 0, len(st.Trackers))
	for i, t := range st.Trackers {
		out = append(out, rtorrent.TrackerEntry{
			Index:   int64(i),
			ID:      fmt.Sprintf("%s:%d", in.Hash, i),
			URL:     t.URL,
			Enabled: true,
			Open:    true,
		})
	}
	return out, nil
}

func transmissionFiles(ctx context.Context, in DetailsInput) ([]rtorrent.FileEntry, error) {
	raw, err := transmissionTorrentGet(ctx, in, []string{"files", "priorities", "wanted"})
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Arguments struct {
			Torrents []struct {
				Files []struct {
					Name           string `json:"name"`
					Length         int64  `json:"length"`
					BytesCompleted int64  `json:"bytesCompleted"`
				} `json:"files"`
				Priorities []int64 `json:"priorities"`
			} `json:"torrents"`
		} `json:"arguments"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil || len(parsed.Arguments.Torrents) == 0 {
		return nil, fmt.Errorf("invalid transmission files")
	}
	t := parsed.Arguments.Torrents[0]
	out := make([]rtorrent.FileEntry, 0, len(t.Files))
	for i, f := range t.Files {
		prio := int64(1)
		if i < len(t.Priorities) {
			prio = t.Priorities[i]
		}
		out = append(out, rtorrent.FileEntry{
			Index:     int64(i),
			Path:      f.Name,
			SizeBytes: f.Length,
			Priority:  prio,
		})
	}
	return out, nil
}

func transmissionTrackers(ctx context.Context, in DetailsInput) ([]rtorrent.TrackerEntry, error) {
	raw, err := transmissionTorrentGet(ctx, in, []string{"trackerStats"})
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Arguments struct {
			Torrents []struct {
				TrackerStats []struct {
					Announce string `json:"announce"`
					SeederCount int64 `json:"seederCount"`
					LeecherCount int64 `json:"leecherCount"`
				} `json:"trackerStats"`
			} `json:"torrents"`
		} `json:"arguments"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil || len(parsed.Arguments.Torrents) == 0 {
		return nil, fmt.Errorf("invalid transmission trackers")
	}
	stats := parsed.Arguments.Torrents[0].TrackerStats
	out := make([]rtorrent.TrackerEntry, 0, len(stats))
	for i, t := range stats {
		out = append(out, rtorrent.TrackerEntry{
			Index:    int64(i),
			ID:       fmt.Sprintf("%s:%d", in.Hash, i),
			URL:      t.Announce,
			Enabled:  true,
			Open:     true,
			Seeders:  t.SeederCount,
			Leechers: t.LeecherCount,
		})
	}
	return out, nil
}

func transmissionPeers(ctx context.Context, in DetailsInput) ([]rtorrent.PeerEntry, error) {
	raw, err := transmissionTorrentGet(ctx, in, []string{"peers"})
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Arguments struct {
			Torrents []struct {
				Peers []struct {
					Address    string  `json:"address"`
					ClientName string  `json:"clientName"`
					Progress   float64 `json:"progress"`
					RateToClient int64 `json:"rateToClient"`
					RateToPeer   int64 `json:"rateToPeer"`
				} `json:"peers"`
			} `json:"torrents"`
		} `json:"arguments"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil || len(parsed.Arguments.Torrents) == 0 {
		return nil, fmt.Errorf("invalid transmission peers")
	}
	peers := parsed.Arguments.Torrents[0].Peers
	out := make([]rtorrent.PeerEntry, 0, len(peers))
	for _, p := range peers {
		out = append(out, rtorrent.PeerEntry{
			Address:          p.Address,
			ClientVersion:    p.ClientName,
			DownRate:         p.RateToClient,
			UpRate:           p.RateToPeer,
			CompletedPercent: p.Progress * 100,
		})
	}
	return out, nil
}

func transmissionTorrentGet(ctx context.Context, in DetailsInput, fields []string) (json.RawMessage, error) {
	client := addHTTPClient()
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	args := map[string]any{
		"ids":    []string{strings.ToLower(in.Hash)},
		"fields": fields,
	}
	payload, _ := json.Marshal(map[string]any{"method": "torrent-get", "arguments": args})
	var lastMsg string
	for _, endpoint := range transmissionRPCCandidates(in.URL) {
		session := ""
		for attempt := 0; attempt < 3; attempt++ {
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(payload)))
			if err != nil {
				return nil, err
			}
			req.Header.Set("Content-Type", "application/json")
			if session != "" {
				req.Header.Set("X-Transmission-Session-Id", session)
			}
			if in.Username != "" || in.Password != "" {
				req.SetBasicAuth(in.Username, in.Password)
			}
			res, err := client.Do(req)
			if err != nil {
				lastMsg = err.Error()
				break
			}
			body, _ := io.ReadAll(io.LimitReader(res.Body, 8<<20))
			_ = res.Body.Close()
			if res.StatusCode == http.StatusConflict {
				session = res.Header.Get("X-Transmission-Session-Id")
				continue
			}
			if res.StatusCode >= 400 {
				lastMsg = fmt.Sprintf("HTTP %d", res.StatusCode)
				break
			}
			return body, nil
		}
	}
	if lastMsg == "" {
		lastMsg = "transmission get failed"
	}
	return nil, fmt.Errorf("%s", lastMsg)
}
