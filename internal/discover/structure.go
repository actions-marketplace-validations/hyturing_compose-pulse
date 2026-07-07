package discover

import (
	"sort"
	"strings"

	"github.com/hyturing/compose-pulse/internal/compose"
)

// SameStructure reports whether two snapshots have identical project/service
// topology (names and depends_on). Container state and IDs may differ.
func (s *Snapshot) SameStructure(other *Snapshot) bool {
	if s == nil || other == nil {
		return s == other
	}
	if len(s.Projects) != len(other.Projects) {
		return false
	}
	for i := range s.Projects {
		if !sameProjectStructure(&s.Projects[i], &other.Projects[i]) {
			return false
		}
	}
	if len(s.Standalone) != len(other.Standalone) {
		return false
	}
	for i := range s.Standalone {
		if s.Standalone[i].ID != other.Standalone[i].ID {
			return false
		}
	}
	return true
}

func sameProjectStructure(a, b *Project) bool {
	if a.Name != b.Name {
		return false
	}
	if strings.Join(a.ConfigFiles, "\x00") != strings.Join(b.ConfigFiles, "\x00") {
		return false
	}
	if a.Graph == nil || b.Graph == nil {
		return a.Graph == b.Graph
	}
	if len(a.Graph.ByName) != len(b.Graph.ByName) {
		return false
	}
	for name, nodeA := range a.Graph.ByName {
		nodeB, ok := b.Graph.ByName[name]
		if !ok {
			return false
		}
		if depsKey(nodeA.Deps) != depsKey(nodeB.Deps) {
			return false
		}
	}
	return true
}

func depsKey(deps []string) string {
	if len(deps) == 0 {
		return ""
	}
	cp := append([]string(nil), deps...)
	sort.Strings(cp)
	return strings.Join(cp, ",")
}

// ApplyStatesFrom copies runtime state from other into s without changing
// graph topology. SameStructure must be true.
func (s *Snapshot) ApplyStatesFrom(other *Snapshot) {
	for pi := range s.Projects {
		src := other.Projects[pi]
		dst := &s.Projects[pi]
		dst.Containers = src.Containers
		dst.ConfigFiles = src.ConfigFiles
		for name, srcNode := range src.Graph.ByName {
			if dstNode, ok := dst.Graph.ByName[name]; ok {
				dstNode.ContainerID = srcNode.ContainerID
				dstNode.State = srcNode.State
				dstNode.ExitCode = srcNode.ExitCode
				dstNode.Image = srcNode.Image
				dstNode.Ports = srcNode.Ports
				dstNode.CreatedAt = srcNode.CreatedAt
			}
		}
	}
	for i := range s.Standalone {
		s.Standalone[i].State = other.Standalone[i].State
		s.Standalone[i].Name = other.Standalone[i].Name
		s.Standalone[i].Image = other.Standalone[i].Image
	}
}

// ProjectConfigKey returns a stable fingerprint of service topology for one project.
func ProjectConfigKey(services map[string]compose.Service) string {
	names := make([]string, 0, len(services))
	for name := range services {
		names = append(names, name)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, name := range names {
		b.WriteString(name)
		b.WriteByte(':')
		deps := sortedDepNames(services[name].DependsOn)
		b.WriteString(strings.Join(deps, ","))
		b.WriteByte(';')
	}
	return b.String()
}

func sortedDepNames(deps compose.DependsOn) []string {
	if len(deps) == 0 {
		return nil
	}
	names := make([]string, 0, len(deps))
	for name := range deps {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
