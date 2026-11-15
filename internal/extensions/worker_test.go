package extensions

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/krate-apps/fleet/internal/storage"
	"github.com/krate-apps/fleet/sdk/backend"
)

func buildEchoWorker(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
	src := filepath.Join(repoRoot, "sdk/backend/cmd/echo-worker")
	out := filepath.Join(t.TempDir(), "echo-worker")
	cmd := exec.Command("go", "build", "-o", out, ".")
	cmd.Dir = src
	cmd.Env = os.Environ()
	if outBytes, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build echo-worker: %v\n%s", err, outBytes)
	}
	return out
}

func writeWorkerPackage(t *testing.T, root, id, name, workerBin string) {
	t.Helper()
	pkg := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Join(pkg, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(workerBin)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "bin", "worker"), data, 0o700); err != nil {
		t.Fatal(err)
	}
	manifest := []byte(`
schema: 1
id: ` + id + `
name: ` + name + `
version: 0.1.0
api: ">=1.0.0"
backend:
  type: process
  executable: bin/worker
  protocol: jsonrpc-2
`)
	if err := os.WriteFile(filepath.Join(pkg, "fleet.ext.yaml"), manifest, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestWorkerHandshakeAndHealth(t *testing.T) {
	bin := buildEchoWorker(t)
	root := t.TempDir()
	writeWorkerPackage(t, root, "org.fleet.demo", "demo", bin)

	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h := NewHostContext(ctx, store, root)
	if err := h.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	rec, err := h.Activate("org.fleet.demo")
	if err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if rec.State != StateActive {
		t.Fatalf("state = %q", rec.State)
	}

	h.mu.RLock()
	sess := h.workers["org.fleet.demo"]
	h.mu.RUnlock()
	if sess == nil || !sess.Alive() {
		t.Fatal("expected live worker")
	}
	if _, err := sess.Call(ctx, backend.MethodHealth, map[string]any{}); err != nil {
		t.Fatalf("health: %v", err)
	}
	if _, err := h.Deactivate("org.fleet.demo"); err != nil {
		t.Fatalf("Deactivate: %v", err)
	}
}

func TestWorkerCrashIsolatedFromHost(t *testing.T) {
	bin := buildEchoWorker(t)
	root := t.TempDir()
	writeWorkerPackage(t, root, "org.fleet.crash", "crashy", bin)

	// Crash during handshake must fail StartWorker without panicking.
	m, err := loadPackage(filepath.Join(root, "crashy"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := StartWorker(context.Background(), m.Manifest, m.Path, WorkerStartOptions{
		ExtraArgs: []string{"--crash-after-handshake"},
	}); err == nil {
		t.Fatal("expected start failure after crash")
	}

	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	h := NewHost(store, root)
	if err := h.Refresh(); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Activate("org.fleet.crash"); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	h.mu.RLock()
	sess := h.workers["org.fleet.crash"]
	h.mu.RUnlock()
	if sess == nil {
		t.Fatal("missing worker session")
	}
	// Ask the worker to exit itself so the host observes a crash without Process.Kill.
	_, _ = sess.Call(context.Background(), "ext.debug.exit", map[string]any{})
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		got, err := h.Get("org.fleet.crash")
		if err == nil && got.State == StateError {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("expected extension state error after worker kill")
}
