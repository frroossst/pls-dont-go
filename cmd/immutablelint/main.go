package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/frroossst/pls-dont-go/immutablecheck"

	"golang.org/x/tools/go/analysis/singlechecker"
)

// These will be set by ldflags during build
var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	// quick-and-dirty: if -V or -V=* or --version seen, print version and exit
	for _, a := range os.Args[1:] {
		if a == "-V" || strings.HasPrefix(a, "-V=") || a == "--version" {
			printVersion()
			os.Exit(0)
		}
	}

	// Parse the --log flag before singlechecker processes the args
	var logDest string
	for i, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "--log=") {
			logDest = strings.TrimPrefix(arg, "--log=")
			// Remove this flag from os.Args so singlechecker doesn't see it
			os.Args = append(os.Args[:i+1], os.Args[i+2:]...)
			break
		}
	}

	// Set the log destination in the analyzer
	immutablecheck.SetLogDestination(logDest)

	singlechecker.Main(immutablecheck.Analyzer)
}

func printVersion() {
	fmt.Printf("immutablelint %s\n", getVersion())
	if commit != "unknown" {
		fmt.Printf("  commit: %s\n", commit)
	}
	if buildDate != "unknown" {
		fmt.Printf("  built:  %s\n", buildDate)
	}
}

func getVersion() string {
	// If version was set via ldflags (local build with make)
	if version != "dev" {
		return version
	}

	// Try to get version from Go module info (go install)
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			return info.Main.Version
		}
	}

	return "dev"
}
