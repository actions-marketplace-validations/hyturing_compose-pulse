package dag

import (
	"testing"

	"github.com/hyturing/compose-pulse/internal/compose"
)

func TestDirectAndTransitiveDependents(t *testing.T) {
	cfg := &compose.Config{Services: map[string]compose.Service{
		"postgres": {},
		"api":      {DependsOn: compose.DependsOn{"postgres": {Condition: "service_healthy"}}},
		"worker":   {DependsOn: compose.DependsOn{"api": {Condition: "service_started"}}},
		"web":      {DependsOn: compose.DependsOn{"api": {Condition: "service_started"}}},
	}}
	g, err := Build(cfg)
	if err != nil {
		t.Fatal(err)
	}

	direct := DirectDependents(g, "postgres")
	if len(direct) != 1 || direct[0] != "api" {
		t.Fatalf("DirectDependents(postgres) = %v", direct)
	}
	trans := TransitiveDependents(g, "postgres")
	if len(trans) != 2 {
		t.Fatalf("TransitiveDependents(postgres) = %v", trans)
	}
	all := AllDependents(g, "postgres")
	if len(all) != 3 {
		t.Fatalf("AllDependents(postgres) = %v", all)
	}
}
