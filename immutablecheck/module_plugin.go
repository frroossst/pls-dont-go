package immutablecheck

import (
	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"
)

// pluginModule implements the module plugin interface for golangci-lint v2
type pluginModule struct {
	settings any // reserved for future settings
}

// PluginNew is registered with golangci-lint module plugin system.
// It returns a linter plugin instance that exposes our analyzers.
func PluginNew(settings any) (register.LinterPlugin, error) {
	return &pluginModule{settings: settings}, nil
}

// BuildAnalyzers returns the list of analyzers provided by this plugin.
func (p *pluginModule) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	return []*analysis.Analyzer{Analyzer}, nil
}

// GetLoadMode specifies which loading mode is required by this plugin.
// Our analyzer uses types information, hence LoadModeTypesInfo.
func (p *pluginModule) GetLoadMode() string {
	return register.LoadModeTypesInfo
}

// Register the plugin at init time under the name "immutablecheck".
func init() {
	register.Plugin("immutablecheck", PluginNew)
}
