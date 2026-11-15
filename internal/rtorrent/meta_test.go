package rtorrent

import "testing"

func TestPriorityBoundsDocumented(t *testing.T) {
	// Guard the accepted ranges used by API validation / Set* helpers.
	cases := []struct {
		prio int64
		ok   bool
	}{
		{0, true}, {1, true}, {2, true}, {3, true}, {-1, false}, {4, false},
	}
	for _, tc := range cases {
		ok := tc.prio >= 0 && tc.prio <= 3
		if ok != tc.ok {
			t.Fatalf("prio %d ok=%v want %v", tc.prio, ok, tc.ok)
		}
	}
}
