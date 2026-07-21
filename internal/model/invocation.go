package model

import "time"

// Invocation captures the exact context of a recorded Compose run.
// EnvValues is omitted unless the user explicitly opts in; EnvNames is always safe.
type Invocation struct {
	Command        []string          `json:"command"`
	WorkingDir     string            `json:"working_dir"`
	ComposeFiles   []string          `json:"compose_files,omitempty"`
	Profiles       []string          `json:"profiles,omitempty"`
	ProjectName    string            `json:"project_name,omitempty"`
	EnvNames       []string          `json:"env_names,omitempty"`
	EnvValues      map[string]string `json:"env_values,omitempty"` // only when opted in
	DockerContext  string            `json:"docker_context,omitempty"`
	DockerHost     string            `json:"docker_host,omitempty"`
	DockerVersion  string            `json:"docker_version,omitempty"`
	ComposeVersion string            `json:"compose_version,omitempty"`
	OS             string            `json:"os,omitempty"`
	Arch           string            `json:"arch,omitempty"`
	Timestamp      time.Time         `json:"timestamp"`
	GitRoot        string            `json:"git_root,omitempty"`
	GitCommit      string            `json:"git_commit,omitempty"`
	GitDirty       bool              `json:"git_dirty,omitempty"`
}
