package docker

import (
	"testing"

	"github.com/docker/docker/api/types/container"
)

func TestParseExitCode(t *testing.T) {
	tests := []struct {
		status string
		want   *int
	}{
		{"Exited (0) 5 minutes ago", intPtr(0)},
		{"Exited (137) About an hour ago", intPtr(137)},
		{"Up 2 minutes (healthy)", nil},
		{"Created", nil},
	}
	for _, tt := range tests {
		got := parseExitCode(tt.status)
		if (got == nil) != (tt.want == nil) {
			t.Errorf("parseExitCode(%q) = %v, want %v", tt.status, got, tt.want)
			continue
		}
		if got != nil && *got != *tt.want {
			t.Errorf("parseExitCode(%q) = %d, want %d", tt.status, *got, *tt.want)
		}
	}
}

func intPtr(i int) *int { return &i }

func TestFormatPorts(t *testing.T) {
	tests := []struct {
		name  string
		ports []container.Port
		want  []string
	}{
		{
			name:  "published",
			ports: []container.Port{{PrivatePort: 80, PublicPort: 8080, Type: "tcp"}},
			want:  []string{"8080:80/tcp"},
		},
		{
			name:  "exposed only",
			ports: []container.Port{{PrivatePort: 80, Type: "tcp"}},
			want:  []string{"80/tcp"},
		},
		{
			name: "dedupe dual-stack",
			ports: []container.Port{
				{IP: "0.0.0.0", PrivatePort: 80, PublicPort: 8080, Type: "tcp"},
				{IP: "::", PrivatePort: 80, PublicPort: 8080, Type: "tcp"},
			},
			want: []string{"8080:80/tcp"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatPorts(tt.ports)
			if len(got) != len(tt.want) {
				t.Fatalf("formatPorts() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("formatPorts()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
