package rtorrent

import (
	"context"
	"testing"
	"time"
)

func TestProbeMatrixDisconnected(t *testing.T) {
	c, err := NewClient("unix:///tmp/fleet-rtprobe-does-not-exist.sock")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	row := c.ProbeMatrix(ctx)
	if row.Connected {
		t.Fatalf("expected disconnected: %+v", row)
	}
	if row.Address == "" || row.ProbedAt == "" {
		t.Fatalf("%+v", row)
	}
}
