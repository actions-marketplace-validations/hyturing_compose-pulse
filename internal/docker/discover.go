package docker

import (
	"context"

	"github.com/docker/docker/api/types/container"
)

// ContainerInfo holds discovered container metadata from a single list call.
type ContainerInfo struct {
	ID     string
	Names  []string
	Image  string
	Labels map[string]string
	State  ContainerState
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
			ID:     ctr.ID,
			Names:  ctr.Names,
			Image:  ctr.Image,
			Labels: labels,
			State:  mapContainerState(ctr),
		})
	}
	return out, nil
}
