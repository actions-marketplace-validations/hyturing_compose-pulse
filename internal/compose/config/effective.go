package config

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/hyturing/compose-pulse/internal/compose"
	"github.com/hyturing/compose-pulse/internal/model"
)

// Runner executes docker compose config (mockable).
type Runner interface {
	ComposeConfig(cwd string, composeArgs []string) ([]byte, error)
}

// ExecRunner shells out to docker compose config.
type ExecRunner struct{}

// ComposeConfig runs `docker compose … config`.
func (ExecRunner) ComposeConfig(cwd string, composeArgs []string) ([]byte, error) {
	args := append([]string{}, composeArgs...)
	args = append(args, "config")
	cmd := exec.Command("docker", append([]string{"compose"}, args...)...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("docker compose config: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// Capture runs docker compose config and builds EffectiveConfig with override notes.
func Capture(cwd string, composeFiles []string, composeArgs []string, runner Runner) (*model.EffectiveConfig, error) {
	if runner == nil {
		runner = ExecRunner{}
	}
	raw, err := runner.ComposeConfig(cwd, composeArgs)
	if err != nil {
		return nil, err
	}
	eff, err := ParseMerged(raw)
	if err != nil {
		return nil, err
	}
	eff.RawYAML = string(raw)
	eff.SourceFiles = append([]string(nil), composeFiles...)

	if len(composeFiles) >= 2 {
		notes, err := AttributeOverrides(cwd, composeFiles)
		if err == nil {
			eff.Overrides = notes
		}
	}
	return eff, nil
}

// ParseMerged parses `docker compose config` YAML into EffectiveConfig.
func ParseMerged(raw []byte) (*model.EffectiveConfig, error) {
	var cfg compose.Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse merged config: %w", err)
	}
	eff := &model.EffectiveConfig{}
	names := make([]string, 0, len(cfg.Services))
	for name := range cfg.Services {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		svc := cfg.Services[name]
		es := model.EffectiveService{
			Name:  name,
			Image: svc.Image,
			Ports: append([]string(nil), svc.Ports...),
		}
		switch b := svc.Build.(type) {
		case string:
			es.Build = b
		case map[string]any:
			if ctx, ok := b["context"].(string); ok {
				es.Build = ctx
			}
		}
		if svc.Healthcheck != nil && len(svc.Healthcheck.Test) > 0 {
			es.Healthcheck = strings.Join(svc.Healthcheck.Test, " ")
		}
		if len(svc.Environment) > 0 {
			keys := make([]string, 0, len(svc.Environment))
			for k := range svc.Environment {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			es.EnvNames = keys
		}
		if len(svc.DependsOn) > 0 {
			es.DependsOn = make(map[string]string, len(svc.DependsOn))
			for dep, cond := range svc.DependsOn {
				es.DependsOn[dep] = cond.Condition
			}
		}
		eff.Services = append(eff.Services, es)
	}
	return eff, nil
}

// AttributeOverrides compares per-file parses and explains port (and image) overrides.
func AttributeOverrides(cwd string, files []string) ([]model.OverrideNote, error) {
	type fileSvc struct {
		file string
		cfg  *compose.Config
	}
	parsed := make([]fileSvc, 0, len(files))
	for _, f := range files {
		path := f
		if !filepath.IsAbs(path) && cwd != "" {
			path = filepath.Join(cwd, f)
		}
		cfg, err := compose.Parse(path)
		if err != nil {
			// Missing file or partial parse — skip attribution for this file.
			continue
		}
		parsed = append(parsed, fileSvc{file: filepath.Base(f), cfg: cfg})
	}
	if len(parsed) < 2 {
		return nil, nil
	}

	var notes []model.OverrideNote
	// Union of service names.
	services := map[string]struct{}{}
	for _, p := range parsed {
		for name := range p.cfg.Services {
			services[name] = struct{}{}
		}
	}
	names := make([]string, 0, len(services))
	for n := range services {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, name := range names {
		var lastPorts []string
		var lastFile string
		var lastImage, lastImageFile string
		for _, p := range parsed {
			svc, ok := p.cfg.Services[name]
			if !ok {
				continue
			}
			if len(svc.Ports) > 0 {
				if lastPorts != nil && !stringSlicesEqual(lastPorts, svc.Ports) {
					from := strings.Join(lastPorts, ",")
					to := strings.Join(svc.Ports, ",")
					notes = append(notes, model.OverrideNote{
						Service:  name,
						Field:    "ports",
						From:     from,
						FromFile: lastFile,
						To:       to,
						ToFile:   p.file,
						Message:  fmt.Sprintf("Port %s was defined in %s and replaced with %s by %s.", from, lastFile, to, p.file),
					})
				}
				lastPorts = append([]string(nil), svc.Ports...)
				lastFile = p.file
			}
			if svc.Image != "" {
				if lastImage != "" && lastImage != svc.Image {
					notes = append(notes, model.OverrideNote{
						Service:  name,
						Field:    "image",
						From:     lastImage,
						FromFile: lastImageFile,
						To:       svc.Image,
						ToFile:   p.file,
						Message:  fmt.Sprintf("Image %s was defined in %s and replaced with %s by %s.", lastImage, lastImageFile, svc.Image, p.file),
					})
				}
				lastImage = svc.Image
				lastImageFile = p.file
			}
		}
	}
	return notes, nil
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// WriteTempCompose is a test helper exported for fixtures under this package.
func WriteTempCompose(dir, name, contents string) (string, error) {
	path := filepath.Join(dir, name)
	return path, os.WriteFile(path, []byte(contents), 0o644)
}
