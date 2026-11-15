package api

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/krate-apps/fleet/internal/jobs"
	"github.com/krate-apps/fleet/internal/tools"
)

type dumpRequest struct {
	TorrentBase64 string `json:"torrentBase64"`
}

type createTorrentRequest struct {
	Paths       []string `json:"paths"`
	Announce    []string `json:"announce"`
	PieceLength int      `json:"pieceLength"`
	Private     bool     `json:"private"`
	Name        string   `json:"name"`
	Comment     string   `json:"comment"`
	OutputName  string   `json:"outputName"`
}

func (s *Server) handleToolsStatus(w http.ResponseWriter, _ *http.Request) {
	if s.tools == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "tools runner unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, s.tools.Status())
}

func (s *Server) handleDumpTorrent(w http.ResponseWriter, r *http.Request) {
	if s.tools == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "tools runner unavailable"})
		return
	}
	var req dumpRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 32<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.TorrentBase64 == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "torrentBase64 required"})
		return
	}
	data, err := decodeBase64(req.TorrentBase64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid torrentBase64"})
		return
	}
	res, err := s.tools.DumpTorrent(r.Context(), data)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":  err.Error(),
			"status": s.tools.Status(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"result": res,
		"status": s.tools.Status(),
	})
}

func (s *Server) handleCreateTorrent(w http.ResponseWriter, r *http.Request) {
	if s.tools == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "tools runner unavailable"})
		return
	}
	if s.jobs == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "jobs unavailable"})
		return
	}
	var req createTorrentRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	job, err := tools.StartMktorrentJob(s.jobs, s.tools, s.cfg.DataDir, "", tools.CreateInput{
		Paths:       req.Paths,
		Announce:    req.Announce,
		PieceLength: req.PieceLength,
		Private:     req.Private,
		Name:        req.Name,
		Comment:     req.Comment,
		OutputName:  req.OutputName,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"ok":    true,
		"jobId": job.ID,
		"job":   job,
	})
}

func (s *Server) handleJobDownload(w http.ResponseWriter, r *http.Request) {
	if s.jobs == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "jobs unavailable"})
		return
	}
	id := r.PathValue("id")
	job, err := s.jobs.Get(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	if job.Kind != "tools.mktorrent" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "job has no downloadable artifact"})
		return
	}
	if job.Status != jobs.StatusSucceeded {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "job not succeeded", "status": job.Status})
		return
	}
	path := filepath.Join(s.cfg.DataDir, "artifacts", id, "out.torrent")
	if custom := artifactNameFromResult(job.Result); custom != "" {
		candidate := filepath.Join(s.cfg.DataDir, "artifacts", id, custom)
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			path = candidate
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		// Fall back to base64 embedded in job result.
		if b64 := torrentBase64FromResult(job.Result); b64 != "" {
			raw, err := decodeBase64(b64)
			if err == nil {
				w.Header().Set("Content-Type", "application/x-bittorrent")
				w.Header().Set("Content-Disposition", `attachment; filename="fleet.torrent"`)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(raw)
				return
			}
		}
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "artifact missing"})
		return
	}
	name := filepath.Base(path)
	w.Header().Set("Content-Type", "application/x-bittorrent")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) handleMediaInfo(w http.ResponseWriter, r *http.Request) {
	if s.tools == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "tools runner unavailable"})
		return
	}
	var req struct {
		Hash  string `json:"hash"`
		Index int    `json:"index"`
		Path  string `json:"path"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	path := req.Path
	if path == "" {
		if s.engine == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "sync engine unavailable"})
			return
		}
		resolved, _, err := s.engine.ResolveDataFile(r.Context(), req.Hash, req.Index, s.cfg.RTorrent.DownloadRoots)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		path = resolved
	}
	text, err := s.tools.MediaInfo(r.Context(), path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error(), "status": s.tools.Status()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "text": text, "path": path})
}

func artifactNameFromResult(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	if p, ok := m["outputPath"].(string); ok && p != "" {
		return filepath.Base(p)
	}
	return ""
}

func torrentBase64FromResult(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	s, _ := m["torrentBase64"].(string)
	return s
}
