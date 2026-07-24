// Package scenariocatalog parses the testkit metadata that defines tagged
// integration and end-to-end scenarios. It does not execute those scenarios.
package scenariocatalog

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Package identifies test files selected by the Go tool for one package.
type Package struct {
	Dir          string
	ImportPath   string
	TestGoFiles  []string
	XTestGoFiles []string
}

// Entry is the metadata declared by one testkit.BeginScenario call.
type Entry struct {
	TestName    string
	Layer       string
	Domain      string
	ScenarioID  string
	Suite       string
	Tags        []string
	Speed       string
	Environment string
}

// ListPackages uses the same Go build-tag selection as the tagged scenario
// runner and returns only files that participate in that build.
func ListPackages(root, tags string, patterns []string) ([]Package, error) {
	args := []string{"list"}
	if strings.TrimSpace(tags) != "" {
		args = append(args, "-tags", tags)
	}
	args = append(args, "-json")
	args = append(args, patterns...)
	command := exec.Command("go", args...)
	if root != "" {
		command.Dir = root
	}
	command.Stderr = os.Stderr
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := command.Start(); err != nil {
		return nil, err
	}
	var packages []Package
	decoder := json.NewDecoder(stdout)
	for {
		var item Package
		if err := decoder.Decode(&item); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		packages = append(packages, item)
	}
	if err := command.Wait(); err != nil {
		return nil, err
	}
	return packages, nil
}

// Files returns selected test files in package order.
func Files(packages []Package) []string {
	var files []string
	for _, item := range packages {
		names := append(append([]string{}, item.TestGoFiles...), item.XTestGoFiles...)
		for _, name := range names {
			files = append(files, filepath.Join(item.Dir, name))
		}
	}
	return files
}

// ParseFile returns the complete, valid scenario entries declared by Test
// functions in one Go test file.
func ParseFile(path string) ([]Entry, error) {
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	aliases, err := testkitAliases(file)
	if err != nil || len(aliases) == 0 {
		return nil, err
	}
	var entries []Entry
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Recv != nil || function.Body == nil || !strings.HasPrefix(function.Name.Name, "Test") {
			continue
		}
		entry, found, findErr := findScenario(function, aliases)
		if findErr != nil {
			return nil, fmt.Errorf("%s: %s: %w", path, function.Name.Name, findErr)
		}
		if found {
			entry.TestName = function.Name.Name
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

func testkitAliases(file *ast.File) (map[string]struct{}, error) {
	aliases := map[string]struct{}{}
	for _, spec := range file.Imports {
		value, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return nil, err
		}
		if value != "ardents/tests/testkit" {
			continue
		}
		alias := "testkit"
		if spec.Name != nil {
			alias = spec.Name.Name
		}
		aliases[alias] = struct{}{}
	}
	return aliases, nil
}

func findScenario(function *ast.FuncDecl, aliases map[string]struct{}) (Entry, bool, error) {
	var found Entry
	var matched bool
	var validationErr error
	ast.Inspect(function.Body, func(node ast.Node) bool {
		if validationErr != nil {
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if !ok || !isSelector(call.Fun, aliases, "BeginScenario") {
			return true
		}
		if matched {
			validationErr = fmt.Errorf("multiple BeginScenario calls")
			return false
		}
		if len(call.Args) < 2 {
			validationErr = fmt.Errorf("incomplete scenario metadata")
			return false
		}
		entry, ok := parseSpec(call.Args[1], aliases)
		if !ok {
			validationErr = fmt.Errorf("incomplete scenario metadata")
			return false
		}
		found, matched = entry, true
		return true
	})
	return found, matched, validationErr
}

func parseSpec(expression ast.Expr, aliases map[string]struct{}) (Entry, bool) {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok || !isSelector(literal.Type, aliases, "Spec") {
		return Entry{}, false
	}
	var entry Entry
	for _, element := range literal.Elts {
		field, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := field.Key.(*ast.Ident)
		if !ok {
			continue
		}
		switch key.Name {
		case "Layer":
			entry.Layer = parseLayer(field.Value)
		case "Domain":
			entry.Domain = parseString(field.Value)
		case "ScenarioID":
			entry.ScenarioID = parseString(field.Value)
		case "Suite":
			entry.Suite = parseString(field.Value)
		case "Tags":
			entry.Tags = parseStrings(field.Value)
		case "Speed":
			entry.Speed = parseString(field.Value)
		case "Environment":
			entry.Environment = parseString(field.Value)
		}
	}
	if entry.Layer == "" || entry.Domain == "" || entry.ScenarioID == "" || entry.Suite == "" {
		return Entry{}, false
	}
	return entry, true
}

func isSelector(expression ast.Expr, aliases map[string]struct{}, name string) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != name {
		return false
	}
	identifier, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}
	_, ok = aliases[identifier.Name]
	return ok
}

func parseLayer(expression ast.Expr) string {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	switch selector.Sel.Name {
	case "LayerIntegration":
		return "integration"
	case "LayerE2E":
		return "e2e"
	default:
		return ""
	}
}

func parseString(expression ast.Expr) string {
	literal, ok := expression.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return ""
	}
	value, err := strconv.Unquote(literal.Value)
	if err != nil {
		return ""
	}
	return value
}

func parseStrings(expression ast.Expr) []string {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return nil
	}
	values := make([]string, 0, len(literal.Elts))
	for _, element := range literal.Elts {
		if value := parseString(element); value != "" {
			values = append(values, value)
		}
	}
	return values
}
