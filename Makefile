.PHONY: all build clean test test-advanced test-manual test-runner help run plugin lint ensure-golangci custom-gcl

LATEST_TAG := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
RAW_VER := $(shell echo "$(LATEST_TAG)" | sed -E 's/^v?(.+)/\1/')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
VERSION := adhyan-dev-v$(RAW_VER)

PACKAGE := github.com/frroossst/pls-dont-go/cmd/immutablelint

LDFLAGS := -X '$(PACKAGE).version=$(VERSION)' \
           -X '$(PACKAGE).commit=$(GIT_COMMIT)' \
           -X '$(PACKAGE).buildDate=$(BUILD_DATE)'

help: ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

all: build

tag: ## ask user for a new tag and push it
	@echo "curr = $(RAW_VER)"
	@echo "make sure to commit all changes before tagging!!!"
	@git status
	@read -p "Enter new tag (without 'v' prefix): " newtag; \
	if [ -z "$$newtag" ]; then \
		echo "No tag entered. Aborting."; \
		exit 1; \
	fi; \
	git tag -a "v$$newtag" -m "v$$newtag"; \
	git push origin "v$$newtag"

install: ## Install from source with version info
	go install -ldflags "$(LDFLAGS)" ./cmd/immutablelint

run: build ## Runs linter with examples/all.go
	./immutablelint examples/all.go

build: ## Build the immutable linter
	go build -ldflags "$(LDFLAGS)" -o immutablelint ./cmd/immutablelint

test: build ## Run linter tests against example files
	./test_runner.bash examples/all.go
	make regress

regress: build ## Run regression tests against examples/regression.go
	./test_runner.bash examples/regression.go

plugin: ## Build a golangci-lint compatible plugin
	go build -buildmode=plugin -o immutablecheck.so ./plugin/plugin.go

clean: ## Clean build artifacts
	rm -rf ./immutablelint ./custom-gcl

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

