// Package auth provides native per-instance authentication and sessions (Argon2id).
//
// SEC-001 native auth, SEC-002 first-admin setup, SEC-003 Argon2id hashes,
// SEC-004 HttpOnly session cookies, SEC-005 CSRF + WebSocket origin checks,
// SEC-006 login attempt lockout.
package auth
