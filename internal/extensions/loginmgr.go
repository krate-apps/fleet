package extensions

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// LoginAccount is a configurable site credential (ruTorrent loginmgr parity,
// without shipping per-tracker scrapers).
type LoginAccount struct {
	Name           string `json:"name"`
	Origin         string `json:"origin"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	LoginPath      string `json:"loginPath"`
	UsernameField  string `json:"usernameField,omitempty"`
	PasswordField  string `json:"passwordField,omitempty"`
	ExtraForm      string `json:"extraForm,omitempty"`
	WarmupGET      bool   `json:"warmupGet"`
	Enabled        bool   `json:"enabled"`
}

// LoginStore persists site accounts and can perform a generic form login that
// upserts session cookies into CookieStore.
type LoginStore struct {
	DataDir string
	Cookies *CookieStore
	HTTP    *http.Client

	mu sync.Mutex
}

func (s *LoginStore) path() (string, error) {
	if s == nil || strings.TrimSpace(s.DataDir) == "" {
		return "", fmt.Errorf("loginmgr store unavailable")
	}
	dir := filepath.Join(s.DataDir, "loginmgr")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "accounts.json"), nil
}

// List returns configured accounts.
func (s *LoginStore) List() ([]LoginAccount, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

// Set replaces the account list after validation.
func (s *LoginStore) Set(accounts []LoginAccount) error {
	if accounts == nil {
		accounts = []LoginAccount{}
	}
	if len(accounts) > 50 {
		return fmt.Errorf("too many accounts (max 50)")
	}
	cleaned := make([]LoginAccount, 0, len(accounts))
	seen := map[string]struct{}{}
	for _, a := range accounts {
		name := strings.TrimSpace(a.Name)
		if name == "" {
			return fmt.Errorf("account name required")
		}
		if len(name) > 80 {
			return fmt.Errorf("account name too long")
		}
		key := strings.ToLower(name)
		if _, dup := seen[key]; dup {
			return fmt.Errorf("duplicate account name %q", name)
		}
		seen[key] = struct{}{}
		origin, err := normalizeOrigin(a.Origin)
		if err != nil {
			return fmt.Errorf("account %q: %w", name, err)
		}
		user := strings.TrimSpace(a.Username)
		pass := a.Password
		if user == "" || strings.TrimSpace(pass) == "" {
			return fmt.Errorf("account %q: username and password required", name)
		}
		loginPath := strings.TrimSpace(a.LoginPath)
		if loginPath == "" {
			loginPath = "/login"
		}
		if !strings.HasPrefix(loginPath, "/") {
			return fmt.Errorf("account %q: loginPath must start with /", name)
		}
		userField := strings.TrimSpace(a.UsernameField)
		if userField == "" {
			userField = "username"
		}
		passField := strings.TrimSpace(a.PasswordField)
		if passField == "" {
			passField = "password"
		}
		cleaned = append(cleaned, LoginAccount{
			Name:          name,
			Origin:        origin,
			Username:      user,
			Password:      pass,
			LoginPath:     loginPath,
			UsernameField: userField,
			PasswordField: passField,
			ExtraForm:     strings.TrimSpace(a.ExtraForm),
			WarmupGET:     a.WarmupGET,
			Enabled:       a.Enabled,
		})
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

// Login performs a form POST for the named account and upserts cookies.
func (s *LoginStore) Login(ctx context.Context, name string) (host string, cookieCount int, err error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", 0, fmt.Errorf("account name required")
	}
	s.mu.Lock()
	accounts, err := s.loadLocked()
	s.mu.Unlock()
	if err != nil {
		return "", 0, err
	}
	var acct *LoginAccount
	for i := range accounts {
		if strings.EqualFold(accounts[i].Name, name) {
			acct = &accounts[i]
			break
		}
	}
	if acct == nil {
		return "", 0, fmt.Errorf("account %q not found", name)
	}
	if !acct.Enabled {
		return "", 0, fmt.Errorf("account %q is disabled", name)
	}
	if s.Cookies == nil {
		return "", 0, fmt.Errorf("cookie store unavailable")
	}

	originURL, err := url.Parse(acct.Origin)
	if err != nil || originURL.Host == "" {
		return "", 0, fmt.Errorf("invalid origin")
	}
	loginURL := strings.TrimRight(acct.Origin, "/") + acct.LoginPath

	jar, err := cookiejar.New(nil)
	if err != nil {
		return "", 0, err
	}
	client := s.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	// Clone with jar; do not mutate a shared client.
	client = &http.Client{
		Timeout: client.Timeout,
		Jar:     jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	if acct.WarmupGET {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, acct.Origin+"/", nil)
		if err != nil {
			return "", 0, err
		}
		req.Header.Set("User-Agent", "Fleet-Extension/1.0")
		res, err := client.Do(req)
		if err != nil {
			return "", 0, err
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 1<<20))
		_ = res.Body.Close()
	}

	form := url.Values{}
	form.Set(acct.UsernameField, acct.Username)
	form.Set(acct.PasswordField, acct.Password)
	if extra := strings.TrimSpace(acct.ExtraForm); extra != "" {
		extraValues, err := url.ParseQuery(extra)
		if err != nil {
			return "", 0, fmt.Errorf("invalid extraForm: %w", err)
		}
		for k, vals := range extraValues {
			for _, v := range vals {
				form.Add(k, v)
			}
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, loginURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Fleet-Extension/1.0")
	req.Header.Set("Referer", acct.Origin+"/")
	res, err := client.Do(req)
	if err != nil {
		return "", 0, err
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 1<<20))
	_ = res.Body.Close()

	host = normalizeCookieHost(originURL.Hostname())
	cookies := jar.Cookies(originURL)
	if len(cookies) == 0 {
		return host, 0, fmt.Errorf("login produced no cookies (HTTP %d)", res.StatusCode)
	}
	parts := make([]string, 0, len(cookies))
	for _, c := range cookies {
		if c == nil || c.Name == "" {
			continue
		}
		parts = append(parts, c.Name+"="+c.Value)
	}
	header := normalizeCookieHeader(strings.Join(parts, "; "))
	if header == "" {
		return host, 0, fmt.Errorf("login produced no usable cookies")
	}
	if err := s.Cookies.Upsert(host, header); err != nil {
		return host, 0, err
	}
	return host, len(cookieMap(header)), nil
}

func (s *LoginStore) loadLocked() ([]LoginAccount, error) {
	p, err := s.path()
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return []LoginAccount{}, nil
		}
		return nil, err
	}
	var accounts []LoginAccount
	if err := json.Unmarshal(raw, &accounts); err != nil {
		return []LoginAccount{}, nil
	}
	if accounts == nil {
		accounts = []LoginAccount{}
	}
	return accounts, nil
}

func normalizeOrigin(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("origin required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid origin")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "https" && scheme != "http" {
		return "", fmt.Errorf("origin must be http(s)")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("origin must be scheme://host[:port] only")
	}
	if u.Path != "" && u.Path != "/" {
		return "", fmt.Errorf("origin must not include a path")
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "" {
		return "", fmt.Errorf("origin host required")
	}
	out := scheme + "://" + host
	if u.Port() != "" {
		out += ":" + u.Port()
	}
	return out, nil
}
