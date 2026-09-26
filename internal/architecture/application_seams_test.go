package architecture

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"
)

const (
	ardentsModulePath         = "github.com/dianabuilds/ardents-network"
	retiredAAI2ConnectionPath = ardentsModulePath + "/internal/application/interfacev1/connection"
)

type commandClosureReceipt struct {
	Source       string
	Profile      string
	Command      string
	ScopeLimit   string
	Dependencies map[string]bool
}

type listedPackage struct {
	ImportPath string
	Dir        string
	GoFiles    []string
	CgoFiles   []string
}

func TestArdentsLinuxClosureRetiresAAI2(t *testing.T) {
	receipt, err := validateArdentsLinuxClosure(t.Context(), repositoryRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	wantReceipt := commandClosureReceipt{
		Source:     "go list -deps -json",
		Profile:    "GOOS=linux GOARCH=amd64 CGO_ENABLED=0",
		Command:    "./cmd/ardents",
		ScopeLimit: "repository-owned production Go files in the Linux cmd/ardents dependency closure",
	}
	if receipt.Source != wantReceipt.Source || receipt.Profile != wantReceipt.Profile ||
		receipt.Command != wantReceipt.Command || receipt.ScopeLimit != wantReceipt.ScopeLimit {
		t.Fatalf("receipt = %+v", receipt)
	}
	for _, required := range []string{
		ardentsModulePath + "/internal/application/interfacev1/administration",
		ardentsModulePath + "/internal/application/interfacev2/connection",
	} {
		if !receipt.Dependencies[required] {
			t.Errorf("Linux %s closure does not retain %s", receipt.Command, required)
		}
	}
}

func TestArdentsLinuxClosureGateRejectsRetiredAAI2Fixture(t *testing.T) {
	root := t.TempDir()
	writeApplicationClosureFixture(t, root, "go.mod", "module github.com/dianabuilds/ardents-network\n\ngo 1.26\n")
	writeApplicationClosureFixture(t, root, "cmd/ardents/main.go", `package main

import _ "github.com/dianabuilds/ardents-network/internal/application/interfacev1/connection"

func main() {}
`)
	writeApplicationClosureFixture(t, root, "internal/application/interfacev1/connection/connection.go", `package connection

func RunParticipant() {}
`)

	_, violations, err := inspectArdentsLinuxClosure(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"forbidden dependency github.com/dianabuilds/ardents-network/internal/application/interfacev1/connection",
		"forbidden symbol RunParticipant in internal/application/interfacev1/connection/connection.go",
	}
	if !slices.Equal(violations, want) {
		t.Fatalf("violations = %q, want %q", violations, want)
	}
	if _, err := validateArdentsLinuxClosure(t.Context(), root); err == nil {
		t.Fatal("gate accepted a fixture with the retired AAI2 dependency and RunParticipant")
	}
}

func TestArdentsLinuxClosureSeparatesGoDiagnosticsFromJSON(t *testing.T) {
	t.Setenv("GOFLAGS", "-x")
	root := t.TempDir()
	writeApplicationClosureFixture(t, root, "go.mod", "module github.com/dianabuilds/ardents-network\n\ngo 1.26\n")
	writeApplicationClosureFixture(t, root, "cmd/ardents/main.go", "package main\n\nfunc main() {}\n")
	if _, err := validateArdentsLinuxClosure(t.Context(), root); err != nil {
		t.Fatal(err)
	}
}

func writeApplicationClosureFixture(t *testing.T, root, relative, contents string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func inspectArdentsLinuxClosure(ctx context.Context, root string) (commandClosureReceipt, []string, error) {
	receipt := commandClosureReceipt{
		Source:       "go list -deps -json",
		Profile:      "GOOS=linux GOARCH=amd64 CGO_ENABLED=0",
		Command:      "./cmd/ardents",
		ScopeLimit:   "repository-owned production Go files in the Linux cmd/ardents dependency closure",
		Dependencies: make(map[string]bool),
	}
	listContext, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	command := exec.CommandContext(listContext, "go", "list", "-deps", "-json", receipt.Command)
	command.Dir = root
	command.Env = linuxGoEnvironment()
	command.WaitDelay = 5 * time.Second
	var diagnostics bytes.Buffer
	command.Stderr = &diagnostics
	output, err := command.Output()
	if err != nil {
		if cause := context.Cause(listContext); cause != nil {
			return receipt, nil, fmt.Errorf("list %s under %s: %w; output: %s",
				receipt.Command, receipt.Profile, cause, strings.TrimSpace(diagnostics.String()))
		}
		return receipt, nil, fmt.Errorf("list %s under %s: %w; output: %s",
			receipt.Command, receipt.Profile, err, strings.TrimSpace(diagnostics.String()))
	}
	decoder := json.NewDecoder(strings.NewReader(string(output)))
	var violations []string
	for decoder.More() {
		var listed listedPackage
		if err := decoder.Decode(&listed); err != nil {
			return receipt, nil, fmt.Errorf("decode command closure: %w", err)
		}
		receipt.Dependencies[listed.ImportPath] = true
		if listed.ImportPath == retiredAAI2ConnectionPath {
			violations = append(violations, "forbidden dependency "+retiredAAI2ConnectionPath)
		}
		if !strings.HasPrefix(listed.ImportPath, ardentsModulePath+"/") {
			continue
		}
		for _, name := range append(listed.GoFiles, listed.CgoFiles...) {
			relative, found, err := sourceContainsIdentifier(root, listed.Dir, name, "RunParticipant")
			if err != nil {
				return receipt, nil, err
			}
			if found {
				violations = append(violations, "forbidden symbol RunParticipant in "+relative)
			}
		}
	}
	sort.Strings(violations)
	return receipt, violations, nil
}

func validateArdentsLinuxClosure(ctx context.Context, root string) (commandClosureReceipt, error) {
	receipt, violations, err := inspectArdentsLinuxClosure(ctx, root)
	if err != nil {
		return receipt, err
	}
	if len(violations) > 0 {
		return receipt, fmt.Errorf("%s under %s rejected %s from %s; scope: %s",
			receipt.Command, receipt.Profile, strings.Join(violations, "; "), receipt.Source, receipt.ScopeLimit)
	}
	return receipt, nil
}

func linuxGoEnvironment() []string {
	environment := make([]string, 0, len(os.Environ())+3)
	for _, variable := range os.Environ() {
		if strings.HasPrefix(variable, "GOOS=") || strings.HasPrefix(variable, "GOARCH=") || strings.HasPrefix(variable, "CGO_ENABLED=") {
			continue
		}
		environment = append(environment, variable)
	}
	return append(environment, "GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=0")
}

func sourceContainsIdentifier(root, directory, name, identifier string) (string, bool, error) {
	path := filepath.Join(directory, name)
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", false, fmt.Errorf("repository source %s is outside %s", path, root)
	}
	parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		return "", false, fmt.Errorf("parse %s: %w", relative, err)
	}
	found := false
	ast.Inspect(parsed, func(node ast.Node) bool {
		if named, ok := node.(*ast.Ident); ok && named.Name == identifier {
			found = true
		}
		return !found
	})
	return filepath.ToSlash(relative), found, nil
}

func TestSelectedApplicationSeamsMatchTheirAdapters(t *testing.T) {
	root := repositoryRoot(t)
	retiredConnection := "github.com/dianabuilds/ardents-network/internal/application/interfacev1/connection"
	if listedDirectImports(t, root, "./cmd/ardents")[retiredConnection] {
		t.Error("./cmd/ardents directly imports the retired generic AAI2 Connection client")
	}
	selectedConnection := "github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
	selectedAdapters := []string{"./cmd/ardents-text"}
	if runtime.GOOS == "linux" {
		selectedAdapters = append(selectedAdapters, "./internal/endpoint")
	}
	for _, packagePath := range selectedAdapters {
		if !listedDependencies(t, root, packagePath)[selectedConnection] {
			t.Errorf("%s does not use the selected protected text Connection Module", packagePath)
		}
	}
	administration := "github.com/dianabuilds/ardents-network/internal/application/interfacev1/administration"
	administrationAdapters := []string{"./cmd/ardents"}
	if runtime.GOOS == "linux" {
		administrationAdapters = append(administrationAdapters, "./internal/endpoint")
	}
	for _, packagePath := range administrationAdapters {
		if !listedDependencies(t, root, packagePath)[administration] {
			t.Errorf("%s does not use the shared Application Administration Module", packagePath)
		}
	}
}

func listedDirectImports(t *testing.T, root, packagePath string) map[string]bool {
	t.Helper()
	command := exec.Command("go", "list", "-f", `{{join .Imports "\n"}}`, packagePath)
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		t.Fatalf("list direct imports for %s: %v", packagePath, err)
	}
	return packageSet(t, string(output))
}
