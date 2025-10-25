#! /bin/bash

cat << 'EOF' > .golangci.yml
version: "2"

linters:
  enable:
    - immutablecheck
  settings:
    custom:
      immutablecheck:
        type: "module"
        description: Immutable type mutation checker (pls-dont-go)
        # settings: {} # add plugin-specific settings here if needed

EOF


cat << 'EOF' > .custom-gcl.yml
version: v2.5.0
plugins:
    - module: "github.com/frroossst/pls-dont-go"
      path: .
      import: "github.com/frroossst/pls-dont-go/immutablecheck"
EOF

command -v golangci-lint >/dev/null 2>&1 || { \
	echo "Installing golangci-lint v2.5.0..."; \
	GO111MODULE=on go install github.com/golangci/golangci-lint/cmd/golangci-lint@v2.5.0; \
}

golangci-lint custom -v

./custom-gcl run

echo "Run ./cutom-gcl run to execute the immutablecheck linter or install it with"
echo "go install github.com/frroossst/pls-dont-go/cmd/immutablelint@latest"
echo "and run it like immutablelint *.go"
