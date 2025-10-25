package gcplugin

import (
	"github.com/frroossst/pls-dont-go/immutablecheck"
	"golang.org/x/tools/go/analysis"
)

// New is the factory function required by golangci-lint module plugin interface.
// This must be in an importable (non-main) package for golangci-lint v2 to load it.
func New(conf any) ([]*analysis.Analyzer, error) {
	return []*analysis.Analyzer{immutablecheck.Analyzer}, nil
}
