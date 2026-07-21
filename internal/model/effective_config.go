package model

// EffectiveConfig is the fully-merged Compose model plus override explanations.
type EffectiveConfig struct {
	RawYAML     string             `json:"raw_yaml,omitempty"`
	Services    []EffectiveService `json:"services,omitempty"`
	Overrides   []OverrideNote     `json:"overrides,omitempty"`
	SourceFiles []string           `json:"source_files,omitempty"`
}

// EffectiveService is one service from `docker compose config`.
type EffectiveService struct {
	Name        string            `json:"name"`
	Image       string            `json:"image,omitempty"`
	Build       string            `json:"build,omitempty"`
	Ports       []string          `json:"ports,omitempty"`
	Volumes     []string          `json:"volumes,omitempty"`
	Networks    []string          `json:"networks,omitempty"`
	Command     []string          `json:"command,omitempty"`
	EnvNames    []string          `json:"env_names,omitempty"`
	Healthcheck string            `json:"healthcheck,omitempty"`
	DependsOn   map[string]string `json:"depends_on,omitempty"`
}

// OverrideNote explains a value that differed across compose files.
type OverrideNote struct {
	Service  string `json:"service,omitempty"`
	Field    string `json:"field"`
	From     string `json:"from"`
	FromFile string `json:"from_file,omitempty"`
	To       string `json:"to"`
	ToFile   string `json:"to_file,omitempty"`
	Message  string `json:"message"`
}
