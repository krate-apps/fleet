package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/krate-apps/fleet/internal/updates"
)

func (s *Server) handleUpdatesStatus(w http.ResponseWriter, _ *http.Request) {
	if s.updates == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "updates unavailable"})
		return
	}
	st, err := s.updates.Status()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleUpdatesSetChannel(w http.ResponseWriter, r *http.Request) {
	if s.updates == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "updates unavailable"})
		return
	}
	var body struct {
		Channel string `json:"channel"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	ch, err := s.updates.SetChannel(body.Channel)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "channel": ch})
}

func (s *Server) handleUpdatesAvailable(w http.ResponseWriter, r *http.Request) {
	if s.updates == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "updates unavailable"})
		return
	}
	list, err := s.updates.ListAvailable(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"releases": list})
}

func (s *Server) handleUpdatesInstalled(w http.ResponseWriter, _ *http.Request) {
	if s.updates == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "updates unavailable"})
		return
	}
	list, err := s.updates.ListInstalled()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"releases": list})
}

func (s *Server) handleUpdatesPrepare(w http.ResponseWriter, r *http.Request) {
	if s.updates == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "updates unavailable"})
		return
	}
	var man updates.Manifest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&man); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	pending, err := s.updates.Prepare(r.Context(), man)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "pending": pending})
}

func (s *Server) handleUpdatesApply(w http.ResponseWriter, r *http.Request) {
	if s.updates == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "updates unavailable"})
		return
	}
	var body struct {
		Prefix    string `json:"prefix"`
		HealthURL string `json:"healthUrl"`
	}
	_ = json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body)
	if body.Prefix == "" {
		body.Prefix = s.cfg.InstallPrefix
	}
	if body.HealthURL == "" {
		body.HealthURL = "http://" + s.cfg.Addr() + "/api/v1/health"
	}
	plan, err := s.updates.PlanApplyWithOptions(r.Context(), updates.PlanApplyOptions{
		InstallPrefix: body.Prefix,
		HealthURL:     body.HealthURL,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "plan": plan})
}

func (s *Server) handleUpdatesSwitch(w http.ResponseWriter, r *http.Request) {
	if s.updates == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "updates unavailable"})
		return
	}
	var body struct {
		Version   string `json:"version"`
		Prefix    string `json:"prefix"`
		HealthURL string `json:"healthUrl"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if body.Prefix == "" {
		body.Prefix = s.cfg.InstallPrefix
	}
	if body.HealthURL == "" {
		body.HealthURL = "http://" + s.cfg.Addr() + "/api/v1/health"
	}
	plan, err := s.updates.PlanSwitch(r.Context(), body.Version, updates.PlanApplyOptions{
		InstallPrefix: body.Prefix,
		HealthURL:     body.HealthURL,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "plan": plan})
}

func (s *Server) handleUpdatesBackup(w http.ResponseWriter, r *http.Request) {
	if s.updates == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "updates unavailable"})
		return
	}
	path, err := s.updates.BackupNow(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "path": path})
}
