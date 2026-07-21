package progress_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/hyturing/compose-pulse/internal/compose/progress"
)

func TestParseGoldenFixtures(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	root := filepath.Join(filepath.Dir(file), "..", "..", "..", "testdata", "progress")
	cases := []struct {
		name string
		want []string // kinds that must appear
	}{
		{"pull-auth.txt", []string{"auth_error"}},
		{"pull-ok.txt", []string{"pull_start", "pull_complete"}},
		{"build-fail.txt", []string{"build_step", "build_error"}},
		{"arch-mismatch.txt", []string{"arch_mismatch"}},
		{"manifest-missing.txt", []string{"manifest_missing"}},
		{"cache-hit.txt", []string{"build_cache"}},
	}
	now := time.Date(2026, 7, 22, 4, 0, 0, 0, time.UTC)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(root, tc.name))
			if err != nil {
				t.Fatal(err)
			}
			events := progress.ParseLines(splitLines(string(raw)), now)
			got := map[string]bool{}
			for _, e := range events {
				got[e.Kind] = true
			}
			for _, kind := range tc.want {
				if !got[kind] {
					t.Fatalf("missing kind %q in %#v", kind, events)
				}
			}
		})
	}
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}
