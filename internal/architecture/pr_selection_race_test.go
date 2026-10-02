package architecture

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Exercise every owner visitation order at the production closure seam instead
// of relying on Go's random map order to reproduce the missing race flag.
func TestPRSelectionRaceClosureIsOrderIndependent(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	fixture := t.TempDir()
	var files []string
	for _, name := range []string{"select-pr-checks.go", "select-pr-check-registry.go"} {
		body, err := os.ReadFile(filepath.Join(root, "scripts", name))
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(fixture, name)
		if err := os.WriteFile(path, body, 0600); err != nil {
			t.Fatal(err)
		}
		files = append(files, path)
	}
	harness := filepath.Join(fixture, "closure_test.go")
	if err := os.WriteFile(harness, []byte(raceClosureHarness), 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", append([]string{"test", "-count=1"}, append(files, harness)...)...)
	command.Env = isolatedFixtureEnvironment()
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("closure regression: %v\n%s", err, output)
	}
}

const raceClosureHarness = `
package main

import (
	"fmt"
	"testing"
)

func TestMixedDependencyClosure(t *testing.T) {
	var visit func([]int, []int)
	visit = func(prefix, remaining []int) {
		if len(remaining) > 0 {
			for i, next := range remaining {
				rest := append([]int{}, remaining[:i]...)
				rest = append(rest, remaining[i+1:]...)
				visit(append(append([]int{}, prefix...), next), rest)
			}
			return
		}
		t.Run(fmt.Sprint(prefix), func(t *testing.T) {
			for _, concurrent := range []bool{true, false} {
				for _, preselected := range []bool{false, true} {
					owners := map[string]*owner{}
					add := func(pkg, source string, changed, race bool) *owner {
						decls, _, err := parseDeclarations(pkg+".go", []byte(source))
						if err != nil {
							t.Fatal(err)
						}
						current := &owner{packageInfo: packageInfo{ImportPath: pkg}, declarations: decls,
							changed: map[string]bool{}, reasons: map[string]bool{}, compile: changed, race: race}
						if changed {
							current.changed["Value"] = true
						}
						owners[pkg] = current
						return current
					}
					source := add("source", "package source; func Value() int { return 1 }", true, concurrent)
					middle := add("middle", "package middle; import \"source\"; func Value() int { return source.Value() }", false, false)
					plain := add("plain", "package plain; func Value() int { return 2 }", true, false)
					top := add("top", "package top; import (\"middle\"; \"plain\"); func Value() int { return middle.Value()+plain.Value() }; func Stable() int { return 42 }", false, false)
					tests, _, err := parseDeclarations("top_test.go", []byte("package top; import \"testing\"; func TestValue(t *testing.T){_=Value()}; func TestUnrelated(t *testing.T){_=Stable()}"))
					if err != nil {
						t.Fatal(err)
					}
					top.declarations = append(top.declarations, tests...)
					all := []*owner{source, middle, plain, top}
					if preselected {
						middle.changed["Value"], top.changed["Value"], top.changed["TestValue"] = true, true, true
						middle.compile, top.compile = true, true
					}
					order := make([]*owner, 0, len(prefix))
					for _, index := range prefix {
						order = append(order, all[index])
					}
					propagateChanges(owners, order)
					if !top.changed["TestValue"] || top.changed["TestUnrelated"] || top.race != concurrent {
						t.Fatalf("concurrent=%t: selected=%v race=%t", concurrent, top.changed, top.race)
					}
					if !top.reasons["consumer of middle.Value"] || !top.reasons["consumer of plain.Value"] {
						t.Fatalf("incomplete dependency closure: %v", top.reasons)
					}
					// A completed closure must be stable on a second traversal.
					propagateChanges(owners, order)
					if top.race != concurrent {
						t.Fatalf("closure changed on second traversal")
					}
				}
			}
		})
	}
	visit(nil, []int{0, 1, 2, 3})
}
`
