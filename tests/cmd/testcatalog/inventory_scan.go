package main

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

func collectInventoryTests(patterns []string, modulePath string) ([]parsedTest, error) {
	roots, err := inventoryRoots(patterns)
	if err != nil {
		return nil, err
	}

	var tests []parsedTest
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() || !strings.HasSuffix(path, "_test.go") {
				return nil
			}

			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
			if err != nil {
				return fmt.Errorf("parse %s: %w", path, err)
			}

			fileTests, err := parseInventoryFile(file, path, modulePath)
			if err != nil {
				return err
			}
			tests = append(tests, fileTests...)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	if containsInventoryRoot(roots, "tests/e2e") {
		external, err := collectExternalScenarioTests()
		if err != nil {
			return nil, err
		}
		tests = append(tests, external...)
	}
	return tests, nil
}

func containsInventoryRoot(roots []string, wanted string) bool {
	for _, root := range roots {
		if filepath.Clean(root) == filepath.Clean(wanted) {
			return true
		}
	}
	return false
}

func collectExternalScenarioTests() ([]parsedTest, error) {
	var tests []parsedTest
	err := filepath.WalkDir("tests/ci", func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !strings.HasSuffix(path, ".ps1") {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		fields := map[string]string{}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if !strings.HasPrefix(line, "# testkit:") {
				if line != "" && !strings.HasPrefix(line, "#") {
					break
				}
				continue
			}
			parts := strings.SplitN(strings.TrimPrefix(line, "# testkit:"), " ", 2)
			if len(parts) == 2 {
				fields[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
		if err := scanner.Err(); err != nil {
			return err
		}
		if fields["scenario"] == "" {
			return nil
		}
		if fields["layer"] == "" || fields["domain"] == "" {
			return fmt.Errorf("external scenario %s is missing layer or domain metadata", path)
		}
		tests = append(tests, parsedTest{
			Package: modulePathForExternal(), TestName: strings.TrimSuffix(filepath.Base(path), ".ps1"),
			File: filepath.Base(path), Layer: fields["layer"], Domain: fields["domain"],
			ScenarioID: fields["scenario"], BindingSource: "formal",
		})
		return nil
	})
	return tests, err
}

func modulePathForExternal() string {
	modulePath, err := findModulePath()
	if err != nil {
		return "tests/ci"
	}
	return modulePath + "/tests/ci"
}

func inventoryRoots(patterns []string) ([]string, error) {
	if len(patterns) == 0 {
		return []string{"tests/integration", "tests/e2e"}, nil
	}

	seen := map[string]struct{}{}
	var roots []string
	for _, pattern := range patterns {
		normalized := filepath.Clean(strings.TrimPrefix(pattern, "./"))
		switch {
		case normalized == filepath.Clean("tests/...") || normalized == "tests":
			roots = appendInventoryRoot(roots, seen, "tests/integration")
			roots = appendInventoryRoot(roots, seen, "tests/e2e")
		case strings.HasPrefix(normalized, filepath.Clean("tests/integration")):
			roots = appendInventoryRoot(roots, seen, "tests/integration")
		case strings.HasPrefix(normalized, filepath.Clean("tests/e2e")):
			roots = appendInventoryRoot(roots, seen, "tests/e2e")
		}
	}
	if len(roots) == 0 {
		return nil, fmt.Errorf("inventory mode expects tests/integration or tests/e2e patterns")
	}
	return roots, nil
}

func appendInventoryRoot(roots []string, seen map[string]struct{}, root string) []string {
	if _, exists := seen[root]; exists {
		return roots
	}
	seen[root] = struct{}{}
	return append(roots, root)
}

func parseInventoryFile(file *ast.File, path string, modulePath string) ([]parsedTest, error) {
	aliases, err := testkitAliases(file)
	if err != nil {
		return nil, err
	}

	importPath, err := importPathForFile(path, modulePath)
	if err != nil {
		return nil, err
	}

	var tests []parsedTest
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || fn.Body == nil || !strings.HasPrefix(fn.Name.Name, "Test") || isHelperProcessTest(fn.Name.Name) {
			continue
		}
		tests = append(tests, parsedInventoryTest(fn, path, importPath, aliases))
	}
	return tests, nil
}

func isHelperProcessTest(name string) bool {
	return strings.HasSuffix(name, "Helper")
}

func parseInventorySource(path string, source string, modulePath string) ([]parsedTest, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, source, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	return parseInventoryFile(file, path, modulePath)
}

func parsedInventoryTest(fn *ast.FuncDecl, path string, importPath string, aliases map[string]struct{}) parsedTest {
	test := parsedTest{
		Package:       importPath,
		TestName:      fn.Name.Name,
		File:          filepath.Base(path),
		Layer:         inferLayerFromPath(path),
		BindingSource: "missing",
	}

	if spec, ok := findScenarioSpec(fn.Body, aliases); ok {
		test.Domain = spec.Domain
		test.ScenarioID = spec.ScenarioID
		test.BindingSource = "formal"
		return test
	}
	return test
}
