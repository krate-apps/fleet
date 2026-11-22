package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type QueueItem struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	BytesDone  int64  `json:"bytesDone"`
	BytesTotal int64  `json:"bytesTotal"`
	Speed      int64  `json:"speed"`
	EtaSec     int64  `json:"etaSec"`
	Kind       string `json:"kind"`
	Enabled    bool   `json:"enabled"`
}

type pyClient struct {
	mu         sync.Mutex
	base       *url.URL
	username   string
	password   string
	apiKey     string // pyLoad 0.5+ X-API-Key (pl_…)
	httpClient *http.Client
	loggedIn   bool
}

func newPyClient(baseURL, username, password string) (*pyClient, error) {
	u, err := validatePrivateBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	pass := strings.TrimSpace(password)
	cli := &pyClient{
		base:     u,
		username: username,
		password: pass,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
			Jar:     jar,
		},
	}
	if strings.HasPrefix(pass, "pl_") {
		cli.apiKey = pass
	}
	return cli, nil
}

func (c *pyClient) login() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.loginLocked()
}

func (c *pyClient) loginLocked() error {
	if c.apiKey != "" {
		// Validate key with a cheap authenticated call.
		req, err := http.NewRequest(http.MethodGet, joinAPI(c.base, "/api/getServerVersion"), nil)
		if err != nil {
			return err
		}
		req.Header.Set("X-API-Key", c.apiKey)
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if resp.StatusCode >= 400 {
			return fmt.Errorf("pyLoad API key HTTP %d: %s", resp.StatusCode, truncate(string(body), 160))
		}
		c.loggedIn = true
		return nil
	}
	form := url.Values{}
	form.Set("username", c.username)
	form.Set("password", c.password)
	req, err := http.NewRequest(http.MethodPost, joinAPI(c.base, "/api/login"), strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		return fmt.Errorf("pyLoad login HTTP %d: %s", resp.StatusCode, truncate(string(body), 160))
	}
	trim := strings.TrimSpace(string(body))
	if trim == "false" || trim == `"false"` || strings.Contains(trim, "Obsolete API") {
		return fmt.Errorf("pyLoad login failed (use a pl_ API key for pyLoad 0.5+, or check credentials)")
	}
	// Some builds return true / "true" / JSON true.
	c.loggedIn = true
	return nil
}

func (c *pyClient) applyAuth(req *http.Request) {
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}
}

func (c *pyClient) ensureLogin() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.loggedIn {
		return nil
	}
	return c.loginLocked()
}

func (c *pyClient) getJSON(path string) (json.RawMessage, error) {
	if err := c.ensureLogin(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	req, err := http.NewRequest(http.MethodGet, joinAPI(c.base, path), nil)
	if err != nil {
		return nil, err
	}
	c.applyAuth(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		c.loggedIn = false
		return nil, fmt.Errorf("pyLoad auth expired")
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("pyLoad HTTP %d: %s", resp.StatusCode, truncate(string(body), 160))
	}
	return json.RawMessage(body), nil
}

func (c *pyClient) postJSON(path string, payload any) (json.RawMessage, error) {
	if err := c.ensureLogin(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	var bodyReader io.Reader
	contentType := "application/json"
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		bodyReader = strings.NewReader(string(b))
	}
	req, err := http.NewRequest(http.MethodPost, joinAPI(c.base, path), bodyReader)
	if err != nil {
		return nil, err
	}
	c.applyAuth(req)
	if payload != nil {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("pyLoad HTTP %d: %s", resp.StatusCode, truncate(string(body), 160))
	}
	return json.RawMessage(body), nil
}

// callRPC posts form-style api method used by classic pyLoad: /api/<method>
func (c *pyClient) callForm(path string, form url.Values) (json.RawMessage, error) {
	if err := c.ensureLogin(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	req, err := http.NewRequest(http.MethodPost, joinAPI(c.base, path), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	c.applyAuth(req)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("pyLoad HTTP %d: %s", resp.StatusCode, truncate(string(body), 160))
	}
	return json.RawMessage(body), nil
}

func normalizePyStatus(raw json.RawMessage) ([]QueueItem, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return []QueueItem{}, nil
	}
	// statusDownloads returns a list of dicts (classic) or map id→obj.
	var asList []map[string]any
	if err := json.Unmarshal(raw, &asList); err == nil {
		return mapsToItems(asList), nil
	}
	var asMap map[string]map[string]any
	if err := json.Unmarshal(raw, &asMap); err == nil {
		out := make([]map[string]any, 0, len(asMap))
		for id, m := range asMap {
			if _, ok := m["id"]; !ok {
				m["id"] = id
			}
			out = append(out, m)
		}
		return mapsToItems(out), nil
	}
	var asAnyMap map[string]any
	if err := json.Unmarshal(raw, &asAnyMap); err == nil {
		out := make([]map[string]any, 0, len(asAnyMap))
		for id, v := range asAnyMap {
			m, ok := v.(map[string]any)
			if !ok {
				continue
			}
			if _, ok := m["id"]; !ok {
				m["id"] = id
			}
			out = append(out, m)
		}
		return mapsToItems(out), nil
	}
	return nil, fmt.Errorf("statusDownloads decode failed")
}

func mapsToItems(rows []map[string]any) []QueueItem {
	out := make([]QueueItem, 0, len(rows))
	for _, m := range rows {
		id := anyString(m["id"])
		if id == "" {
			id = anyString(m["fid"])
		}
		if id == "" {
			continue
		}
		name := anyString(m["name"])
		if name == "" {
			name = anyString(m["filename"])
		}
		status := anyString(m["statusmsg"])
		if status == "" {
			status = anyString(m["status"])
		}
		if status == "" {
			status = anyString(m["format_status"])
		}
		done := anyInt64(m["size"]) - anyInt64(m["bleft"])
		if v, ok := m["bytesLoaded"]; ok {
			done = anyInt64(v)
		}
		total := anyInt64(m["size"])
		if v, ok := m["bytesTotal"]; ok {
			total = anyInt64(v)
		}
		if total == 0 {
			total = anyInt64(m["totalsize"])
		}
		speed := anyInt64(m["speed"])
		eta := anyInt64(m["eta"])
		if eta < 0 {
			eta = 0
		}
		out = append(out, QueueItem{
			ID:         id,
			Name:       name,
			Status:     status,
			BytesDone:  done,
			BytesTotal: total,
			Speed:      speed,
			EtaSec:     eta,
			Kind:       "download",
			Enabled:    true,
		})
	}
	return out
}

func anyString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case float64:
		return strconv.FormatInt(int64(t), 10)
	case json.Number:
		return t.String()
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

func anyInt64(v any) int64 {
	switch t := v.(type) {
	case nil:
		return 0
	case float64:
		return int64(t)
	case int64:
		return t
	case int:
		return int64(t)
	case json.Number:
		n, _ := t.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
		return n
	default:
		n, _ := strconv.ParseInt(fmt.Sprint(t), 10, 64)
		return n
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
