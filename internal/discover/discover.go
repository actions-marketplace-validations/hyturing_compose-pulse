package discover

import (
	"context"
	"encoding/json"
	"os"
	"sort"
	"strings"

	"github.com/hyturing/compose-pulse/internal/compose"
	"github.com/hyturing/compose-pulse/internal/dag"
	"github.com/hyturing/compose-pulse/internal/docker"
)

const (
	labelService     = "com.docker.compose.service"
	labelProject     = "com.docker.compose.project"
	labelDependsOn   = "com.docker.compose.depends_on"
	labelConfigFiles = "com.docker.compose.project.config_files"
)

// FromDocker lists all containers and builds a Snapshot.
func FromDocker(ctx context.Context, client *docker.Client) (*Snapshot, error) {
	containers, err := client.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	return FromContainers(containers)
}

// FromContainers builds a Snapshot from pre-fetched container metadata (testable without Docker).
func FromContainers(containers []docker.ContainerInfo) (*Snapshot, error) {
	byProject := make(map[string][]docker.ContainerInfo)
	var standalone []Standalone

	for _, ctr := range containers {
		svc, isCompose := ctr.Labels[labelService]
		if !isCompose || svc == "" {
			standalone = append(standalone, Standalone{
				ID:    ctr.ID,
				Name:  displayName(ctr.Names),
				Image: ctr.Image,
				State: ctr.State,
			})
			continue
		}
		project := ctr.Labels[labelProject]
		if project == "" {
			project = "default"
		}
		byProject[project] = append(byProject[project], ctr)
	}

	sort.Slice(standalone, func(i, j int) bool {
		return standalone[i].Name < standalone[j].Name
	})

	projectNames := sortedKeys(byProject)
	projects := make([]Project, 0, len(projectNames))
	for _, name := range projectNames {
		cfg, serviceIDs, configFiles := buildProjectConfig(byProject[name])
		graph, err := dag.Build(cfg)
		if err != nil {
			return nil, err
		}
		for _, ctr := range byProject[name] {
			svc := ctr.Labels[labelService]
			if node, ok := graph.ByName[svc]; ok {
				node.ContainerID = ctr.ID
				node.State = ctr.State
				node.ExitCode = ctr.ExitCode
				node.Image = ctr.Image
				node.Ports = ctr.Ports
				node.CreatedAt = ctr.Created
			}
		}
		projects = append(projects, Project{
			Name:        name,
			Graph:       graph,
			Containers:  serviceIDs,
			ConfigFiles: configFiles,
		})
	}

	return &Snapshot{
		Projects:   projects,
		Standalone: standalone,
	}, nil
}

func buildProjectConfig(containers []docker.ContainerInfo) (*compose.Config, map[string]string, []string) {
	serviceIDs := make(map[string]string, len(containers))
	services := make(map[string]compose.Service)
	configFiles := findConfigFiles(containers)

	if len(configFiles) > 0 {
		if fileCfg, err := compose.Parse(configFiles[0]); err == nil {
			for name, svc := range fileCfg.Services {
				services[name] = svc
			}
		}
	}

	for _, ctr := range containers {
		svc := ctr.Labels[labelService]
		serviceIDs[svc] = ctr.ID
		labelDeps := parseDependsOnLabel(ctr.Labels[labelDependsOn])

		existing, ok := services[svc]
		if !ok {
			services[svc] = compose.Service{DependsOn: labelDeps}
			continue
		}
		if len(existing.DependsOn) == 0 && len(labelDeps) > 0 {
			existing.DependsOn = labelDeps
			services[svc] = existing
		}
	}

	return &compose.Config{Services: services}, serviceIDs, configFiles
}

func findConfigFiles(containers []docker.ContainerInfo) []string {
	paths := make(map[string]struct{})
	for _, ctr := range containers {
		raw := ctr.Labels[labelConfigFiles]
		if raw == "" {
			continue
		}
		for _, path := range strings.Split(raw, ",") {
			path = strings.TrimSpace(path)
			if path == "" {
				continue
			}
			if _, err := os.Stat(path); err == nil {
				paths[path] = struct{}{}
			}
		}
	}
	if len(paths) == 0 {
		return nil
	}
	sorted := make([]string, 0, len(paths))
	for path := range paths {
		sorted = append(sorted, path)
	}
	sort.Strings(sorted)
	return sorted
}

func parseDependsOnLabel(raw string) compose.DependsOn {
	if raw == "" {
		return nil
	}
	// JSON form: {"postgres":{"condition":"service_healthy","required":true}}
	var deps map[string]compose.DependsOnCondition
	if err := json.Unmarshal([]byte(raw), &deps); err == nil && len(deps) > 0 {
		out := make(compose.DependsOn, len(deps))
		for name, cond := range deps {
			if cond.Condition == "" {
				cond.Condition = "service_started"
			}
			out[name] = cond
		}
		return out
	}
	// Colon-separated form: "postgres:service_healthy:false,redis:service_healthy:false"
	out := make(compose.DependsOn)
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		segments := strings.Split(part, ":")
		name := segments[0]
		if name == "" {
			continue
		}
		cond := "service_started"
		if len(segments) >= 2 && segments[1] != "" {
			cond = segments[1]
		}
		out[name] = compose.DependsOnCondition{Condition: cond}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func displayName(names []string) string {
	if len(names) == 0 {
		return ""
	}
	return strings.TrimPrefix(names[0], "/")
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
