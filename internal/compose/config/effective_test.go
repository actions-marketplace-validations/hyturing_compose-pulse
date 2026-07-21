package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hyturing/compose-pulse/internal/compose/config"
)

type fakeRunner struct {
	raw []byte
	err error
}

func (f fakeRunner) ComposeConfig(string, []string) ([]byte, error) {
	return f.raw, f.err
}

func TestParseMergedAndOverrides(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "compose.yml")
	dev := filepath.Join(dir, "compose.dev.yml")
	if err := write(base, `
services:
  api:
    image: api:base
    ports:
      - "8080:80"
`); err != nil {
		t.Fatal(err)
	}
	if err := write(dev, `
services:
  api:
    image: api:dev
    ports:
      - "3000:80"
`); err != nil {
		t.Fatal(err)
	}

	merged := []byte(`
services:
  api:
    image: api:dev
    ports:
      - "3000:80"
`)
	eff, err := config.Capture(dir, []string{"compose.yml", "compose.dev.yml"}, []string{"-f", "compose.yml", "-f", "compose.dev.yml"}, fakeRunner{raw: merged})
	if err != nil {
		t.Fatal(err)
	}
	if len(eff.Services) != 1 || eff.Services[0].Image != "api:dev" {
		t.Fatalf("services: %+v", eff.Services)
	}
	if len(eff.Overrides) == 0 {
		t.Fatal("expected override notes")
	}
	foundPort := false
	for _, n := range eff.Overrides {
		if n.Field == "ports" && strings.Contains(n.Message, "8080") && strings.Contains(n.Message, "3000") && strings.Contains(n.Message, "compose.dev.yml") {
			foundPort = true
		}
	}
	if !foundPort {
		t.Fatalf("missing port override note: %+v", eff.Overrides)
	}
}

func write(path, body string) error {
	return os.WriteFile(path, []byte(body), 0o644)
}
