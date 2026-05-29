package components

import "testing"

// equalWidth builds a widths slice of n entries each of value w.
func equalWidth(n, w int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = w
	}
	return out
}

func TestTabWindowAlwaysIncludesActive(t *testing.T) {
	// Width too small for even one full tab — still must include the active tab.
	lo, hi := tabWindow(equalWidth(12, 10), 5, 4)
	if lo != 5 || hi != 5 {
		t.Fatalf("got [%d,%d], want [5,5] (active always visible)", lo, hi)
	}
}

func TestTabWindowExpandsToFit(t *testing.T) {
	// 12 tabs of width 10, gap 1, arrows 2 each. Width 100 fits several around active.
	lo, hi := tabWindow(equalWidth(12, 10), 5, 100)
	if lo > 5 || hi < 5 {
		t.Fatalf("window [%d,%d] does not contain active 5", lo, hi)
	}
	if lo == 0 && hi == 11 {
		t.Fatal("expected a windowed (not full) view at width 100 with 12 tabs")
	}
	// Verify the chosen window actually fits the budget.
	used := windowUsedWidth(equalWidth(12, 10), lo, hi, 12)
	if used > 100 {
		t.Fatalf("window width %d exceeds 100", used)
	}
}

func TestTabWindowAtLeftEdgeNoLeftArrow(t *testing.T) {
	// Active at 0: enough width for two tabs + a right arrow.
	lo, hi := tabWindow(equalWidth(12, 10), 0, 23)
	if lo != 0 {
		t.Fatalf("got lo=%d, want 0 (no tabs hidden on the left)", lo)
	}
	if hi < 1 {
		t.Fatalf("got hi=%d, want >=1 (should fit a second tab)", hi)
	}
}

func TestTabWindowFitsAllWhenWide(t *testing.T) {
	lo, hi := tabWindow(equalWidth(5, 10), 2, 1000)
	if lo != 0 || hi != 4 {
		t.Fatalf("got [%d,%d], want [0,4] (all tabs fit)", lo, hi)
	}
}
