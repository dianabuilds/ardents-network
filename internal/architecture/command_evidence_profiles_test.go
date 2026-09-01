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

func TestLocalCeremonyProfileHasClosedCommandAndArtifactBoundary(t *testing.T) {
	root := repositoryRoot(t)
	registry := readTestProfileRegistry(t, root)
	var ceremony *testProfile
	for index := range registry.Profiles {
		if registry.Profiles[index].ID == "local-ceremony" {
			ceremony = &registry.Profiles[index]
			break
		}
	}
	if ceremony == nil {
		t.Fatal("local ceremony execution profile is not registered")
	}
	if ceremony.MakeTarget != "ceremony-check" {
		t.Errorf("local ceremony make target = %q, want ceremony-check", ceremony.MakeTarget)
	}

	commands := strings.Fields(string(readProjectFile(t, root, "tests/profiles/local-ceremony-commands.txt")))
	want := []string{"./cmd/ardents-release-custody", "./cmd/ardents-state-custody"}
	if len(commands) != len(want) {
		t.Fatalf("local ceremony command inventory = %v, want %v", commands, want)
	}
	participant := packageSet(t, string(readProjectFile(t, root, "tests/profiles/headless-commands.txt")))
	browser := packageSet(t, string(readProjectFile(t, root, "tests/profiles/browser-commands.txt")))
	for index, command := range want {
		if commands[index] != command {
			t.Errorf("local ceremony command %d = %q, want %q", index, commands[index], command)
		}
		if participant[command] || browser[command] {
			t.Errorf("local ceremony command %q is mixed into a product artifact inventory", command)
		}
	}

	makefile := string(readProjectFile(t, root, "Makefile"))
	for _, required := range []string{
		"CEREMONY_COMMANDS := $(subst $(newline), ,$(file <tests/profiles/local-ceremony-commands.txt))",
		"ceremony-build:",
		"$(foreach command,$(CEREMONY_COMMANDS)",
		"ceremony-check: ceremony-build",
	} {
		if !strings.Contains(makefile, required) {
			t.Errorf("local ceremony Make boundary lacks %q", required)
		}
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
