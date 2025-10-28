package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/frroossst/pls-dont-go/immutablecheck"

	"golang.org/x/tools/go/analysis/singlechecker"
)

// set by -ldflags "-X main.version=..."
var version = "adhyan-dev-v<unset>"

func main() {
	// quick-and-dirty: if -V or -V=* seen, print version and exit
	for _, a := range os.Args[1:] {
		if a == "-V" || strings.HasPrefix(a, "-V=") || a == "--version" {
			fmt.Println(version)
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
