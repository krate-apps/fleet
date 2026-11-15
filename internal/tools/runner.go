package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultTimeout     = 30 * time.Second
	createTimeout      = 10 * time.Minute
	maxOutputBytes     = 8 << 20 // 8 MiB
	maxTorrentDownload = 64 << 20
)

// Runner executes embedded/configured helper binaries without a shell.
type Runner struct {
	MktorrentPath   string
	DumptorrentPath string
	MediainfoPath   string
	FfmpegPath      string
	SoxPath         string
	// AllowedRoots limits filesystem paths for mktorrent source inputs.
	AllowedRoots []string
	// ArtifactsDir stores created .torrent files for download (job artifacts).
	ArtifactsDir string
}

// CreateInput is a validated mktorrent request (CORE-014).
type CreateInput struct {
	Paths       []string `json:"paths"`
	Announce    []string `json:"announce"`
	PieceLength int      `json:"pieceLength,omitempty"` // bytes; 0 = tool default
	Private     bool     `json:"private,omitempty"`
	Name        string   `json:"name,omitempty"`
	Comment     string   `json:"comment,omitempty"`
	OutputName  string   `json:"outputName,omitempty"`
}

// CreateResult is returned after a successful create (JSON and/or file artifact).
type CreateResult struct {
	OutputPath    string         `json:"outputPath"`
	TorrentBase64 string         `json:"torrentBase64,omitempty"`
	Meta          map[string]any `json:"meta,omitempty"`
	JSONCLI       bool           `json:"jsonCli"`
}

// DumpResult is the parsed JSON object returned by dumptorrent -j / --json when available.
type DumpResult struct {
	Raw    json.RawMessage `json:"raw"`
	Parsed map[string]any  `json:"parsed,omitempty"`
}

// Status reports whether helpers are configured and executable.
func (r *Runner) Status() map[string]any {
	mediaPath := strings.TrimSpace(r.MediainfoPath)
	if mediaPath == "" {
		if p, err := exec.LookPath("mediainfo"); err == nil {
			mediaPath = p
		}
	}
	ffmpegPath := strings.TrimSpace(r.FfmpegPath)
	if ffmpegPath == "" {
		if p, err := exec.LookPath("ffmpeg"); err == nil {
			ffmpegPath = p
		}
	}
	soxPath := strings.TrimSpace(r.SoxPath)
	if soxPath == "" {
		if p, err := exec.LookPath("sox"); err == nil {
			soxPath = p
		}
	}
	return map[string]any{
		"mktorrent":   toolStatus(r.MktorrentPath),
		"dumptorrent": toolStatus(r.DumptorrentPath),
		"mediainfo":   toolStatus(mediaPath),
		"ffmpeg":      toolStatus(ffmpegPath),
		"sox":         toolStatus(soxPath),
	}
}

func toolStatus(path string) map[string]any {
	out := map[string]any{"path": path, "available": false}
	if path == "" {
		out["message"] = "not configured"
		return out
	}
	st, err := os.Stat(path)
	if err != nil {
		out["message"] = err.Error()
		return out
	}
	if st.IsDir() || st.Mode()&0o111 == 0 {
		out["message"] = "not an executable file"
		return out
	}
	out["available"] = true
	return out
}

// DumpTorrent inspects a .torrent payload via dumptorrent.
// Prefers JSON flags; if the binary rejects them, returns a clear capability error (OPEN-11).
func (r *Runner) DumpTorrent(ctx context.Context, torrentData []byte) (*DumpResult, error) {
	if len(torrentData) == 0 {
		return nil, fmt.Errorf("empty torrent data")
	}
	path := strings.TrimSpace(r.DumptorrentPath)
	if path == "" {
		return nil, fmt.Errorf("dumptorrent not configured (set tools.dumptorrent)")
	}
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("dumptorrent unavailable: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "fleet-dump-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	torrentPath := filepath.Join(tmpDir, "input.torrent")
	if err := os.WriteFile(torrentPath, torrentData, 0o600); err != nil {
		return nil, err
	}

	callCtx := ctx
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(ctx, defaultTimeout)
		defer cancel()
	}

	// Try common JSON flags first (forks may use -j or --json).
	for _, args := range [][]string{
		{"--json", torrentPath},
		{"-j", torrentPath},
		{"-J", torrentPath},
	} {
		out, err := runLimited(callCtx, path, args...)
		if err == nil {
			return parseJSONOutput(out)
		}
		// If the binary exists but rejects the flag, keep trying alternatives.
		if isExecFailure(err) {
			continue
		}
	}

	return nil, fmt.Errorf("dumptorrent JSON output not supported by %q (OPEN-11: verify fork CLI)", path)
}

// CreateTorrent builds a .torrent via the configured mktorrent JSON fork.
func (r *Runner) CreateTorrent(ctx context.Context, in CreateInput) (*CreateResult, error) {
	path := strings.TrimSpace(r.MktorrentPath)
	if path == "" {
		return nil, fmt.Errorf("mktorrent not configured (set tools.mktorrent)")
	}
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("mktorrent unavailable: %w", err)
	}
	if len(in.Paths) == 0 {
		return nil, fmt.Errorf("paths required")
	}
	if len(in.Announce) == 0 {
		return nil, fmt.Errorf("announce required")
	}
	cleanPaths := make([]string, 0, len(in.Paths))
	for _, p := range in.Paths {
		abs, err := r.resolveAllowedPath(p)
		if err != nil {
			return nil, err
		}
		cleanPaths = append(cleanPaths, abs)
	}
	for _, a := range in.Announce {
		if !isHTTPURL(a) {
			return nil, fmt.Errorf("invalid announce URL %q", a)
		}
	}
	if in.PieceLength < 0 || (in.PieceLength > 0 && (in.PieceLength&(in.PieceLength-1)) != 0) {
		return nil, fmt.Errorf("pieceLength must be 0 or a power of two")
	}

	artifacts := strings.TrimSpace(r.ArtifactsDir)
	if artifacts == "" {
		return nil, fmt.Errorf("artifacts dir not configured")
	}
	if err := os.MkdirAll(artifacts, 0o700); err != nil {
		return nil, err
	}

	outName := strings.TrimSpace(in.OutputName)
	if outName == "" {
		outName = "out.torrent"
	}
	outName = filepath.Base(outName)
	if !strings.HasSuffix(strings.ToLower(outName), ".torrent") {
		outName += ".torrent"
	}
	outPath := filepath.Join(artifacts, outName)

	args := []string{}
	for _, a := range in.Announce {
		args = append(args, "-a", a)
	}
	if in.PieceLength > 0 {
		// mktorrent -l is log2(piece length)
		l := 0
		for n := in.PieceLength; n > 1; n >>= 1 {
			l++
		}
		args = append(args, "-l", fmt.Sprintf("%d", l))
	}
	if in.Private {
		args = append(args, "-p")
	}
	if strings.TrimSpace(in.Name) != "" {
		args = append(args, "-n", in.Name)
	}
	if strings.TrimSpace(in.Comment) != "" {
		args = append(args, "-c", in.Comment)
	}
	args = append(args, "-o", outPath)
	args = append(args, cleanPaths...)

	callCtx := ctx
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(ctx, createTimeout)
		defer cancel()
	}

	// Prefer JSON-capable forks (OPEN-11).
	var (
		lastErr  error
		meta     map[string]any
		usedJSON bool
	)
	for _, flag := range []string{"--json", "-j", "-J"} {
		tryArgs := append([]string{flag}, args...)
		out, err := runLimited(callCtx, path, tryArgs...)
		if err != nil {
			lastErr = err
			if isExecFailure(err) {
				continue
			}
			return nil, err
		}
		usedJSON = true
		if dump, err := parseJSONOutput(out); err == nil {
			meta = dump.Parsed
			if p, ok := stringField(meta, "path", "output", "torrentPath", "file"); ok && p != "" {
				if abs, err := filepath.Abs(p); err == nil {
					if data, err := os.ReadFile(abs); err == nil && len(data) > 0 && abs != outPath {
						if err := os.WriteFile(outPath, data, 0o600); err != nil {
							return nil, err
						}
					}
				}
			}
			if b64, ok := stringField(meta, "torrentBase64", "torrent_base64", "torrent"); ok && b64 != "" {
				data, err := decodeMaybeBase64(b64)
				if err == nil && len(data) > 0 {
					if err := os.WriteFile(outPath, data, 0o600); err != nil {
						return nil, err
					}
				}
			}
		}
		break
	}
	if !usedJSON {
		if lastErr != nil {
			return nil, fmt.Errorf("mktorrent JSON output not supported by %q (OPEN-11: verify fork CLI): %w", path, lastErr)
		}
		return nil, fmt.Errorf("mktorrent JSON output not supported by %q (OPEN-11: verify fork CLI)", path)
	}

	if _, err := os.Stat(outPath); err != nil {
		return nil, fmt.Errorf("mktorrent JSON CLI did not produce %q: %w", outPath, err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		return nil, err
	}
	if len(data) > maxTorrentDownload {
		return nil, fmt.Errorf("created torrent exceeds %d bytes", maxTorrentDownload)
	}
	return &CreateResult{
		OutputPath:    outPath,
		TorrentBase64: base64.StdEncoding.EncodeToString(data),
		Meta:          meta,
		JSONCLI:       true,
	}, nil
}

func (r *Runner) resolveAllowedPath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", fmt.Errorf("empty path")
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)
	if _, err := os.Stat(abs); err != nil {
		return "", fmt.Errorf("path %q: %w", abs, err)
	}
	if len(r.AllowedRoots) == 0 {
		return "", fmt.Errorf("no download_roots configured; refuse path access")
	}
	for _, root := range r.AllowedRoots {
		root = filepath.Clean(root)
		if root == "" {
			continue
		}
		rel, err := filepath.Rel(root, abs)
		if err != nil {
			continue
		}
		if rel == "." || (!strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel)) {
			return abs, nil
		}
	}
	return "", fmt.Errorf("path %q is outside allowed download_roots", abs)
}

func isHTTPURL(u string) bool {
	u = strings.TrimSpace(strings.ToLower(u))
	return strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "udp://")
}

func stringField(m map[string]any, keys ...string) (string, bool) {
	if m == nil {
		return "", false
	}
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s, true
			}
		}
	}
	return "", false
}

func decodeMaybeBase64(s string) ([]byte, error) {
	// Accept raw base64 or data already looking like bencode (starts with d/l/i).
	if len(s) > 0 && (s[0] == 'd' || s[0] == 'l' || s[0] == 'i') {
		return []byte(s), nil
	}
	return base64.StdEncoding.DecodeString(s)
}

func parseJSONOutput(out []byte) (*DumpResult, error) {
	out = bytes.TrimSpace(out)
	if len(out) == 0 {
		return nil, fmt.Errorf("empty dumptorrent output")
	}
	if !json.Valid(out) {
		return nil, fmt.Errorf("dumptorrent output is not valid JSON")
	}
	res := &DumpResult{Raw: append(json.RawMessage(nil), out...)}
	var parsed map[string]any
	if err := json.Unmarshal(out, &parsed); err == nil {
		res.Parsed = parsed
	}
	return res, nil
}

func runLimited(ctx context.Context, bin string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdin = nil
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("%s", msg)
	}
	if stdout.Len() > maxOutputBytes {
		return nil, fmt.Errorf("tool output exceeds %d bytes", maxOutputBytes)
	}
	return stdout.Bytes(), nil
}

func isExecFailure(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "unknown") ||
		strings.Contains(s, "invalid") ||
		strings.Contains(s, "unrecognized") ||
		strings.Contains(s, "usage")
}
