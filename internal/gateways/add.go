package gateways

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

func addHTTPClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Timeout: 30 * time.Second,
		Jar:     jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
}

// AddInput is the material needed to inject a torrent into a remote client.
type AddInput struct {
	Kind     Kind
	URL      string
	Username string
	Password string
	// Filename is a suggested .torrent name for multipart uploads.
	Filename string
	// Torrent is raw .torrent bytes.
	Torrent []byte
	// SavePath is the existing data directory on the seeder host.
	SavePath string
	// SkipIfPresent treats "already exists" as success.
	SkipIfPresent bool
}

// AddResult is the outcome of a remote add.
type AddResult struct {
	OK      bool   `json:"ok"`
	Kind    Kind   `json:"kind"`
	Message string `json:"message,omitempty"`
	Skipped bool   `json:"skipped,omitempty"`
}

// AddTorrent pushes a .torrent to a remote WebUI/API with an existing save path.
func AddTorrent(ctx context.Context, in AddInput) AddResult {
	in.Kind = Kind(strings.ToLower(string(in.Kind)))
	in.URL = strings.TrimRight(strings.TrimSpace(in.URL), "/")
	if in.URL == "" {
		return AddResult{OK: false, Kind: in.Kind, Message: "url required"}
	}
	if len(in.Torrent) == 0 {
		return AddResult{OK: false, Kind: in.Kind, Message: "torrent bytes required"}
	}
	if strings.TrimSpace(in.SavePath) == "" {
		return AddResult{OK: false, Kind: in.Kind, Message: "savePath required"}
	}
	if in.Filename == "" {
		in.Filename = "crossseed.torrent"
	}
	switch in.Kind {
	case KindRTorrent:
		return AddResult{OK: true, Kind: in.Kind, Message: "skipped rtorrent source", Skipped: true}
	case KindQBittorrent:
		return addQBittorrent(ctx, in)
	case KindTransmission:
		return addTransmission(ctx, in)
	case KindDeluge:
		return addDeluge(ctx, in)
	default:
		return AddResult{OK: false, Kind: in.Kind, Message: "unsupported kind"}
	}
}

func addQBittorrent(ctx context.Context, in AddInput) AddResult {
	client := addHTTPClient()
	form := url.Values{}
	form.Set("username", in.Username)
	form.Set("password", in.Password)
	loginReq, err := http.NewRequestWithContext(ctx, http.MethodPost, in.URL+"/api/v2/auth/login", strings.NewReader(form.Encode()))
	if err != nil {
		return AddResult{OK: false, Kind: in.Kind, Message: err.Error()}
	}
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginRes, err := client.Do(loginReq)
	if err != nil {
		return AddResult{OK: false, Kind: in.Kind, Message: err.Error()}
	}
	body, _ := io.ReadAll(io.LimitReader(loginRes.Body, 4096))
	_ = loginRes.Body.Close()
	if loginRes.StatusCode >= 400 || strings.TrimSpace(string(body)) == "Fails." {
		return AddResult{OK: false, Kind: in.Kind, Message: "login failed"}
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("savepath", in.SavePath)
	_ = w.WriteField("skip_checking", "true")
	_ = w.WriteField("paused", "false")
	part, err := w.CreateFormFile("torrents", in.Filename)
	if err != nil {
		return AddResult{OK: false, Kind: in.Kind, Message: err.Error()}
	}
	if _, err := part.Write(in.Torrent); err != nil {
		return AddResult{OK: false, Kind: in.Kind, Message: err.Error()}
	}
	_ = w.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, in.URL+"/api/v2/torrents/add", &buf)
	if err != nil {
		return AddResult{OK: false, Kind: in.Kind, Message: err.Error()}
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	res, err := client.Do(req)
	if err != nil {
		return AddResult{OK: false, Kind: in.Kind, Message: err.Error()}
	}
	respBody, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	_ = res.Body.Close()
	msg := strings.TrimSpace(string(respBody))
	if res.StatusCode >= 400 {
		if in.SkipIfPresent && looksAlreadyExists(msg) {
			return AddResult{OK: true, Kind: in.Kind, Message: msg, Skipped: true}
		}
		return AddResult{OK: false, Kind: in.Kind, Message: fmt.Sprintf("HTTP %d: %s", res.StatusCode, msg)}
	}
	if looksAlreadyExists(msg) {
		return AddResult{OK: true, Kind: in.Kind, Message: msg, Skipped: true}
	}
	return AddResult{OK: true, Kind: in.Kind, Message: "added"}
}

func addTransmission(ctx context.Context, in AddInput) AddResult {
	client := addHTTPClient()
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	meta := base64.StdEncoding.EncodeToString(in.Torrent)
	payload, _ := json.Marshal(map[string]any{
		"method": "torrent-add",
		"arguments": map[string]any{
			"metainfo":     meta,
			"download-dir": in.SavePath,
			"paused":       false,
		},
	})
	var lastMsg string
	for _, endpoint := range transmissionRPCCandidates(in.URL) {
		res := addTransmissionEndpoint(ctx, client, endpoint, in.Username, in.Password, payload, in.SkipIfPresent)
		if res.OK || res.Skipped {
			return res
		}
		lastMsg = res.Message
	}
	return AddResult{OK: false, Kind: KindTransmission, Message: lastMsg}
}

func addTransmissionEndpoint(ctx context.Context, client *http.Client, endpoint, username, password string, payload []byte, skipIfPresent bool) AddResult {
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
		body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		_ = res.Body.Close()
		return res, body, nil
	}
	res, body, err := do("")
	if err != nil {
		return AddResult{OK: false, Kind: KindTransmission, Message: err.Error()}
	}
	if res.StatusCode == http.StatusConflict {
		sid := res.Header.Get("X-Transmission-Session-Id")
		res, body, err = do(sid)
		if err != nil {
			return AddResult{OK: false, Kind: KindTransmission, Message: err.Error()}
		}
	}
	if res.StatusCode == http.StatusUnauthorized {
		return AddResult{OK: false, Kind: KindTransmission, Message: "unauthorized"}
	}
	if res.StatusCode >= 300 {
		return AddResult{OK: false, Kind: KindTransmission, Message: fmt.Sprintf("HTTP %d", res.StatusCode)}
	}
	var parsed struct {
		Result    string `json:"result"`
		Arguments struct {
			TorrentAdded     *struct{} `json:"torrent-added"`
			TorrentDuplicate *struct{} `json:"torrent-duplicate"`
		} `json:"arguments"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return AddResult{OK: false, Kind: KindTransmission, Message: "invalid JSON response"}
	}
	if parsed.Arguments.TorrentDuplicate != nil {
		return AddResult{OK: true, Kind: KindTransmission, Message: "duplicate", Skipped: true}
	}
	if parsed.Result != "" && parsed.Result != "success" {
		if skipIfPresent && looksAlreadyExists(parsed.Result) {
			return AddResult{OK: true, Kind: KindTransmission, Message: parsed.Result, Skipped: true}
		}
		return AddResult{OK: false, Kind: KindTransmission, Message: parsed.Result}
	}
	return AddResult{OK: true, Kind: KindTransmission, Message: "added"}
}

func addDeluge(ctx context.Context, in AddInput) AddResult {
	rpc := newDelugeCaller(in.URL, addHTTPClient().Timeout)
	if err := rpc.login(ctx, delugePassword(in.Username, in.Password)); err != nil {
		return AddResult{OK: false, Kind: in.Kind, Message: err.Error()}
	}
	if _, err := rpc.ensureDaemon(ctx); err != nil {
		return AddResult{OK: false, Kind: in.Kind, Message: err.Error()}
	}

	meta := base64.StdEncoding.EncodeToString(in.Torrent)
	options := map[string]any{
		"download_location": in.SavePath,
		"seed_mode":         true,
		"add_paused":        false,
	}
	if _, err := rpc.call(ctx, "core.add_torrent_file", []any{in.Filename, meta, options}, 4); err != nil {
		if in.SkipIfPresent && looksAlreadyExists(err.Error()) {
			return AddResult{OK: true, Kind: in.Kind, Message: err.Error(), Skipped: true}
		}
		return AddResult{OK: false, Kind: in.Kind, Message: err.Error()}
	}
	return AddResult{OK: true, Kind: in.Kind, Message: "added"}
}

func looksAlreadyExists(msg string) bool {
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "already") ||
		strings.Contains(lower, "duplicate") ||
		strings.Contains(lower, "exists") ||
		strings.Contains(lower, "torrent is already")
}
