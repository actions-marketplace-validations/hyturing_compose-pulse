package compose

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestPorts_LongFormFromComposeConfig(t *testing.T) {
	raw := []byte(`
services:
  web:
    image: nginx:alpine
    ports:
      - mode: ingress
        target: 80
        published: "18081"
        protocol: tcp
      - "8080:80"
`)
	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	web := cfg.Services["web"]
	if len(web.Ports) != 2 {
		t.Fatalf("ports = %#v", web.Ports)
	}
	if web.Ports[0] != "18081:80" {
		t.Fatalf("long-form port = %q, want 18081:80", web.Ports[0])
	}
	if web.Ports[1] != "8080:80" {
		t.Fatalf("short-form port = %q", web.Ports[1])
	}
}
