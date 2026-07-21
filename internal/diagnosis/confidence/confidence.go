package confidence

import "github.com/hyturing/compose-pulse/internal/model"

// High / Medium / Possible re-export model confidence for rule packages.
const (
	High     = model.ConfidenceHigh
	Medium   = model.ConfidenceMedium
	Possible = model.ConfidencePossible
)

// AtLeast reports whether c meets the minimum bar.
func AtLeast(c, min model.Confidence) bool {
	return c >= min
}
