package gateways

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// LabelInput is the material needed to set a torrent's label/category on a remote client.
type LabelInput struct {
	Kind     Kind
	URL      string
	Username string
	Password string
	Hash     string
	Label    string
}

// LabelResult is the outcome of a remote label/category write.
type LabelResult struct {
	OK      bool   `json:"ok"`
	Kind    Kind   `json:"kind"`
	Message string `json:"message,omitempty"`
}

// SetTorrentLabel sets the unified Fleet label on a remote client
// (qBit category, Deluge label plugin, Transmission labels).
func SetTorrentLabel(ctx context.Context, in LabelInput) LabelResult {
	in.Kind = Kind(strings.ToLower(string(in.Kind)))
	in.URL = strings.TrimRight(strings.TrimSpace(in.URL), "/")
	in.Hash = strings.ToUpper(strings.TrimSpace(in.Hash))
	in.Label = strings.TrimSpace(in.Label)
	if in.URL == "" {
		return LabelResult{OK: false, Kind: in.Kind, Message: "url required"}
	}
	if in.Hash == "" {
		return LabelResult{OK: false, Kind: in.Kind, Message: "hash required"}
	}
	switch in.Kind {
	case KindRTorrent:
		return LabelResult{OK: false, Kind: in.Kind, Message: "use local rtorrent setLabel for rtorrent"}
	case KindQBittorrent:
		return setQBittorrentCategory(ctx, in)
	case KindDeluge:
		return setDelugeLabel(ctx, in)
	case KindTransmission:
		return setTransmissionLabel(ctx, in)
	default:
		return LabelResult{OK: false, Kind: in.Kind, Message: "unsupported kind"}
	}
}

func setQBittorrentCategory(ctx context.Context, in LabelInput) LabelResult {
	client := addHTTPClient()
	form := url.Values{}
	form.Set("username", in.Username)
	form.Set("password", in.Password)
	loginReq, err := http.NewRequestWithContext(ctx, http.MethodPost, in.URL+"/api/v2/auth/login", strings.NewReader(form.Encode()))
	if err != nil {
		return LabelResult{OK: false, Kind: in.Kind, Message: err.Error()}
	}
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginRes, err := client.Do(loginReq)
	if err != nil {
		return LabelResult{OK: false, Kind: in.Kind, Message: err.Error()}
	}
	body, _ := io.ReadAll(io.LimitReader(loginRes.Body, 4096))
	_ = loginRes.Body.Close()
	if loginRes.StatusCode >= 400 || strings.TrimSpace(string(body)) == "Fails." {
		return LabelResult{OK: false, Kind: in.Kind, Message: "login failed"}
	}

	if in.Label != "" {
		create := url.Values{}
		create.Set("category", in.Label)
		create.Set("savePath", "")
		creq, err := http.NewRequestWithContext(ctx, http.MethodPost, in.URL+"/api/v2/torrents/createCategory", strings.NewReader(create.Encode()))
		if err != nil {
			return LabelResult{OK: false, Kind: in.Kind, Message: err.Error()}
		}
		creq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		cres, err := client.Do(creq)
		if err != nil {
			return LabelResult{OK: false, Kind: in.Kind, Message: err.Error()}
		}
		cbody, _ := io.ReadAll(io.LimitReader(cres.Body, 1<<20))
		_ = cres.Body.Close()
		// 409 "Category name already exists" is fine.
		if cres.StatusCode >= 400 && cres.StatusCode != http.StatusConflict {
			msg := strings.TrimSpace(string(cbody))
			lower := strings.ToLower(msg)
			if !strings.Contains(lower, "already") && !strings.Contains(lower, "exist") {
				return LabelResult{OK: false, Kind: in.Kind, Message: fmt.Sprintf("createCategory HTTP %d: %s", cres.StatusCode, msg)}
			}
		}
	}

	cat := url.Values{}
	cat.Set("hashes", in.Hash)
	cat.Set("category", in.Label)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, in.URL+"/api/v2/torrents/setCategory", strings.NewReader(cat.Encode()))
	if err != nil {
		return LabelResult{OK: false, Kind: in.Kind, Message: err.Error()}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := client.Do(req)
	if err != nil {
		return LabelResult{OK: false, Kind: in.Kind, Message: err.Error()}
	}
	respBody, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	_ = res.Body.Close()
	msg := strings.TrimSpace(string(respBody))
	if res.StatusCode >= 400 {
		return LabelResult{OK: false, Kind: in.Kind, Message: fmt.Sprintf("HTTP %d: %s", res.StatusCode, msg)}
	}
	return LabelResult{OK: true, Kind: in.Kind, Message: "ok"}
}

func setDelugeLabel(ctx context.Context, in LabelInput) LabelResult {
	rpc := newDelugeCaller(in.URL, addHTTPClient().Timeout)
	if err := rpc.login(ctx, delugePassword(in.Username, in.Password)); err != nil {
		return LabelResult{OK: false, Kind: in.Kind, Message: err.Error()}
	}
	if _, err := rpc.ensureDaemon(ctx); err != nil {
		return LabelResult{OK: false, Kind: in.Kind, Message: err.Error()}
	}
	hash := strings.ToLower(in.Hash)
	if in.Label != "" {
		// Ensure the label exists; ignore "already exists" style errors.
		if _, err := rpc.call(ctx, "label.add", []any{in.Label}, 20); err != nil {
			lower := strings.ToLower(err.Error())
			if !strings.Contains(lower, "already") && !strings.Contains(lower, "exist") {
				return LabelResult{OK: false, Kind: in.Kind, Message: err.Error()}
			}
		}
	}
	if _, err := rpc.call(ctx, "label.set_torrent", []any{hash, in.Label}, 21); err != nil {
		return LabelResult{OK: false, Kind: in.Kind, Message: err.Error()}
	}
	return LabelResult{OK: true, Kind: in.Kind, Message: "ok"}
}

// setTransmissionLabel writes torrent labels via torrent-set (Transmission 3+/4).
// Empty Fleet label clears labels to [].
func setTransmissionLabel(ctx context.Context, in LabelInput) LabelResult {
	labels := []string{}
	if in.Label != "" {
		labels = []string{in.Label}
	}
	payload, _ := json.Marshal(map[string]any{
		"method": "torrent-set",
		"arguments": map[string]any{
			"ids":    []string{strings.ToLower(in.Hash)},
			"labels": labels,
		},
	})
	client := addHTTPClient()
	var lastMsg string
	for _, endpoint := range transmissionRPCCandidates(in.URL) {
		res := transmissionRPCCall(ctx, client, endpoint, in.Username, in.Password, payload)
		if res.OK {
			return LabelResult{OK: true, Kind: in.Kind, Message: "ok"}
		}
		lastMsg = res.Message
	}
	if lastMsg == "" {
		lastMsg = "transmission label write failed"
	}
	return LabelResult{OK: false, Kind: in.Kind, Message: lastMsg}
}
