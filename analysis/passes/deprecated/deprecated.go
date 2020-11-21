package deprecated

import (
	"go/ast"
	"go/types"
	"log"
	"path"
	"reflect"
	"strings"

	"github.com/vmware/govmomi/compat"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/types/typeutil"
)

const (
	vmomiPackage   = "github.com/vmware/govmomi"
	vimMethodsPath = "vim25/methods"
)

// Analyzer is the main analysis object for the deprecated pass.
var Analyzer = &analysis.Analyzer{
	Name:       "deprecated",
	Doc:        "deprecated",
	Requires:   []*analysis.Analyzer{inspect.Analyzer},
	Run:        run,
	ResultType: reflect.TypeOf((*Result)(nil)),
	FactTypes:  []analysis.Fact{new(isDeprecated), new(hasDeprecated)},
}

// Result contains a map of functions marked as deprecated, associated with the corresponding deprecated API call.
type Result struct {
	funcs map[*types.Func]string
}

// A fact to mark a function as deprecated.
// FQN keeps track of the corresponding API identifier.
type isDeprecated struct {
	FQN string
}

func (d *isDeprecated) AFact() {}

// A fact to mark a package as containing deprecated functions.
type hasDeprecated struct{}

func (d *hasDeprecated) AFact() {}

func run(pass *analysis.Pass) (interface{}, error) {
	// The "methods" package contains a 1:1 mapping between vmomi operations and Go functions.
	// This is our starting point for creating a map of all deprecated functions.
	if pass.Pkg.Path() == path.Join(vmomiPackage, vimMethodsPath) {
		return runMethods(pass)
	}

	// Is this package even interesting? Only if it depends on something that has deprecated functions.
	interesting := false
	for _, pkg := range pass.Pkg.Imports() {
		var fact hasDeprecated
		if pass.ImportPackageFact(pkg, &fact) {
			interesting = true
		}
	}

	// Not interesting, so no point analysing further.
	if !interesting {
		return nil, nil
	}

	// There are convenience wrappers associated with the methods above throughout the govmomi module.
	// No point complaining about them, but we need to consider them transitively deprecated too.
	if strings.HasPrefix(pass.Pkg.Path(), vmomiPackage+"/") {
		return runCallers(pass, false)
	}

	// At this point we're in a consumer package, let the complaining begin.
	return runCallers(pass, true)
}

func runMethods(pass *analysis.Pass) (interface{}, error) {
	// This is the root of all deprecations.
	deprecated := compat.GetAPI("7.0").GetDeprecatedMethods()

	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	// We just need function declarations as we'll identify by name.
	nodeFilter := []ast.Node{
		(*ast.FuncDecl)(nil),
	}

	counter := 0
	inspect.Preorder(nodeFilter, func(n ast.Node) {
		fn := n.(*ast.FuncDecl)
		if f, ok := deprecated[fn.Name.String()]; ok {
			obj := pass.TypesInfo.Defs[fn.Name].(*types.Func)
			pass.ExportObjectFact(obj, &isDeprecated{
				FQN: f.FQN,
			})
			counter++
		}
	})

	// We expect the methods package to have a function for each deprecated operation.
	// TODO: better log message, with a diff.
	if counter != len(deprecated) {
		log.Println("some deprecated methods are not mapped")
	}

	if counter != 0 {
		pass.ExportPackageFact(&hasDeprecated{})
	}

	return nil, nil
}

func runCallers(pass *analysis.Pass, report bool) (interface{}, error) {
	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	// We are going to inspect all function declarations to find calls to deprecated
	// functions, and transitively mark them deprecated.
	nodeFilter := []ast.Node{
		(*ast.FuncDecl)(nil),
	}

	// In case the package contains a wrapper, we'll make it interesting.
	makeInteresting := false

	inspect.Preorder(nodeFilter, func(n ast.Node) {
		fn := n.(*ast.FuncDecl)
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			callee := typeutil.Callee(pass.TypesInfo, call)
			// Giving up if typeutil does.
			if callee == nil {
				return true
			}

			var fact isDeprecated
			if pass.ImportObjectFact(callee, &fact) {
				// Calling a deprecated method makes you deprecated !
				obj := pass.TypesInfo.Defs[fn.Name].(*types.Func)
				pass.ExportObjectFact(obj, &isDeprecated{
					FQN: fact.FQN,
				})

				// And the current package interesting.
				makeInteresting = true

				if report {
					pass.Reportf(call.Pos(), "deprecated API detected: %s", fact.FQN)
				}
			}

			return true
		})
	})

	if makeInteresting {
		pass.ExportPackageFact(&hasDeprecated{})
	}

	return nil, nil
}
