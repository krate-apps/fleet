package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

type trackerAddRequest struct {
	URL string `json:"url"`
}

type trackerEnableRequest struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
}

type trackerRemoveRequest struct {
	ID string `json:"id"`
}

func (s *Server) handleAddTracker(w http.ResponseWriter, r *http.Request) {
	if s.engine == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "sync engine unavailable"})
		return
	}
	hash := strings.ToUpper(r.PathValue("id"))
	var req trackerAddRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if err := s.engine.AddTracker(r.Context(), hash, req.URL); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "hash": hash})
}

func (s *Server) handleSetTrackerEnabled(w http.ResponseWriter, r *http.Request) {
	if s.engine == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "sync engine unavailable"})
		return
	}
	hash := strings.ToUpper(r.PathValue("id"))
	var req trackerEnableRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if !strings.HasPrefix(strings.ToUpper(req.ID), hash+":T") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "tracker id does not match torrent"})
		return
	}
	if err := s.engine.SetTrackerEnabled(r.Context(), req.ID, req.Enabled); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": req.ID, "enabled": req.Enabled})
}

func (s *Server) handleRemoveTracker(w http.ResponseWriter, r *http.Request) {
	if s.engine == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "sync engine unavailable"})
		return
	}
	hash := strings.ToUpper(r.PathValue("id"))
	var req trackerRemoveRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if !strings.HasPrefix(strings.ToUpper(req.ID), hash+":T") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "tracker id does not match torrent"})
		return
	}
	if err := s.engine.RemoveTracker(r.Context(), req.ID); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": req.ID})
}
