package compose

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Parse reads and unmarshals the compose file at path into a Config.
func Parse(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	if cfg.Services == nil {
		return nil, fmt.Errorf("compose file has no services")
	}
	return &cfg, nil
}
