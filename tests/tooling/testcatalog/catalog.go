package main

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
	"slices"
	"strconv"
	"strings"
)

type catalogEntry struct {
	Package     string   `json:"package"`
	TestName    string   `json:"test_name"`
	File        string   `json:"file"`
	Layer       string   `json:"layer"`
	Domain      string   `json:"domain"`
	ScenarioID  string   `json:"scenario_id"`
	Suite       string   `json:"suite"`
	Tags        []string `json:"tags,omitempty"`
	Speed       string   `json:"speed,omitempty"`
	Environment string   `json:"environment,omitempty"`
}

type goListPackage struct {
	Dir          string   `json:"Dir"`
	ImportPath   string   `json:"ImportPath"`
	TestGoFiles  []string `json:"TestGoFiles"`
	XTestGoFiles []string `json:"XTestGoFiles"`
}

type specData struct {
	Layer       string
	Domain      string
	ScenarioID  string
	Suite       string
	Tags        []string
	Speed       string
	Environment string
}

func listPackages(tags string, patterns []string) ([]goListPackage, error) {
	args := []string{"list"}
	if strings.TrimSpace(tags) != "" {
		args = append(args, "-tags", tags)
	}
	args = append(args, "-json")
	args = append(args, patterns...)

	cmd := exec.Command("go", args...)
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	var packages []goListPackage
	decoder := json.NewDecoder(stdout)
	for {
		var pkg goListPackage
		if err := decoder.Decode(&pkg); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		packages = append(packages, pkg)
	}

	if err := cmd.Wait(); err != nil {
		return nil, err
	}
	return packages, nil
}

func buildCatalog(packages []goListPackage) ([]catalogEntry, error) {
	var entries []catalogEntry
	for _, pkg := range packages {
		files := append([]string{}, pkg.TestGoFiles...)
		files = append(files, pkg.XTestGoFiles...)
		for _, name := range files {
			path := filepath.Join(pkg.Dir, name)
			fileEntries, err := parseCatalogFile(path, pkg.ImportPath)
			if err != nil {
				return nil, err
			}
			entries = append(entries, fileEntries...)
		}
	}
	return entries, nil
}

func parseCatalogFile(path string, importPath string) ([]catalogEntry, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	aliases, err := testkitAliases(file)
	if err != nil {
		return nil, err
	}
	if len(aliases) == 0 {
		return nil, nil
	}

	var entries []catalogEntry
	for _, decl := range file.Decls {
		entry, ok := catalogEntryFromDecl(decl, path, importPath, aliases)
		if !ok {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func testkitAliases(file *ast.File) (map[string]struct{}, error) {
	aliases := map[string]struct{}{}
	for _, spec := range file.Imports {
		importPathValue, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return nil, err
		}
		if importPathValue != "ardents/tests/testkit" {
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

func catalogEntryFromDecl(decl ast.Decl, path string, importPath string, aliases map[string]struct{}) (catalogEntry, bool) {
	fn, ok := decl.(*ast.FuncDecl)
	if !ok || fn.Recv != nil || fn.Body == nil || !strings.HasPrefix(fn.Name.Name, "Test") {
		return catalogEntry{}, false
	}

	spec, ok := findScenarioSpec(fn.Body, aliases)
	if !ok {
		return catalogEntry{}, false
	}

	return catalogEntry{
		Package:     importPath,
		TestName:    fn.Name.Name,
		File:        filepath.Base(path),
		Layer:       spec.Layer,
		Domain:      spec.Domain,
		ScenarioID:  spec.ScenarioID,
		Suite:       spec.Suite,
		Tags:        spec.Tags,
		Speed:       spec.Speed,
		Environment: spec.Environment,
	}, true
}

func findScenarioSpec(body *ast.BlockStmt, aliases map[string]struct{}) (specData, bool) {
	var found specData
	var ok bool
	ast.Inspect(body, func(node ast.Node) bool {
		call, isCall := node.(*ast.CallExpr)
		if !isCall {
			return true
		}

		selector, isSelector := call.Fun.(*ast.SelectorExpr)
		if !isSelector || selector.Sel.Name != "BeginScenario" {
			return true
		}

		ident, isIdent := selector.X.(*ast.Ident)
		if !isIdent {
			return true
		}
		if _, exists := aliases[ident.Name]; !exists || len(call.Args) < 2 {
			return true
		}

		spec, matched := parseSpecLiteral(call.Args[1], aliases)
		if matched {
			found = spec
			ok = true
		}
		return false
	})
	return found, ok
}

func parseSpecLiteral(expr ast.Expr, aliases map[string]struct{}) (specData, bool) {
	lit, ok := expr.(*ast.CompositeLit)
	if !ok || !isTestkitSpecLiteral(lit, aliases) {
		return specData{}, false
	}

	var spec specData
	for _, elt := range lit.Elts {
		field, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := field.Key.(*ast.Ident)
		if !ok {
			continue
		}
		switch key.Name {
		case "Layer":
			spec.Layer = parseLayerValue(field.Value)
		case "Domain":
			spec.Domain = parseStringValue(field.Value)
		case "ScenarioID":
			spec.ScenarioID = parseStringValue(field.Value)
		case "Suite":
			spec.Suite = parseStringValue(field.Value)
		case "Tags":
			spec.Tags = parseStringSlice(field.Value)
		case "Speed":
			spec.Speed = parseStringValue(field.Value)
		case "Environment":
			spec.Environment = parseStringValue(field.Value)
		}
	}

	if spec.Layer == "" || spec.Domain == "" || spec.ScenarioID == "" || spec.Suite == "" {
		return specData{}, false
	}
	return spec, true
}

func isTestkitSpecLiteral(lit *ast.CompositeLit, aliases map[string]struct{}) bool {
	selector, ok := lit.Type.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Spec" {
		return false
	}
	ident, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}
	_, exists := aliases[ident.Name]
	return exists
}

func parseLayerValue(expr ast.Expr) string {
	selector, ok := expr.(*ast.SelectorExpr)
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

func parseStringValue(expr ast.Expr) string {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return ""
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return ""
	}
	return value
}

func parseStringSlice(expr ast.Expr) []string {
	lit, ok := expr.(*ast.CompositeLit)
	if !ok {
		return nil
	}
	values := make([]string, 0, len(lit.Elts))
	for _, elt := range lit.Elts {
		value := parseStringValue(elt)
		if value == "" {
			continue
		}
		values = append(values, value)
	}
	return values
}

func filterCatalog(entries []catalogEntry, layer string, domain string, scenario string, tag string, suite string) []catalogEntry {
	filtered := make([]catalogEntry, 0, len(entries))
	for _, entry := range entries {
		if layer != "" && entry.Layer != layer {
			continue
		}
		if domain != "" && entry.Domain != domain {
			continue
		}
		if scenario != "" && entry.ScenarioID != scenario {
			continue
		}
		if suite != "" && entry.Suite != suite {
			continue
		}
		if tag != "" && !contains(entry.Tags, tag) {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

func contains(values []string, want string) bool {
	return slices.Contains(values, want)
}
