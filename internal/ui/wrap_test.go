package ui

import (
	"testing"

	"github.com/mattn/go-runewidth"
)

func TestWrapToWidth_ShortLine(t *testing.T) {
	lines := wrapToWidth("hello", 20)
	if len(lines) != 1 || lines[0] != "hello" {
		t.Fatalf("unexpected wrap: %v", lines)
	}
}

func TestWrapToWidth_LongNoSpaces(t *testing.T) {
	s := stringsRepeat("x", 100)
	lines := wrapToWidth(s, 20)
	for _, line := range lines {
		if runewidth.StringWidth(line) > 20 {
			t.Errorf("line too wide: %d", runewidth.StringWidth(line))
		}
	}
}

func stringsRepeat(s string, n int) string {
	b := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		b = append(b, s...)
	}
	return string(b)
}
