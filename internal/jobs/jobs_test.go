package jobs

import (
	"testing"

	"github.com/krate-apps/fleet/internal/storage"
)

func TestManagerCreateGetCancel(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	m := NewManager(store)
	j, err := m.Create(CreateInput{Kind: "tools.dumptorrent", Payload: map[string]string{"file": "a.torrent"}})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if j.Status != StatusQueued {
		t.Fatalf("status = %q", j.Status)
	}

	got, err := m.Get(j.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Kind != "tools.dumptorrent" {
		t.Fatalf("kind = %q", got.Kind)
	}

	cancelled, err := m.Cancel(j.ID)
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if cancelled.Status != StatusCancelled {
		t.Fatalf("status = %q", cancelled.Status)
	}

	list, err := m.List(StatusCancelled, 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("list len = %d", len(list))
	}
}

func TestManagerCancelTerminalFails(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	m := NewManager(store)
	j, err := m.Create(CreateInput{Kind: "demo"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := m.Succeed(j.ID, map[string]bool{"ok": true}); err != nil {
		t.Fatalf("Succeed: %v", err)
	}
	if _, err := m.Cancel(j.ID); err == nil {
		t.Fatal("expected cancel error")
	}
}
