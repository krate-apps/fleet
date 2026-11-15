package extensions

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var forbiddenExecBases = map[string]struct{}{
	"sh": {}, "bash": {}, "dash": {}, "zsh": {}, "fish": {}, "csh": {}, "tcsh": {},
	"cmd": {}, "cmd.exe": {}, "powershell": {}, "pwsh": {},
	"python": {}, "python2": {}, "python3": {}, "perl": {}, "ruby": {}, "php": {}, "node": {}, "nodejs": {},
	"curl": {}, "wget": {}, "nc": {}, "ncat": {}, "netcat": {}, "socat": {},
}

const (
	processExecMaxOutput = 64 << 10
	processExecMaxArgs   = 32
)

var (
	processExecMu       sync.RWMutex
	processExecExact    = map[string]struct{}{}
	processExecAllowDir []string
)

// SetProcessExecPolicy configures absolute binaries / directories allowed for fleet.process.exec.
// Empty policy denies all exec (fail closed). Shells and common interpreters stay denied
// even when listed, unless you only use ProcessExecDirs for a dedicated hooks tree of non-shell tools.
func SetProcessExecPolicy(exactPaths, allowDirs []string) {
	exact := make(map[string]struct{}, len(exactPaths))
	for _, p := range exactPaths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		cleaned := filepath.Clean(p)
		if resolved, err := filepath.EvalSymlinks(cleaned); err == nil {
			cleaned = resolved
		}
		exact[cleaned] = struct{}{}
	}
	dirs := make([]string, 0, len(allowDirs))
	for _, d := range allowDirs {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		cleaned := filepath.Clean(d)
		if resolved, err := filepath.EvalSymlinks(cleaned); err == nil {
			cleaned = resolved
		}
		dirs = append(dirs, cleaned)
	}
	processExecMu.Lock()
	processExecExact = exact
	processExecAllowDir = dirs
	processExecMu.Unlock()
}

func execPolicyAllows(resolved string) bool {
	processExecMu.RLock()
	defer processExecMu.RUnlock()
	if _, ok := processExecExact[resolved]; ok {
		return true
	}
	for _, dir := range processExecAllowDir {
		rel, err := filepath.Rel(dir, resolved)
		if err != nil {
			continue
		}
		if rel == "." {
			continue
		}
		if rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

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
	if !execPolicyAllows(resolved) {
		return "", fmt.Errorf("binary not on process_exec allowlist (configure security.process_exec_allowlist or process_exec_dirs)")
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
		return out, nil
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
