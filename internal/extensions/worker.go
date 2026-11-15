package extensions

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/krate-apps/fleet/sdk/backend"
	"github.com/krate-apps/fleet/sdk/manifest"
)

const (
	defaultCallTimeout   = 10 * time.Second
	defaultMaxRestarts   = 3
	defaultRestartWindow = time.Minute
	maxLineBytes         = 8 << 20
)

// WorkerSession is a supervised JSON-RPC worker process for one extension.
type WorkerSession struct {
	ExtensionID string
	caps        []string
	bridge      HostBridge
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	stdout      *bufio.Scanner
	cancel      context.CancelFunc
	mu          sync.Mutex
	nextID      atomic.Int64
	pending     map[int64]chan rpcReply
	done        chan struct{}
	err         error
	restarts    []time.Time
}

type rpcReply struct {
	result json.RawMessage
	err    *backend.ErrorObject
}

// WorkerStartOptions configures process supervision.
type WorkerStartOptions struct {
	DataDir   string
	Bridge    HostBridge
	ExtraArgs []string
	Env       map[string]string
}

// StartWorker launches a backend process under the extension package directory.
func StartWorker(ctx context.Context, m *manifest.Manifest, packageDir string, opts WorkerStartOptions) (*WorkerSession, error) {
	if m == nil || m.Backend == nil {
		return nil, fmt.Errorf("extension has no backend")
	}
	exe := filepath.Join(packageDir, filepath.FromSlash(m.Backend.Executable))
	st, err := os.Stat(exe)
	if err != nil {
		return nil, fmt.Errorf("worker executable: %w", err)
	}
	if st.IsDir() || st.Mode()&0o111 == 0 {
		return nil, fmt.Errorf("worker executable is not runnable: %s", exe)
	}

	dataDir := opts.DataDir
	if dataDir == "" {
		dataDir = filepath.Join(packageDir, ".fleet-data")
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}

	runCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(runCtx, exe, opts.ExtraArgs...)
	cmd.Dir = packageDir
	cmd.Env = append(os.Environ(),
		"FLEET_EXTENSION_ID="+m.ID,
		"FLEET_EXTENSION_VERSION="+m.Version,
		"FLEET_EXTENSION_DATA_DIR="+dataDir,
	)
	for k, v := range opts.Env {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		cancel()
		return nil, err
	}
	cmd.Stderr = os.Stderr

	s := &WorkerSession{
		ExtensionID: m.ID,
		caps:        append([]string(nil), m.Capabilities...),
		bridge:      opts.Bridge,
		cmd:         cmd,
		stdin:       stdin,
		stdout:      bufio.NewScanner(stdoutPipe),
		cancel:      cancel,
		pending:     map[int64]chan rpcReply{},
		done:        make(chan struct{}),
	}
	s.stdout.Buffer(make([]byte, 0, 64*1024), maxLineBytes)

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		cancel()
		return nil, fmt.Errorf("start worker: %w", err)
	}
	go s.readLoop()
	go s.waitLoop()

	hs := backend.HandshakeParams{
		ProtocolVersion:  backend.ProtocolVersion,
		ExtensionID:      m.ID,
		ExtensionVersion: m.Version,
		Capabilities:     append([]string(nil), m.Capabilities...),
	}
	if _, err := s.Call(runCtx, backend.MethodHandshake, hs); err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("handshake: %w", err)
	}
	return s, nil
}

// Call sends a JSON-RPC request and waits for the matching response.
func (s *WorkerSession) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if s == nil {
		return nil, fmt.Errorf("worker unavailable")
	}
	select {
	case <-s.done:
		if s.err != nil {
			return nil, s.err
		}
		return nil, fmt.Errorf("worker exited")
	default:
	}

	id := s.nextID.Add(1)
	req, err := backend.NewRequest(method, id, params)
	if err != nil {
		return nil, err
	}
	ch := make(chan rpcReply, 1)
	s.mu.Lock()
	s.pending[id] = ch
	s.mu.Unlock()

	line, err := json.Marshal(req)
	if err != nil {
		s.clearPending(id)
		return nil, err
	}
	line = append(line, '\n')
	s.mu.Lock()
	_, err = s.stdin.Write(line)
	s.mu.Unlock()
	if err != nil {
		s.clearPending(id)
		return nil, err
	}

	callCtx := ctx
	var cancel context.CancelFunc
	if _, ok := ctx.Deadline(); !ok {
		callCtx, cancel = context.WithTimeout(ctx, defaultCallTimeout)
		defer cancel()
	}

	select {
	case <-callCtx.Done():
		s.clearPending(id)
		return nil, callCtx.Err()
	case <-s.done:
		s.clearPending(id)
		if s.err != nil {
			return nil, s.err
		}
		return nil, fmt.Errorf("worker exited during call")
	case rep := <-ch:
		if rep.err != nil {
			return nil, fmt.Errorf("rpc error %d: %s", rep.err.Code, rep.err.Message)
		}
		return rep.result, nil
	}
}

// Close asks the worker to shut down, then kills it if needed.
func (s *WorkerSession) Close() error {
	if s == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _ = s.Call(ctx, backend.MethodShutdown, map[string]any{})
	s.cancel()
	<-s.done
	return s.err
}

// Alive reports whether the worker process is still running.
func (s *WorkerSession) Alive() bool {
	if s == nil {
		return false
	}
	select {
	case <-s.done:
		return false
	default:
		return true
	}
}

// noteRestart records a restart and returns false when the crash-loop limit is exceeded.
func (s *WorkerSession) noteRestart(now time.Time) bool {
	cut := now.Add(-defaultRestartWindow)
	kept := s.restarts[:0]
	for _, t := range s.restarts {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}
	s.restarts = append(kept, now)
	return len(s.restarts) <= defaultMaxRestarts
}

func (s *WorkerSession) clearPending(id int64) {
	s.mu.Lock()
	delete(s.pending, id)
	s.mu.Unlock()
}

func (s *WorkerSession) readLoop() {
	for s.stdout.Scan() {
		line := append([]byte(nil), s.stdout.Bytes()...)
		var probe struct {
			Method string               `json:"method"`
			ID     any                  `json:"id"`
			Result json.RawMessage      `json:"result"`
			Error  *backend.ErrorObject `json:"error"`
			Params json.RawMessage      `json:"params"`
		}
		if err := json.Unmarshal(line, &probe); err != nil {
			continue
		}
		if probe.Method != "" {
			go s.handleWorkerRequest(probe.Method, probe.Params, probe.ID)
			continue
		}
		idNum, ok := asInt64(probe.ID)
		if !ok {
			continue
		}
		s.mu.Lock()
		ch, ok := s.pending[idNum]
		if ok {
			delete(s.pending, idNum)
		}
		s.mu.Unlock()
		if !ok {
			continue
		}
		ch <- rpcReply{result: probe.Result, err: probe.Error}
	}
}

func (s *WorkerSession) handleWorkerRequest(method string, params json.RawMessage, id any) {
	resp := backend.Response{JSONRPC: "2.0", ID: id}
	if s.bridge == nil {
		resp.Error = &backend.ErrorObject{Code: -32001, Message: "host bridge unavailable"}
	} else {
		result, err := s.bridge.Handle(context.Background(), s.ExtensionID, s.caps, method, params)
		if err != nil {
			resp.Error = &backend.ErrorObject{Code: -32002, Message: err.Error()}
		} else if result != nil {
			raw, err := json.Marshal(result)
			if err != nil {
				resp.Error = &backend.ErrorObject{Code: -32603, Message: err.Error()}
			} else {
				resp.Result = raw
			}
		} else {
			resp.Result = json.RawMessage("null")
		}
	}
	line, err := json.Marshal(resp)
	if err != nil {
		return
	}
	line = append(line, '\n')
	s.mu.Lock()
	_, _ = s.stdin.Write(line)
	s.mu.Unlock()
}

func (s *WorkerSession) waitLoop() {
	err := s.cmd.Wait()
	s.mu.Lock()
	if err != nil {
		s.err = fmt.Errorf("worker exited: %w", err)
	} else {
		s.err = fmt.Errorf("worker exited")
	}
	for id, ch := range s.pending {
		ch <- rpcReply{err: &backend.ErrorObject{Code: -32000, Message: "worker exited"}}
		delete(s.pending, id)
	}
	s.mu.Unlock()
	_ = s.stdin.Close()
	close(s.done)
}

func asInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case int:
		return int64(n), true
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	default:
		return 0, false
	}
}
