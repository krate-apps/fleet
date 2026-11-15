package api

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
)

type themeInstallRequest struct {
	Source   string `json:"source"` // "bucket" | "local"
	Path     string `json:"path,omitempty"`
	BucketID string `json:"bucketId,omitempty"`
	ThemeID  string `json:"themeId,omitempty"`
}

type themeUninstallRequest struct {
	ID string `json:"id"`
}

func (s *Server) handleThemesList(w http.ResponseWriter, r *http.Request) {
	if s.themes == nil {
		writeJSON(w, http.StatusOK, map[string]any{"themes": []any{}})
		return
	}
	all, err := s.themes.List()
	if err != nil {
		// Pack themes are optional; do not break Settings if themes-src is unreadable.
		s.logger.Warn("themes list", "err", err)
		writeJSON(w, http.StatusOK, map[string]any{"themes": []any{}})
		return
	}
	out := make([]map[string]any, 0, len(all))
	for _, t := range all {
		out = append(out, map[string]any{
			"id":          t.Manifest.ShortID(),
			"packageId":   t.Manifest.ID,
			"label":       t.Manifest.Name,
			"colorScheme": t.Manifest.ColorScheme,
			"version":     t.Manifest.Version,
			"cssPath":     "/api/v1/themes/" + t.Manifest.ShortID() + "/css",
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"themes": out})
}

func (s *Server) handleThemeCSS(w http.ResponseWriter, r *http.Request) {
	if s.themes == nil {
		http.NotFound(w, r)
		return
	}
	id := r.PathValue("id")
	inst, err := s.themes.GetByShortID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	raw, err := os.ReadFile(inst.CSSPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(raw)
}

func (s *Server) handleThemesCatalog(w http.ResponseWriter, _ *http.Request) {
	if s.buckets == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "buckets unavailable"})
		return
	}
	cat, err := s.buckets.ThemesCatalog()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"themes": cat})
}

func (s *Server) handleInstallTheme(w http.ResponseWriter, r *http.Request) {
	if s.themes == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "themes unavailable"})
		return
	}
	var req themeInstallRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	var (
		src string
		err error
	)
	switch strings.TrimSpace(req.Source) {
	case "local":
		src = strings.TrimSpace(req.Path)
		if src == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "path required"})
			return
		}
	case "bucket":
		if s.buckets == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "buckets unavailable"})
			return
		}
		src, err = s.buckets.ResolveThemePackageDir(req.BucketID, req.ThemeID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "source must be bucket or local"})
		return
	}
	inst, err := s.themes.InstallLocal(src)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"id":        inst.Manifest.ShortID(),
		"packageId": inst.Manifest.ID,
		"label":     inst.Manifest.Name,
		"version":   inst.Manifest.Version,
	})
}

func (s *Server) handleUninstallTheme(w http.ResponseWriter, r *http.Request) {
	if s.themes == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "themes unavailable"})
		return
	}
	var req themeUninstallRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if err := s.themes.Uninstall(req.ID); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": req.ID})
}
