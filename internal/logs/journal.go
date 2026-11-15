package logs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// JournalRunner executes journalctl (overridable in tests).
type JournalRunner func(ctx context.Context, unit string, limit int) ([]byte, error)

func defaultJournalRunner(ctx context.Context, unit string, limit int) ([]byte, error) {
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}
	cmd := exec.CommandContext(ctx, "journalctl",
		"-u", unit+".service",
		"-o", "json",
		"-n", strconv.Itoa(limit),
		"--no-pager",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("journalctl: %s", msg)
	}
	return stdout.Bytes(), nil
}

// ReadJournal pulls recent lines for an allowlisted unit.
func ReadJournal(ctx context.Context, unit string, limit int, run JournalRunner) ([]Entry, error) {
	unit, err := ValidateUnit(unit)
	if err != nil {
		return nil, err
	}
	if run == nil {
		run = defaultJournalRunner
	}
	raw, err := run(ctx, unit, limit)
	if err != nil {
		return nil, err
	}
	return parseJournalJSON(raw, unit)
}

func parseJournalJSON(raw []byte, unit string) ([]Entry, error) {
	lines := bytes.Split(raw, []byte("\n"))
	out := make([]Entry, 0, len(lines))
	kind, name := splitUnit(unit)
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var row map[string]any
		if err := json.Unmarshal(line, &row); err != nil {
			continue
		}
		msg, _ := row["MESSAGE"].(string)
		if msg == "" {
			continue
		}
		ts := journalTime(row)
		level := journalLevel(row)
		attrs := map[string]string{"unit": unit}
		ts, level, msg, extra := unwrapStructuredMessage(msg, ts, level)
		for k, v := range extra {
			attrs[k] = v
		}
		out = append(out, Entry{
			TS:         ts,
			Level:      level,
			Message:    msg,
			SourceID:   "journal:" + unit,
			SourceKind: kind,
			SourceName: name,
			Stream:     "journal",
			Attrs:      attrs,
		})
	}
	return out, nil
}

// unwrapStructuredMessage expands slog/zerolog-style JSON payloads captured by journald.
func unwrapStructuredMessage(msg string, ts time.Time, level string) (time.Time, string, string, map[string]string) {
	msg = strings.TrimSpace(msg)
	if !strings.HasPrefix(msg, "{") {
		return ts, level, msg, nil
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(msg), &payload); err != nil {
		return ts, level, msg, nil
	}
	text, _ := payload["msg"].(string)
	if text == "" {
		text, _ = payload["message"].(string)
	}
	if text == "" {
		return ts, level, msg, nil
	}
	if lv, ok := payload["level"].(string); ok && lv != "" {
		level = strings.ToLower(lv)
	}
	if t, ok := payload["time"].(string); ok {
		if parsed, err := time.Parse(time.RFC3339Nano, t); err == nil {
			ts = parsed.UTC()
		} else if parsed, err := time.Parse(time.RFC3339, t); err == nil {
			ts = parsed.UTC()
		}
	}
	attrs := make(map[string]string)
	for k, v := range payload {
		switch k {
		case "msg", "message", "level", "time", "ts", "timestamp", "source":
			continue
		}
		attrs[k] = fmt.Sprint(v)
	}
	return ts, level, text, attrs
}

func splitUnit(unit string) (kind, name string) {
	parts := strings.SplitN(unit, "@", 2)
	if len(parts) != 2 {
		return "unknown", unit
	}
	kind = parts[0]
	switch kind {
	case "fleet":
		name = "Fleet"
	case "rtorrent":
		name = "rTorrent"
	case "deluge":
		name = "Deluge"
	case "qbittorrent":
		name = "qBittorrent"
	case "transmission":
		name = "Transmission"
	default:
		name = kind
	}
	return kind, name + " (" + parts[1] + ")"
}

func journalTime(row map[string]any) time.Time {
	switch v := row["__REALTIME_TIMESTAMP"].(type) {
	case string:
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			// microseconds since epoch
			return time.UnixMicro(n).UTC()
		}
	case float64:
		return time.UnixMicro(int64(v)).UTC()
	}
	return time.Now().UTC()
}

func journalLevel(row map[string]any) string {
	prio := ""
	switch v := row["PRIORITY"].(type) {
	case string:
		prio = v
	case float64:
		prio = strconv.Itoa(int(v))
	}
	switch prio {
	case "0", "1", "2", "3":
		return "error"
	case "4":
		return "warn"
	case "7":
		return "debug"
	default:
		return "info"
	}
}
