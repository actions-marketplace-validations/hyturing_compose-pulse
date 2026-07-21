package regression

import (
	"fmt"
	"time"

	"github.com/hyturing/compose-pulse/internal/profiler/criticalpath"
)

// Thresholds for flagging regressions.
const (
	MinAbsolute = 5 * time.Second
	MinRelative = 0.25 // 25%
)

// Kind classifies a delay.
type Kind string

// Delay kinds.
const (
	KindNone          Kind = "none"
	KindRealStartup   Kind = "real_startup"
	KindReadinessPoll Kind = "readiness_polling"
	KindUnknown       Kind = "unknown"
)

// Result compares a run against a baseline.
type Result struct {
	CurrentTotal  time.Duration
	BaselineTotal time.Duration
	Delta         time.Duration
	IsRegression  bool
	Kind          Kind
	Explanation   string
	Path          *criticalpath.Path
}

// BaselineStats holds historical aggregates.
type BaselineStats struct {
	Count   int
	Last    time.Duration
	Median  time.Duration
	P50     time.Duration
	P90     time.Duration
	Fastest time.Duration
	Slowest time.Duration
}

// Compare flags a regression when current exceeds baseline by absolute and relative thresholds.
func Compare(current, baseline time.Duration, path *criticalpath.Path) Result {
	res := Result{
		CurrentTotal:  current,
		BaselineTotal: baseline,
		Delta:         current - baseline,
		Path:          path,
		Kind:          KindNone,
	}
	if baseline <= 0 {
		res.Kind = KindUnknown
		res.Explanation = "no baseline available"
		return res
	}
	rel := float64(res.Delta) / float64(baseline)
	if res.Delta >= MinAbsolute && rel >= MinRelative {
		res.IsRegression = true
		res.Kind = KindRealStartup
		res.Explanation = fmt.Sprintf("startup regressed by %s (%.0f%% over baseline)", res.Delta, rel*100)
	}
	return res
}

// ClassifyPollingDelay detects healthcheck-interval artifacts:
// readyAt is when the dependency accepted connections; nextCheck is when healthcheck passed.
func ClassifyPollingDelay(readyAt, nextCheck time.Duration) (Kind, string) {
	if nextCheck <= readyAt {
		return KindNone, ""
	}
	gap := nextCheck - readyAt
	if gap < 5*time.Second {
		return KindNone, ""
	}
	return KindReadinessPoll, fmt.Sprintf(
		"dependency accepted connections at %s; next healthcheck at %s; readiness polling added ~%s",
		readyAt, nextCheck, gap,
	)
}

// Stats computes median/P50/P90/fastest/slowest over durations (successful runs).
func Stats(durations []time.Duration) BaselineStats {
	if len(durations) == 0 {
		return BaselineStats{}
	}
	ds := append([]time.Duration(nil), durations...)
	// insertion sort — N is small
	for i := 1; i < len(ds); i++ {
		j := i
		for j > 0 && ds[j] < ds[j-1] {
			ds[j], ds[j-1] = ds[j-1], ds[j]
			j--
		}
	}
	percentile := func(p float64) time.Duration {
		if len(ds) == 1 {
			return ds[0]
		}
		idx := int(float64(len(ds)-1) * p)
		if idx < 0 {
			idx = 0
		}
		if idx >= len(ds) {
			idx = len(ds) - 1
		}
		return ds[idx]
	}
	med := percentile(0.5)
	return BaselineStats{
		Count:   len(ds),
		Last:    durations[len(durations)-1],
		Median:  med,
		P50:     percentile(0.5),
		P90:     percentile(0.9),
		Fastest: ds[0],
		Slowest: ds[len(ds)-1],
	}
}
