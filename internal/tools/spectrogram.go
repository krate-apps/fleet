package tools

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const spectrogramTimeout = 3 * time.Minute

// SpectrogramOpts controls sox spectrogram rendering (ruTorrent spectrogram parity).
type SpectrogramOpts struct {
	Title string
	Width int // optional -x pixels; 0 = sox default
}

// SpectrogramImage is the rendered PNG.
type SpectrogramImage struct {
	Mime    string `json:"mime"`
	DataURL string `json:"dataUrl"`
	Bytes   int    `json:"bytes"`
}

// Spectrogram runs sox on a path under AllowedRoots and returns a PNG data URL.
func (r *Runner) Spectrogram(ctx context.Context, path string, opts SpectrogramOpts) (*SpectrogramImage, error) {
	if r == nil {
		return nil, fmt.Errorf("tools runner unavailable")
	}
	abs, err := r.resolveAllowedPath(path)
	if err != nil {
		return nil, err
	}
	st, err := os.Lstat(abs)
	if err != nil {
		return nil, err
	}
	if st.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("refusing symlink path")
	}
	if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("path must be a regular file")
	}

	bin := r.lockedPath(func(r *Runner) string { return r.SoxPath })
	if bin == "" {
		bin = "sox"
	}
	resolved, err := exec.LookPath(bin)
	if err != nil {
		return nil, fmt.Errorf("sox binary not found (%q): %w", bin, err)
	}

	dir, err := os.MkdirTemp("", "fleet-spectrogram-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	out := filepath.Join(dir, "frame.png")

	title := strings.TrimSpace(opts.Title)
	if title == "" {
		title = filepath.Base(abs)
	}
	// Keep title free of control characters for -t.
	title = strings.Map(func(r rune) rune {
		if r < 32 {
			return -1
		}
		return r
	}, title)
	if len(title) > 120 {
		title = title[:120]
	}

	args := []string{abs, "-n", "spectrogram", "-t", title}
	if opts.Width > 0 {
		w := opts.Width
		if w > 4096 {
			w = 4096
		}
		args = append(args, "-x", fmt.Sprintf("%d", w))
	}
	args = append(args, "-o", out)

	cctx, cancel := context.WithTimeout(ctx, spectrogramTimeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, resolved, args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("sox: %s", msg)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("sox produced an empty spectrogram")
	}
	if len(raw) > 4<<20 {
		raw = raw[:4<<20]
	}
	return &SpectrogramImage{
		Mime:    "image/png",
		DataURL: "data:image/png;base64," + base64.StdEncoding.EncodeToString(raw),
		Bytes:   len(raw),
	}, nil
}
