package main

import "testing"

func TestCompareBaselineMode_LastFlagTakesValue(t *testing.T) {
	if got := compareBaselineMode("successful", nil); got != "successful" {
		t.Fatalf("got %q", got)
	}
	if got := compareBaselineMode("successful", []string{"ignored"}); got != "successful" {
		t.Fatalf("flag wins: %q", got)
	}
	if got := compareBaselineMode("", []string{"successful"}); got != "successful" {
		t.Fatalf("positional: %q", got)
	}
	if got := compareBaselineMode("", nil); got != "" {
		t.Fatalf("empty: %q", got)
	}
}
