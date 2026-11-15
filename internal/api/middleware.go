package api

import (
	"net/http"
	"strings"

	"github.com/krate-apps/fleet/internal/auth"
)

func (s *Server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.auth == nil || !requiresAuth(r) {
			next.ServeHTTP(w, r)
			return
		}
		needs, err := s.auth.NeedsSetup()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if needs {
			if isSetupPath(r) {
				next.ServeHTTP(w, r)
				return
			}
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "setup required", "code": "needs_setup"})
			return
		}
		if isPublicAuthPath(r) {
			next.ServeHTTP(w, r)
			return
		}
		sess, _, err := s.sessionFromRequest(r)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			if err := s.auth.ValidateCSRF(r, sess); err != nil {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func requiresAuth(r *http.Request) bool {
	path := r.URL.Path
	if !strings.HasPrefix(path, "/api/") {
		return false
	}
	switch path {
	case "/api/v1/health", "/api/v1/version", "/api/v1/auth/status", "/api/v1/themes":
		return false
	default:
		// Installed theme CSS (needed before login for FOUC-free pack themes).
		if strings.HasPrefix(path, "/api/v1/themes/") && strings.HasSuffix(path, "/css") && r.Method == http.MethodGet {
			return false
		}
		// Public file-share download tokens (filemanager-share parity).
		if strings.HasPrefix(path, "/api/v1/share/") && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
			return false
		}
		// Public extension webhook hooks (arrbridge and future ingest endpoints).
		if strings.HasPrefix(path, "/api/v1/hooks/") && r.Method == http.MethodPost {
			return false
		}
		return true
	}
}

func isPublicAuthPath(r *http.Request) bool {
	switch r.URL.Path {
	case "/api/v1/auth/login", "/api/v1/auth/logout", "/api/v1/auth/status":
		return true
	default:
		return false
	}
}

func isSetupPath(r *http.Request) bool {
	return r.URL.Path == "/api/v1/auth/setup" || r.URL.Path == "/api/v1/auth/status"
}

// ensure auth package symbols stay referenced for cookie names in docs.
var _ = auth.SessionCookie
