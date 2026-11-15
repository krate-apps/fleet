package api

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/krate-apps/fleet/internal/extensions"
)

const maxHookBodyBytes = 1 << 20 // 1 MiB

func (s *Server) handleExtensionHook(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}
	provided := hookHeaderToken(r)
	if provided == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	if s.ext == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "extensions unavailable"})
		return
	}
	rec, err := s.ext.Get(id)
	if err != nil || rec == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "extension not found"})
		return
	}
	if rec.State != extensions.StateActive {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "extension not active"})
		return
	}

	expected, err := loadWebhookToken(s.ext.Storage(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "settings unavailable"})
		return
	}
	if strings.TrimSpace(expected) == "" || !tokenMatches(expected, provided) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxHookBodyBytes+1))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unable to read body"})
		return
	}
	if len(body) > maxHookBodyBytes {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "body too large"})
		return
	}

	method := hookIngestMethod(id)
	raw, err := s.ext.CallWorker(r.Context(), id, method, map[string]any{
		"body":   string(body),
		"source": r.Header.Get("User-Agent"),
	})
	if err != nil {
		msg := err.Error()
		status := http.StatusBadGateway
		if strings.Contains(strings.ToLower(msg), "invalid") ||
			strings.Contains(strings.ToLower(msg), "reject") ||
			strings.Contains(strings.ToLower(msg), "required") {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, map[string]string{"error": msg})
		return
	}

	var result struct {
		OK      bool   `json:"ok"`
		Status  int    `json:"status"`
		Reason  string `json:"reason"`
		Message string `json:"message"`
	}
	if len(raw) > 0 && string(raw) != "null" {
		_ = json.Unmarshal(raw, &result)
	}
	if result.Status == 0 {
		if result.OK || (result.Reason == "" && result.Message == "") {
			writeJSON(w, http.StatusOK, rawResultOrOK(raw))
			return
		}
		result.Status = http.StatusBadRequest
	}
	if !result.OK && result.Status >= 400 {
		msg := result.Reason
		if msg == "" {
			msg = result.Message
		}
		if msg == "" {
			msg = "rejected"
		}
		writeJSON(w, result.Status, map[string]string{"error": msg})
		return
	}
	writeJSON(w, http.StatusOK, rawResultOrOK(raw))
}

func rawResultOrOK(raw json.RawMessage) any {
	if len(raw) == 0 || string(raw) == "null" {
		return map[string]any{"ok": true}
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return map[string]any{"ok": true}
	}
	return v
}

func hookIngestMethod(extensionID string) string {
	parts := strings.Split(strings.TrimSpace(extensionID), ".")
	short := parts[len(parts)-1]
	if short == "" {
		short = "hook"
	}
	return "ext." + short + ".ingest"
}

func loadWebhookToken(store *extensions.StorageBridge, extensionID string) (string, error) {
	if store == nil {
		return "", nil
	}
	raw, err := store.Get(extensionID, "settings")
	if err != nil {
		return "", err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}
	var st struct {
		WebhookToken string `json:"webhookToken"`
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		return "", nil
	}
	return strings.TrimSpace(st.WebhookToken), nil
}

// hookHeaderToken returns the webhook secret from X-Api-Key or Authorization Bearer.
func hookHeaderToken(r *http.Request) string {
	if key := strings.TrimSpace(r.Header.Get("X-Api-Key")); key != "" {
		return key
	}
	authz := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(authz) >= 7 && strings.EqualFold(authz[:7], "bearer ") {
		return strings.TrimSpace(authz[7:])
	}
	return ""
}

func tokenMatches(expected string, candidates ...string) bool {
	exp := []byte(expected)
	ok := false
	for _, c := range candidates {
		cand := []byte(strings.TrimSpace(c))
		if len(cand) == 0 {
			continue
		}
		if len(cand) == len(exp) && subtle.ConstantTimeCompare(cand, exp) == 1 {
			ok = true
		}
	}
	return ok
}
