package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

type labelRequest struct {
	Label    string `json:"label"`
	ClientID string `json:"clientId"`
}

type nameRequest struct {
	Name string `json:"name"`
}

type priorityRequest struct {
	Priority int64 `json:"priority"`
}

type filePriorityRequest struct {
	Indexes  []int64 `json:"indexes"`
	Priority int64   `json:"priority"`
}

func (s *Server) handleSetLabel(w http.ResponseWriter, r *http.Request) {
	if s.engine == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "sync engine unavailable"})
		return
	}
	rawID := r.PathValue("id")
	hash := strings.ToUpper(rawID)
	clientID := ""
	if i := strings.LastIndex(rawID, "@"); i > 0 {
		hash = strings.ToUpper(rawID[:i])
		clientID = rawID[i+1:]
	}
	var req labelRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if strings.TrimSpace(req.ClientID) != "" {
		clientID = strings.TrimSpace(req.ClientID)
	}
	if err := s.engine.SetLabelOnClient(r.Context(), hash, clientID, req.Label); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"hash": hash, "label": req.Label, "clientId": clientID})
}

func (s *Server) handleSetName(w http.ResponseWriter, r *http.Request) {
	if s.engine == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "sync engine unavailable"})
		return
	}
	hash := strings.ToUpper(r.PathValue("id"))
	var req nameRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name required"})
		return
	}
	if err := s.engine.SetName(r.Context(), hash, name); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"hash": hash, "name": name})
}

func (s *Server) handleSetTorrentPriority(w http.ResponseWriter, r *http.Request) {
	if s.engine == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "sync engine unavailable"})
		return
	}
	hash := strings.ToUpper(r.PathValue("id"))
	var req priorityRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.Priority < 0 || req.Priority > 3 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "priority must be 0..3"})
		return
	}
	if err := s.engine.SetTorrentPriority(r.Context(), hash, req.Priority); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"hash": hash, "priority": req.Priority})
}

func (s *Server) handleSetFilePriorities(w http.ResponseWriter, r *http.Request) {
	if s.engine == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "sync engine unavailable"})
		return
	}
	hash := strings.ToUpper(r.PathValue("id"))
	var req filePriorityRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if len(req.Indexes) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "indexes required"})
		return
	}
	if req.Priority < 0 || req.Priority > 2 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file priority must be 0..2"})
		return
	}
	if err := s.engine.SetFilePriorities(r.Context(), hash, req.Indexes, req.Priority); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"hash":     hash,
		"indexes":  req.Indexes,
		"priority": req.Priority,
	})
}
