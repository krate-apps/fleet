package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func (s *Server) handleTorrentFileContent(w http.ResponseWriter, r *http.Request) {
	if s.engine == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "sync engine unavailable"})
		return
	}
	hash := strings.ToUpper(strings.TrimSpace(r.PathValue("id")))
	if hash == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing torrent id"})
		return
	}
	idx, err := strconv.Atoi(strings.TrimSpace(r.PathValue("index")))
	if err != nil || idx < 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid file index"})
		return
	}
	path, name, err := s.engine.ResolveDataFile(r.Context(), hash, idx, s.cfg.RTorrent.DownloadRoots)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	f, err := os.Open(path)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if name == "" {
		name = filepath.Base(path)
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+strings.ReplaceAll(name, `"`, "")+`"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, name, st.ModTime(), f)
}
