package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/krate-apps/fleet/internal/storage"
)

const (
	SessionCookie = "fleet_session"
	CSRFCookie    = "fleet_csrf"
	CSRFHeader    = "X-CSRF-Token"
	sessionTTL    = 12 * time.Hour
	maxFailures   = 5
	lockoutFor    = 5 * time.Minute
)

// Service owns users, sessions, and login rate limiting (SEC-001…006).
type Service struct {
	store  *storage.Store
	params Argon2Params
	secure bool // set Secure on cookies when behind HTTPS

	mu       sync.Mutex
	failures map[string]loginAttempt
}

type loginAttempt struct {
	count    int
	lockedUntil time.Time
}

// User is a Fleet account for this instance.
type User struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	IsAdmin      bool   `json:"isAdmin"`
	CreatedAt    string `json:"createdAt"`
	PasswordHash string `json:"-"`
}

// Session is a server-side session row.
type Session struct {
	ID        string
	UserID    int64
	CSRFToken string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// NewService constructs the auth service and migrates tables.
func NewService(store *storage.Store, secureCookies bool) (*Service, error) {
	s := &Service{
		store:    store,
		params:   DefaultArgon2Params(),
		secure:   secureCookies,
		failures: make(map[string]loginAttempt),
	}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Service) migrate() error {
	_, err := s.store.DB().Exec(`
CREATE TABLE IF NOT EXISTS users (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	username TEXT NOT NULL UNIQUE COLLATE NOCASE,
	password_hash TEXT NOT NULL,
	is_admin INTEGER NOT NULL DEFAULT 1,
	created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS sessions (
	id TEXT PRIMARY KEY NOT NULL,
	user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	csrf_token TEXT NOT NULL,
	expires_at TEXT NOT NULL,
	created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);
`)
	return err
}

// NeedsSetup reports whether the first administrator must still be created (SEC-002).
func (s *Service) NeedsSetup() (bool, error) {
	var n int
	err := s.store.DB().QueryRow(`SELECT COUNT(1) FROM users`).Scan(&n)
	if err != nil {
		return false, err
	}
	return n == 0, nil
}

// CreateAdmin creates the first administrator. Fails if any user already exists.
func (s *Service) CreateAdmin(username, password string) (*User, error) {
	needs, err := s.NeedsSetup()
	if err != nil {
		return nil, err
	}
	if !needs {
		return nil, fmt.Errorf("setup already completed")
	}
	username = strings.TrimSpace(username)
	if username == "" || len(password) < 8 {
		return nil, fmt.Errorf("username required and password must be at least 8 characters")
	}
	hash, err := HashPassword(password, s.params)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.store.DB().Exec(
		`INSERT INTO users(username, password_hash, is_admin, created_at) VALUES(?,?,1,?)`,
		username, hash, now,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &User{ID: id, Username: username, IsAdmin: true, CreatedAt: now}, nil
}

// Authenticate verifies credentials and returns the user (SEC-003).
func (s *Service) Authenticate(username, password string) (*User, error) {
	u, err := s.findUser(username)
	if err != nil {
		return nil, err
	}
	ok, err := VerifyPassword(u.PasswordHash, password)
	if err != nil || !ok {
		return nil, fmt.Errorf("invalid credentials")
	}
	return u, nil
}

func (s *Service) findUser(username string) (*User, error) {
	var u User
	var admin int
	err := s.store.DB().QueryRow(
		`SELECT id, username, password_hash, is_admin, created_at FROM users WHERE username = ? COLLATE NOCASE`,
		strings.TrimSpace(username),
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &admin, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("invalid credentials")
	}
	if err != nil {
		return nil, err
	}
	u.IsAdmin = admin != 0
	return &u, nil
}

// CreateSession issues a new session for the user.
func (s *Service) CreateSession(userID int64) (*Session, error) {
	id, err := randomToken(32)
	if err != nil {
		return nil, err
	}
	csrf, err := randomToken(32)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	exp := now.Add(sessionTTL)
	_, err = s.store.DB().Exec(
		`INSERT INTO sessions(id, user_id, csrf_token, expires_at, created_at) VALUES(?,?,?,?,?)`,
		id, userID, csrf, exp.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, err
	}
	return &Session{ID: id, UserID: userID, CSRFToken: csrf, ExpiresAt: exp, CreatedAt: now}, nil
}

// LookupSession returns a valid session by id.
func (s *Service) LookupSession(id string) (*Session, *User, error) {
	if strings.TrimSpace(id) == "" {
		return nil, nil, fmt.Errorf("not authenticated")
	}
	var sess Session
	var expStr, createdStr string
	err := s.store.DB().QueryRow(
		`SELECT id, user_id, csrf_token, expires_at, created_at FROM sessions WHERE id = ?`, id,
	).Scan(&sess.ID, &sess.UserID, &sess.CSRFToken, &expStr, &createdStr)
	if err == sql.ErrNoRows {
		return nil, nil, fmt.Errorf("not authenticated")
	}
	if err != nil {
		return nil, nil, err
	}
	exp, err := time.Parse(time.RFC3339Nano, expStr)
	if err != nil {
		return nil, nil, err
	}
	if time.Now().UTC().After(exp) {
		_ = s.DeleteSession(id)
		return nil, nil, fmt.Errorf("session expired")
	}
	sess.ExpiresAt = exp
	var u User
	var admin int
	err = s.store.DB().QueryRow(
		`SELECT id, username, password_hash, is_admin, created_at FROM users WHERE id = ?`, sess.UserID,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &admin, &u.CreatedAt)
	if err != nil {
		return nil, nil, err
	}
	u.IsAdmin = admin != 0
	return &sess, &u, nil
}

// DeleteSession invalidates one session (logout).
func (s *Service) DeleteSession(id string) error {
	_, err := s.store.DB().Exec(`DELETE FROM sessions WHERE id = ?`, id)
	return err
}

// CheckLoginAllowed enforces temporary lockout after repeated failures (SEC-006).
func (s *Service) CheckLoginAllowed(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.failures[key]
	if !a.lockedUntil.IsZero() && time.Now().Before(a.lockedUntil) {
		return fmt.Errorf("too many login attempts; try again later")
	}
	return nil
}

// RecordLoginFailure increments the failure counter for key.
func (s *Service) RecordLoginFailure(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.failures[key]
	a.count++
	if a.count >= maxFailures {
		a.lockedUntil = time.Now().Add(lockoutFor)
		a.count = 0
	}
	s.failures[key] = a
}

// RecordLoginSuccess clears the failure counter for key.
func (s *Service) RecordLoginSuccess(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.failures, key)
}

// SetSessionCookies writes session and CSRF cookies (SEC-004).
// Secure is set when auth.secure_cookies is true or the request is HTTPS
// via X-Forwarded-Proto (typical reverse-proxy deployments).
func (s *Service) SetSessionCookies(w http.ResponseWriter, r *http.Request, sess *Session) {
	secure := s.secure || requestIndicatesHTTPS(r)
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    sess.ID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		Expires:  sess.ExpiresAt,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     CSRFCookie,
		Value:    sess.CSRFToken,
		Path:     "/",
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		Expires:  sess.ExpiresAt,
	})
}

// ClearSessionCookies removes auth cookies.
func (s *Service) ClearSessionCookies(w http.ResponseWriter, r *http.Request) {
	expired := time.Unix(0, 0).UTC()
	secure := s.secure || requestIndicatesHTTPS(r)
	http.SetCookie(w, &http.Cookie{Name: SessionCookie, Value: "", Path: "/", HttpOnly: true, Secure: secure, Expires: expired, MaxAge: -1})
	http.SetCookie(w, &http.Cookie{Name: CSRFCookie, Value: "", Path: "/", Secure: secure, Expires: expired, MaxAge: -1})
}

func requestIndicatesHTTPS(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https")
}

// ValidateCSRF checks the double-submit CSRF token (SEC-005).
// The token must arrive in X-CSRF-Token (JS reads the non-HttpOnly cookie); cookie-only
// submission is rejected so cross-site form posts cannot satisfy the check.
func (s *Service) ValidateCSRF(r *http.Request, sess *Session) error {
	token := r.Header.Get(CSRFHeader)
	if token == "" || sess == nil || token != sess.CSRFToken {
		return fmt.Errorf("csrf validation failed")
	}
	return nil
}

// AllowedWebSocketOrigin validates Origin against the request Host (SEC-005).
// Browser clients must send Origin; the host must match exactly (no prefix spoofing).
func AllowedWebSocketOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		// Non-browser clients (CLI) omit Origin; browsers always send it for WS.
		return true
	}
	host := strings.TrimSpace(r.Host)
	if host == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	originHost := u.Host
	if originHost == "" {
		return false
	}
	return strings.EqualFold(originHost, host)
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
