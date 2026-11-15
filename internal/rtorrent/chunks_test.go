package rtorrent

import "testing"

func TestCountBitfieldOnes(t *testing.T) {
	// 0xf0 = 11110000 → 4 ones
	if got := countBitfieldOnes("f0", 8); got != 4 {
		t.Fatalf("got %d want 4", got)
	}
	// cap to sizeChunks=2 → only first two bits of 0xf0 (11......) → 2
	if got := countBitfieldOnes("f0", 2); got != 2 {
		t.Fatalf("cap: got %d want 2", got)
	}
	if got := countBitfieldOnes("", 10); got != 0 {
		t.Fatalf("empty: %d", got)
	}
	if got := countBitfieldOnes("zz", 10); got != 0 {
		t.Fatalf("invalid hex: %d", got)
	}
}
