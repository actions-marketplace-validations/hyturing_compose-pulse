package compose

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseHealthcheckForms(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "docker-compose.yml")
	yaml := `
services:
  string-form:
    image: nginx
    restart: on-failure
    healthcheck:
      test: curl -f http://localhost/
      interval: 5s
      start_period: 10s
      start_interval: 1s
      retries: 3
  list-form:
    image: postgres
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      timeout: 3s
      disable: false
`
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Parse(path)
	if err != nil {
		t.Fatal(err)
	}

	sf := cfg.Services["string-form"]
	if sf.Restart != "on-failure" {
		t.Fatalf("restart = %q", sf.Restart)
	}
	if sf.Healthcheck == nil {
		t.Fatal("string-form healthcheck nil")
	}
	want := HealthcheckTest{"CMD-SHELL", "curl -f http://localhost/"}
	if len(sf.Healthcheck.Test) != 2 || sf.Healthcheck.Test[0] != want[0] || sf.Healthcheck.Test[1] != want[1] {
		t.Fatalf("string-form test = %#v", sf.Healthcheck.Test)
	}
	if sf.Healthcheck.StartPeriod != "10s" || sf.Healthcheck.StartInterval != "1s" {
		t.Fatalf("start fields = %+v", sf.Healthcheck)
	}

	lf := cfg.Services["list-form"]
	if lf.Healthcheck == nil || len(lf.Healthcheck.Test) != 2 || lf.Healthcheck.Test[1] != "pg_isready -U postgres" {
		t.Fatalf("list-form test = %#v", lf.Healthcheck)
	}
}
