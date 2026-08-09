package architecture

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

const modulePath = "github.com/dianabuilds/ardents-network"

var forbiddenPackageNames = map[string]bool{
	"adapters":   true,
	"api":        true,
	"common":     true,
	"interfaces": true,
	"misc":       true,
	"models":     true,
	"pkg":        true,
	"sdk":        true,
	"services":   true,
	"src":        true,
	"types":      true,
	"util":       true,
}

var laboratoryPackages = map[string]bool{
	"internal/directcontrol": true,
	"internal/harness":       true,
	"internal/preflight":     true,
}

func TestRepositoryArchitecture(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	assertRequiredProjectFiles(t, root)
	assertSingleModule(t, root)
	assertDependenciesRegistered(t, root)
	assertGoFilesAreProjectCode(t, root)
	assertPackages(t, root)
	assertQualityWiring(t, root)
	assertRepositoryContainsNoArtifacts(t, root)
}

func assertQualityWiring(t *testing.T, root string) {
	t.Helper()
	makefile := readProjectFile(t, root, "Makefile")
	for _, required := range []string{
		"quick-check: format-check vet test build mod-check",
		"check: tools-check quick-check test-race staticcheck vuln",
		"GOTOOLCHAIN := local",
		"GOMODCACHE := $(QUALITY_CACHE_ROOT)/go-mod",
	} {
		if !bytes.Contains(makefile, []byte(required)) {
			t.Errorf("Makefile is missing mandatory quality control %q", required)
		}
	}
	hook := readProjectFile(t, root, ".githooks/pre-commit")
	if !bytes.Contains(hook, []byte("exec make quick-check")) {
		t.Error("pre-commit hook must run make quick-check")
	}
	workflow := readProjectFile(t, root, ".github/workflows/quality.yml")
	for _, required := range []string{"contents: read", "go-version: 1.26.5", "run: make check"} {
		if !bytes.Contains(workflow, []byte(required)) {
			t.Errorf("CI workflow is missing mandatory quality control %q", required)
		}
	}
	actionPin := regexp.MustCompile(`^[[:space:]]*uses:[[:space:]]+[^@[:space:]]+@[0-9a-f]{40}([[:space:]]+#.*)?$`)
	scanner := bufio.NewScanner(bytes.NewReader(workflow))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "uses:") && !actionPin.MatchString(line) {
			t.Errorf("GitHub Action must be pinned to a full commit SHA: %s", strings.TrimSpace(line))
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan CI workflow: %v", err)
	}
}

func assertDependenciesRegistered(t *testing.T, root string) {
	t.Helper()
	moduleFile, err := os.Open(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("open go.mod: %v", err)
	}
	defer moduleFile.Close()
	registry, err := os.ReadFile(filepath.Join(root, "docs", "development", "dependencies.md"))
	if err != nil {
		t.Fatalf("read dependency register: %v", err)
	}
	inRequireBlock := false
	scanner := bufio.NewScanner(moduleFile)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "require (" {
			inRequireBlock = true
			continue
		}
		if inRequireBlock && line == ")" {
			inRequireBlock = false
			continue
		}
		if strings.HasPrefix(line, "require ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "require "))
		} else if !inRequireBlock {
			continue
		}
		module := strings.Fields(line)
		if len(module) > 0 && !bytes.Contains(registry, []byte("`"+module[0]+"`")) {
			t.Errorf("dependency %q is missing from docs/development/dependencies.md", module[0])
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan go.mod: %v", err)
	}
}

func assertRequiredProjectFiles(t *testing.T, root string) {
	t.Helper()
	required := []string{
		"go.mod", "Makefile", "CONTRIBUTING.md", ".github/workflows/quality.yml", ".githooks/pre-commit",
		"docs/development/go-engineering.md", "docs/development/dependencies.md",
		"docs/development/repository-layout.md", "docs/development/package-map.md",
	}
	for _, relative := range required {
		if info, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); err != nil || info.IsDir() {
			t.Errorf("required project file %q is missing", relative)
		}
	}
}

func assertSingleModule(t *testing.T, root string) {
	t.Helper()
	var modules []string
	walk(t, root, func(path string, entry os.DirEntry) {
		if !entry.IsDir() && entry.Name() == "go.mod" {
			modules = append(modules, relativePath(t, root, path))
		}
	})
	if len(modules) != 1 || modules[0] != "go.mod" {
		t.Errorf("project must have exactly one root Go module; found %v", modules)
	}
}

func assertGoFilesAreProjectCode(t *testing.T, root string) {
	t.Helper()
	walk(t, root, func(path string, entry os.DirEntry) {
		if entry.IsDir() || (!strings.HasSuffix(entry.Name(), ".go") && entry.Name() != "go.mod") {
			return
		}
		relative := relativePath(t, root, path)
		if strings.HasPrefix(relative, "experiments/") {
			if strings.HasSuffix(relative, ".go") {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Errorf("read experiment source %s: %v", relative, err)
				} else if !isBuildIgnored(data) {
					t.Errorf("Go experiment must be build-ignored so it is not a maintained root-module package: %s", relative)
				}
			}
			return
		}
		if strings.HasSuffix(relative, ".go") &&
			!strings.HasPrefix(relative, "cmd/") &&
			!strings.HasPrefix(relative, "internal/") &&
			!strings.HasPrefix(relative, "scripts/") {
			t.Errorf("Go code must live in cmd, internal, or scripts: %s", relative)
		}
	})
}

func assertPackages(t *testing.T, root string) {
	t.Helper()
	packages := make(map[string][]string)
	seen := make(map[string]bool)
	registry := readPackageRegistry(t, root)
	assertRegisteredDependencies(t, registry)
	walk(t, root, func(path string, entry os.DirEntry) {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			return
		}
		directory := filepath.Dir(path)
		packages[directory] = append(packages[directory], path)
	})

	for directory, files := range packages {
		relativeDirectory := relativePath(t, root, directory)
		packageName, maintained := assertPackage(t, root, relativeDirectory, files)
		if !maintained {
			continue
		}
		registration, registered := registry[relativeDirectory]
		if !registered {
			t.Errorf("maintained package is not registered in docs/development/package-map.md: %s", relativeDirectory)
			continue
		}
		seen[relativeDirectory] = true
		if packageName != registration.name {
			t.Errorf("package %s is named %q, want %q", relativeDirectory, packageName, registration.name)
		}
		assertPackageImports(t, relativeDirectory, files, registration, registry)
	}
	for directory := range registry {
		if !seen[directory] {
			t.Errorf("package map entry has no maintained package: %s", directory)
		}
	}
}

type packageRegistration struct {
	name           string
	allowedImports map[string]bool
}

func readPackageRegistry(t *testing.T, root string) map[string]packageRegistration {
	t.Helper()
	registry := make(map[string]packageRegistration)
	contents := string(readProjectFile(t, root, "docs/development/package-map.md"))
	inlineCode := regexp.MustCompile("`([^`]+)`")
	for _, line := range strings.Split(contents, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "| `cmd/") && !strings.HasPrefix(trimmed, "| `internal/") {
			continue
		}
		cells := strings.Split(strings.Trim(trimmed, "|"), "|")
		if len(cells) != 4 {
			t.Fatalf("package map row must have four columns: %s", trimmed)
		}
		directoryMatch := inlineCode.FindStringSubmatch(cells[0])
		declarationMatch := inlineCode.FindStringSubmatch(cells[1])
		if len(directoryMatch) != 2 || len(declarationMatch) != 2 || !strings.HasPrefix(declarationMatch[1], "package ") {
			t.Fatalf("invalid package map registration: %s", trimmed)
		}
		directory := directoryMatch[1]
		if _, duplicate := registry[directory]; duplicate {
			t.Fatalf("duplicate package map registration: %s", directory)
		}
		allowed := make(map[string]bool)
		for _, match := range inlineCode.FindAllStringSubmatch(cells[3], -1) {
			if strings.HasPrefix(match[1], "cmd/") || strings.HasPrefix(match[1], "internal/") {
				allowed[match[1]] = true
			}
		}
		registry[directory] = packageRegistration{
			name:           strings.TrimPrefix(declarationMatch[1], "package "),
			allowedImports: allowed,
		}
	}
	if len(registry) == 0 {
		t.Fatal("package map contains no maintained package registrations")
	}
	return registry
}

func assertRegisteredDependencies(t *testing.T, registry map[string]packageRegistration) {
	t.Helper()
	for owner, registration := range registry {
		for dependency := range registration.allowedImports {
			if _, exists := registry[dependency]; !exists {
				t.Errorf("package %s permits unregistered dependency %s", owner, dependency)
			}
			if isProductPackage(owner) && laboratoryPackages[dependency] {
				t.Errorf("product package %s cannot depend on laboratory package %s", owner, dependency)
			}
		}
	}

	state := make(map[string]uint8)
	var visit func(string, []string)
	visit = func(current string, path []string) {
		if state[current] == 2 {
			return
		}
		if state[current] == 1 {
			t.Errorf("package map permits a dependency cycle: %s", strings.Join(append(path, current), " -> "))
			return
		}
		state[current] = 1
		for dependency := range registry[current].allowedImports {
			visit(dependency, append(path, current))
		}
		state[current] = 2
	}
	for directory := range registry {
		visit(directory, nil)
	}
}

func assertPackageImports(t *testing.T, directory string, files []string, registration packageRegistration, registry map[string]packageRegistration) {
	t.Helper()
	actual := make(map[string]bool)
	set := token.NewFileSet()
	for _, path := range files {
		file, err := parser.ParseFile(set, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Errorf("parse imports for %s: %v", relativePath(t, repositoryRoot(t), path), err)
			continue
		}
		for _, imported := range file.Imports {
			importPath := strings.Trim(imported.Path.Value, "\"")
			if !strings.HasPrefix(importPath, modulePath+"/") {
				continue
			}
			dependency := strings.TrimPrefix(importPath, modulePath+"/")
			if dependency == directory {
				continue
			}
			actual[dependency] = true
			if strings.HasPrefix(dependency, "experiments/") || strings.HasPrefix(dependency, "scripts/") {
				t.Errorf("maintained package %s cannot import %s", directory, dependency)
				continue
			}
			if _, exists := registry[dependency]; !exists {
				t.Errorf("package %s imports unregistered project package %s", directory, dependency)
				continue
			}
			if !registration.allowedImports[dependency] {
				t.Errorf("package %s imports %s without permission in package-map.md", directory, dependency)
			}
			if isProductPackage(directory) && laboratoryPackages[dependency] {
				t.Errorf("product package %s imports laboratory package %s", directory, dependency)
			}
		}
	}
	for dependency := range registration.allowedImports {
		if !actual[dependency] {
			t.Errorf("package map permits unused project import %s -> %s; keep the registry factual", directory, dependency)
		}
	}
}

func isProductPackage(directory string) bool {
	return strings.HasPrefix(directory, "internal/") &&
		directory != "internal/architecture" &&
		!laboratoryPackages[directory]
}

func assertPackage(t *testing.T, root, relativeDirectory string, files []string) (string, bool) {
	t.Helper()
	set := token.NewFileSet()
	hasPackageDoc := false
	exported := 0
	productionFiles := 0
	productionLines := 0
	packageName := ""

	for _, path := range files {
		relative := relativePath(t, root, path)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("read %s: %v", relative, err)
			continue
		}
		if formatted, err := format.Source(data); err != nil {
			t.Errorf("parse/format %s: %v", relative, err)
		} else if !bytes.Equal(data, formatted) {
			t.Errorf("Go file is not gofmt-formatted: %s", relative)
		}
		file, err := parser.ParseFile(set, path, data, parser.ParseComments)
		if err != nil {
			t.Errorf("parse %s: %v", relative, err)
			continue
		}
		if packageName == "" || !strings.HasSuffix(relative, "_test.go") {
			packageName = file.Name.Name
		}
		if file.Doc != nil {
			hasPackageDoc = true
		}
		if !strings.HasSuffix(relative, "_test.go") && !isBuildIgnored(data) {
			productionFiles++
			lines := bytes.Count(data, []byte{'\n'})
			productionLines += lines
			if lines > 500 {
				t.Errorf("production file exceeds 500 lines; split by responsibility: %s (%d lines)", relative, lines)
			}
			if strings.HasPrefix(relative, "cmd/") && lines > 120 {
				t.Errorf("command must remain a thin adapter (max 120 lines): %s", relative)
			}
			exported += inspectProductionFile(t, relative, file)
		}
	}
	if forbiddenPackageNames[packageName] {
		t.Errorf("package %q is a vague junk-drawer name: %s", packageName, relativeDirectory)
	}
	for _, segment := range strings.Split(relativeDirectory, "/") {
		if forbiddenPackageNames[segment] {
			t.Errorf("package path uses vague junk-drawer segment %q: %s", segment, relativeDirectory)
		}
	}
	if productionFiles > 0 && !hasPackageDoc {
		t.Errorf("package lacks a package comment: %s", relativeDirectory)
	}
	if strings.HasPrefix(relativeDirectory, "cmd/") {
		if productionLines > 300 {
			t.Errorf("command package exceeds thin-adapter budget (max 300 lines): %s (%d lines)", relativeDirectory, productionLines)
		}
		if exported > 0 {
			t.Errorf("command package exposes %d symbols; behavior belongs in an internal Module: %s", exported, relativeDirectory)
		}
	}
	if strings.HasPrefix(relativeDirectory, "internal/") && exported > 8 {
		t.Errorf("internal package exposes %d symbols; deepen its interface (max 8): %s", exported, relativeDirectory)
	}
	return packageName, productionFiles > 0
}

func inspectProductionFile(t *testing.T, relative string, file *ast.File) int {
	t.Helper()
	exported := 0
	for _, imported := range file.Imports {
		path := strings.Trim(imported.Path.Value, "\"")
		if path == "C" || path == "unsafe" {
			t.Errorf("first-party cgo/unsafe requires a superseding ADR: %s imports %q", relative, path)
		}
		if strings.HasPrefix(path, modulePath+"/experiments") {
			t.Errorf("project code cannot import experiments: %s", relative)
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.FuncDecl:
			if ast.IsExported(value.Name.Name) {
				exported++
			}
			if value.Name.Name == "init" {
				t.Errorf("implicit init functions are forbidden: %s", relative)
			}
		case *ast.GenDecl:
			for _, spec := range value.Specs {
				switch declaration := spec.(type) {
				case *ast.TypeSpec:
					if ast.IsExported(declaration.Name.Name) {
						exported++
					}
				case *ast.ValueSpec:
					for _, name := range declaration.Names {
						if ast.IsExported(name.Name) {
							exported++
						}
					}
				}
			}
		case *ast.CallExpr:
			if identifier, ok := value.Fun.(*ast.Ident); ok && identifier.Name == "panic" {
				t.Errorf("first-party panic is forbidden: %s", relative)
			}
		}
		return true
	})
	return exported
}

func assertRepositoryContainsNoArtifacts(t *testing.T, root string) {
	t.Helper()
	forbiddenDirectories := map[string]bool{
		".artifacts": true, ".cache": true, ".tmp": true, "build": true,
		"coverage": true, "dist": true, "evidence": true, "node_modules": true,
		"target": true, "tmp": true, "var": true, "vendor": true,
	}
	forbiddenSuffixes := []string{".db", ".db-shm", ".db-wal", ".out", ".pcap", ".pcapng", ".prof", ".test"}
	walk(t, root, func(path string, entry os.DirEntry) {
		relative := relativePath(t, root, path)
		if entry.IsDir() && forbiddenDirectories[entry.Name()] {
			t.Errorf("generated or sensitive directory is forbidden: %s", relative)
		}
		if !entry.IsDir() {
			lowerName := strings.ToLower(entry.Name())
			if (entry.Name() == ".env" || strings.HasPrefix(entry.Name(), ".env.")) && entry.Name() != ".env.example" {
				t.Errorf("generated or sensitive file is forbidden: %s", relative)
			}
			if strings.HasSuffix(lowerName, ".key") {
				t.Errorf("key material is forbidden: %s", relative)
			}
			for _, suffix := range forbiddenSuffixes {
				if strings.HasSuffix(lowerName, suffix) {
					t.Errorf("generated or sensitive file is forbidden: %s", relative)
				}
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Errorf("read repository file %s: %v", relative, err)
				return
			}
			trimmed := bytes.TrimSpace(data)
			if strings.HasSuffix(lowerName, ".pem") &&
				(!pathHasSegment(relative, "testdata") ||
					(!bytes.HasPrefix(trimmed, []byte("-----BEGIN CERTIFICATE-----")) &&
						!bytes.HasPrefix(trimmed, []byte("-----BEGIN PUBLIC KEY-----"))) ||
					bytes.Contains(trimmed, []byte("PRIVATE KEY"))) {
				t.Errorf("PEM files must be owned public-certificate or public-key test fixtures: %s", relative)
			}
		}
	})
}

func pathHasSegment(path, wanted string) bool {
	for _, segment := range strings.Split(filepath.ToSlash(path), "/") {
		if segment == wanted {
			return true
		}
	}
	return false
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate architecture test")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(current), "..", ".."))
}

func walk(t *testing.T, root string, visit func(string, os.DirEntry)) {
	t.Helper()
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && (entry.Name() == ".git" || entry.Name() == ".idea") {
			return filepath.SkipDir
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		t.Fatalf("walk repository: %v", err)
	}
	sort.Strings(paths)
	for _, path := range paths {
		entry, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		visit(path, fileInfoEntry{FileInfo: entry})
	}
}

func relativePath(t *testing.T, root, path string) string {
	t.Helper()
	relative, err := filepath.Rel(root, path)
	if err != nil {
		t.Fatalf("relative path for %s: %v", path, err)
	}
	return filepath.ToSlash(relative)
}

func readProjectFile(t *testing.T, root, relative string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		t.Fatalf("read %s: %v", relative, err)
	}
	return data
}

func isBuildIgnored(data []byte) bool {
	return bytes.HasPrefix(data, []byte("//go:build ignore\n"))
}

type fileInfoEntry struct{ os.FileInfo }

func (entry fileInfoEntry) Type() os.FileMode          { return entry.Mode().Type() }
func (entry fileInfoEntry) Info() (os.FileInfo, error) { return entry.FileInfo, nil }

func Example_projectShape() {
	fmt.Println("cmd -> internal")
	// Output: cmd -> internal
}
