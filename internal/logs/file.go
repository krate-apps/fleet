package logs

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// qBittorrent: (N) 2026-11-12T03:37:57 - message
var qbitLine = regexp.MustCompile(`^\(([A-Za-z])\)\s+(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})\s+-\s+(.*)$`)

// ValidateLogPath ensures path is a non-empty cleaned path under homeDir.
func ValidateLogPath(path, homeDir string) (string, error) {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" || path == "." {
		return "", fmt.Errorf("log path required")
	}
	homeDir = filepath.Clean(strings.TrimSpace(homeDir))
	if homeDir == "" {
		return "", fmt.Errorf("home dir required")
	}
	if !isUnder(path, homeDir) {
		return "", fmt.Errorf("log path must be under home directory")
	}
	return path, nil
}

// FirstExistingLogPath returns the first readable regular file under homeDir.
func FirstExistingLogPath(candidates []string, homeDir string) string {
	for _, c := range candidates {
		path, err := ValidateLogPath(c, homeDir)
		if err != nil {
			continue
		}
		st, err := os.Stat(path)
		if err != nil || !st.Mode().IsRegular() {
			continue
		}
		return path
	}
	return ""
}

// ReadLogFile tails the last limit lines from a path under homeDir.
func ReadLogFile(path, homeDir, sourceID, sourceKind, sourceName string, limit int) ([]Entry, error) {
	path, err := ValidateLogPath(path, homeDir)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("log path is not a regular file")
	}
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	lines := make([]string, 0, limit)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		lines = append(lines, sc.Text())
		if len(lines) > limit {
			lines = lines[1:]
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	out := make([]Entry, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		ts, level, msg := parseFileLine(line, now)
		out = append(out, Entry{
			TS:         ts,
			Level:      level,
			Message:    msg,
			SourceID:   sourceID,
			SourceKind: sourceKind,
			SourceName: sourceName,
			Stream:     "file",
			Attrs:      map[string]string{"path": path},
		})
	}
	return out, nil
}

func isUnder(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func parseFileLine(line string, fallback time.Time) (time.Time, string, string) {
	if m := qbitLine.FindStringSubmatch(line); m != nil {
		level := qbitLevel(m[1])
		ts := fallback
		if parsed, err := time.ParseInLocation("2006-01-02T15:04:05", m[2], time.Local); err == nil {
			ts = parsed.UTC()
		}
		return ts, level, m[3]
	}
	return fallback, guessFileLevel(line), line
}

func qbitLevel(code string) string {
	switch strings.ToUpper(code) {
	case "C", "E":
		return "error"
	case "W":
		return "warn"
	case "D":
		return "debug"
	case "I", "N":
		return "info"
	default:
		return "info"
	}
}

func guessFileLevel(line string) string {
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(lower, "error") || strings.Contains(lower, "fatal") || strings.Contains(lower, "critical"):
		return "error"
	case strings.Contains(lower, "warn"):
		return "warn"
	case strings.Contains(lower, "debug"):
		return "debug"
	default:
		return "info"
	}
}
