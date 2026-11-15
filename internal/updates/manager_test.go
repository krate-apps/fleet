package updates

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/krate-apps/fleet/internal/storage"
)

func TestPrepareAndPlanApply(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	pkgDir := t.TempDir()
	archive := filepath.Join(pkgDir, "fleet.tar.gz")
	sum := writeArchive(t, archive, map[string]string{"fleet/VERSION": "9.9.9\n"})
	raw, _ := os.ReadFile(archive)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(raw)
	}))
	defer srv.Close()

	mgr := NewManager(store, dir)
	mgr.httpClient = srv.Client()
	pending, err := mgr.Prepare(context.Background(), Manifest{
		Version: "9.9.9",
		URL:     srv.URL + "/fleet.tar.gz",
		SHA256:  sum,
	})
	if err != nil {
		t.Fatal(err)
	}
	if pending.Version != "9.9.9" {
		t.Fatalf("%+v", pending)
	}
	st, err := mgr.Status()
	if err != nil || st.Pending == nil || st.AutoUpdate {
		t.Fatalf("%+v %v", st, err)
	}
	plan, err := mgr.PlanApply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if plan.BackupPath == "" || plan.PendingVersion != "9.9.9" {
		t.Fatalf("%+v", plan)
	}
	if _, err := os.Stat(plan.BackupPath); err != nil {
		t.Fatal(err)
	}
}

func writeArchive(t *testing.T, path string, files map[string]string) string {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(body))}
		_ = tw.WriteHeader(hdr)
		_, _ = io.WriteString(tw, body)
	}
	_ = tw.Close()
	_ = gz.Close()
	_ = f.Close()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
