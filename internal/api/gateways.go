package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/krate-apps/fleet/internal/gateways"
	"github.com/krate-apps/fleet/internal/storage"
)

type gatewayWriteRequest struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	Enabled     *bool  `json:"enabled"`
	JournalUnit string `json:"journalUnit"`
	LogPath     string `json:"logPath"`
}

type gatewayProbeRequest struct {
	Kind     string `json:"kind"`
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type gatewayClientDTO struct {
	storage.GatewayClient
	Capabilities gateways.Capabilities `json:"capabilities"`
}

func (s *Server) handleListGateways(w http.ResponseWriter, _ *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusOK, map[string]any{"clients": []any{}})
		return
	}
	list, err := s.store.ListGatewayClients()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	out := make([]gatewayClientDTO, 0, len(list))
	for _, c := range list {
		out = append(out, gatewayClientDTO{
			GatewayClient: c,
			Capabilities:  gateways.CapabilitiesFor(c.Kind),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"clients": out})
}

func (s *Server) handleUpsertGateway(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "storage unavailable"})
		return
	}
	var req gatewayWriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	kind := gateways.Kind(strings.ToLower(strings.TrimSpace(req.Kind)))
	switch kind {
	case gateways.KindRTorrent, gateways.KindDeluge, gateways.KindTransmission, gateways.KindQBittorrent:
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "kind must be rtorrent, deluge, transmission, or qbittorrent"})
		return
	}
	name := strings.TrimSpace(req.Name)
	url := strings.TrimSpace(req.URL)
	if kind != gateways.KindRTorrent {
		url = strings.TrimRight(url, "/")
	}
	if name == "" || url == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and url required"})
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		id = strings.TrimSpace(req.ID)
	}
	keepPassword := false
	if id == "" {
		id = newGatewayID()
	} else {
		keepPassword = true
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	journalUnit, logPath, err := validateGatewayLogFields(req.JournalUnit, req.LogPath)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	saved, err := s.store.UpsertGatewayClient(storage.GatewayClient{
		ID:          id,
		Kind:        kind,
		Name:        name,
		URL:         url,
		Username:    strings.TrimSpace(req.Username),
		Password:    req.Password,
		Enabled:     enabled,
		JournalUnit: journalUnit,
		LogPath:     logPath,
	}, keepPassword)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"client": saved})
}

func (s *Server) handleSetDefaultGateway(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "storage unavailable"})
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}
	if err := s.store.SetDefaultGatewayClient(id); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	c, err := s.store.GetGatewayClient(id)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id})
		return
	}
	c.Password = ""
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "client": c})
}

func (s *Server) handleClearDefaultGateway(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "storage unavailable"})
		return
	}
	if err := s.store.ClearDefaultGatewayClient(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleDeleteGateway(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "storage unavailable"})
		return
	}
	id := r.PathValue("id")
	if err := s.store.DeleteGatewayClient(id); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleProbeGateway(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "storage unavailable"})
		return
	}
	id := r.PathValue("id")
	c, err := s.store.GetGatewayClient(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	result := gateways.Probe(r.Context(), gateways.ProbeInput{
		Kind:     c.Kind,
		URL:      c.URL,
		Username: c.Username,
		Password: c.Password,
	})
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleProbeGatewayDraft(w http.ResponseWriter, r *http.Request) {
	var req gatewayProbeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	kind := gateways.Kind(strings.ToLower(strings.TrimSpace(req.Kind)))
	result := gateways.Probe(r.Context(), gateways.ProbeInput{
		Kind:     kind,
		URL:      strings.TrimRight(strings.TrimSpace(req.URL), "/"),
		Username: strings.TrimSpace(req.Username),
		Password: req.Password,
	})
	writeJSON(w, http.StatusOK, result)
}

func newGatewayID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "gw_" + hex.EncodeToString(b[:])
}
