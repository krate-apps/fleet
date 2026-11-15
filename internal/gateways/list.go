package gateways

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/krate-apps/fleet/internal/rtorrent"
)

// ListInput is the connection material for listing torrents on a remote client.
type ListInput struct {
	Kind     Kind
	URL      string
	Username string
	Password string
}

// ListedTorrent is a normalized row from a remote BitTorrent client.
type ListedTorrent struct {
	Hash           string
	Name           string
	SizeBytes      int64
	BytesDone      int64
	LeftBytes      int64
	Complete       bool
	Active         bool
	Open           bool
	Hashing        bool
	DownRate       int64
	UpRate         int64
	DownTotal      int64
	UpTotal        int64
	Ratio          float64
	Priority       int64
	Directory      string
	Label          string
	State          string
	Progress       float64
	ETASeconds     int64
	SeedsComplete  int64
	PeersConnected int64
}

// ToRTorrent maps a listed row onto the shared torrent DTO used by sync/API.
func (t ListedTorrent) ToRTorrent() rtorrent.Torrent {
	out := rtorrent.Torrent{
		Hash:           strings.ToUpper(strings.TrimSpace(t.Hash)),
		Name:           t.Name,
		SizeBytes:      t.SizeBytes,
		BytesDone:      t.BytesDone,
		LeftBytes:      t.LeftBytes,
		Complete:       t.Complete,
		Active:         t.Active,
		Open:           t.Open,
		Hashing:        t.Hashing,
		DownRate:       t.DownRate,
		UpRate:         t.UpRate,
		DownTotal:      t.DownTotal,
		UpTotal:        t.UpTotal,
		Ratio:          t.Ratio,
		Priority:       t.Priority,
		Directory:      t.Directory,
		Label:          t.Label,
		State:          t.State,
		Progress:       t.Progress,
		ETASeconds:     t.ETASeconds,
		SeedsComplete:  t.SeedsComplete,
		PeersConnected: t.PeersConnected,
	}
	if out.State == "" {
		out.State = deriveListedState(out)
	}
	if out.Progress == 0 && out.SizeBytes > 0 {
		out.Progress = float64(out.BytesDone) / float64(out.SizeBytes)
	}
	if out.Complete && out.Progress == 0 {
		out.Progress = 1
	}
	return out
}

func deriveListedState(t rtorrent.Torrent) string {
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

// ListTorrents returns torrents from a remote WebUI/API.
func ListTorrents(ctx context.Context, in ListInput) ([]ListedTorrent, error) {
	in.Kind = Kind(strings.ToLower(string(in.Kind)))
	in.URL = strings.TrimSpace(in.URL)
	if in.Kind != KindRTorrent {
		in.URL = strings.TrimRight(in.URL, "/")
	}
	if in.URL == "" {
		return nil, fmt.Errorf("url required")
	}
	switch in.Kind {
	case KindRTorrent:
		return listRTorrent(ctx, in)
	case KindQBittorrent:
		return listQBittorrent(ctx, in)
	case KindTransmission:
		return listTransmission(ctx, in)
	case KindDeluge:
		return listDeluge(ctx, in)
	default:
		return nil, fmt.Errorf("unsupported kind")
	}
}

func listRTorrent(ctx context.Context, in ListInput) ([]ListedTorrent, error) {
	client, err := rtorrent.NewClient(in.URL)
	if err != nil {
		return nil, err
	}
	rows, err := client.ListTorrents(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ListedTorrent, 0, len(rows))
	for _, t := range rows {
		out = append(out, ListedTorrent{
			Hash:           t.Hash,
			Name:           t.Name,
			SizeBytes:      t.SizeBytes,
			BytesDone:      t.BytesDone,
			LeftBytes:      t.LeftBytes,
			Complete:       t.Complete,
			Active:         t.Active,
			Open:           t.Open,
			Hashing:        t.Hashing,
			DownRate:       t.DownRate,
			UpRate:         t.UpRate,
			DownTotal:      t.DownTotal,
			UpTotal:        t.UpTotal,
			Ratio:          t.Ratio,
			Priority:       t.Priority,
			Directory:      t.Directory,
			Label:          t.Label,
			State:          t.State,
			Progress:       t.Progress,
			ETASeconds:     t.ETASeconds,
			SeedsComplete:  t.SeedsComplete,
			PeersConnected: t.PeersConnected,
		})
	}
	return out, nil
}

func listQBittorrent(ctx context.Context, in ListInput) ([]ListedTorrent, error) {
	client := addHTTPClient()
	form := url.Values{}
	form.Set("username", in.Username)
	form.Set("password", in.Password)
	loginReq, err := http.NewRequestWithContext(ctx, http.MethodPost, in.URL+"/api/v2/auth/login", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginRes, err := client.Do(loginReq)
	if err != nil {
		return nil, err
	}
	body, _ := io.ReadAll(io.LimitReader(loginRes.Body, 4096))
	_ = loginRes.Body.Close()
	if loginRes.StatusCode >= 400 || strings.TrimSpace(string(body)) == "Fails." {
		return nil, fmt.Errorf("login failed")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, in.URL+"/api/v2/torrents/info", nil)
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
		Hash         string  `json:"hash"`
		Name         string  `json:"name"`
		Size         int64   `json:"size"`
		Downloaded   int64   `json:"downloaded"`
		Uploaded     int64   `json:"uploaded"`
		AmountLeft   int64   `json:"amount_left"`
		Dlspeed      int64   `json:"dlspeed"`
		Upspeed      int64   `json:"upspeed"`
		Progress     float64 `json:"progress"`
		Ratio        float64 `json:"ratio"`
		State        string  `json:"state"`
		SavePath     string  `json:"save_path"`
		Category     string  `json:"category"`
		Priority     int64   `json:"priority"`
		NumComplete  int64   `json:"num_complete"`
		NumLeechs    int64   `json:"num_leechs"`
		Eta          int64   `json:"eta"`
	}
	if err := json.Unmarshal(payload, &rows); err != nil {
		return nil, fmt.Errorf("invalid JSON response")
	}
	out := make([]ListedTorrent, 0, len(rows))
	for _, r := range rows {
		complete := r.Progress >= 0.999 || r.AmountLeft == 0
		active, open, hashing, state := mapQBitState(r.State, complete)
		eta := r.Eta
		if eta < 0 || eta > 8640000 {
			eta = 0
		}
		out = append(out, ListedTorrent{
			Hash:           r.Hash,
			Name:           r.Name,
			SizeBytes:      r.Size,
			BytesDone:      r.Downloaded,
			LeftBytes:      r.AmountLeft,
			Complete:       complete,
			Active:         active,
			Open:           open,
			Hashing:        hashing,
			DownRate:       r.Dlspeed,
			UpRate:         r.Upspeed,
			DownTotal:      r.Downloaded,
			UpTotal:        r.Uploaded,
			Ratio:          r.Ratio,
			Priority:       r.Priority,
			Directory:      r.SavePath,
			Label:          r.Category,
			State:          state,
			Progress:       r.Progress,
			ETASeconds:     eta,
			SeedsComplete:  r.NumComplete,
			PeersConnected: r.NumComplete + r.NumLeechs,
		})
	}
	return out, nil
}

func mapQBitState(raw string, complete bool) (active, open, hashing bool, state string) {
	s := strings.ToLower(strings.TrimSpace(raw))
	open = true
	switch s {
	case "error", "missingfiles":
		return false, true, false, "error"
	case "uploading", "stalledup", "forcedup", "queuedup":
		return true, true, false, "seeding"
	case "downloading", "stalleddl", "forceddl", "queueddl", "metaDL", "metadl":
		return true, true, false, "downloading"
	case "pausedup", "pauseddl", "stoppedup", "stoppeddl":
		if complete {
			return false, true, false, "stopped"
		}
		return false, true, false, "paused"
	case "checkingup", "checkingdl", "checkingresumedata", "moving":
		return true, true, true, "hashing"
	case "allocating":
		return true, true, false, "downloading"
	default:
		if complete {
			return false, true, false, "stopped"
		}
		return false, true, false, "unknown"
	}
}

func listTransmission(ctx context.Context, in ListInput) ([]ListedTorrent, error) {
	client := addHTTPClient()
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	payload, _ := json.Marshal(map[string]any{
		"method": "torrent-get",
		"arguments": map[string]any{
			"fields": []string{
				"hashString", "name", "totalSize", "downloadedEver", "uploadedEver",
				"leftUntilDone", "rateDownload", "rateUpload", "uploadRatio",
				"status", "downloadDir", "labels", "isFinished", "percentDone",
				"eta", "peersSendingToUs", "peersGettingFromUs", "peersConnected",
			},
		},
	})
	var lastErr error
	for _, endpoint := range transmissionRPCCandidates(in.URL) {
		rows, err := listTransmissionEndpoint(ctx, client, endpoint, in.Username, in.Password, payload)
		if err == nil {
			return rows, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no working RPC endpoint")
	}
	return nil, lastErr
}

func listTransmissionEndpoint(ctx context.Context, client *http.Client, endpoint, username, password string, payload []byte) ([]ListedTorrent, error) {
	do := func(sessionID string) (*http.Response, []byte, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			return nil, nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		if sessionID != "" {
			req.Header.Set("X-Transmission-Session-Id", sessionID)
		}
		if username != "" || password != "" {
			req.SetBasicAuth(username, password)
		}
		res, err := client.Do(req)
		if err != nil {
			return nil, nil, err
		}
		body, _ := io.ReadAll(io.LimitReader(res.Body, 8<<20))
		_ = res.Body.Close()
		return res, body, nil
	}
	res, body, err := do("")
	if err != nil {
		return nil, err
	}
	if res.StatusCode == http.StatusConflict {
		sid := res.Header.Get("X-Transmission-Session-Id")
		res, body, err = do(sid)
		if err != nil {
			return nil, err
		}
	}
	if res.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("unauthorized")
	}
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", res.StatusCode)
	}
	var parsed struct {
		Result    string `json:"result"`
		Arguments struct {
			Torrents []struct {
				HashString         string   `json:"hashString"`
				Name               string   `json:"name"`
				TotalSize          int64    `json:"totalSize"`
				DownloadedEver     int64    `json:"downloadedEver"`
				UploadedEver       int64    `json:"uploadedEver"`
				LeftUntilDone      int64    `json:"leftUntilDone"`
				RateDownload       int64    `json:"rateDownload"`
				RateUpload         int64    `json:"rateUpload"`
				UploadRatio        float64  `json:"uploadRatio"`
				Status             int      `json:"status"`
				DownloadDir        string   `json:"downloadDir"`
				Labels             []string `json:"labels"`
				IsFinished         bool     `json:"isFinished"`
				PercentDone        float64  `json:"percentDone"`
				Eta                int64    `json:"eta"`
				PeersSendingToUs   int64    `json:"peersSendingToUs"`
				PeersGettingFromUs int64    `json:"peersGettingFromUs"`
				PeersConnected     int64    `json:"peersConnected"`
			} `json:"torrents"`
		} `json:"arguments"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("invalid JSON response")
	}
	if parsed.Result != "" && parsed.Result != "success" {
		return nil, fmt.Errorf("%s", parsed.Result)
	}
	out := make([]ListedTorrent, 0, len(parsed.Arguments.Torrents))
	for _, r := range parsed.Arguments.Torrents {
		complete := r.IsFinished || r.LeftUntilDone == 0 || r.PercentDone >= 0.999
		active, open, hashing, state := mapTransmissionStatus(r.Status, complete)
		eta := r.Eta
		if eta < 0 {
			eta = 0
		}
		label := ""
		if len(r.Labels) > 0 {
			label = r.Labels[0]
		}
		ratio := r.UploadRatio
		if ratio < 0 {
			ratio = 0
		}
		out = append(out, ListedTorrent{
			Hash:           r.HashString,
			Name:           r.Name,
			SizeBytes:      r.TotalSize,
			BytesDone:      r.DownloadedEver,
			LeftBytes:      r.LeftUntilDone,
			Complete:       complete,
			Active:         active,
			Open:           open,
			Hashing:        hashing,
			DownRate:       r.RateDownload,
			UpRate:         r.RateUpload,
			DownTotal:      r.DownloadedEver,
			UpTotal:        r.UploadedEver,
			Ratio:          ratio,
			Directory:      r.DownloadDir,
			Label:          label,
			State:          state,
			Progress:       r.PercentDone,
			ETASeconds:     eta,
			SeedsComplete:  r.PeersSendingToUs,
			PeersConnected: r.PeersConnected,
		})
	}
	return out, nil
}

// Transmission status codes: 0 stopped, 1 check wait, 2 check, 3 download wait, 4 download, 5 seed wait, 6 seed.
func mapTransmissionStatus(status int, complete bool) (active, open, hashing bool, state string) {
	open = true
	switch status {
	case 0:
		if complete {
			return false, true, false, "stopped"
		}
		return false, true, false, "paused"
	case 1, 2:
		return true, true, true, "hashing"
	case 3, 4:
		return true, true, false, "downloading"
	case 5, 6:
		return true, true, false, "seeding"
	default:
		if complete {
			return false, true, false, "stopped"
		}
		return false, true, false, "unknown"
	}
}

func listDeluge(ctx context.Context, in ListInput) ([]ListedTorrent, error) {
	rpc := newDelugeCaller(in.URL, 30*time.Second)
	if err := rpc.login(ctx, delugePassword(in.Username, in.Password)); err != nil {
		return nil, err
	}
	if _, err := rpc.ensureDaemon(ctx); err != nil {
		return nil, err
	}

	keys := []string{
		"name", "total_size", "progress", "download_payload_rate", "upload_payload_rate",
		"total_done", "total_uploaded", "ratio", "state", "save_path", "label",
		"num_seeds", "num_peers", "eta", "is_seed", "paused",
	}
	statusRaw, err := rpc.call(ctx, "core.get_torrents_status", []any{map[string]any{}, keys}, 4)
	if err != nil {
		return nil, err
	}
	var byHash map[string]map[string]any
	if err := json.Unmarshal(statusRaw, &byHash); err != nil {
		return nil, fmt.Errorf("invalid torrents status")
	}
	out := make([]ListedTorrent, 0, len(byHash))
	for hash, fields := range byHash {
		name, _ := fields["name"].(string)
		size := jsonInt64(fields["total_size"])
		done := jsonInt64(fields["total_done"])
		progressRaw := jsonFloat64(fields["progress"])
		progress := progressRaw
		if progressRaw > 1.0 {
			progress = progressRaw / 100.0
		}
		complete := false
		if v, ok := fields["is_seed"].(bool); ok {
			complete = v
		} else {
			complete = progress >= 0.999
		}
		paused := false
		if v, ok := fields["paused"].(bool); ok {
			paused = v
		}
		stateRaw, _ := fields["state"].(string)
		active, open, hashing, state := mapDelugeState(stateRaw, complete, paused)
		left := size - done
		if left < 0 {
			left = 0
		}
		label, _ := fields["label"].(string)
		dir, _ := fields["save_path"].(string)
		out = append(out, ListedTorrent{
			Hash:           hash,
			Name:           name,
			SizeBytes:      size,
			BytesDone:      done,
			LeftBytes:      left,
			Complete:       complete,
			Active:         active,
			Open:           open,
			Hashing:        hashing,
			DownRate:       jsonInt64(fields["download_payload_rate"]),
			UpRate:         jsonInt64(fields["upload_payload_rate"]),
			DownTotal:      done,
			UpTotal:        jsonInt64(fields["total_uploaded"]),
			Ratio:          jsonFloat64(fields["ratio"]),
			Directory:      dir,
			Label:          label,
			State:          state,
			Progress:       progress,
			ETASeconds:     jsonInt64(fields["eta"]),
			SeedsComplete:  jsonInt64(fields["num_seeds"]),
			PeersConnected: jsonInt64(fields["num_seeds"]) + jsonInt64(fields["num_peers"]),
		})
	}
	return out, nil
}

func mapDelugeState(raw string, complete, paused bool) (active, open, hashing bool, state string) {
	open = true
	s := strings.ToLower(strings.TrimSpace(raw))
	switch s {
	case "checking":
		return true, true, true, "hashing"
	case "downloading":
		return true, true, false, "downloading"
	case "seeding":
		return true, true, false, "seeding"
	case "paused", "queued":
		if complete {
			return false, true, false, "stopped"
		}
		return false, true, false, "paused"
	case "error":
		return false, true, false, "error"
	default:
		if paused {
			if complete {
				return false, true, false, "stopped"
			}
			return false, true, false, "paused"
		}
		if complete {
			return true, true, false, "seeding"
		}
		return true, true, false, "downloading"
	}
}

func jsonInt64(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case int:
		return int64(n)
	case json.Number:
		i, _ := n.Int64()
		return i
	default:
		return 0
	}
}

func jsonFloat64(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int64:
		return float64(n)
	case int:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	default:
		return 0
	}
}
