package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/frroossst/pls-dont-go/immutablecheck"

	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	// quick-and-dirty: if -V or -V=* seen, print version and exit
	for _, a := range os.Args[1:] {
		if a == "-V" || strings.HasPrefix(a, "-V=") || a == "--version" {
			fmt.Println("adhyan-dev-v0.3.0")
		}
	}

	singlechecker.Main(immutablecheck.Analyzer)
}
