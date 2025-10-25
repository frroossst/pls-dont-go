.PHONY: all build clean test test-advanced test-manual test-runner help run

help: ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

all: build

run: build ## Runs linter with examples/all.go
	./pls-dont-go examples/all.go

build: ## Build the immutable linter
	go build -o pls-dont-go ./cmd/immutablelint

test: build ## Run linter tests against example files
	./test_runner.bash

clean: ## Clean build artifacts
	rm -rf ./pls-dont-go

