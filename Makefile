BINARY   := cpulse
MODULE   := github.com/hyturing/compose-pulse
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  := -ldflags "-s -w -X main.version=$(VERSION)"
OUT      := bin/$(BINARY)

.PHONY: build run test lint clean deps help

build: ## Build the binary to bin/cpulse
	@mkdir -p bin
	go build $(LDFLAGS) -o $(OUT) ./cmd/cpulse

run: ## Run directly with go run (discovers all local Docker containers)
	go run $(LDFLAGS) ./cmd/cpulse $(ARGS)

test: ## Run all tests
	go test -race -count=1 ./...

test-cover: ## Run tests with coverage report
	go test -race -coverprofile=coverage.txt -covermode=atomic ./...
	go tool cover -html=coverage.txt -o coverage.html

lint: ## Run golangci-lint
	golangci-lint run ./...

deps: ## Install / tidy Go dependencies
	go get \
		github.com/charmbracelet/bubbletea \
		github.com/charmbracelet/lipgloss \
		github.com/charmbracelet/bubbles \
		github.com/docker/docker \
		github.com/docker/go-connections \
		gopkg.in/yaml.v3
	go mod tidy

clean: ## Remove build artifacts
	rm -rf bin/ dist/ coverage.txt coverage.html

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
