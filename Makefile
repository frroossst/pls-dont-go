.PHONY: all build clean test test-advanced test-manual test-runner help run plugin lint ensure-golangci custom-gcl

help: ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

all: build

run: build ## Runs linter with examples/all.go
	./immutablelint examples/all.go

build: ## Build the immutable linter
	go build -o immutablelint ./cmd/immutablelint

test: build ## Run linter tests against example files
	./test_runner.bash

plugin: ## Build a golangci-lint compatible plugin
	go build -buildmode=plugin -o immutablecheck.so ./plugin/plugin.go

clean: ## Clean build artifacts
	rm -rf ./immutablelint ./pls-dont-go ./immutablecheck.so ./custom-gcl

ensure-golangci: ## Install golangci-lint if missing
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "Installing golangci-lint v2.5.0..."; \
		GO111MODULE=on go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.5.0; \
	}

custom-gcl: ensure-golangci .custom-gcl.yml ## Build the custom golangci-lint binary
	@echo "Building custom golangci-lint from .custom-gcl.yml..."
	golangci-lint custom -v

lint: custom-gcl ## Build custom-gcl and run all linters
	./custom-gcl run

