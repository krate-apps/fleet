package api

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
)

func (s *Server) handleShareList(w http.ResponseWriter, r *http.Request) {
	if s.shares == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "share store unavailable"})
		return
	}
	links, err := s.shares.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"shares": links})
}

func (s *Server) handleShareCreate(w http.ResponseWriter, r *http.Request) {
	if s.shares == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "share store unavailable"})
		return
	}
	m := s.fsManager()
	if m == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "download_roots not configured"})
		return
	}
	var req struct {
		Path         string `json:"path"`
		TTLSec       int    `json:"ttlSec"`
		MaxDownloads int    `json:"maxDownloads"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	abs, _, err := m.MustFile(req.Path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if req.TTLSec < 0 || req.TTLSec > 86400*365 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ttlSec out of range"})
		return
	}
	if req.MaxDownloads < 0 || req.MaxDownloads > 100000 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "maxDownloads out of range"})
		return
	}
	link, err := s.shares.Create(abs, filepath.Base(abs), req.TTLSec, req.MaxDownloads)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	prefix := strings.TrimRight(r.Header.Get("X-Forwarded-Prefix"), "/")
	writeJSON(w, http.StatusOK, map[string]any{
		"share": link,
		"url":   prefix + "/api/v1/share/" + link.Token,
	})
}

func (s *Server) handleShareDelete(w http.ResponseWriter, r *http.Request) {
	if s.shares == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "share store unavailable"})
		return
	}
	token := strings.TrimSpace(r.PathValue("token"))
	if token == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "token required"})
		return
	}
	if err := s.shares.Delete(token); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleShareGet(w http.ResponseWriter, r *http.Request) {
	if s.shares == nil {
		http.Error(w, "share unavailable", http.StatusServiceUnavailable)
		return
	}
	token := strings.TrimSpace(r.PathValue("token"))
	if token == "" {
		http.NotFound(w, r)
		return
	}
	link, ok, err := s.shares.Get(token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.NotFound(w, r)
		return
	}
	m := s.fsManager()
	if m == nil {
		http.Error(w, "download_roots not configured", http.StatusServiceUnavailable)
		return
	}
	f, st, err := m.OpenFile(link.Path)
	if err != nil {
		http.Error(w, "file unavailable", http.StatusNotFound)
		return
	}
	defer f.Close()
	name := link.Name
	if name == "" {
		name = filepath.Base(link.Path)
	}
	if r.Method == http.MethodHead {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", `attachment; filename="`+strings.ReplaceAll(name, `"`, "")+`"`)
		w.Header().Set("Content-Length", strconv.FormatInt(st.Size(), 10))
		w.WriteHeader(http.StatusOK)
		return
	}
	if _, ok, err := s.shares.Consume(token); err != nil || !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+strings.ReplaceAll(name, `"`, "")+`"`)
	w.Header().Set("Content-Length", strconv.FormatInt(st.Size(), 10))
	_, _ = io.Copy(w, f)
}
