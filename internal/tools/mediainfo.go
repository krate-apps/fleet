package tools

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const mediaInfoTimeout = 2 * time.Minute

// MediaInfo runs the mediainfo CLI on a path under AllowedRoots (no shell).
func (r *Runner) MediaInfo(ctx context.Context, path string) (string, error) {
	if r == nil {
		return "", fmt.Errorf("tools runner unavailable")
	}
	abs, err := r.resolveAllowedPath(path)
	if err != nil {
		return "", err
	}
	st, err := os.Lstat(abs)
	if err != nil {
		return "", err
	}
	if st.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("refusing symlink path")
	}
	if !st.Mode().IsRegular() {
		return "", fmt.Errorf("path must be a regular file")
	}

	bin := strings.TrimSpace(r.MediainfoPath)
	if bin == "" {
		bin = "mediainfo"
	}
	resolved, err := exec.LookPath(bin)
	if err != nil {
		return "", fmt.Errorf("mediainfo binary not found (%q): %w", bin, err)
	}

	cctx, cancel := context.WithTimeout(ctx, mediaInfoTimeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, resolved, abs)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("mediainfo: %s", msg)
	}
	out := stdout.String()
	if len(out) > maxOutputBytes {
		out = out[:maxOutputBytes]
	}
	return out, nil
}
