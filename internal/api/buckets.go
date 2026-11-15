package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/krate-apps/fleet/internal/buckets"
)

type addBucketRequest struct {
	Path string `json:"path"`
}

type installRequest struct {
	Source      string `json:"source"` // "local" | "bucket" | "archive" | "url"
	Path        string `json:"path,omitempty"`
	URL         string `json:"url,omitempty"`
	SHA256      string `json:"sha256,omitempty"`
	BucketID    string `json:"bucketId,omitempty"`
	ExtensionID string `json:"extensionId,omitempty"`
}

func (s *Server) handleListBuckets(w http.ResponseWriter, _ *http.Request) {
	if s.buckets == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "buckets unavailable"})
		return
	}
	list, err := s.buckets.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"buckets": list})
}

func (s *Server) handleAddBucket(w http.ResponseWriter, r *http.Request) {
	if s.buckets == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "buckets unavailable"})
		return
	}
	var req addBucketRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	rec, err := s.buckets.AddLocal(req.Path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

func (s *Server) handleRemoveBucket(w http.ResponseWriter, r *http.Request) {
	if s.buckets == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "buckets unavailable"})
		return
	}
	if err := s.buckets.Remove(r.PathValue("id")); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleSetBucketEnabled(w http.ResponseWriter, r *http.Request) {
	if s.buckets == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "buckets unavailable"})
		return
	}
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	rec, err := s.buckets.SetEnabled(r.PathValue("id"), body.Enabled)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

func (s *Server) handleBucketCatalog(w http.ResponseWriter, _ *http.Request) {
	if s.buckets == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "buckets unavailable"})
		return
	}
	cat, err := s.buckets.Catalog()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"extensions": cat})
}

func (s *Server) handleUninstallExtension(w http.ResponseWriter, r *http.Request) {
	if s.buckets == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "buckets unavailable"})
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "extension id required"})
		return
	}
	if s.ext != nil {
		_, _ = s.ext.Deactivate(id)
	}
	if err := s.buckets.UninstallPackage(id); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if s.ext != nil {
		_ = s.ext.Refresh()
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id})
}

func (s *Server) handleInstallExtension(w http.ResponseWriter, r *http.Request) {
	if s.buckets == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "buckets unavailable"})
		return
	}
	var req installRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	var (
		m   any
		err error
	)
	switch req.Source {
	case "local":
		m, err = s.buckets.InstallLocal(req.Path)
	case "bucket":
		m, err = s.buckets.InstallFromBucket(req.BucketID, req.ExtensionID)
	case "archive":
		m, err = s.buckets.InstallArchive(req.Path, req.SHA256)
	case "url":
		m, err = s.buckets.InstallURL(r.Context(), req.URL, req.SHA256, buckets.DownloadOptions{})
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "source must be local, bucket, archive, or url"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if s.ext != nil {
		_ = s.ext.Refresh()
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "manifest": m})
}
