package docker

import (
	"context"
	"fmt"
	"sort"

	"github.com/docker/docker/api/types/container"
)

// ContainerInfo holds discovered container metadata from a single list call.
type ContainerInfo struct {
	ID       string
	Names    []string
	Image    string
	Labels   map[string]string
	State    ContainerState
	ExitCode *int     // parsed from Status "Exited (N) ..."; nil while running
	Status   string   // raw human-readable Docker status string
	Ports    []string // formatted "8080:80/tcp" (published) or "80/tcp" (exposed only)
	Created  int64    // unix seconds, from container.Summary.Created
}

// ListAll returns every container on the daemon (running and stopped).
func (c *Client) ListAll(ctx context.Context) ([]ContainerInfo, error) {
	containers, err := c.api.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	out := make([]ContainerInfo, 0, len(containers))
	for _, ctr := range containers {
		labels := ctr.Labels
		if labels == nil {
			labels = map[string]string{}
		}
		out = append(out, ContainerInfo{
			ID:       ctr.ID,
			Names:    ctr.Names,
			Image:    ctr.Image,
			Labels:   labels,
			State:    mapContainerState(ctr),
			ExitCode: parseExitCode(ctr.Status),
			Status:   ctr.Status,
			Ports:    formatPorts(ctr.Ports),
			Created:  ctr.Created,
		})
	}
	return out, nil
}

// formatPorts renders a container's ports as "8080:80/tcp" (published) or
// "80/tcp" (exposed only), deduping IPv4/IPv6 duplicates.
func formatPorts(ports []container.Port) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, p := range ports {
		var s string
		if p.PublicPort != 0 {
			s = fmt.Sprintf("%d:%d/%s", p.PublicPort, p.PrivatePort, p.Type)
		} else {
			s = fmt.Sprintf("%d/%s", p.PrivatePort, p.Type)
		}
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
