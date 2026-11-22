package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

// localJDClient talks to the deprecated-but-still-used JD2 Remote API over plain HTTP.
type localJDClient struct {
	mu     sync.Mutex
	base   *url.URL
	http   *http.Client
	rid    int64
}

func newLocalJD(baseURL string) (*localJDClient, error) {
	u, err := validatePrivateBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	return &localJDClient{
		base: u,
		http: &http.Client{Timeout: 15 * time.Second},
		rid:  time.Now().UnixMilli(),
	}, nil
}

func (c *localJDClient) nextRID() int64 {
	return atomic.AddInt64(&c.rid, 1)
}

func (c *localJDClient) ping() error {
	req, err := http.NewRequest(http.MethodGet, joinAPI(c.base, "/jdcheck.js"), nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode >= 400 {
		return fmt.Errorf("local JD HTTP %d on /jdcheck.js", resp.StatusCode)
	}
	return nil
}

func (c *localJDClient) action(path string, params []any) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	payload := map[string]any{
		"apiVer": 1,
		"url":    path,
		"params": adaptParams(params),
		"rid":    c.nextRID(),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, joinAPI(c.base, path), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("local JD HTTP %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		// Some RemoteAPI builds return the payload directly.
		return json.RawMessage(raw), nil
	}
	if len(envelope.Data) == 0 {
		return json.RawMessage(raw), nil
	}
	return envelope.Data, nil
}
