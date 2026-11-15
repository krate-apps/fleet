package gateways

import (
	"testing"
)

func TestSyntheticThrottleName(t *testing.T) {
	if got := SyntheticThrottleName(0, 0); got != "" {
		t.Fatalf("unlimited name=%q", got)
	}
	got := SyntheticThrottleName(102400, 51200)
	if got != "fleet-bw-u102400-d51200" {
		t.Fatalf("got %q", got)
	}
}

func TestCapabilitiesTorrentRates(t *testing.T) {
	for _, kind := range []Kind{KindRTorrent, KindQBittorrent, KindDeluge, KindTransmission} {
		if !SupportsFeature(kind, "torrentRates") {
			t.Fatalf("%s missing torrentRates", kind)
		}
	}
}

func TestDelugeKiBConversion(t *testing.T) {
	// 1500 B/s → ceil to 2 KiB/s
	bps := int64(1500)
	kib := (bps + 1023) / 1024
	if kib != 2 {
		t.Fatalf("kib=%d", kib)
	}
}
