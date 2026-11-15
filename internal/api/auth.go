package api

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/krate-apps/fleet/internal/auth"
)

type credentialsRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, _ *http.Request) {
	if s.auth == nil {
		writeJSON(w, http.StatusOK, map[string]any{"enabled": false, "needsSetup": false})
		return
	}
	needs, err := s.auth.NeedsSetup()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"enabled": true, "needsSetup": needs})
}

func (s *Server) handleAuthSetup(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "auth unavailable"})
		return
	}
	var req credentialsRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	user, err := s.auth.CreateAdmin(req.Username, req.Password)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	sess, err := s.auth.CreateSession(user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.auth.SetSessionCookies(w, r, sess)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":   true,
		"user": map[string]any{"id": user.ID, "username": user.Username, "isAdmin": user.IsAdmin},
	})
}

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "auth unavailable"})
		return
	}
	var req credentialsRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	key := clientKey(r) + ":" + strings.ToLower(strings.TrimSpace(req.Username))
	if err := s.auth.CheckLoginAllowed(key); err != nil {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": err.Error()})
		return
	}
	user, err := s.auth.Authenticate(req.Username, req.Password)
	if err != nil {
		s.auth.RecordLoginFailure(key)
		s.logger.Info("login failed", "user", req.Username, "remote", clientKey(r))
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}
	s.auth.RecordLoginSuccess(key)
	sess, err := s.auth.CreateSession(user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.auth.SetSessionCookies(w, r, sess)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":   true,
		"user": map[string]any{"id": user.ID, "username": user.Username, "isAdmin": user.IsAdmin},
	})
}

func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	if c, err := r.Cookie(auth.SessionCookie); err == nil {
		_ = s.auth.DeleteSession(c.Value)
	}
	s.auth.ClearSessionCookies(w, r)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
		return
	}
	sess, user, err := s.sessionFromRequest(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"user":          map[string]any{"id": user.ID, "username": user.Username, "isAdmin": user.IsAdmin},
		"csrfToken":     sess.CSRFToken,
	})
}

func (s *Server) sessionFromRequest(r *http.Request) (*auth.Session, *auth.User, error) {
	c, err := r.Cookie(auth.SessionCookie)
	if err != nil {
		return nil, nil, err
	}
	return s.auth.LookupSession(c.Value)
}

func clientKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
