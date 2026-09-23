//go:build ignore

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
	powershellPath := flag.String("powershell", "", "write whether qualification PowerShell changed")
	execute := flag.Bool("execute", false, "run affected checks")
	flag.Parse()
	checks, powershell, err := selectChecks(*base, *head)
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
	if *powershellPath != "" {
		if err := os.WriteFile(*powershellPath, []byte(fmt.Sprintf("%t\n", powershell)), 0600); err != nil {
			fail(err)
		}
	}
	if !*execute {
		return
	}
	var failures []error
	for _, check := range checks {
		// Endpoint network fixtures may deliberately wait up to two minutes for
		// the next complete Permission hour before starting their bounded work.
		// Keep that admission wait inside, rather than consuming, the test budget.
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
		args := []string{"test", "-count=1", "-timeout=5m", "-run", check.run}
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
func selectChecks(base, head string) ([]check, bool, error) {
	merged, err := git("merge-base", base, head)
	if err != nil {
		return nil, false, fmt.Errorf("merge base: %w", err)
	}
	base = strings.TrimSpace(string(merged))
	diff, err := git("diff", "--name-only", base, head)
	if err != nil {
		return nil, false, err
	}
	fmt.Printf("pr-check-selection base=%s head=%s\n", base, head)
	changedPaths := strings.Split(strings.TrimSpace(string(diff)), "\n")
	root, err := os.Getwd()
	if err != nil {
		return nil, false, err
	}
	mappings, err := loadPRCheckMappings(root)
	if err != nil {
		return nil, false, fmt.Errorf("load PR check ownership: %w", err)
	}
	listArguments := []string{"list", "-e", "-json", "./cmd/...", "./internal/...", "./tests/..."}
	listed := map[string]bool{"./cmd/...": true, "./internal/...": true, "./tests/...": true}
	for _, mapping := range mappings {
		for _, owner := range mapping.Owners {
			argument := "./" + owner.Package
			if !listed[argument] {
				listArguments, listed[argument] = append(listArguments, argument), true
			}
		}
	}
	output, err := exec.Command("go", listArguments...).Output()
	if err != nil {
		return nil, false, fmt.Errorf("load current package owners: %w", err)
	}
	owners := map[string]*owner{}
	directories := map[string]*owner{}
	decoder := json.NewDecoder(bytes.NewReader(output))
	for {
		var info packageInfo
		if err := decoder.Decode(&info); err == io.EOF {
			break
		} else if err != nil {
			return nil, false, err
		}
		if info.Error != nil && !strings.HasPrefix(info.Error.Err, "build constraints exclude all Go files") {
			return nil, false, fmt.Errorf("load package owner %s: %s", info.ImportPath, info.Error.Err)
		}
		current := &owner{packageInfo: info, changed: map[string]bool{}, reasons: map[string]bool{}}
		owners[info.ImportPath], directories[filepath.Clean(info.Dir)] = current, current
		files := append(append(append(append([]string{}, info.GoFiles...), info.CgoFiles...), info.TestGoFiles...), info.XTestGoFiles...)
		for _, name := range files {
			body, err := os.ReadFile(filepath.Join(info.Dir, name))
			if err != nil {
				return nil, false, err
			}
			declarations, _, err := parseDeclarations(name, body)
			if err != nil {
				return nil, false, err
			}
			current.declarations = append(current.declarations, declarations...)
		}
	}
	architecture := directories[filepath.Join(root, "internal", "architecture")]
	powershell := false
	for _, path := range changedPaths {
		fmt.Printf("changed %s\n", path)
		powershell = powershell || strings.HasPrefix(path, "tests/qualification/") && strings.HasSuffix(path, ".ps1")
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
		}
		if !strings.HasSuffix(path, ".go") {
			if err := applyPRCheckMappings(root, path, mappings, directories); err != nil {
				return nil, false, err
			}
			if strings.HasPrefix(path, "internal/") || strings.HasPrefix(path, "cmd/") || strings.HasPrefix(path, "tests/") {
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
				for _, decl := range architecture.declarations {
					if decl.test && strings.HasPrefix(decl.name, "TestPRSelection") {
						architecture.changed[decl.name] = true
					}
				}
				architecture.reasons[path] = true
			}
			continue
		}
		current.compile = true
		current.reasons[path] = true
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
				return nil, false, err
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
	return checks, powershell, nil
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

func writeMatrix(path string, checks []check) error {
	const maxChecksPerJob = 16

	type entry struct {
		ID      int    `json:"id"`
		Package string `json:"package"`
		Run     string `json:"run"`
		Race    bool   `json:"race"`
	}
	entries := make([]entry, 0)
	appendEntry := func(selected check, names []string) {
		pattern := "^(" + strings.Join(names, "|") + ")$"
		race := selected.race
		if len(names) == 1 && dedicatedPRCheck(names[0]) {
			race = true
		}
		entries = append(entries, entry{len(entries), selected.pkg, pattern, race})
	}
	for _, selected := range checks {
		names := []string{}
		if selected.run != "^$" {
			names = strings.Split(strings.TrimSuffix(strings.TrimPrefix(selected.run, "^("), ")$"), "|")
		}
		if len(names) == 0 {
			entries = append(entries, entry{len(entries), selected.pkg, "^$", false})
		}
		group := make([]string, 0, maxChecksPerJob)
		groupLimit := maxChecksPerJob
		flush := func() {
			if len(group) == 0 {
				return
			}
			appendEntry(selected, group)
			group = group[:0]
		}
		for _, name := range names {
			if dedicatedPRCheck(name) {
				flush()
				appendEntry(selected, []string{name})
				continue
			}
			limit := maxChecksPerJob
			if expensiveEndpointPRCheck(selected.pkg, name) {
				limit = 2
			}
			if len(group) > 0 && limit != groupLimit {
				flush()
			}
			groupLimit = limit
			group = append(group, name)
			if len(group) == groupLimit {
				flush()
			}
		}
		flush()
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

// This evidence cell owns multiple isolated role processes, heap captures and
// a fixed bootstrap-retirement interval. Keep it independently bounded so its
// deliberate runtime cannot consume the budget of unrelated affected tests.
func dedicatedPRCheck(name string) bool {
	return name == "TestTextPublicationIsolatedRoleObservations"
}

// These Endpoint families construct real role topologies and perform custody
// derivation under the race detector. Their measured cost is bounded in pairs;
// ordinary unit families retain the wider default group.
func expensiveEndpointPRCheck(pkg, name string) bool {
	if !strings.HasSuffix(pkg, "/internal/endpoint") {
		return false
	}
	return strings.HasPrefix(name, "TestTextPublication") ||
		strings.HasPrefix(name, "TestTextPublisher") ||
		strings.HasPrefix(name, "TestTextRecovery")
}
