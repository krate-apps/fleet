package api

import (
	"net/http"
	"strings"

	"github.com/krate-apps/fleet/internal/sysinfo"
)

func (s *Server) handleSystemStatus(w http.ResponseWriter, r *http.Request) {
	roots := append([]string(nil), s.cfg.RTorrent.DownloadRoots...)
	disks := sysinfo.DiskForRoots(roots)
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	if path != "" {
		m := s.fsManager()
		if m == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "download_roots not configured"})
			return
		}
		abs, _, err := m.Resolve(path, true)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		u, err := sysinfo.DiskForPath(abs)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		disks = []sysinfo.DiskUsage{u}
	}
	resp := map[string]any{"disks": disks}
	if load, err := sysinfo.Load(); err == nil {
		resp["load"] = load
	} else {
		resp["loadError"] = err.Error()
	}
	writeJSON(w, http.StatusOK, resp)
}
