package extensions

import (
	"fmt"
	"strings"
	"time"
)

const consentWarning = "Third-party extensions run as processes under your Unix account without a strong sandbox. Activate only software you trust."

// IsOfficialExtension reports Fleet-maintained reference packages (no consent gate).
func IsOfficialExtension(id string) bool {
	return strings.HasPrefix(id, "org.fleet.")
}

// ConsentRequired reports whether activation needs an explicit operator ack (SEC-008).
func (h *Host) ConsentRequired(id string) bool {
	return !IsOfficialExtension(id)
}

// HasConsent checks whether the operator accepted risks for this extension.
func (h *Host) HasConsent(id string) (bool, error) {
	if !h.ConsentRequired(id) {
		return true, nil
	}
	if h.store == nil {
		return false, nil
	}
	var v string
	err := h.store.DB().QueryRow(
		`SELECT value FROM meta WHERE key = ?`, consentKey(id),
	).Scan(&v)
	if err != nil {
		return false, nil
	}
	return v != "", nil
}

// GrantConsent records operator acknowledgment for a third-party extension.
func (h *Host) GrantConsent(id string) error {
	if !h.ConsentRequired(id) {
		return nil
	}
	h.mu.RLock()
	_, ok := h.records[id]
	h.mu.RUnlock()
	if !ok {
		return fmt.Errorf("extension not found")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := h.store.DB().Exec(
		`INSERT INTO meta(key, value) VALUES(?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		consentKey(id), now,
	)
	return err
}

// ConsentWarning is the fixed SEC-008 disclosure shown before activation.
func ConsentWarning() string {
	return consentWarning
}

func consentKey(id string) string {
	return "ext_consent:" + id
}

func (h *Host) annotateConsent(rec *Record) {
	if rec == nil {
		return
	}
	rec.Official = IsOfficialExtension(rec.ID)
	rec.ConsentRequired = h.ConsentRequired(rec.ID)
	if rec.ConsentRequired {
		rec.ConsentWarning = ConsentWarning()
		ok, _ := h.HasConsent(rec.ID)
		rec.Consented = ok
	} else {
		rec.Consented = true
	}
}
