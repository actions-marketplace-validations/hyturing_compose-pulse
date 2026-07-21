package regression_test

import (
	"testing"
	"time"

	"github.com/hyturing/compose-pulse/internal/profiler/regression"
)

func TestCompare_FlagsLargeRegression(t *testing.T) {
	res := regression.Compare(48*time.Second, 18*time.Second, nil)
	if !res.IsRegression {
		t.Fatalf("expected regression: %+v", res)
	}
}

func TestCompare_IgnoresNoise(t *testing.T) {
	res := regression.Compare(19*time.Second, 18*time.Second, nil)
	if res.IsRegression {
		t.Fatalf("noise flagged: %+v", res)
	}
}

func TestClassifyPollingDelay(t *testing.T) {
	kind, msg := regression.ClassifyPollingDelay(4200*time.Millisecond, 30*time.Second)
	if kind != regression.KindReadinessPoll {
		t.Fatalf("kind = %s", kind)
	}
	if msg == "" {
		t.Fatal("expected explanation")
	}
}

func TestStats(t *testing.T) {
	st := regression.Stats([]time.Duration{
		10 * time.Second,
		12 * time.Second,
		20 * time.Second,
		30 * time.Second,
		40 * time.Second,
	})
	if st.Fastest != 10*time.Second || st.Slowest != 40*time.Second {
		t.Fatalf("stats = %+v", st)
	}
	if st.P50 != 20*time.Second {
		t.Fatalf("P50 = %s", st.P50)
	}
}
