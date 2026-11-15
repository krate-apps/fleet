package extensions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// URLRewriteRule is a host-shared rewrite rule (ruTorrent rssurlrewrite parity).
type URLRewriteRule struct {
	Name        string `json:"name"`
	Pattern     string `json:"pattern"`
	Replacement string `json:"replacement"`
	Enabled     bool   `json:"enabled"`
}

// URLRewriteStore persists rewrite rules outside per-extension storage so RSS
// and org.fleet.rssfeedsrewrite can share them.
type URLRewriteStore struct {
	DataDir string

	mu sync.Mutex
}

func (s *URLRewriteStore) path() (string, error) {
	if s == nil || strings.TrimSpace(s.DataDir) == "" {
		return "", fmt.Errorf("urlrewrite store unavailable")
	}
	dir := filepath.Join(s.DataDir, "urlrewrite")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "rules.json"), nil
}

// List returns the current rule set.
func (s *URLRewriteStore) List() ([]URLRewriteRule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

// Set replaces the rule set after validating patterns.
func (s *URLRewriteStore) Set(rules []URLRewriteRule) error {
	if rules == nil {
		rules = []URLRewriteRule{}
	}
	if len(rules) > 100 {
		return fmt.Errorf("too many rules (max 100)")
	}
	cleaned := make([]URLRewriteRule, 0, len(rules))
	seen := map[string]struct{}{}
	for _, r := range rules {
		name := strings.TrimSpace(r.Name)
		pattern := strings.TrimSpace(r.Pattern)
		if name == "" || pattern == "" {
			return fmt.Errorf("rule name and pattern required")
		}
		if len(name) > 80 {
			return fmt.Errorf("rule name too long")
		}
		key := strings.ToLower(name)
		if _, dup := seen[key]; dup {
			return fmt.Errorf("duplicate rule name %q", name)
		}
		seen[key] = struct{}{}
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("invalid pattern for %q: %w", name, err)
		}
		cleaned = append(cleaned, URLRewriteRule{
			Name:        name,
			Pattern:     pattern,
			Replacement: r.Replacement,
			Enabled:     r.Enabled,
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

// Apply runs enabled rules in order against url (Go RE2 semantics).
// matchedRule is the name of the last rule that changed the URL (empty if none).
func (s *URLRewriteStore) Apply(url string) (out string, changed bool, matchedRule string, err error) {
	url = strings.TrimSpace(url)
	if url == "" {
		return "", false, "", nil
	}
	s.mu.Lock()
	rules, err := s.loadLocked()
	s.mu.Unlock()
	if err != nil {
		return "", false, "", err
	}
	out = url
	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			continue
		}
		next := re.ReplaceAllString(out, r.Replacement)
		if next != out {
			out = next
			changed = true
			matchedRule = r.Name
		}
	}
	return out, changed, matchedRule, nil
}

func (s *URLRewriteStore) loadLocked() ([]URLRewriteRule, error) {
	p, err := s.path()
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return []URLRewriteRule{}, nil
		}
		return nil, err
	}
	var rules []URLRewriteRule
	if err := json.Unmarshal(raw, &rules); err != nil {
		return []URLRewriteRule{}, nil
	}
	if rules == nil {
		rules = []URLRewriteRule{}
	}
	return rules, nil
}
