package extensions

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// HostCookieEntry maps a host to a Cookie header fragment (ruTorrent cookies parity).
type HostCookieEntry struct {
	Host    string `json:"host"`
	Cookies string `json:"cookies"`
}

// CookieStore persists host→cookie mappings outside per-extension storage so
// fleet.http.fetch and org.fleet.cookies share them.
type CookieStore struct {
	DataDir string

	mu sync.Mutex
}

func (s *CookieStore) path() (string, error) {
	if s == nil || strings.TrimSpace(s.DataDir) == "" {
		return "", fmt.Errorf("cookie store unavailable")
	}
	dir := filepath.Join(s.DataDir, "cookies")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "hosts.json"), nil
}

// List returns the current host cookie entries.
func (s *CookieStore) List() ([]HostCookieEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

// Set replaces the jar after validating hosts and cookie fragments.
func (s *CookieStore) Set(entries []HostCookieEntry) error {
	if entries == nil {
		entries = []HostCookieEntry{}
	}
	if len(entries) > 200 {
		return fmt.Errorf("too many hosts (max 200)")
	}
	cleaned := make([]HostCookieEntry, 0, len(entries))
	seen := map[string]struct{}{}
	for _, e := range entries {
		host := normalizeCookieHost(e.Host)
		cookies := normalizeCookieHeader(e.Cookies)
		if host == "" {
			return fmt.Errorf("host required")
		}
		if cookies == "" {
			continue
		}
		if _, dup := seen[host]; dup {
			return fmt.Errorf("duplicate host %q", host)
		}
		seen[host] = struct{}{}
		cleaned = append(cleaned, HostCookieEntry{Host: host, Cookies: cookies})
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.path()
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(cleaned, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// Upsert merges cookies for host into the jar (new names overwrite).
func (s *CookieStore) Upsert(host, cookies string) error {
	host = normalizeCookieHost(host)
	cookies = normalizeCookieHeader(cookies)
	if host == "" {
		return fmt.Errorf("host required")
	}
	if cookies == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := s.loadLocked()
	if err != nil {
		return err
	}
	merged := cookieMap(cookies)
	found := false
	for i, e := range entries {
		if e.Host != host {
			continue
		}
		cur := cookieMap(e.Cookies)
		for k, v := range merged {
			cur[k] = v
		}
		entries[i].Cookies = cookieHeader(cur)
		found = true
		break
	}
	if !found {
		entries = append(entries, HostCookieEntry{Host: host, Cookies: cookies})
	}
	if len(entries) > 200 {
		return fmt.Errorf("too many hosts (max 200)")
	}
	p, err := s.path()
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// HeaderForURL returns the Cookie header value for rawURL's host, if any.
func (s *CookieStore) HeaderForURL(rawURL string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Host == "" {
		return "", nil
	}
	host := normalizeCookieHost(u.Hostname())
	if host == "" {
		return "", nil
	}
	s.mu.Lock()
	entries, err := s.loadLocked()
	s.mu.Unlock()
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.Host == host {
			return e.Cookies, nil
		}
	}
	return "", nil
}

func (s *CookieStore) loadLocked() ([]HostCookieEntry, error) {
	p, err := s.path()
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return []HostCookieEntry{}, nil
		}
		return nil, err
	}
	var entries []HostCookieEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return []HostCookieEntry{}, nil
	}
	if entries == nil {
		entries = []HostCookieEntry{}
	}
	return entries, nil
}

func normalizeCookieHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	host = strings.TrimPrefix(host, ".")
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	return host
}

func normalizeCookieHeader(raw string) string {
	return cookieHeader(cookieMap(raw))
}

func cookieMap(raw string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(raw, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, val, ok := strings.Cut(part, "=")
		name = strings.TrimSpace(name)
		val = strings.TrimSpace(val)
		if !ok || name == "" || val == "" {
			continue
		}
		if strings.ContainsAny(name, "\r\n;") || strings.ContainsAny(val, "\r\n;") {
			continue
		}
		out[name] = val
	}
	return out
}

func cookieHeader(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, name+"="+m[name])
	}
	return strings.Join(parts, "; ")
}
