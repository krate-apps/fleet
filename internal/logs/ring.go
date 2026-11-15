package logs

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

const defaultRingSize = 2000

// Ring is a bounded circular buffer of Fleet slog records.
type Ring struct {
	mu   sync.RWMutex
	buf  []Entry
	size int
	next int
	full bool
}

// NewRing constructs a ring with the given capacity (minimum 64).
func NewRing(size int) *Ring {
	if size < 64 {
		size = defaultRingSize
	}
	return &Ring{buf: make([]Entry, size), size: size}
}

// Handler returns an slog.Handler that mirrors records into the ring.
func (r *Ring) Handler(level slog.Leveler) slog.Handler {
	if level == nil {
		level = slog.LevelInfo
	}
	return &ringHandler{ring: r, level: level}
}

// Snapshot returns a chronological copy of buffered entries (oldest first).
func (r *Ring) Snapshot(limit int) []Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	n := r.size
	if !r.full {
		n = r.next
	}
	if n == 0 {
		return nil
	}
	out := make([]Entry, 0, n)
	start := 0
	if r.full {
		start = r.next
	}
	for i := 0; i < n; i++ {
		out = append(out, r.buf[(start+i)%r.size])
	}
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out
}

func (r *Ring) append(e Entry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf[r.next] = e
	r.next = (r.next + 1) % r.size
	if r.next == 0 {
		r.full = true
	}
}

type ringHandler struct {
	ring  *Ring
	level slog.Leveler
	attrs []slog.Attr
	group string
}

func (h *ringHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *ringHandler) Handle(_ context.Context, rec slog.Record) error {
	attrs := make(map[string]string, len(h.attrs)+rec.NumAttrs())
	for _, a := range h.attrs {
		attrs[a.Key] = attrString(a.Value)
	}
	rec.Attrs(func(a slog.Attr) bool {
		key := a.Key
		if h.group != "" {
			key = h.group + "." + key
		}
		attrs[key] = attrString(a.Value)
		return true
	})
	h.ring.append(Entry{
		TS:         rec.Time.UTC(),
		Level:      levelName(rec.Level),
		Message:    rec.Message,
		Attrs:      attrs,
		SourceID:   "fleet",
		SourceKind: "fleet",
		SourceName: "Fleet",
		Stream:     "ring",
	})
	return nil
}

func (h *ringHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := append([]slog.Attr{}, h.attrs...)
	next = append(next, attrs...)
	return &ringHandler{ring: h.ring, level: h.level, attrs: next, group: h.group}
}

func (h *ringHandler) WithGroup(name string) slog.Handler {
	g := name
	if h.group != "" {
		g = h.group + "." + name
	}
	return &ringHandler{ring: h.ring, level: h.level, attrs: h.attrs, group: g}
}

func levelName(level slog.Level) string {
	switch {
	case level >= slog.LevelError:
		return "error"
	case level >= slog.LevelWarn:
		return "warn"
	case level >= slog.LevelInfo:
		return "info"
	default:
		return "debug"
	}
}

func attrString(v slog.Value) string {
	switch v.Kind() {
	case slog.KindString:
		return v.String()
	case slog.KindTime:
		return v.Time().UTC().Format(time.RFC3339Nano)
	default:
		return v.String()
	}
}

// MultiHandler fans records to multiple handlers (stdout + ring).
type MultiHandler struct {
	handlers []slog.Handler
}

// NewMultiHandler builds a fan-out handler.
func NewMultiHandler(handlers ...slog.Handler) *MultiHandler {
	out := make([]slog.Handler, 0, len(handlers))
	for _, h := range handlers {
		if h != nil {
			out = append(out, h)
		}
	}
	return &MultiHandler{handlers: out}
}

func (m *MultiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *MultiHandler) Handle(ctx context.Context, rec slog.Record) error {
	var first error
	for _, h := range m.handlers {
		if !h.Enabled(ctx, rec.Level) {
			continue
		}
		if err := h.Handle(ctx, rec.Clone()); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (m *MultiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		next[i] = h.WithAttrs(attrs)
	}
	return &MultiHandler{handlers: next}
}

func (m *MultiHandler) WithGroup(name string) slog.Handler {
	next := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		next[i] = h.WithGroup(name)
	}
	return &MultiHandler{handlers: next}
}
