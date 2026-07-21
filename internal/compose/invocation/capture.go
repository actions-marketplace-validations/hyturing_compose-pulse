package invocation

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/hyturing/compose-pulse/internal/model"
)

// EnvProvider supplies process environment for capture (mockable).
type EnvProvider interface {
	Environ() []string
	Getenv(key string) string
	Getwd() (string, error)
}

// GitProvider supplies git metadata (mockable).
type GitProvider interface {
	Root(cwd string) (string, error)
	Commit(cwd string) (string, error)
	Dirty(cwd string) (bool, error)
}

// VersionProvider supplies docker/compose versions (mockable).
type VersionProvider interface {
	DockerVersion() (string, error)
	ComposeVersion() (string, error)
	DockerContext() (string, error)
}

// OSEnv is the real process environment.
type OSEnv struct{}

// Environ returns the process environment.
func (OSEnv) Environ() []string { return os.Environ() }

// Getenv returns one environment variable.
func (OSEnv) Getenv(key string) string { return os.Getenv(key) }

// Getwd returns the working directory.
func (OSEnv) Getwd() (string, error) { return os.Getwd() }

// ExecGit runs git via the shell.
type ExecGit struct{}

// Root returns the git toplevel directory.
func (ExecGit) Root(cwd string) (string, error) {
	return runGit(cwd, "rev-parse", "--show-toplevel")
}

// Commit returns HEAD.
func (ExecGit) Commit(cwd string) (string, error) {
	return runGit(cwd, "rev-parse", "HEAD")
}

// Dirty reports whether the worktree has changes.
func (ExecGit) Dirty(cwd string) (bool, error) {
	out, err := runGit(cwd, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// ExecVersions runs docker / docker compose version commands.
type ExecVersions struct{}

// DockerVersion returns the Docker server version.
func (ExecVersions) DockerVersion() (string, error) {
	return runCmd("", "docker", "version", "--format", "{{.Server.Version}}")
}

// ComposeVersion returns the Compose plugin version.
func (ExecVersions) ComposeVersion() (string, error) {
	return runCmd("", "docker", "compose", "version", "--short")
}

// DockerContext returns the active docker context name.
func (ExecVersions) DockerContext() (string, error) {
	return runCmd("", "docker", "context", "show")
}

// Options controls Capture.
type Options struct {
	Command        []string
	IncludeEnvVals bool
	Env            EnvProvider
	Git            GitProvider
	Versions       VersionProvider
	Now            time.Time
}

// Capture builds an Invocation from the given options.
func Capture(opts Options) (*model.Invocation, error) {
	env := opts.Env
	if env == nil {
		env = OSEnv{}
	}
	git := opts.Git
	if git == nil {
		git = ExecGit{}
	}
	vers := opts.Versions
	if vers == nil {
		vers = ExecVersions{}
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	cwd, err := env.Getwd()
	if err != nil {
		cwd = ""
	}

	inv := &model.Invocation{
		Command:      append([]string(nil), opts.Command...),
		WorkingDir:   cwd,
		ComposeFiles: extractComposeFiles(opts.Command),
		Profiles:     extractFlagValues(opts.Command, "--profile"),
		ProjectName:  firstNonEmpty(extractFlagValues(opts.Command, "--project-name"), extractFlagValues(opts.Command, "-p")),
		EnvNames:     envNames(env.Environ()),
		DockerHost:   env.Getenv("DOCKER_HOST"),
		OS:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		Timestamp:    now.UTC(),
	}
	if opts.IncludeEnvVals {
		inv.EnvValues = envValues(env.Environ())
	}

	if v, err := vers.DockerVersion(); err == nil {
		inv.DockerVersion = strings.TrimSpace(v)
	}
	if v, err := vers.ComposeVersion(); err == nil {
		inv.ComposeVersion = strings.TrimSpace(v)
	}
	if v, err := vers.DockerContext(); err == nil {
		inv.DockerContext = strings.TrimSpace(v)
	}

	if cwd != "" {
		if root, err := git.Root(cwd); err == nil {
			inv.GitRoot = strings.TrimSpace(root)
			if commit, err := git.Commit(cwd); err == nil {
				inv.GitCommit = strings.TrimSpace(commit)
			}
			if dirty, err := git.Dirty(cwd); err == nil {
				inv.GitDirty = dirty
			}
		}
	}
	return inv, nil
}

func envNames(environ []string) []string {
	names := make([]string, 0, len(environ))
	for _, e := range environ {
		if i := strings.IndexByte(e, '='); i > 0 {
			names = append(names, e[:i])
		}
	}
	sort.Strings(names)
	return names
}

func envValues(environ []string) map[string]string {
	out := make(map[string]string, len(environ))
	for _, e := range environ {
		if i := strings.IndexByte(e, '='); i > 0 {
			out[e[:i]] = e[i+1:]
		}
	}
	return out
}

func extractComposeFiles(cmd []string) []string {
	var files []string
	for i := 0; i < len(cmd); i++ {
		arg := cmd[i]
		switch {
		case arg == "-f" || arg == "--file":
			if i+1 < len(cmd) {
				files = append(files, cmd[i+1])
				i++
			}
		case strings.HasPrefix(arg, "--file="):
			files = append(files, strings.TrimPrefix(arg, "--file="))
		case strings.HasPrefix(arg, "-f") && len(arg) > 2 && arg[2] != '-':
			files = append(files, arg[2:])
		}
	}
	return files
}

func extractFlagValues(cmd []string, flag string) []string {
	var out []string
	eq := flag + "="
	for i := 0; i < len(cmd); i++ {
		arg := cmd[i]
		switch {
		case arg == flag:
			if i+1 < len(cmd) {
				out = append(out, cmd[i+1])
				i++
			}
		case strings.HasPrefix(arg, eq):
			out = append(out, strings.TrimPrefix(arg, eq))
		}
	}
	return out
}

func firstNonEmpty(lists ...[]string) string {
	for _, list := range lists {
		if len(list) > 0 && list[0] != "" {
			return list[0]
		}
	}
	return ""
}

func runGit(cwd string, args ...string) (string, error) {
	return runCmd(cwd, "git", args...)
}

func runCmd(cwd, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// ResolveComposeFiles returns absolute compose file paths when possible.
func ResolveComposeFiles(cwd string, files []string) []string {
	if len(files) == 0 {
		return nil
	}
	out := make([]string, 0, len(files))
	for _, f := range files {
		if filepath.IsAbs(f) || cwd == "" {
			out = append(out, f)
			continue
		}
		out = append(out, filepath.Join(cwd, f))
	}
	return out
}
