package api

import (
	"net/http"
	"strconv"
)

func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	if s.jobs == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "jobs unavailable"})
		return
	}
	status := r.URL.Query().Get("status")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	list, err := s.jobs.List(status, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": list})
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	if s.jobs == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "jobs unavailable"})
		return
	}
	id := r.PathValue("id")
	j, err := s.jobs.Get(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, j)
}

func (s *Server) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	if s.jobs == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "jobs unavailable"})
		return
	}
	id := r.PathValue("id")
	j, err := s.jobs.Cancel(id)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, j)
}
