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
	write("docs/development/ownership.json", "{\"pr_check_mappings\":[]}\n")
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
	write("internal/architecture/architecture_test.go", "package architecture\nimport \"testing\"\nfunc TestRepositoryArchitecture(t *testing.T){}\nfunc TestPRSelectionSelf(t *testing.T){}\nfunc TestArchitectureUnrelated(t *testing.T){}\n")
	write("scripts/select-pr-checks.go", "package main\n")
	write("tests/e2e/probe/probe_test.go", "package probe\nimport \"testing\"\nfunc TestIndependent(t *testing.T){}\n")
	command("git", "init", "-q")
	command("git", "add", ".")
	commit := func() {
		command("git", "-c", "core.hooksPath="+filepath.Join(fixture, "absent-hooks"), "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "fixture")
	}
	commit()
	base := strings.TrimSpace(string(command("git", "rev-parse", "HEAD")))
	write("internal/source/value.go", "package source\nfunc Value() int {return 2}\n")
	write("scripts/select-pr-checks.go", "package main\n// changed selector\n")
	command("git", "add", ".")
	commit()
	matrixPath := filepath.Join(fixture, "matrix.json")
	command("go", "run", filepath.Join(root, "scripts", "select-pr-checks.go"), filepath.Join(root, "scripts", "select-pr-check-registry.go"), "--base", base, "--head", "HEAD", "--matrix", matrixPath)
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
	selectionSelf, repositoryArchitecture := false, false
	for _, entry := range matrix.Include {
		if strings.Contains(entry.Run, "TestUnrelated") || strings.Contains(entry.Package, "tests/e2e") {
			t.Fatalf("unrelated check selected: %+v", entry)
		}
		if !strings.HasSuffix(entry.Package, "/consumer") {
			selectionSelf = selectionSelf || strings.Contains(entry.Run, "TestPRSelectionSelf")
			repositoryArchitecture = repositoryArchitecture || strings.Contains(entry.Run, "TestRepositoryArchitecture")
			continue
		}
		consumerGroups++
		for _, name := range strings.Split(strings.TrimSuffix(strings.TrimPrefix(entry.Run, "^("), ")$"), "|") {
			selected[name]++
		}
	}
	if !selectionSelf || !repositoryArchitecture {
		t.Fatalf("selector change omitted its own behavior or repository test: %+v", matrix.Include)
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
	writeFile("docs/development/ownership.json", "{\"pr_check_mappings\":[{\"path_prefix\":\"tests/qualification/stream-network-two-host/\",\"race\":true,\"owners\":[{\"package\":\"cmd/ardents-qualification\",\"test_pattern\":\"^Test(?:Qualification|ResourceVerdict|FailedNET14V)\"},{\"package\":\"tests/e2e/node/fixturecommand/netem-relay\",\"test_pattern\":\"^TestRelayConfiguration\"},{\"package\":\"tests/epochfixture/network\",\"test_pattern\":\"^TestCanonicalNetworkFixture\"},{\"package\":\"tests/qualification/stream-network-two-host/fixturecommand/qualification-network\",\"test_pattern\":\"^TestRunCreatesCompleteBoundedNetworkFixture\"},{\"package\":\"internal/endpoint\",\"test_pattern\":\"^TestQualificationStream\"},{\"package\":\"internal/node\",\"test_pattern\":\"^TestClosedHosting\"}]}]}\n")
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
	writeFile("cmd/ardents-qualification/failed_test.go", "package main\nimport \"testing\"\nfunc TestFailedNET14VEvidence(t *testing.T){}\n")
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
	run("go", "run", filepath.Join(root, "scripts", "select-pr-checks.go"), filepath.Join(root, "scripts", "select-pr-check-registry.go"), "--base", base, "--head", "HEAD", "--matrix", matrixPath)
	body, err := os.ReadFile(matrixPath)
	if err != nil {
		t.Fatal(err)
	}
	var matrix struct {
		Include    []struct{ Package, Run string }
		PowerShell bool
	}
	if err := json.Unmarshal(body, &matrix); err != nil {
		t.Fatal(err)
	}
	joined := ""
	for _, selected := range matrix.Include {
		joined += selected.Package + " " + selected.Run + "\n"
	}
	if !matrix.PowerShell {
		t.Error("qualification PowerShell change did not select parser gate")
	}
	for _, wanted := range []string{"TestQualificationEvidence", "TestFailedNET14VEvidence", "TestQualificationStream", "TestClosedHosting", "TestRepositoryArchitecture",
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

func TestPRSelectionRejectsStaleRegistryMappings(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, owner, pattern, want string
	}{
		{"missing owner", "cmd/missing", "^TestLive$", "owner package cmd/missing is unavailable"},
		{"renamed test", "cmd/live", "^TestRenamed$", `owner cmd/live pattern "^TestRenamed$" matches no test`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			fixture := t.TempDir()
			write := func(name, body string) {
				path := filepath.Join(fixture, filepath.FromSlash(name))
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
			}
			run := func(arguments ...string) []byte {
				command := exec.Command(arguments[0], arguments[1:]...)
				command.Dir = fixture
				output, err := command.CombinedOutput()
				if err != nil {
					t.Fatalf("%s: %v\n%s", arguments[0], err, output)
				}
				return output
			}
			write("go.mod", "module example.com/stale-selection\n\ngo 1.26.8\n")
			registry := fmt.Sprintf(`{"pr_check_mappings":[{"path_prefix":"tests/qualification/probe/","race":true,"owners":[{"package":%q,"test_pattern":%q}]}]}`+"\n", test.owner, test.pattern)
			write("docs/development/ownership.json", registry)
			write("cmd/live/main.go", "package main\nfunc main(){}\n")
			write("cmd/live/main_test.go", "package main\nimport \"testing\"\nfunc TestLive(t *testing.T){}\n")
			write("internal/placeholder/doc.go", "package placeholder\n")
			write("tests/placeholder/doc.go", "package placeholder\n")
			write("tests/qualification/probe/run.ps1", "Write-Output old\n")
			run("git", "init", "-q")
			run("git", "add", ".")
			commit := func() {
				run("git", "-c", "core.hooksPath="+filepath.Join(fixture, "absent-hooks"), "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "fixture")
			}
			commit()
			base := strings.TrimSpace(string(run("git", "rev-parse", "HEAD")))
			write("tests/qualification/probe/run.ps1", "Write-Output changed\n")
			run("git", "add", ".")
			commit()
			command := exec.Command("go", "run", filepath.Join(root, "scripts", "select-pr-checks.go"), filepath.Join(root, "scripts", "select-pr-check-registry.go"), "--base", base, "--head", "HEAD")
			command.Dir = fixture
			output, err := command.CombinedOutput()
			if err == nil || !strings.Contains(string(output), test.want) {
				t.Fatalf("stale registry result = %v\n%s; want %q", err, output, test.want)
			}
		})
	}
}
