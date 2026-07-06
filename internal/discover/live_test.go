//go:build integration

package discover

import (
	"context"
	"testing"

	"github.com/hyturing/compose-pulse/internal/docker"
)

func TestFromDocker_LiveComposeProjects(t *testing.T) {
	dc, err := docker.NewClient()
	if err != nil {
		t.Skip(err)
	}
	defer func() { _ = dc.Close() }()

	snap, err := FromDocker(context.Background(), dc)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Projects) == 0 {
		t.Skip("no compose projects running")
	}

	for _, project := range snap.Projects {
		if project.Name == "" {
			t.Fatal("discovered compose project with empty name")
		}
		if project.Graph == nil || len(project.Graph.ByName) == 0 {
			t.Fatalf("project %q has no discovered services", project.Name)
		}
	}
}
