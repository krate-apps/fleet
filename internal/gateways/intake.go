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
	"net/url"
	"strings"

	"github.com/krate-apps/fleet/internal/rtorrent"
)

// IntakeInput adds a new download to a client (magnets and/or .torrent).
// Unlike AddTorrent (cross-seed), SavePath is optional and uses the client default when empty.
type IntakeInput struct {
	Kind     Kind
	URL      string
	Username string
	Password string
	Filename string
	Torrent  []byte
	Magnets  []string
	SavePath string
	Start    bool
}

// IntakeDownload pushes magnets and/or a .torrent file to a remote client.
func IntakeDownload(ctx context.Context, in IntakeInput) AddResult {
	in.Kind = Kind(strings.ToLower(string(in.Kind)))
	in.URL = strings.TrimSpace(in.URL)
	if in.Kind != KindRTorrent {
		in.URL = strings.TrimRight(in.URL, "/")
	}
	if in.URL == "" {
		return AddResult{OK: false, Kind: in.Kind, Message: "url required"}
	}
	if len(in.Torrent) == 0 && len(in.Magnets) == 0 {
		return AddResult{OK: false, Kind: in.Kind, Message: "magnets or torrent required"}
	}
	if in.Filename == "" {
		in.Filename = "download.torrent"
	}
	switch in.Kind {
	case KindRTorrent:
		return intakeRTorrent(ctx, in)
	case KindQBittorrent:
		return intakeQBittorrent(ctx, in)
	case KindTransmission:
		return intakeTransmission(ctx, in)
	case KindDeluge:
		return intakeDeluge(ctx, in)
	default:
		return AddResult{OK: false, Kind: in.Kind, Message: "unsupported kind"}
	}
}

func intakeRTorrent(ctx context.Context, in IntakeInput) AddResult {
	client, err := rtorrent.NewClient(in.URL)
	if err != nil {
		return AddResult{OK: false, Kind: KindRTorrent, Message: err.Error()}
	}
	if len(in.Magnets) > 0 {
		if err := client.AddMagnet(ctx, in.Magnets, in.Start); err != nil {
			return AddResult{OK: false, Kind: KindRTorrent, Message: err.Error()}
		}
	}
	if len(in.Torrent) > 0 {
		if err := client.AddTorrentFile(ctx, in.Torrent, in.Start); err != nil {
			return AddResult{OK: false, Kind: KindRTorrent, Message: err.Error()}
		}
	}
	return AddResult{OK: true, Kind: KindRTorrent, Message: "added"}
}

func intakeQBittorrent(ctx context.Context, in IntakeInput) AddResult {
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
	if len(in.Magnets) > 0 {
		_ = w.WriteField("urls", strings.Join(in.Magnets, "\n"))
	}
	if strings.TrimSpace(in.SavePath) != "" {
		_ = w.WriteField("savepath", in.SavePath)
	}
	if !in.Start {
		_ = w.WriteField("paused", "true")
	} else {
		_ = w.WriteField("paused", "false")
	}
	if len(in.Torrent) > 0 {
		part, err := w.CreateFormFile("torrents", in.Filename)
		if err != nil {
			return AddResult{OK: false, Kind: in.Kind, Message: err.Error()}
		}
		if _, err := part.Write(in.Torrent); err != nil {
			return AddResult{OK: false, Kind: in.Kind, Message: err.Error()}
		}
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
		return AddResult{OK: false, Kind: in.Kind, Message: fmt.Sprintf("HTTP %d: %s", res.StatusCode, msg)}
	}
	return AddResult{OK: true, Kind: in.Kind, Message: "added"}
}

func intakeTransmission(ctx context.Context, in IntakeInput) AddResult {
	client := addHTTPClient()
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	var lastMsg string
	addOne := func(args map[string]any) AddResult {
		payload, _ := json.Marshal(map[string]any{"method": "torrent-add", "arguments": args})
		for _, endpoint := range transmissionRPCCandidates(in.URL) {
			res := addTransmissionEndpoint(ctx, client, endpoint, in.Username, in.Password, payload, true)
			if res.OK || res.Skipped {
				return res
			}
			lastMsg = res.Message
		}
		return AddResult{OK: false, Kind: KindTransmission, Message: lastMsg}
	}

	paused := !in.Start
	if len(in.Torrent) > 0 {
		args := map[string]any{
			"metainfo": base64.StdEncoding.EncodeToString(in.Torrent),
			"paused":   paused,
		}
		if sp := strings.TrimSpace(in.SavePath); sp != "" {
			args["download-dir"] = sp
		}
		if res := addOne(args); !res.OK {
			return res
		}
	}
	for _, magnet := range in.Magnets {
		magnet = strings.TrimSpace(magnet)
		if magnet == "" {
			continue
		}
		args := map[string]any{
			"filename": magnet,
			"paused":   paused,
		}
		if sp := strings.TrimSpace(in.SavePath); sp != "" {
			args["download-dir"] = sp
		}
		if res := addOne(args); !res.OK {
			return res
		}
	}
	return AddResult{OK: true, Kind: KindTransmission, Message: "added"}
}

func intakeDeluge(ctx context.Context, in IntakeInput) AddResult {
	rpc := newDelugeCaller(in.URL, addHTTPClient().Timeout)
	if err := rpc.login(ctx, delugePassword(in.Username, in.Password)); err != nil {
		return AddResult{OK: false, Kind: in.Kind, Message: err.Error()}
	}
	if _, err := rpc.ensureDaemon(ctx); err != nil {
		return AddResult{OK: false, Kind: in.Kind, Message: err.Error()}
	}

	options := map[string]any{
		"add_paused": !in.Start,
	}
	if sp := strings.TrimSpace(in.SavePath); sp != "" {
		options["download_location"] = sp
	}

	if len(in.Torrent) > 0 {
		meta := base64.StdEncoding.EncodeToString(in.Torrent)
		if _, err := rpc.call(ctx, "core.add_torrent_file", []any{in.Filename, meta, options}, 4); err != nil {
			return AddResult{OK: false, Kind: in.Kind, Message: err.Error()}
		}
	}
	for i, magnet := range in.Magnets {
		magnet = strings.TrimSpace(magnet)
		if magnet == "" {
			continue
		}
		if _, err := rpc.call(ctx, "core.add_torrent_magnet", []any{magnet, options}, 5+i); err != nil {
			return AddResult{OK: false, Kind: in.Kind, Message: err.Error()}
		}
	}
	return AddResult{OK: true, Kind: in.Kind, Message: "added"}
}
