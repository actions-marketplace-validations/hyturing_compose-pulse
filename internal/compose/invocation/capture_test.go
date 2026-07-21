package invocation_test

import (
	"testing"
	"time"

	"github.com/hyturing/compose-pulse/internal/compose/invocation"
)

type fakeEnv struct {
	environ []string
	vars    map[string]string
	wd      string
}

func (f fakeEnv) Environ() []string      { return f.environ }
func (f fakeEnv) Getenv(k string) string { return f.vars[k] }
func (f fakeEnv) Getwd() (string, error) { return f.wd, nil }

type fakeGit struct {
	root, commit string
	dirty        bool
}

func (f fakeGit) Root(string) (string, error)   { return f.root, nil }
func (f fakeGit) Commit(string) (string, error) { return f.commit, nil }
func (f fakeGit) Dirty(string) (bool, error)    { return f.dirty, nil }

type fakeVers struct {
	docker, compose, ctx string
}

func (f fakeVers) DockerVersion() (string, error)  { return f.docker, nil }
func (f fakeVers) ComposeVersion() (string, error) { return f.compose, nil }
func (f fakeVers) DockerContext() (string, error)  { return f.ctx, nil }

func TestCaptureNamesOnly(t *testing.T) {
	inv, err := invocation.Capture(invocation.Options{
		Command: []string{"docker", "compose", "-f", "compose.yml", "-f", "compose.dev.yml", "--profile", "workers", "up", "--build"},
		Env: fakeEnv{
			environ: []string{"PATH=/bin", "SECRET_TOKEN=super-secret", "HOME=/home/u"},
			vars:    map[string]string{"DOCKER_HOST": "unix:///var/run/docker.sock"},
			wd:      "/proj",
		},
		Git:      fakeGit{root: "/proj", commit: "abc123", dirty: true},
		Versions: fakeVers{docker: "28.0.0", compose: "2.30.0", ctx: "default"},
		Now:      time.Date(2026, 7, 22, 3, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if inv.WorkingDir != "/proj" || inv.GitCommit != "abc123" || !inv.GitDirty {
		t.Fatalf("git/cwd mismatch: %+v", inv)
	}
	if inv.DockerVersion != "28.0.0" || inv.ComposeVersion != "2.30.0" {
		t.Fatalf("versions: %+v", inv)
	}
	if len(inv.ComposeFiles) != 2 || inv.ComposeFiles[0] != "compose.yml" {
		t.Fatalf("compose files: %v", inv.ComposeFiles)
	}
	if len(inv.Profiles) != 1 || inv.Profiles[0] != "workers" {
		t.Fatalf("profiles: %v", inv.Profiles)
	}
	if inv.EnvValues != nil {
		t.Fatalf("env values must be nil by default: %v", inv.EnvValues)
	}
	foundSecretName := false
	for _, n := range inv.EnvNames {
		if n == "SECRET_TOKEN" {
			foundSecretName = true
		}
		if n == "super-secret" {
			t.Fatal("secret value leaked into env names")
		}
	}
	if !foundSecretName {
		t.Fatal("expected SECRET_TOKEN name captured")
	}
}
