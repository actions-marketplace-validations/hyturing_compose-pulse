package main

import (
	"errors"
	"testing"

	"github.com/hyturing/compose-pulse/internal/model"
)

func TestClassifyRecordedFindings(t *testing.T) {
	cases := []struct {
		name   string
		failOn model.Confidence
		finds  []model.Finding
		want   int
	}{
		{"healthy", model.ConfidenceHigh, nil, exitOK},
		{"high fails on high", model.ConfidenceHigh, []model.Finding{{Confidence: model.ConfidenceHigh}}, exitFailure},
		{"medium ignored when fail-on high", model.ConfidenceHigh, []model.Finding{{Confidence: model.ConfidenceMedium}}, exitOK},
		{"medium fails when fail-on medium", model.ConfidenceMedium, []model.Finding{{Confidence: model.ConfidenceMedium}}, exitFailure},
		{"possible fails when fail-on possible", model.ConfidencePossible, []model.Finding{{Confidence: model.ConfidencePossible}}, exitFailure},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyRecordedFindings(tc.finds, tc.failOn); got != tc.want {
				t.Fatalf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestParseFailOn(t *testing.T) {
	c, err := parseFailOn("high")
	if err != nil || c != model.ConfidenceHigh {
		t.Fatalf("high: %v %v", c, err)
	}
	if _, err := parseFailOn("nope"); err == nil {
		t.Fatal("expected error")
	}
}

func TestUsageErrorCode(t *testing.T) {
	err := usageError("bad")
	var ec exitCodeError
	if !errors.As(err, &ec) || ec.code != exitUsage {
		t.Fatalf("got %#v", err)
	}
}
