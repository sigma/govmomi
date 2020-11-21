// The vmomi-deprecated command runs the corresponding analyzer.
package main

import (
	"github.com/vmware/govmomi/analysis/passes/deprecated"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() { singlechecker.Main(deprecated.Analyzer) }
