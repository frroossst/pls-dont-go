package main

import (
	"github.com/frroossst/pls-dont-go/immutablecheck"
	"golang.org/x/tools/go/analysis"
)

// New is the factory function required by golangci-lint plugin interface.
// For module plugins in v2, this function must exist and return the analyzers.
func New(conf any) ([]*analysis.Analyzer, error) {
	// Return the analyzer directly - golangci-lint will handle it
	return []*analysis.Analyzer{immutablecheck.Analyzer}, nil
}
