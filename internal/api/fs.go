package api

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/krate-apps/fleet/internal/fsops"
)

func (s *Server) fsManager() *fsops.Manager {
	roots := s.cfg.RTorrent.DownloadRoots
	if len(roots) == 0 {
		return nil
	}
	return &fsops.Manager{
		AllowedRoots: append([]string(nil), roots...),
		HideDot:      true,
	}
}

func (s *Server) handleFSRoots(w http.ResponseWriter, r *http.Request) {
	m := s.fsManager()
	if m == nil || len(m.Roots()) == 0 {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "download_roots not configured"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"roots": m.Roots()})
}

func (s *Server) handleFSList(w http.ResponseWriter, r *http.Request) {
	m := s.fsManager()
	if m == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "download_roots not configured"})
		return
	}
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	entries, cwd, err := m.List(path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"cwd": cwd, "entries": entries})
}

func (s *Server) handleFSMkdir(w http.ResponseWriter, r *http.Request) {
	m := s.fsManager()
	if m == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "download_roots not configured"})
		return
	}
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if err := m.Mkdir(req.Path); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "path": req.Path})
}

func (s *Server) handleFSRename(w http.ResponseWriter, r *http.Request) {
	s.handleFSFromTo(w, r, false)
}

func (s *Server) handleFSMove(w http.ResponseWriter, r *http.Request) {
	s.handleFSFromTo(w, r, true)
}

func (s *Server) handleFSFromTo(w http.ResponseWriter, r *http.Request, _ bool) {
	m := s.fsManager()
	if m == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "download_roots not configured"})
		return
	}
	var req struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if err := m.Rename(req.From, req.To); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "from": req.From, "to": req.To})
}

func (s *Server) handleFSCopy(w http.ResponseWriter, r *http.Request) {
	m := s.fsManager()
	if m == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "download_roots not configured"})
		return
	}
	var req struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if err := m.Copy(req.From, req.To); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "from": req.From, "to": req.To})
}

func (s *Server) handleFSRemove(w http.ResponseWriter, r *http.Request) {
	m := s.fsManager()
	if m == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "download_roots not configured"})
		return
	}
	var req struct {
		Path      string `json:"path"`
		Recursive bool   `json:"recursive"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if err := m.Remove(req.Path, req.Recursive); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "path": req.Path})
}

func (s *Server) handleFSText(w http.ResponseWriter, r *http.Request) {
	m := s.fsManager()
	if m == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "download_roots not configured"})
		return
	}
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	encodingHint := strings.TrimSpace(r.URL.Query().Get("encoding"))
	text, encoding, err := m.ReadTextEncoded(path, encodingHint)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": path, "text": text, "encoding": encoding})
}

func (s *Server) handleFSChecksum(w http.ResponseWriter, r *http.Request) {
	m := s.fsManager()
	if m == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "download_roots not configured"})
		return
	}
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	if path == "" {
		var req struct {
			Path string `json:"path"`
		}
		_ = json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req)
		path = req.Path
	}
	sum, n, err := m.ChecksumSHA256(path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": path, "algo": "sha256", "hash": sum, "size": n})
}

func (s *Server) handleFSContent(w http.ResponseWriter, r *http.Request) {
	m := s.fsManager()
	if m == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "download_roots not configured"})
		return
	}
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	f, st, err := m.OpenFile(path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	defer f.Close()
	name := filepath.Base(path)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+strings.ReplaceAll(name, `"`, "")+`"`)
	w.Header().Set("Content-Length", strconv.FormatInt(st.Size(), 10))
	_, _ = io.Copy(w, f)
}
