package main

import (
	"fmt"
	"strings"

	"github.com/hyturing/compose-pulse/internal/model"
)

// CI exit-code contract (master plan).
const (
	exitOK         = 0
	exitFailure    = 1
	exitTimeout    = 2
	exitLaunchFail = 3
	exitUsage      = 4
)

func usageError(msg string) error {
	return exitCodeError{code: exitUsage, msg: msg}
}

func classifyRecordedFindings(findings []model.Finding, failOn model.Confidence) int {
	for _, f := range findings {
		if f.Confidence >= failOn {
			return exitFailure
		}
	}
	return exitOK
}

func parseFailOn(s string) (model.Confidence, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "high":
		return model.ConfidenceHigh, nil
	case "medium":
		return model.ConfidenceMedium, nil
	case "possible", "low":
		return model.ConfidencePossible, nil
	default:
		return 0, fmt.Errorf("unknown --fail-on %q (want high|medium|possible)", s)
	}
}
