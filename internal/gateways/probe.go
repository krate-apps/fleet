package gateways

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/krate-apps/fleet/internal/rtorrent"
)

// Kind identifies a supported BitTorrent client endpoint.
type Kind string

const (
	KindRTorrent     Kind = "rtorrent"
	KindDeluge       Kind = "deluge"
	KindTransmission Kind = "transmission"
	KindQBittorrent  Kind = "qbittorrent"
)

// ProbeInput is the connection material for a one-shot health check.
type ProbeInput struct {
	Kind     Kind
	URL      string
	Username string
	Password string
}

// ProbeResult is a non-authoritative health snapshot for Settings.
type ProbeResult struct {
	OK      bool   `json:"ok"`
	Kind    Kind   `json:"kind"`
	Version string `json:"version,omitempty"`
	Message string `json:"message,omitempty"`
}

// Probe talks to a client endpoint without mutating torrents.
func Probe(ctx context.Context, in ProbeInput) ProbeResult {
	in.Kind = Kind(strings.ToLower(string(in.Kind)))
	in.URL = strings.TrimSpace(in.URL)
	if in.Kind != KindRTorrent {
		in.URL = strings.TrimRight(in.URL, "/")
	}
	if in.URL == "" {
		return ProbeResult{OK: false, Kind: in.Kind, Message: "url required"}
	}
	switch in.Kind {
	case KindRTorrent:
		return probeRTorrent(ctx, in)
	case KindQBittorrent:
		return probeQBittorrent(ctx, in)
	case KindTransmission:
		return probeTransmission(ctx, in)
	case KindDeluge:
		return probeDeluge(ctx, in)
	default:
		return ProbeResult{OK: false, Kind: in.Kind, Message: "unsupported kind"}
	}
}

func probeRTorrent(ctx context.Context, in ProbeInput) ProbeResult {
	client, err := rtorrent.NewClient(in.URL)
	if err != nil {
		return ProbeResult{OK: false, Kind: KindRTorrent, Message: err.Error()}
	}
	ver, err := client.CallString(ctx, "system.client_version")
	if err != nil {
		return ProbeResult{OK: false, Kind: KindRTorrent, Message: err.Error()}
	}
	return ProbeResult{OK: true, Kind: KindRTorrent, Version: ver, Message: "connected"}
}

func httpClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Timeout: 8 * time.Second,
		Jar:     jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
}

func probeQBittorrent(ctx context.Context, in ProbeInput) ProbeResult {
	client := httpClient()
	form := url.Values{}
	form.Set("username", in.Username)
	form.Set("password", in.Password)
	loginReq, err := http.NewRequestWithContext(ctx, http.MethodPost, in.URL+"/api/v2/auth/login", strings.NewReader(form.Encode()))
	if err != nil {
		return ProbeResult{OK: false, Kind: in.Kind, Message: err.Error()}
	}
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginRes, err := client.Do(loginReq)
	if err != nil {
		return ProbeResult{OK: false, Kind: in.Kind, Message: err.Error()}
	}
	body, _ := io.ReadAll(io.LimitReader(loginRes.Body, 4096))
	_ = loginRes.Body.Close()
	if loginRes.StatusCode >= 400 {
		return ProbeResult{OK: false, Kind: in.Kind, Message: fmt.Sprintf("login HTTP %d", loginRes.StatusCode)}
	}
	if strings.TrimSpace(string(body)) == "Fails." {
		return ProbeResult{OK: false, Kind: in.Kind, Message: "login failed"}
	}

	verReq, err := http.NewRequestWithContext(ctx, http.MethodGet, in.URL+"/api/v2/app/version", nil)
	if err != nil {
		return ProbeResult{OK: false, Kind: in.Kind, Message: err.Error()}
	}
	verRes, err := client.Do(verReq)
	if err != nil {
		return ProbeResult{OK: false, Kind: in.Kind, Message: err.Error()}
	}
	verBody, _ := io.ReadAll(io.LimitReader(verRes.Body, 4096))
	_ = verRes.Body.Close()
	if verRes.StatusCode >= 400 {
		return ProbeResult{OK: false, Kind: in.Kind, Message: fmt.Sprintf("version HTTP %d", verRes.StatusCode)}
	}
	version := strings.TrimSpace(string(verBody))
	return ProbeResult{OK: true, Kind: in.Kind, Version: version, Message: "connected"}
}

func probeTransmission(ctx context.Context, in ProbeInput) ProbeResult {
	client := httpClient()
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	payload := []byte(`{"method":"session-get","arguments":{"fields":["version"]}}`)
	candidates := transmissionRPCCandidates(in.URL)

	var lastMsg string
	for _, endpoint := range candidates {
		res := probeTransmissionEndpoint(ctx, client, endpoint, in.Username, in.Password, payload)
		if res.OK {
			return res
		}
		lastMsg = res.Message
	}
	if lastMsg == "" {
		lastMsg = "no working RPC endpoint"
	}
	return ProbeResult{OK: false, Kind: in.Kind, Message: lastMsg}
}

func transmissionRPCCandidates(base string) []string {
	base = strings.TrimRight(base, "/")
	out := []string{}
	seen := map[string]bool{}
	add := func(u string) {
		if u == "" || seen[u] {
			return
		}
		seen[u] = true
		out = append(out, u)
	}
	lower := strings.ToLower(base)
	if strings.Contains(lower, "/transmission") {
		add(base + "/rpc")
	}
	add(base + "/transmission/rpc")
	add(base + "/rpc")
	return out
}

func probeTransmissionEndpoint(ctx context.Context, client *http.Client, endpoint, username, password string, payload []byte) ProbeResult {
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
		return ProbeResult{OK: false, Kind: KindTransmission, Message: err.Error()}
	}
	if res.StatusCode == http.StatusConflict {
		sid := res.Header.Get("X-Transmission-Session-Id")
		res, body, err = do(sid)
		if err != nil {
			return ProbeResult{OK: false, Kind: KindTransmission, Message: err.Error()}
		}
	}
	if res.StatusCode == http.StatusUnauthorized {
		return ProbeResult{OK: false, Kind: KindTransmission, Message: "unauthorized"}
	}
	if res.StatusCode >= 300 {
		return ProbeResult{OK: false, Kind: KindTransmission, Message: fmt.Sprintf("HTTP %d", res.StatusCode)}
	}
	var parsed struct {
		Result    string `json:"result"`
		Arguments struct {
			Version string `json:"version"`
		} `json:"arguments"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ProbeResult{OK: false, Kind: KindTransmission, Message: "invalid JSON response"}
	}
	if parsed.Result != "" && parsed.Result != "success" {
		return ProbeResult{OK: false, Kind: KindTransmission, Message: parsed.Result}
	}
	return ProbeResult{OK: true, Kind: KindTransmission, Version: parsed.Arguments.Version, Message: "connected"}
}

func probeDeluge(ctx context.Context, in ProbeInput) ProbeResult {
	// Deluge may set _session_id with Path=/trevor/deluge/ while JSON lives at /json.
	// Carry cookies manually so Path mismatches do not drop the session.
	rpc := newDelugeCaller(in.URL, 20*time.Second)
	if err := rpc.login(ctx, delugePassword(in.Username, in.Password)); err != nil {
		return ProbeResult{OK: false, Kind: in.Kind, Message: err.Error()}
	}
	version, err := rpc.ensureDaemon(ctx)
	if err != nil {
		return ProbeResult{OK: false, Kind: in.Kind, Message: err.Error()}
	}
	return ProbeResult{OK: true, Kind: in.Kind, Version: version, Message: "connected"}
}

func joinCookies(cookies []*http.Cookie) string {
	if len(cookies) == 0 {
		return ""
	}
	parts := make([]string, 0, len(cookies))
	for _, c := range cookies {
		if c == nil || c.Name == "" {
			continue
		}
		parts = append(parts, c.Name+"="+c.Value)
	}
	return strings.Join(parts, "; ")
}
