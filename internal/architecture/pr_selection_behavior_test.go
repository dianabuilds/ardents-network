package architecture

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPRSelectionFollowsConsumersAndKeepsUnrelatedTestsOut(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	fixture := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		path := filepath.Join(fixture, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	command := func(name string, arguments ...string) []byte {
		t.Helper()
		cmd := exec.Command(name, arguments...)
		cmd.Dir = fixture
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s: %v\n%s", name, err, output)
		}
		return output
	}
	write("go.mod", "module example.com/selection\n\ngo 1.26.8\n")
	write("cmd/tool/main.go", "package main\nimport \"example.com/selection/internal/consumer\"\nfunc main(){ _=consumer.ReadValue() }\n")
	write("internal/source/value.go", "package source\nfunc Value() int {return 1}\n")
	write("internal/source/stable.go", "package source\nfunc Stable() int {return 42}\n")
	write("internal/consumer/read.go", "package consumer\nimport \"example.com/selection/internal/source\"\nfunc ReadValue() int {return source.Value()}\nfunc ReadStable() int {return source.Stable()}\n")
	tests := "package consumer\nimport \"testing\"\n"
	for i := range 20 {
		tests += fmt.Sprintf("func TestAffected%02d(t *testing.T){if ReadValue()==0 {t.Fatal(\"zero\")}}\n", i)
	}
	tests += "func TestUnrelated(t *testing.T){if ReadStable()!=42 {t.Fatal(\"stable\")}}\n"
	write("internal/consumer/read_test.go", tests)
	write("tests/e2e/probe/probe_test.go", "package probe\nimport \"testing\"\nfunc TestIndependent(t *testing.T){}\n")
	command("git", "init", "-q")
	command("git", "add", ".")
	commit := func() {
		command("git", "-c", "core.hooksPath="+filepath.Join(fixture, "absent-hooks"), "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "fixture")
	}
	commit()
	base := strings.TrimSpace(string(command("git", "rev-parse", "HEAD")))
	write("internal/source/value.go", "package source\nfunc Value() int {return 2}\n")
	command("git", "add", ".")
	commit()
	matrixPath := filepath.Join(fixture, "matrix.json")
	command("go", "run", filepath.Join(root, "scripts", "select-pr-checks.go"), "--base", base, "--head", "HEAD", "--matrix", matrixPath)
	body, err := os.ReadFile(matrixPath)
	if err != nil {
		t.Fatal(err)
	}
	var matrix struct {
		Include []struct{ Package, Run string }
	}
	if err := json.Unmarshal(body, &matrix); err != nil {
		t.Fatal(err)
	}
	selected := make(map[string]int)
	consumerGroups := 0
	for _, entry := range matrix.Include {
		if strings.Contains(entry.Run, "TestUnrelated") || strings.Contains(entry.Package, "tests/e2e") {
			t.Fatalf("unrelated check selected: %+v", entry)
		}
		if !strings.HasSuffix(entry.Package, "/consumer") {
			continue
		}
		consumerGroups++
		for _, name := range strings.Split(strings.TrimSuffix(strings.TrimPrefix(entry.Run, "^("), ")$"), "|") {
			selected[name]++
		}
	}
	if consumerGroups != 2 || len(selected) != 20 {
		t.Fatalf("bounded groups=%d selected=%v", consumerGroups, selected)
	}
	for name, count := range selected {
		if count != 1 {
			t.Errorf("test %s selected %d times", name, count)
		}
	}
}

func TestPRSelectionMapsQualificationFixturesToBoundedOwners(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	fixture := t.TempDir()
	writeFile := func(name, body string) {
		path := filepath.Join(fixture, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	run := func(name string, arguments ...string) []byte {
		command := exec.Command(name, arguments...)
		command.Dir = fixture
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("%s: %v\n%s", name, err, output)
		}
		return output
	}
	writeFile("go.mod", "module example.com/qualification-selection\n\ngo 1.26.8\n")
	for _, owner := range []struct{ directory, packageName, selected, unrelated string }{
		{"cmd/ardents-qualification", "main", "TestQualificationEvidence", "TestCommandUnrelated"},
		{"cmd/ardents-node", "main", "TestNodePlan", "TestNodeUnrelated"},
		{"internal/endpoint", "endpoint", "TestQualificationStream", "TestEndpointUnrelated"},
		{"internal/node", "node", "TestClosedHosting", "TestNodeUnrelated"},
		{"internal/architecture", "architecture", "TestRepositoryArchitecture", "TestArchitectureUnrelated"},
		{"tests/e2e/node/fixturecommand/netem-relay", "main", "TestRelayConfiguration", "TestRelayUnrelated"},
		{"tests/epochfixture/network", "network", "TestCanonicalNetworkFixture", "TestNetworkFixtureUnrelated"},
		{"tests/qualification/stream-network-two-host/fixturecommand/qualification-network", "main", "TestRunCreatesCompleteBoundedNetworkFixture", "TestGeneratorUnrelated"},
	} {
		writeFile(owner.directory+"/owner.go", "package "+owner.packageName+"\n")
		writeFile(owner.directory+"/owner_test.go", "package "+owner.packageName+"\nimport \"testing\"\nfunc "+owner.selected+"(t *testing.T){}\nfunc "+owner.unrelated+"(t *testing.T){}\n")
	}
	run("git", "init", "-q")
	run("git", "add", ".")
	commit := func() {
		run("git", "-c", "core.hooksPath="+filepath.Join(fixture, "absent-hooks"), "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "fixture")
	}
	commit()
	base := strings.TrimSpace(string(run("git", "rev-parse", "HEAD")))
	writeFile("tests/qualification/stream-network-two-host/run-windows.ps1", "Write-Output qualification\n")
	run("git", "add", ".")
	commit()
	matrixPath := filepath.Join(fixture, "matrix.json")
	run("go", "run", filepath.Join(root, "scripts", "select-pr-checks.go"), "--base", base, "--head", "HEAD", "--matrix", matrixPath)
	body, err := os.ReadFile(matrixPath)
	if err != nil {
		t.Fatal(err)
	}
	var matrix struct {
		Include []struct{ Package, Run string }
	}
	if err := json.Unmarshal(body, &matrix); err != nil {
		t.Fatal(err)
	}
	joined := ""
	for _, selected := range matrix.Include {
		joined += selected.Package + " " + selected.Run + "\n"
	}
	for _, wanted := range []string{"TestQualificationEvidence", "TestQualificationStream", "TestClosedHosting", "TestRepositoryArchitecture",
		"TestRelayConfiguration", "TestCanonicalNetworkFixture", "TestRunCreatesCompleteBoundedNetworkFixture"} {
		if !strings.Contains(joined, wanted) {
			t.Errorf("qualification owner %s was not selected:\n%s", wanted, joined)
		}
	}
	for _, unwanted := range []string{"TestCommandUnrelated", "TestEndpointUnrelated", "TestRelayUnrelated"} {
		if strings.Contains(joined, unwanted) {
			t.Errorf("unrelated test %s selected:\n%s", unwanted, joined)
		}
	}
}
