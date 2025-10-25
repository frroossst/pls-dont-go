package main

import (
	"github.com/frroossst/pls-dont-go/immutablecheck"

	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(immutablecheck.Analyzer)
}
