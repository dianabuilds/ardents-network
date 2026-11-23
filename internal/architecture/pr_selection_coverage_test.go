package architecture

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// A selected group may be omitted only when the concurrently required full
// gate runs its complete, non-short package with the needed instrumentation.
func TestPRSelectionPreservesChecksOutsideTheFullGate(t *testing.T) {
	root := repositoryRoot(t)
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
	harness := filepath.Join(fixture, "coverage_test.go")
	if err := os.WriteFile(harness, []byte(prCoverageHarness), 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", append([]string{"test", "-count=1"}, append(files, harness)...)...)
	command.Env = isolatedFixtureEnvironment()
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("full gate coverage regression: %v\n%s", err, output)
	}
}

const prCoverageHarness = `
package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCoverage(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "tests", "profiles")
	if err := os.MkdirAll(directory, 0700); err != nil { t.Fatal(err) }
	for name, body := range map[string]string{
		"deterministic-packages.txt": "project/unit\nproject/internal/endpoint\n",
		"deterministic-linux-packages.txt": "\n",
		"process-packages.txt": "project/process\n",
	} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(body), 0600); err != nil { t.Fatal(err) }
	}
	input := []check{
		{pkg:"project/unit",run:"^(TestUnit)$"},
		{pkg:"project/unit",run:"^(TestUnit)$",race:true},
		{pkg:"project/unit",run:"^$"},
		{pkg:"project/process",run:"^(TestProcess)$"},
		{pkg:"project/process",run:"^(TestProcess)$",race:true},
		{pkg:"project/unlisted",run:"^(TestUnknown)$"},
		{pkg:"project/internal/endpoint",run:"^(TestTextPublisherReady)$"},
		{pkg:"project/unit",run:"^(TestTextPublicationIsolatedRoleObservations)$"},
	}
	got, err := checksOutsideFullGate(root, input)
	if err != nil { t.Fatal(err) }
	want := []check{input[3],input[4],input[5],input[6],input[7]}
	if !reflect.DeepEqual(got,want) { t.Fatalf("got %+v, want %+v",got,want) }
	if err := os.Remove(filepath.Join(directory,"deterministic-packages.txt")); err != nil { t.Fatal(err) }
	if _, err := checksOutsideFullGate(root,input); err == nil { t.Fatal("missing unit profile accepted") }
}
`
