package extensions

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var forbiddenExecBases = map[string]struct{}{
	"sh": {}, "bash": {}, "dash": {}, "zsh": {}, "fish": {}, "csh": {}, "tcsh": {},
	"cmd": {}, "cmd.exe": {}, "powershell": {}, "pwsh": {},
}

const (
	processExecMaxOutput = 64 << 10
	processExecMaxArgs   = 32
)

func validateExecPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("path required")
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("path must be absolute")
	}
	if strings.Contains(path, "\x00") {
		return "", fmt.Errorf("invalid path")
	}
	cleaned := filepath.Clean(path)
	resolved, err := filepath.EvalSymlinks(cleaned)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	if !filepath.IsAbs(resolved) {
		return "", fmt.Errorf("resolved path must be absolute")
	}
	base := strings.ToLower(filepath.Base(resolved))
	if _, bad := forbiddenExecBases[base]; bad {
		return "", fmt.Errorf("binary %q not allowed", base)
	}
	st, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if st.IsDir() {
		return "", fmt.Errorf("path is a directory")
	}
	return resolved, nil
}

func runAllowlistedExec(ctx context.Context, path string, args []string, timeoutSec int) (map[string]any, error) {
	resolved, err := validateExecPath(path)
	if err != nil {
		return nil, err
	}
	if len(args) > processExecMaxArgs {
		return nil, fmt.Errorf("too many args")
	}
	for _, a := range args {
		if strings.Contains(a, "\x00") {
			return nil, fmt.Errorf("invalid arg")
		}
	}
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	if timeoutSec > 120 {
		timeoutSec = 120
	}
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(runCtx, resolved, args...)
	cmd.Stdin = nil
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &limitWriter{limit: processExecMaxOutput, buf: &stdout}
	cmd.Stderr = &limitWriter{limit: processExecMaxOutput, buf: &stderr}
	err = cmd.Run()
	out := map[string]any{
		"ok":       err == nil,
		"path":     resolved,
		"args":     args,
		"stdout":   stdout.String(),
		"stderr":   stderr.String(),
		"timedOut": runCtx.Err() == context.DeadlineExceeded,
	}
	if err != nil {
		out["error"] = err.Error()
		if ee, ok := err.(*exec.ExitError); ok {
			out["exitCode"] = ee.ExitCode()
		}
		return out, nil // return result payload; caller treats ok=false
	}
	out["exitCode"] = 0
	return out, nil
}

type limitWriter struct {
	limit int
	buf   *bytes.Buffer
	n     int
}

func (w *limitWriter) Write(p []byte) (int, error) {
	remain := w.limit - w.n
	if remain <= 0 {
		return len(p), nil
	}
	if len(p) > remain {
		p = p[:remain]
	}
	n, err := w.buf.Write(p)
	w.n += n
	return len(p), err
}
