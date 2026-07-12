package compose

import "gopkg.in/yaml.v3"

// Config represents the top-level docker-compose.yml structure.
type Config struct {
	Version  string             `yaml:"version"`
	Services map[string]Service `yaml:"services"`
}

// Service represents a single service definition.
type Service struct {
	Image       string            `yaml:"image"`
	Build       interface{}       `yaml:"build"`
	DependsOn   DependsOn         `yaml:"depends_on"`
	Healthcheck *Healthcheck      `yaml:"healthcheck"`
	Restart     string            `yaml:"restart"`
	Ports       []string          `yaml:"ports"`
	Environment map[string]string `yaml:"environment"`
}

// DependsOn supports both forms of the depends_on key:
//
//	depends_on: [db, redis]           ← list form
//	depends_on:
//	  db:
//	    condition: service_healthy    ← map form
type DependsOn map[string]DependsOnCondition

// UnmarshalYAML handles both the list and map forms of depends_on.
func (d *DependsOn) UnmarshalYAML(value *yaml.Node) error {
	*d = make(DependsOn)
	switch value.Kind {
	case yaml.SequenceNode:
		var names []string
		if err := value.Decode(&names); err != nil {
			return err
		}
		for _, name := range names {
			(*d)[name] = DependsOnCondition{Condition: "service_started"}
		}
	case yaml.MappingNode:
		type plain map[string]DependsOnCondition
		var m plain
		if err := value.Decode(&m); err != nil {
			return err
		}
		*d = DependsOn(m)
	}
	return nil
}

// DependsOnCondition holds the optional condition for a single dependency.
type DependsOnCondition struct {
	Condition string `yaml:"condition"`
}

// HealthcheckTest supports both string and list forms of healthcheck.test:
//
//	test: curl -f http://localhost/          → ["CMD-SHELL", "..."]
//	test: ["CMD", "curl", "-f", "http://…"]
type HealthcheckTest []string

// UnmarshalYAML handles scalar (CMD-SHELL) and sequence forms of test.
func (t *HealthcheckTest) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		var s string
		if err := value.Decode(&s); err != nil {
			return err
		}
		*t = HealthcheckTest{"CMD-SHELL", s}
	case yaml.SequenceNode:
		var xs []string
		if err := value.Decode(&xs); err != nil {
			return err
		}
		*t = HealthcheckTest(xs)
	}
	return nil
}

// Healthcheck mirrors the docker-compose healthcheck block.
type Healthcheck struct {
	Test          HealthcheckTest `yaml:"test"`
	Interval      string          `yaml:"interval"`
	Timeout       string          `yaml:"timeout"`
	StartPeriod   string          `yaml:"start_period"`
	StartInterval string          `yaml:"start_interval"`
	Retries       int             `yaml:"retries"`
	Disable       bool            `yaml:"disable"`
}
