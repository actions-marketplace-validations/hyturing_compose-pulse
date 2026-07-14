package ui

import "testing"

func TestScrollFromThumbStart_RoundTrip(t *testing.T) {
	const visible, total = 10, 100
	maxScroll := total - visible
	for scroll := 0; scroll <= maxScroll; scroll++ {
		start, _ := scrollBarThumbRange(visible, scroll, total)
		got := scrollFromThumbStart(start, visible, total)
		// Integer mapping can lose a step; thumb start must round-trip stably.
		start2, _ := scrollBarThumbRange(visible, got, total)
		if start2 != start {
			t.Fatalf("scroll=%d start=%d got=%d start2=%d", scroll, start, got, start2)
		}
	}
}

func TestScrollFromThumbStart_Ends(t *testing.T) {
	const visible, total = 10, 100
	if got := scrollFromThumbStart(0, visible, total); got != 0 {
		t.Fatalf("top = %d, want 0", got)
	}
	thumbSize := scrollBarThumbSize(visible, total)
	track := visible - thumbSize
	if got := scrollFromThumbStart(track, visible, total); got != total-visible {
		t.Fatalf("bottom = %d, want %d", got, total-visible)
	}
}
