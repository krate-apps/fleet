// Package share persists public download tokens for paths under download_roots.
package share

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Link is a time-bounded public download grant for a single file path.
type Link struct {
	Token        string `json:"token"`
	Path         string `json:"path"`
	Name         string `json:"name"`
	CreatedAt    int64  `json:"createdAt"`
	ExpiresAt    int64  `json:"expiresAt,omitempty"`
	MaxDownloads int    `json:"maxDownloads,omitempty"`
	Downloads    int    `json:"downloads"`
}

// Store persists share links under DataDir/shares/links.json.
type Store struct {
	DataDir string

	mu sync.Mutex
}

func (s *Store) path() (string, error) {
	if s == nil || strings.TrimSpace(s.DataDir) == "" {
		return "", fmt.Errorf("share store data dir required")
	}
	dir := filepath.Join(s.DataDir, "shares")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "links.json"), nil
}

// List returns all non-expired links (expired ones are pruned).
func (s *Store) List() ([]Link, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	links, err := s.loadLocked()
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	out := make([]Link, 0, len(links))
	changed := false
	for _, l := range links {
		if l.ExpiresAt > 0 && l.ExpiresAt < now {
			changed = true
			continue
		}
		if l.MaxDownloads > 0 && l.Downloads >= l.MaxDownloads {
			changed = true
			continue
		}
		out = append(out, l)
	}
	if changed {
		if err := s.saveLocked(out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// Create adds a new link. pathAbs must already be validated under download_roots.
func (s *Store) Create(pathAbs, name string, ttlSec, maxDownloads int) (Link, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	links, err := s.loadLocked()
	if err != nil {
		return Link{}, err
	}
	tok, err := randomToken(16)
	if err != nil {
		return Link{}, err
	}
	now := time.Now().Unix()
	link := Link{
		Token:        tok,
		Path:         pathAbs,
		Name:         name,
		CreatedAt:    now,
		MaxDownloads: maxDownloads,
	}
	if ttlSec > 0 {
		link.ExpiresAt = now + int64(ttlSec)
	}
	links = append(links, link)
	if err := s.saveLocked(links); err != nil {
		return Link{}, err
	}
	return link, nil
}

// Get returns a link by token without consuming a download.
func (s *Store) Get(token string) (Link, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	links, err := s.loadLocked()
	if err != nil {
		return Link{}, false, err
	}
	now := time.Now().Unix()
	for _, l := range links {
		if l.Token != token {
			continue
		}
		if l.ExpiresAt > 0 && l.ExpiresAt < now {
			return Link{}, false, nil
		}
		if l.MaxDownloads > 0 && l.Downloads >= l.MaxDownloads {
			return Link{}, false, nil
		}
		return l, true, nil
	}
	return Link{}, false, nil
}

// Consume increments download count; returns false if exhausted/expired/missing.
func (s *Store) Consume(token string) (Link, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	links, err := s.loadLocked()
	if err != nil {
		return Link{}, false, err
	}
	now := time.Now().Unix()
	for i := range links {
		if links[i].Token != token {
			continue
		}
		if links[i].ExpiresAt > 0 && links[i].ExpiresAt < now {
			return Link{}, false, nil
		}
		if links[i].MaxDownloads > 0 && links[i].Downloads >= links[i].MaxDownloads {
			return Link{}, false, nil
		}
		links[i].Downloads++
		link := links[i]
		keep := make([]Link, 0, len(links))
		for _, l := range links {
			if l.MaxDownloads > 0 && l.Downloads >= l.MaxDownloads {
				continue
			}
			if l.ExpiresAt > 0 && l.ExpiresAt < now {
				continue
			}
			keep = append(keep, l)
		}
		if err := s.saveLocked(keep); err != nil {
			return Link{}, false, err
		}
		return link, true, nil
	}
	return Link{}, false, nil
}

// Delete removes a token.
func (s *Store) Delete(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	links, err := s.loadLocked()
	if err != nil {
		return err
	}
	out := make([]Link, 0, len(links))
	for _, l := range links {
		if l.Token == token {
			continue
		}
		out = append(out, l)
	}
	return s.saveLocked(out)
}

func (s *Store) loadLocked() ([]Link, error) {
	p, err := s.path()
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return []Link{}, nil
		}
		return nil, err
	}
	if len(raw) == 0 {
		return []Link{}, nil
	}
	var links []Link
	if err := json.Unmarshal(raw, &links); err != nil {
		return nil, err
	}
	if links == nil {
		links = []Link{}
	}
	return links, nil
}

func (s *Store) saveLocked(links []Link) error {
	p, err := s.path()
	if err != nil {
		return err
	}
	if links == nil {
		links = []Link{}
	}
	raw, err := json.MarshalIndent(links, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func randomToken(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
