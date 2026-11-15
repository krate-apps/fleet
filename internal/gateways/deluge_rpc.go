package gateways

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// delugeCaller is a session-aware Deluge WebUI JSON-RPC helper.
type delugeCaller struct {
	client       *http.Client
	endpoint     string
	cookieHeader string
}

func newDelugeCaller(baseURL string, timeout time.Duration) *delugeCaller {
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	return &delugeCaller{
		client:   &http.Client{Timeout: timeout},
		endpoint: strings.TrimRight(baseURL, "/") + "/json",
	}
}

func (d *delugeCaller) call(ctx context.Context, method string, params []any, id int) (json.RawMessage, error) {
	reqBody, _ := json.Marshal(map[string]any{
		"method": method,
		"params": params,
		"id":     id,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if d.cookieHeader != "" {
		req.Header.Set("Cookie", d.cookieHeader)
	}
	res, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d", res.StatusCode)
	}
	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("invalid JSON response")
	}
	if envelope.Error != nil && envelope.Error.Message != "" {
		return nil, fmt.Errorf("%s", envelope.Error.Message)
	}
	if cookies := res.Cookies(); len(cookies) > 0 {
		if joined := joinCookies(cookies); joined != "" {
			d.cookieHeader = joined
		}
	}
	return envelope.Result, nil
}

func (d *delugeCaller) login(ctx context.Context, password string) error {
	result, err := d.call(ctx, "auth.login", []any{password}, 1)
	if err != nil {
		return err
	}
	if string(result) == "false" {
		return fmt.Errorf("login failed")
	}
	return nil
}

func (d *delugeCaller) daemonVersion(ctx context.Context) (string, error) {
	info, err := d.call(ctx, "daemon.get_version", []any{}, 10)
	if err != nil {
		info, err = d.call(ctx, "daemon.info", []any{}, 11)
	}
	if err != nil {
		return "", err
	}
	return strings.Trim(string(info), `"`), nil
}

func (d *delugeCaller) connected(ctx context.Context) (bool, error) {
	raw, err := d.call(ctx, "web.connected", []any{}, 6)
	if err != nil {
		return false, err
	}
	return string(raw) == "true", nil
}

// ensureDaemon connects Deluge WebUI to a daemon host when needed.
// Returns daemon version on success. Connect timeouts surface a clear message
// (wedged deluge-web often hangs on web.connect / web.disconnect).
func (d *delugeCaller) ensureDaemon(ctx context.Context) (string, error) {
	if ver, err := d.daemonVersion(ctx); err == nil && ver != "" {
		return ver, nil
	}

	if ok, err := d.connected(ctx); err == nil && ok {
		if ver, verr := d.daemonVersion(ctx); verr == nil && ver != "" {
			return ver, nil
		}
	}

	hostsRaw, err := d.call(ctx, "web.get_hosts", []any{}, 3)
	if err != nil {
		return "", fmt.Errorf("web UI reachable; cannot list daemons: %w", err)
	}
	var hosts [][]any
	if err := json.Unmarshal(hostsRaw, &hosts); err != nil || len(hosts) == 0 {
		return "", fmt.Errorf("web UI reachable; no daemon hosts configured")
	}

	var last error
	for i, host := range hosts {
		id, ok := host[0].(string)
		if !ok || id == "" {
			continue
		}
		label := id
		if len(host) >= 3 {
			label = fmt.Sprintf("%v:%v", host[1], host[2])
		}
		connectCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
		_, cerr := d.call(connectCtx, "web.connect", []any{id}, 100+i)
		cancel()
		if cerr != nil {
			last = fmt.Errorf("connect %s: %w", label, classifyDelugeTimeout(cerr))
			continue
		}
		ver, verr := d.daemonVersion(ctx)
		if verr != nil {
			last = fmt.Errorf("connected to %s but daemon RPC failed: %w", label, classifyDelugeTimeout(verr))
			continue
		}
		if ver != "" {
			return ver, nil
		}
	}
	if last != nil {
		return "", fmt.Errorf("web UI reachable; daemon not connected (%v)", last)
	}
	return "", fmt.Errorf("web UI reachable; daemon not connected")
}

func classifyDelugeTimeout(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if strings.Contains(msg, "context deadline exceeded") ||
		strings.Contains(msg, "Client.Timeout") ||
		strings.Contains(msg, "timeout") {
		return fmt.Errorf("timed out — Deluge web may be wedged; restart deluge-web / deluge@user")
	}
	return err
}

func delugePassword(username, password string) string {
	if password != "" {
		return password
	}
	return username
}
