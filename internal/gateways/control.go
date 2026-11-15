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

// ControlInput is the material for a remote torrent control action.
type ControlInput struct {
	Kind     Kind
	URL      string
	Username string
	Password string
	Hash     string
	Action   rtorrent.Action
}

// ApplyTorrentAction runs start/stop/pause/resume/erase/erase_data/reannounce on a remote client.
func ApplyTorrentAction(ctx context.Context, in ControlInput) rtorrent.ActionResult {
	in.Kind = Kind(normalizeKind(string(in.Kind)))
	in.URL = strings.TrimRight(strings.TrimSpace(in.URL), "/")
	in.Hash = strings.ToUpper(strings.TrimSpace(in.Hash))
	if in.URL == "" {
		return rtorrent.ActionResult{Hash: in.Hash, OK: false, Message: "url required"}
	}
	if in.Hash == "" {
		return rtorrent.ActionResult{OK: false, Message: "empty hash"}
	}
	if in.Kind == KindRTorrent {
		return rtorrent.ActionResult{Hash: in.Hash, OK: false, Message: "use local rtorrent ApplyAction"}
	}
	feature := string(in.Action)
	if in.Action == rtorrent.ActionEraseData {
		feature = "erase_data"
	}
	if !SupportsFeature(in.Kind, feature) &&
		!(SupportsFeature(in.Kind, "startStop") && (in.Action == rtorrent.ActionStart || in.Action == rtorrent.ActionStop || in.Action == rtorrent.ActionPause || in.Action == rtorrent.ActionResume)) {
		return rtorrent.ActionResult{Hash: in.Hash, OK: false, Message: "unsupported on this client"}
	}
	switch in.Kind {
	case KindQBittorrent:
		return controlQBittorrent(ctx, in)
	case KindDeluge:
		return controlDeluge(ctx, in)
	case KindTransmission:
		return controlTransmission(ctx, in)
	default:
		return rtorrent.ActionResult{Hash: in.Hash, OK: false, Message: "unsupported kind"}
	}
}

func controlQBittorrent(ctx context.Context, in ControlInput) rtorrent.ActionResult {
	client := addHTTPClient()
	if err := qbitLogin(ctx, client, in.URL, in.Username, in.Password); err != nil {
		return rtorrent.ActionResult{Hash: in.Hash, OK: false, Message: err.Error()}
	}
	endpoint := ""
	form := url.Values{}
	form.Set("hashes", in.Hash)
	switch in.Action {
	case rtorrent.ActionStart, rtorrent.ActionResume:
		endpoint = "/api/v2/torrents/resume"
	case rtorrent.ActionStop, rtorrent.ActionPause:
		endpoint = "/api/v2/torrents/pause"
	case rtorrent.ActionErase:
		endpoint = "/api/v2/torrents/delete"
		form.Set("deleteFiles", "false")
	case rtorrent.ActionEraseData:
		endpoint = "/api/v2/torrents/delete"
		form.Set("deleteFiles", "true")
	case rtorrent.ActionReannounce:
		endpoint = "/api/v2/torrents/reannounce"
	case rtorrent.ActionHash:
		endpoint = "/api/v2/torrents/recheck"
	default:
		return rtorrent.ActionResult{Hash: in.Hash, OK: false, Message: "unsupported action"}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, in.URL+endpoint, strings.NewReader(form.Encode()))
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
		msg := strings.TrimSpace(string(body))
		return rtorrent.ActionResult{Hash: in.Hash, OK: false, Message: fmt.Sprintf("HTTP %d: %s", res.StatusCode, msg)}
	}
	return rtorrent.ActionResult{Hash: in.Hash, OK: true}
}

func controlDeluge(ctx context.Context, in ControlInput) rtorrent.ActionResult {
	rpc := newDelugeCaller(in.URL, addHTTPClient().Timeout)
	if err := rpc.login(ctx, delugePassword(in.Username, in.Password)); err != nil {
		return rtorrent.ActionResult{Hash: in.Hash, OK: false, Message: err.Error()}
	}
	if _, err := rpc.ensureDaemon(ctx); err != nil {
		return rtorrent.ActionResult{Hash: in.Hash, OK: false, Message: err.Error()}
	}
	hash := strings.ToLower(in.Hash)
	var err error
	switch in.Action {
	case rtorrent.ActionStart, rtorrent.ActionResume:
		_, err = rpc.call(ctx, "core.resume_torrent", []any{[]string{hash}}, 30)
	case rtorrent.ActionStop, rtorrent.ActionPause:
		_, err = rpc.call(ctx, "core.pause_torrent", []any{[]string{hash}}, 31)
	case rtorrent.ActionErase:
		_, err = rpc.call(ctx, "core.remove_torrent", []any{hash, false}, 32)
	case rtorrent.ActionEraseData:
		_, err = rpc.call(ctx, "core.remove_torrent", []any{hash, true}, 33)
	case rtorrent.ActionReannounce:
		_, err = rpc.call(ctx, "core.force_reannounce", []any{[]string{hash}}, 34)
	case rtorrent.ActionHash:
		_, err = rpc.call(ctx, "core.force_recheck", []any{[]string{hash}}, 35)
	default:
		return rtorrent.ActionResult{Hash: in.Hash, OK: false, Message: "unsupported action"}
	}
	if err != nil {
		return rtorrent.ActionResult{Hash: in.Hash, OK: false, Message: err.Error()}
	}
	return rtorrent.ActionResult{Hash: in.Hash, OK: true}
}

func controlTransmission(ctx context.Context, in ControlInput) rtorrent.ActionResult {
	client := addHTTPClient()
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	method := ""
	args := map[string]any{"ids": []string{strings.ToLower(in.Hash)}}
	switch in.Action {
	case rtorrent.ActionStart, rtorrent.ActionResume:
		method = "torrent-start"
	case rtorrent.ActionStop, rtorrent.ActionPause:
		method = "torrent-stop"
	case rtorrent.ActionErase:
		method = "torrent-remove"
		args["delete-local-data"] = false
	case rtorrent.ActionEraseData:
		method = "torrent-remove"
		args["delete-local-data"] = true
	case rtorrent.ActionReannounce:
		method = "torrent-reannounce"
	case rtorrent.ActionHash:
		method = "torrent-verify"
	default:
		return rtorrent.ActionResult{Hash: in.Hash, OK: false, Message: "unsupported action"}
	}
	payload, _ := json.Marshal(map[string]any{"method": method, "arguments": args})
	var lastMsg string
	for _, endpoint := range transmissionRPCCandidates(in.URL) {
		res := transmissionRPCCall(ctx, client, endpoint, in.Username, in.Password, payload)
		if res.OK {
			return rtorrent.ActionResult{Hash: in.Hash, OK: true}
		}
		lastMsg = res.Message
	}
	return rtorrent.ActionResult{Hash: in.Hash, OK: false, Message: lastMsg}
}

func qbitLogin(ctx context.Context, client *http.Client, base, user, pass string) error {
	form := url.Values{}
	form.Set("username", user)
	form.Set("password", pass)
	loginReq, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/v2/auth/login", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginRes, err := client.Do(loginReq)
	if err != nil {
		return err
	}
	body, _ := io.ReadAll(io.LimitReader(loginRes.Body, 4096))
	_ = loginRes.Body.Close()
	if loginRes.StatusCode >= 400 || strings.TrimSpace(string(body)) == "Fails." {
		return fmt.Errorf("login failed")
	}
	return nil
}

type simpleResult struct {
	OK      bool
	Message string
}

func transmissionRPCCall(ctx context.Context, client *http.Client, endpoint, user, pass string, payload []byte) simpleResult {
	session := ""
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(payload)))
		if err != nil {
			return simpleResult{OK: false, Message: err.Error()}
		}
		req.Header.Set("Content-Type", "application/json")
		if session != "" {
			req.Header.Set("X-Transmission-Session-Id", session)
		}
		if user != "" || pass != "" {
			req.SetBasicAuth(user, pass)
		}
		res, err := client.Do(req)
		if err != nil {
			return simpleResult{OK: false, Message: err.Error()}
		}
		body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		_ = res.Body.Close()
		if res.StatusCode == http.StatusConflict {
			session = res.Header.Get("X-Transmission-Session-Id")
			continue
		}
		if res.StatusCode >= 400 {
			return simpleResult{OK: false, Message: fmt.Sprintf("HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(body)))}
		}
		var parsed struct {
			Result string `json:"result"`
		}
		_ = json.Unmarshal(body, &parsed)
		if parsed.Result != "" && !strings.EqualFold(parsed.Result, "success") {
			return simpleResult{OK: false, Message: parsed.Result}
		}
		return simpleResult{OK: true, Message: "ok"}
	}
	return simpleResult{OK: false, Message: "transmission session failed"}
}
