package tools

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const screenshotsTimeout = 3 * time.Minute

// ScreenshotOpts controls ffmpeg frame extraction (ruTorrent screenshots parity).
type ScreenshotOpts struct {
	Count    int // default 6, max 12
	Offset   int // seconds before first frame (default 5)
	Interval int // seconds between frames (default 5)
	Width    int // scale width; 0 keeps ffmpeg default scale filter without width
}

// ScreenshotFrame is one extracted still.
type ScreenshotFrame struct {
	Index    int    `json:"index"`
	Offset   int    `json:"offsetSec"`
	Mime     string `json:"mime"`
	DataURL  string `json:"dataUrl"`
	Bytes    int    `json:"bytes"`
}

// Screenshots runs ffmpeg on a path under AllowedRoots and returns JPEG data URLs.
func (r *Runner) Screenshots(ctx context.Context, path string, opts ScreenshotOpts) ([]ScreenshotFrame, error) {
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

	count := opts.Count
	if count <= 0 {
		count = 6
	}
	if count > 12 {
		count = 12
	}
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}
	interval := opts.Interval
	if interval <= 0 {
		interval = 5
	}
	width := opts.Width
	if width < 0 {
		width = 0
	}
	if width > 1920 {
		width = 1920
	}

	bin := strings.TrimSpace(r.FfmpegPath)
	if bin == "" {
		bin = "ffmpeg"
	}
	resolved, err := exec.LookPath(bin)
	if err != nil {
		return nil, fmt.Errorf("ffmpeg binary not found (%q): %w", bin, err)
	}

	dir, err := os.MkdirTemp("", "fleet-screenshots-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	cctx, cancel := context.WithTimeout(ctx, screenshotsTimeout)
	defer cancel()

	frames := make([]ScreenshotFrame, 0, count)
	ss := offset
	for i := 0; i < count; i++ {
		out := filepath.Join(dir, fmt.Sprintf("frame%02d.jpg", i))
		args := []string{
			"-hide_banner", "-loglevel", "error",
			"-ss", strconv.Itoa(ss),
			"-i", abs,
			"-y",
			"-vframes", "1",
			"-an", "-sn",
		}
		if width > 0 {
			args = append(args, "-vf", fmt.Sprintf("scale=%d:-1", width))
		} else {
			args = append(args, "-vf", `scale='max(sar,1)*iw':'max(1/sar,1)*ih'`)
		}
		args = append(args, out)
		cmd := exec.CommandContext(cctx, resolved, args...)
		var stderr strings.Builder
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			msg := strings.TrimSpace(stderr.String())
			if msg == "" {
				msg = err.Error()
			}
			// Stop early if seek past end; keep frames already captured.
			if len(frames) > 0 {
				break
			}
			return nil, fmt.Errorf("ffmpeg: %s", msg)
		}
		raw, err := os.ReadFile(out)
		if err != nil {
			return nil, err
		}
		if len(raw) == 0 {
			ss += interval
			continue
		}
		if len(raw) > 2<<20 {
			raw = raw[:2<<20]
		}
		frames = append(frames, ScreenshotFrame{
			Index:   i,
			Offset:  ss,
			Mime:    "image/jpeg",
			DataURL: "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(raw),
			Bytes:   len(raw),
		})
		ss += interval
	}
	if len(frames) == 0 {
		return nil, fmt.Errorf("ffmpeg produced no frames")
	}
	return frames, nil
}
