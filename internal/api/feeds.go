package api

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/krate-apps/fleet/internal/feeds"
	"github.com/krate-apps/fleet/internal/version"
)

// FeedsSettings controls absolute RSS URLs advertised to readers.
type FeedsSettings struct {
	PublicHost  string `json:"publicHost"`
	HTTPPort    int    `json:"httpPort"`
	HTTPSPort   int    `json:"httpsPort"`
	PreferHTTPS bool   `json:"preferHTTPS"`
	PathPrefix  string `json:"pathPrefix"`
}

const feedsSettingsMetaKey = "feeds_settings"

func (s *Server) handleFeedsIndex(w http.ResponseWriter, r *http.Request) {
	base := strings.TrimRight(s.publicAPIBase(r), "/")
	modes := make([]map[string]string, 0, len(feeds.Modes))
	for _, m := range feeds.Modes {
		// path is always app-relative; the UI applies withBase() once for path-mode proxies.
		rel := "/api/v1/feeds/" + m + ".xml"
		modes = append(modes, map[string]string{
			"mode":  m,
			"title": feeds.ModeTitle(m),
			"path":  rel,
			"url":   base + rel,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"modes": modes})
}

func (s *Server) handleFeedXML(w http.ResponseWriter, r *http.Request) {
	if s.engine == nil {
		http.Error(w, "sync engine unavailable", http.StatusServiceUnavailable)
		return
	}
	mode := strings.TrimSuffix(r.PathValue("mode"), ".xml")
	mode = strings.TrimSpace(mode)
	if !feeds.ValidMode(mode) {
		http.Error(w, "unknown feed mode", http.StatusBadRequest)
		return
	}
	_, torrents, eng := s.engine.Snapshot()
	self := s.publicAPIBase(r) + "/api/v1/feeds/" + mode + ".xml"
	server := "Fleet"
	if host := r.Host; host != "" {
		server = host
	}
	desc := "*** Fleet " + version.Version + " ***"
	if eng.Message != "" {
		desc += " — " + eng.Message
	}
	raw, err := feeds.BuildRSS(torrents, feeds.BuildOptions{
		Mode:        mode,
		SelfLink:    self,
		ServerName:  server,
		Description: desc,
		IncludeDesc: true,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}

func (s *Server) handleGetFeedsSettings(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.loadFeedsSettings())
}

func (s *Server) handlePutFeedsSettings(w http.ResponseWriter, r *http.Request) {
	var body FeedsSettings
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	st := normalizeFeedsSettings(body)
	if err := s.saveFeedsSettings(st); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func normalizeFeedsSettings(st FeedsSettings) FeedsSettings {
	st.PublicHost = strings.TrimSpace(st.PublicHost)
	st.PathPrefix = strings.TrimSpace(st.PathPrefix)
	if st.HTTPPort < 0 || st.HTTPPort > 65535 {
		st.HTTPPort = 0
	}
	if st.HTTPSPort < 0 || st.HTTPSPort > 65535 {
		st.HTTPSPort = 0
	}
	if st.PathPrefix != "" && !strings.HasPrefix(st.PathPrefix, "/") {
		st.PathPrefix = "/" + st.PathPrefix
	}
	st.PathPrefix = strings.TrimRight(st.PathPrefix, "/")
	return st
}

func (s *Server) loadFeedsSettings() FeedsSettings {
	if s == nil || s.store == nil {
		return FeedsSettings{}
	}
	raw, err := s.store.GetMeta(feedsSettingsMetaKey)
	if err != nil || strings.TrimSpace(raw) == "" {
		return FeedsSettings{}
	}
	var st FeedsSettings
	if err := json.Unmarshal([]byte(raw), &st); err != nil {
		return FeedsSettings{}
	}
	return normalizeFeedsSettings(st)
}

func (s *Server) saveFeedsSettings(st FeedsSettings) error {
	if s == nil || s.store == nil {
		return fmt.Errorf("storage unavailable")
	}
	raw, err := json.Marshal(normalizeFeedsSettings(st))
	if err != nil {
		return err
	}
	return s.store.PutMeta(feedsSettingsMetaKey, string(raw))
}

// publicAPIBase builds the absolute origin (+ optional path prefix) for feed URLs.
// Request headers are the default; stored FeedsSettings overrides win when set.
func (s *Server) publicAPIBase(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	host := r.Host
	if host == "" {
		host = "127.0.0.1"
	}
	prefix := strings.TrimRight(r.Header.Get("X-Forwarded-Prefix"), "/")

	st := s.loadFeedsSettings()
	if st.PreferHTTPS {
		scheme = "https"
	}
	if h := strings.TrimSpace(st.PublicHost); h != "" {
		hostOnly := stripHostPort(h)
		port := st.HTTPPort
		if scheme == "https" {
			port = st.HTTPSPort
		}
		host = formatPublicHost(hostOnly, scheme, port)
	}
	if p := strings.TrimSpace(st.PathPrefix); p != "" {
		prefix = strings.TrimRight(p, "/")
	}
	return scheme + "://" + host + prefix
}

func stripHostPort(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return host
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	// host:port without brackets (IPv4 / hostname)
	if i := strings.LastIndex(host, ":"); i > 0 && !strings.Contains(host, "]") && strings.Count(host, ":") == 1 {
		if _, err := strconv.Atoi(host[i+1:]); err == nil {
			return host[:i]
		}
	}
	return host
}

func formatPublicHost(host, scheme string, port int) string {
	host = strings.TrimSpace(host)
	if host == "" || port <= 0 {
		return host
	}
	if scheme == "https" && port == 443 {
		return host
	}
	if scheme == "http" && port == 80 {
		return host
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}
