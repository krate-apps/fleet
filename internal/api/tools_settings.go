package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/krate-apps/fleet/internal/tools"
)

const toolsSettingsMetaKey = "tools_settings"

type toolsSettingsResponse struct {
	Paths     tools.Paths    `json:"paths"`
	YAML      tools.Paths    `json:"yaml"`
	Overrides tools.Paths    `json:"overrides"`
	Status    map[string]any `json:"status"`
	Reloaded  []string       `json:"reloaded,omitempty"`
	ReloadErr map[string]string `json:"reloadFailed,omitempty"`
}

func (s *Server) yamlToolsPaths() tools.Paths {
	if s == nil {
		return tools.Paths{}
	}
	return tools.Paths{
		Mktorrent:   s.cfg.Tools.Mktorrent,
		Dumptorrent: s.cfg.Tools.Dumptorrent,
		Mediainfo:   s.cfg.Tools.Mediainfo,
		Ffmpeg:      s.cfg.Tools.Ffmpeg,
		Sox:         s.cfg.Tools.Sox,
		Unrar:       s.cfg.Tools.Unrar,
		Rclone:      s.cfg.Tools.Rclone,
	}.Normalize()
}

func (s *Server) loadToolsOverrides() tools.Paths {
	if s == nil || s.store == nil {
		return tools.Paths{}
	}
	raw, err := s.store.GetMeta(toolsSettingsMetaKey)
	if err != nil || strings.TrimSpace(raw) == "" {
		return tools.Paths{}
	}
	var p tools.Paths
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return tools.Paths{}
	}
	return tools.OverridesOnly(p)
}

func (s *Server) saveToolsOverrides(p tools.Paths) error {
	if s == nil || s.store == nil {
		return fmt.Errorf("storage unavailable")
	}
	over := tools.OverridesOnly(p)
	raw, err := json.Marshal(over)
	if err != nil {
		return err
	}
	return s.store.PutMeta(toolsSettingsMetaKey, string(raw))
}

func (s *Server) effectiveToolsPaths() tools.Paths {
	return tools.OverlayNonEmpty(s.yamlToolsPaths(), s.loadToolsOverrides())
}

func (s *Server) toolsSettingsPayload() toolsSettingsResponse {
	yaml := s.yamlToolsPaths()
	over := s.loadToolsOverrides()
	status := map[string]any{}
	if s.tools != nil {
		status = s.tools.Status()
	}
	// paths = UI overrides only (empty = YAML then PATH).
	return toolsSettingsResponse{
		Paths:     over,
		YAML:      yaml,
		Overrides: over,
		Status:    status,
	}
}

func (s *Server) handleGetToolsSettings(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.toolsSettingsPayload())
}

func (s *Server) handlePutToolsSettings(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Paths tools.Paths `json:"paths"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	submitted := body.Paths.Normalize()
	if err := tools.ValidatePaths(submitted); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	yaml := s.yamlToolsPaths()
	// Empty field → drop override (fall back to YAML). Non-empty → store as override
	// even when equal to YAML (harmless pin).
	overrides := tools.OverridesOnly(submitted)
	prevRclone := ""
	if s.tools != nil {
		prevRclone = s.tools.Paths().Rclone
	}
	effective := tools.OverlayNonEmpty(yaml, overrides)

	if err := s.saveToolsOverrides(overrides); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if s.tools != nil {
		s.tools.SetPaths(effective)
	}

	rcloneChanged := strings.TrimSpace(prevRclone) != strings.TrimSpace(effective.Rclone)
	out := s.toolsSettingsPayload()
	if rcloneChanged && s.ext != nil {
		s.ext.SetWorkerEnvKey("FLEET_TOOL_RCLONE", effective.Rclone)
		reloaded, failed := s.ext.ReloadWorkers()
		out.Reloaded = reloaded
		out.ReloadErr = failed
	}
	writeJSON(w, http.StatusOK, out)
}
