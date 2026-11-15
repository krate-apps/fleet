package api

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/krate-apps/fleet/internal/logs"
)

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	if s.logs == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "logs unavailable"})
		return
	}
	limit, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
	q := logs.Query{
		Source: strings.TrimSpace(r.URL.Query().Get("source")),
		Level:  strings.TrimSpace(r.URL.Query().Get("level")),
		Limit:  limit,
	}
	s.refreshLogClients()
	sources, entries, err := s.logs.Query(r.Context(), q)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if sources == nil {
		sources = []logs.Source{}
	}
	if entries == nil {
		entries = []logs.Entry{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"sources": sources,
		"entries": entries,
	})
}

func (s *Server) refreshLogClients() {
	if s.logs == nil || s.store == nil {
		return
	}
	list, err := s.store.ListGatewayClients()
	if err != nil {
		return
	}
	refs := make([]logs.ClientRef, 0, len(list))
	for _, c := range list {
		refs = append(refs, logs.ClientRef{
			ID:          c.ID,
			Kind:        string(c.Kind),
			Name:        c.Name,
			Enabled:     c.Enabled,
			JournalUnit: c.JournalUnit,
			LogPath:     c.LogPath,
		})
	}
	s.logs.SetClients(refs)
}

func validateGatewayLogFields(journalUnit, logPath string) (unit, path string, err error) {
	unit = strings.TrimSpace(journalUnit)
	if unit != "" {
		unit, err = logs.ValidateUnit(unit)
		if err != nil {
			return "", "", err
		}
	}
	path = strings.TrimSpace(logPath)
	if path != "" {
		home, herr := os.UserHomeDir()
		if herr != nil {
			return "", "", herr
		}
		path, err = logs.ValidateLogPath(path, home)
		if err != nil {
			return "", "", err
		}
	}
	return unit, path, nil
}
