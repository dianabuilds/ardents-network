package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var selectedTestName = regexp.MustCompile(`Test[A-Za-z0-9_]+`)

func TestHeadlessEvidenceNamesExistingPackageTests(t *testing.T) {
	root := repositoryRoot(t)
	makefile := string(readProjectFile(t, root, "Makefile"))
	recipes := headlessEvidenceGoTests(t, makefile)
	if len(recipes) == 0 {
		t.Fatal("headless-evidence selects no Go behavior tests")
	}
	for _, recipe := range recipes {
		fields := strings.Fields(recipe)
		if len(fields) < 5 || fields[0] != "go" || fields[1] != "test" {
			t.Fatalf("unsupported headless-evidence Go recipe %q", recipe)
		}
		packagePath := fields[2]
		runIndex := -1
		for index, field := range fields {
			if field == "-run" {
				runIndex = index
				break
			}
		}
		if runIndex < 0 || runIndex+1 >= len(fields) {
			t.Fatalf("headless-evidence recipe has no exact test selection: %q", recipe)
		}
		selected := selectedTestName.FindAllString(strings.Trim(fields[runIndex+1], "'\""), -1)
		if len(selected) == 0 {
			t.Fatalf("headless-evidence recipe selects no named tests: %q", recipe)
		}
		available := packageTestFunctions(t, root, packagePath)
		for _, name := range selected {
			if !available[name] {
				t.Errorf("headless-evidence selects absent %s in %s", name, packagePath)
			}
		}
	}
}

func TestHeadlessEvidenceBindsTheExactControlArtifact(t *testing.T) {
	root := repositoryRoot(t)
	makefile := string(readProjectFile(t, root, "Makefile"))
	for _, required := range []string{
		"headless-evidence: export ARDENTS_E2E_CONTROL := $(abspath $(HEADLESS_CONTROL_ARTIFACT))",
		"TestAlphaControlReaderVerifiesPinnedBundleAndCachedRestart",
	} {
		if !strings.Contains(makefile, required) {
			t.Errorf("headless evidence does not bind exact control artifact through %q", required)
		}
	}
	controlTest := string(readProjectFile(t, root, "tests/e2e/endpoint/alpha_control_reader_process_unix_test.go"))
	if !strings.Contains(controlTest, `os.Getenv("ARDENTS_E2E_CONTROL")`) {
		t.Fatal("selected control process test does not consume ARDENTS_E2E_CONTROL")
	}
}

func headlessEvidenceGoTests(t *testing.T, makefile string) []string {
	t.Helper()
	const start = "headless-evidence: headless-build"
	inRecipe := false
	var recipes []string
	for _, line := range strings.Split(makefile, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == start {
			inRecipe = true
			continue
		}
		if !inRecipe {
			continue
		}
		if trimmed != "" && !strings.HasPrefix(line, "\t") {
			break
		}
		if strings.HasPrefix(trimmed, "go test ") {
			recipes = append(recipes, trimmed)
		}
	}
	return recipes
}

func packageTestFunctions(t *testing.T, root, packagePath string) map[string]bool {
	t.Helper()
	directory := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(packagePath, "./")))
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("read selected test package %s: %v", packagePath, err)
	}
	functions := make(map[string]bool)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(token.NewFileSet(), filepath.Join(directory, entry.Name()), nil, 0)
		if parseErr != nil {
			t.Fatalf("parse selected test source %s: %v", entry.Name(), parseErr)
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if ok && function.Recv == nil && selectedTestName.MatchString(function.Name.Name) {
				functions[function.Name.Name] = true
			}
		}
	}
	return functions
}
