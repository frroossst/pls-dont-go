package main

import (
	"github.com/frroossst/pls-dont-go/immutablecheck"
	"golang.org/x/tools/go/analysis"
)

// AnalyzerPlugin is the entry point for golangci-lint plugin system
type analyzerPlugin struct{}

func (analyzerPlugin) GetAnalyzers() []*analysis.Analyzer {
	return []*analysis.Analyzer{immutablecheck.Analyzer}
}

// This variable must be named "AnalyzerPlugin" and be exported for golangci-lint
var AnalyzerPlugin analyzerPlugin
