package api

import (
	"encoding/json"
	"io"
	"net/http"
)

type extensionRPCRequest struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

func (s *Server) handleListExtensions(w http.ResponseWriter, r *http.Request) {
	if s.ext == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "extensions unavailable"})
		return
	}
	if r.URL.Query().Get("refresh") == "1" {
		if err := s.ext.Refresh(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"extensions": s.ext.List()})
}

func (s *Server) handleGetExtension(w http.ResponseWriter, r *http.Request) {
	if s.ext == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "extensions unavailable"})
		return
	}
	rec, err := s.ext.Get(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

func (s *Server) handleActivateExtension(w http.ResponseWriter, r *http.Request) {
	if s.ext == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "extensions unavailable"})
		return
	}
	id := r.PathValue("id")
	rec, err := s.ext.Activate(id)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	// Extensions list Activate is the only on/off; clear legacy settings.enabled=false.
	_ = s.ext.EnsureSoftEnabled(r.Context(), id)
	if fresh, getErr := s.ext.Get(id); getErr == nil {
		rec = fresh
	}
	writeJSON(w, http.StatusOK, rec)
}

func (s *Server) handleReloadExtensionWorkers(w http.ResponseWriter, r *http.Request) {
	if s.ext == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "extensions unavailable"})
		return
	}
	reloaded, failed := s.ext.ReloadWorkers()
	status := http.StatusOK
	if len(failed) > 0 && len(reloaded) == 0 {
		status = http.StatusBadRequest
	}
	writeJSON(w, status, map[string]any{
		"ok":       len(failed) == 0,
		"reloaded": reloaded,
		"failed":   failed,
		"count":    len(reloaded),
	})
}

func (s *Server) handleExtensionConsent(w http.ResponseWriter, r *http.Request) {
	if s.ext == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "extensions unavailable"})
		return
	}
	var body struct {
		Accepted bool `json:"accepted"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if !body.Accepted {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "accepted must be true"})
		return
	}
	id := r.PathValue("id")
	if err := s.ext.GrantConsent(id); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	rec, err := s.ext.Get(id)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "extension": rec})
}

func (s *Server) handleDeactivateExtension(w http.ResponseWriter, r *http.Request) {
	if s.ext == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "extensions unavailable"})
		return
	}
	rec, err := s.ext.Deactivate(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

func (s *Server) handleExtensionRPC(w http.ResponseWriter, r *http.Request) {
	if s.ext == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "extensions unavailable"})
		return
	}
	var req extensionRPCRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.Method == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "method required"})
		return
	}
	var params any
	if len(req.Params) > 0 {
		params = req.Params
	}
	result, err := s.ext.CallWorker(r.Context(), r.PathValue("id"), req.Method, params)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "result": result})
}

func (s *Server) handleExtensionStorageGet(w http.ResponseWriter, r *http.Request) {
	if s.ext == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "extensions unavailable"})
		return
	}
	raw, err := s.ext.Storage().Get(r.PathValue("id"), r.PathValue("key"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}

func (s *Server) handleExtensionStoragePut(w http.ResponseWriter, r *http.Request) {
	if s.ext == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "extensions unavailable"})
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "read body"})
		return
	}
	if err := s.ext.Storage().Put(r.PathValue("id"), r.PathValue("key"), body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
