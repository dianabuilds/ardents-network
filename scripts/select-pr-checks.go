//go:build ignore

// Command select-pr-checks follows changed Go symbols through production and
// test helpers, including imported consumers. It prints the selection reasons
// and retains all independent failures before returning an unsuccessful result.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type packageInfo struct {
	ImportPath                                   string
	Dir                                          string
	GoFiles, CgoFiles, TestGoFiles, XTestGoFiles []string
	Error                                        *struct{ Err string }
}
type declaration struct {
	name     string
	receiver string
	refs     map[string]bool
	imported map[string]map[string]bool
	test     bool
}
type owner struct {
	packageInfo
	declarations []declaration
	changed      map[string]bool
	reasons      map[string]bool
	compile      bool
	race         bool
}
type check struct {
	pkg, run, reason string
	race             bool
}

func main() {
	base := flag.String("base", "origin/main", "base ref")
	head := flag.String("head", "HEAD", "head ref")
	matrixPath := flag.String("matrix", "", "write bounded GitHub job matrix")
	execute := flag.Bool("execute", false, "run affected checks")
	flag.Parse()
	checks, err := selectChecks(*base, *head)
	if err != nil {
		fail(err)
	}
	for _, check := range checks {
		fmt.Printf("selected package=%s race=%t run=%q reason=%s\n", check.pkg, check.race, check.run, check.reason)
	}
	if *matrixPath != "" {
		if err := writeMatrix(*matrixPath, checks); err != nil {
			fail(err)
		}
	}
	if !*execute {
		return
	}
	var failures []error
	for _, check := range checks {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
		args := []string{"test", "-count=1", "-timeout=3m", "-run", check.run}
		if check.race {
			args = append(args, "-race")
		}
		args = append(args, check.pkg)
		command := exec.CommandContext(ctx, "go", args...)
		command.Stdout, command.Stderr = os.Stdout, os.Stderr
		fmt.Printf("run go %s\n", strings.Join(args, " "))
		if err := command.Run(); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", check.pkg, err))
		}
		cancel()
	}
	if err := errors.Join(failures...); err != nil {
		fail(err)
	}
}
func fail(err error)                          { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
func git(arguments ...string) ([]byte, error) { return exec.Command("git", arguments...).Output() }

func selectChecks(base, head string) ([]check, error) {
	merged, err := git("merge-base", base, head)
	if err != nil {
		return nil, fmt.Errorf("merge base: %w", err)
	}
	base = strings.TrimSpace(string(merged))
	diff, err := git("diff", "--name-only", base, head)
	if err != nil {
		return nil, err
	}
	fmt.Printf("pr-check-selection base=%s head=%s\n", base, head)
	changedPaths := strings.Split(strings.TrimSpace(string(diff)), "\n")
	root, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	listArguments := []string{"list", "-json", "./cmd/...", "./internal/...", "./tests/e2e/..."}
	for _, extra := range []string{"tests/epochfixture/network", "tests/qualification/stream-network-two-host/fixturecommand/qualification-network"} {
		if info, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(extra))); statErr == nil && info.IsDir() {
			listArguments = append(listArguments, "./"+extra)
		}
	}
	output, err := exec.Command("go", listArguments...).Output()
	if err != nil {
		return nil, fmt.Errorf("load current package owners: %w", err)
	}
	owners := map[string]*owner{}
	directories := map[string]*owner{}
	decoder := json.NewDecoder(bytes.NewReader(output))
	for {
		var info packageInfo
		if err := decoder.Decode(&info); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		current := &owner{packageInfo: info, changed: map[string]bool{}, reasons: map[string]bool{}}
		owners[info.ImportPath], directories[filepath.Clean(info.Dir)] = current, current
		files := append(append(append(append([]string{}, info.GoFiles...), info.CgoFiles...), info.TestGoFiles...), info.XTestGoFiles...)
		for _, name := range files {
			body, err := os.ReadFile(filepath.Join(info.Dir, name))
			if err != nil {
				return nil, err
			}
			declarations, _, err := parseDeclarations(name, body)
			if err != nil {
				return nil, err
			}
			current.declarations = append(current.declarations, declarations...)
		}
	}
	architecture := directories[filepath.Join(root, "internal", "architecture")]
	for _, path := range changedPaths {
		fmt.Printf("changed %s\n", path)
		if path == "go.mod" || path == "go.sum" {
			for _, current := range owners {
				current.compile = true
				current.reasons[path] = true
				for _, decl := range current.declarations {
					if decl.test {
						current.changed[decl.name] = true
					}
				}
			}
			continue
		}
		if strings.HasPrefix(path, "packaging/") {
			for _, directory := range []string{"internal/endpoint", "internal/architecture"} {
				current := directories[filepath.Join(root, filepath.FromSlash(directory))]
				if current != nil {
					for _, decl := range current.declarations {
						if strings.Contains(decl.name, "Worker") || strings.Contains(decl.name, "Inventory") || decl.name == "TestRepositoryArchitecture" {
							current.changed[decl.name] = true
						}
					}
					current.compile = true
					current.reasons[path] = true
				}
			}
			if strings.HasPrefix(path, "packaging/stream-qualification-worker/") {
				for _, directory := range []string{"cmd/ardents-qualification", "cmd/ardents-stream-qualification", "internal/application/streamqualification"} {
					current := directories[filepath.Join(root, filepath.FromSlash(directory))]
					if current == nil {
						continue
					}
					current.compile = true
					current.race = true
					current.reasons[path] = true
					for _, decl := range current.declarations {
						if decl.test {
							current.changed[decl.name] = true
						}
					}
				}
			}
		}
		if !strings.HasSuffix(path, ".go") {
			qualificationDirectories := []string{}
			if strings.HasPrefix(path, "tests/qualification/stream-network-two-host/") {
				qualificationDirectories = []string{"cmd/ardents-qualification", "tests/e2e/node/fixturecommand/netem-relay",
					"tests/epochfixture/network", "tests/qualification/stream-network-two-host/fixturecommand/qualification-network",
					"internal/endpoint", "internal/node"}
			} else if strings.HasPrefix(path, "tests/qualification/net32-idle-one-host/") {
				qualificationDirectories = []string{"cmd/ardents-qualification", "internal/endpoint"}
			}
			for _, directory := range qualificationDirectories {
				current := directories[filepath.Join(root, filepath.FromSlash(directory))]
				if current == nil {
					continue
				}
				current.compile, current.race = true, true
				current.reasons[path] = true
				for _, decl := range current.declarations {
					if !decl.test {
						continue
					}
					switch directory {
					case "cmd/ardents-qualification":
						if !strings.Contains(decl.name, "Qualification") && !strings.Contains(decl.name, "ResourceVerdict") {
							continue
						}
					case "tests/e2e/node/fixturecommand/netem-relay":
						if !strings.Contains(decl.name, "RelayConfiguration") && !strings.HasPrefix(decl.name, "TestCopyRelayDirection") && !strings.HasPrefix(decl.name, "TestQualificationNetworkCells") {
							continue
						}
					case "internal/endpoint":
						if !strings.Contains(decl.name, "Qualification") && !strings.Contains(decl.name, "Stream") {
							continue
						}
					case "internal/node":
						if !strings.Contains(decl.name, "Closed") && !strings.Contains(decl.name, "Hosting") {
							continue
						}
					}
					current.changed[decl.name] = true
				}
			}
			// Embedded files and shared fixtures affect their owning package.
			if strings.HasPrefix(path, "internal/") || strings.HasPrefix(path, "cmd/") || strings.HasPrefix(path, "tests/e2e/") {
				for directory := filepath.Dir(filepath.Join(root, filepath.FromSlash(path))); directory != root; directory = filepath.Dir(directory) {
					if current := directories[directory]; current != nil {
						current.compile, current.race = true, true
						current.reasons[path] = true
						for _, decl := range current.declarations {
							current.changed[decl.name] = true
						}
						break
					}
				}
			}
			if path == "Makefile" || strings.HasPrefix(path, ".github/") || strings.HasPrefix(path, "tests/profiles/") || strings.HasPrefix(path, "tests/qualification/") || strings.HasPrefix(path, "docs/development/") {
				if architecture != nil {
					architecture.compile = true
					architecture.changed["TestRepositoryArchitecture"] = true
					architecture.reasons[path] = true
				}
			}
			continue
		}
		current := directories[filepath.Join(root, filepath.Dir(filepath.FromSlash(path)))]
		if current == nil {
			if strings.HasPrefix(path, "scripts/") && architecture != nil {
				architecture.compile = true
				architecture.changed["TestRepositoryArchitecture"] = true
				architecture.reasons[path] = true
			}
			continue
		}
		current.compile = true
		current.reasons[path] = true
		// Include removed declarations as well, so deletion follows old consumers.
		for _, previous := range []bool{false, true} {
			var body []byte
			if previous {
				body, err = git("show", base+":"+path)
			} else {
				body, err = os.ReadFile(filepath.FromSlash(path))
			}
			if err != nil {
				continue
			}
			declarations, race, err := parseDeclarations(path, body)
			if err != nil {
				return nil, err
			}
			current.race = current.race || race
			for _, decl := range declarations {
				current.changed[decl.name] = true
			}
		}
	}
	for changed := true; changed; {
		changed = false
		for _, current := range owners {
			for _, decl := range current.declarations {
				if current.changed[decl.name] {
					// A changed method changes the behavior of values produced by
					// constructors, including methods called through an interface.
					if decl.receiver != "" && !current.changed[decl.receiver] {
						current.changed[decl.receiver] = true
						changed = true
					}
					continue
				}
				affected := false
				for name := range decl.refs {
					if current.changed[name] {
						affected = true
						break
					}
				}
				for imported, names := range decl.imported {
					other := owners[imported]
					if other == nil {
						continue
					}
					for name := range names {
						if other.changed[name] {
							affected = true
							current.race = current.race || other.race
							current.reasons["consumer of "+imported+"."+name] = true
						}
					}
				}
				if affected {
					current.changed[decl.name] = true
					current.compile = true
					changed = true
				}
			}
		}
	}
	var checks []check
	for _, current := range owners {
		if !current.compile {
			continue
		}
		var names []string
		for _, decl := range current.declarations {
			if decl.test && current.changed[decl.name] {
				names = append(names, regexp.QuoteMeta(decl.name))
			}
		}
		sort.Strings(names)
		pattern := "^$"
		if len(names) > 0 {
			pattern = "^(" + strings.Join(names, "|") + ")$"
		}
		reasons := make([]string, 0, len(current.reasons))
		for reason := range current.reasons {
			reasons = append(reasons, reason)
		}
		sort.Strings(reasons)
		checks = append(checks, check{pkg: current.ImportPath, run: pattern, reason: strings.Join(reasons, ", "), race: current.race})
	}
	sort.Slice(checks, func(i, j int) bool { return checks[i].pkg < checks[j].pkg })
	if len(checks) == 0 {
		fmt.Println("No executable behavior changed; no runtime tests selected.")
	}
	return checks, nil
}

func parseDeclarations(name string, body []byte) ([]declaration, bool, error) {
	file, err := parser.ParseFile(token.NewFileSet(), name, body, 0)
	if err != nil {
		return nil, false, err
	}
	imports := map[string]string{}
	race := false
	for _, item := range file.Imports {
		path := strings.Trim(item.Path.Value, "\"")
		alias := filepath.Base(path)
		if item.Name != nil {
			alias = item.Name.Name
		}
		imports[alias] = path
		if path == "sync" || path == "sync/atomic" {
			race = true
		}
	}
	var declarations []declaration
	add := func(name string, node ast.Node, test bool) {
		decl := declaration{name: name, refs: map[string]bool{}, imported: map[string]map[string]bool{}, test: test}
		ast.Inspect(node, func(node ast.Node) bool {
			switch value := node.(type) {
			case *ast.Ident:
				decl.refs[value.Name] = true
			case *ast.GoStmt:
				race = true
			case *ast.SelectorExpr:
				if alias, ok := value.X.(*ast.Ident); ok {
					if path := imports[alias.Name]; path != "" {
						if decl.imported[path] == nil {
							decl.imported[path] = map[string]bool{}
						}
						decl.imported[path][value.Sel.Name] = true
					}
				}
			}
			return true
		})
		declarations = append(declarations, decl)
	}
	for _, node := range file.Decls {
		switch decl := node.(type) {
		case *ast.FuncDecl:
			add(decl.Name.Name, decl, strings.HasSuffix(name, "_test.go") && strings.HasPrefix(decl.Name.Name, "Test") && decl.Recv == nil)
			if decl.Recv != nil && len(decl.Recv.List) == 1 {
				receiver := decl.Recv.List[0].Type
				if pointer, ok := receiver.(*ast.StarExpr); ok {
					receiver = pointer.X
				}
				if ident, ok := receiver.(*ast.Ident); ok {
					declarations[len(declarations)-1].receiver = ident.Name
				}
			}
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				switch item := spec.(type) {
				case *ast.TypeSpec:
					add(item.Name.Name, item, false)
				case *ast.ValueSpec:
					for _, name := range item.Names {
						add(name.Name, item, false)
					}
				}
			}
		}
	}
	return declarations, race, nil
}

// A large affected package is divided into independent bounded groups. A timeout
// cannot suppress later groups, and each selected top-level test appears once.
func writeMatrix(path string, checks []check) error {
	type entry struct {
		ID      int    `json:"id"`
		Package string `json:"package"`
		Run     string `json:"run"`
		Race    bool   `json:"race"`
	}
	entries := make([]entry, 0)
	for _, selected := range checks {
		names := []string{}
		if selected.run != "^$" {
			names = strings.Split(strings.TrimSuffix(strings.TrimPrefix(selected.run, "^("), ")$"), "|")
		}
		if len(names) == 0 {
			entries = append(entries, entry{len(entries), selected.pkg, "^$", false})
		}
		for len(names) > 0 {
			count := min(16, len(names))
			pattern := "^(" + strings.Join(names[:count], "|") + ")$"
			entries = append(entries, entry{len(entries), selected.pkg, pattern, selected.race})
			names = names[count:]
		}
	}
	if len(entries) > 256 {
		return errors.New("affected test groups exceed GitHub matrix capacity")
	}
	body, err := json.Marshal(struct {
		Include []entry `json:"include"`
	}{entries})
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(body, '\n'), 0600)
}
