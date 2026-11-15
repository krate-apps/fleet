package sync

import (
	"fmt"
	"testing"

	"github.com/krate-apps/fleet/internal/rtorrent"
)

func makeTorrents(n int) []rtorrent.Torrent {
	out := make([]rtorrent.Torrent, n)
	for i := 0; i < n; i++ {
		out[i] = rtorrent.Torrent{
			Hash:      fmt.Sprintf("%040x", i),
			Name:      fmt.Sprintf("torrent-%d", i),
			SizeBytes: int64(i+1) * 1024,
			BytesDone: int64(i),
			Active:    i%2 == 0,
			State:     "downloading",
			Progress:  float64(i%100) / 100,
		}
	}
	return out
}

func BenchmarkCacheApply1000(b *testing.B) {
	list := makeTorrents(1000)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		c := newCache()
		_ = c.applyTorrents(list)
	}
}

func BenchmarkCacheApply10000(b *testing.B) {
	list := makeTorrents(10_000)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		c := newCache()
		_ = c.applyTorrents(list)
	}
}

func BenchmarkCacheSnapshot10000(b *testing.B) {
	c := newCache()
	_ = c.applyTorrents(makeTorrents(10_000))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, torrents, _ := c.snapshot()
		if len(torrents) != 10_000 {
			b.Fatalf("len=%d", len(torrents))
		}
	}
}

func BenchmarkCacheNoopUpdate1000(b *testing.B) {
	list := makeTorrents(1000)
	c := newCache()
	_ = c.applyTorrents(list)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.applyTorrents(list)
	}
}
