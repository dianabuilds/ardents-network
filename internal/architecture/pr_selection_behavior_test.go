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

func isolatedFixtureEnvironment() []string {
	blocked := map[string]bool{
		"GIT_DIR": true, "GIT_WORK_TREE": true, "GIT_INDEX_FILE": true, "GIT_COMMON_DIR": true,
		"GIT_OBJECT_DIRECTORY": true, "GIT_ALTERNATE_OBJECT_DIRECTORIES": true, "GIT_PREFIX": true,
	}
	environment := make([]string, 0, len(os.Environ()))
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if !blocked[strings.ToUpper(key)] {
			environment = append(environment, entry)
		}
	}
	return environment
}
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
		cmd.Env = isolatedFixtureEnvironment()
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
	tests += "func TestTextPublicationIsolatedRoleObservations(t *testing.T){if ReadValue()==0 {t.Fatal(\"zero\")}}\n"
	tests += "func TestUnrelated(t *testing.T){if ReadStable()!=42 {t.Fatal(\"stable\")}}\n"
	write("internal/consumer/read_test.go", tests)
	write("internal/endpoint/read.go", "package endpoint\nimport \"example.com/selection/internal/source\"\nfunc ReadValue() int {return source.Value()}\n")
	endpointTests := "package endpoint\nimport \"testing\"\n"
	for i := range 5 {
		endpointTests += fmt.Sprintf("func TestTextPublicationFixture%02d(t *testing.T){if ReadValue()==0 {t.Fatal(\"zero\")}}\n", i)
	}
	write("internal/endpoint/read_test.go", endpointTests)
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
	endpointSelected := make(map[string]int)
	endpointGroups := 0
	dedicated := false
	selectionSelf, repositoryArchitecture := false, false
	for _, entry := range matrix.Include {
		if strings.Contains(entry.Run, "TestUnrelated") || strings.Contains(entry.Package, "tests/e2e") {
			t.Fatalf("unrelated check selected: %+v", entry)
		}
		if strings.HasSuffix(entry.Package, "/endpoint") {
			endpointGroups++
			names := strings.Split(strings.TrimSuffix(strings.TrimPrefix(entry.Run, "^("), ")$"), "|")
			if len(names) > 2 {
				t.Fatalf("endpoint PR check group contains %d tests, want at most 2: %s", len(names), entry.Run)
			}
			for _, name := range names {
				endpointSelected[name]++
			}
			continue
		}
		if !strings.HasSuffix(entry.Package, "/consumer") {
			selectionSelf = selectionSelf || strings.Contains(entry.Run, "TestPRSelectionSelf")
			repositoryArchitecture = repositoryArchitecture || strings.Contains(entry.Run, "TestRepositoryArchitecture")
			continue
		}
		consumerGroups++
		if entry.Run == "^(TestTextPublicationIsolatedRoleObservations)$" {
			dedicated = true
		}
		names := strings.Split(strings.TrimSuffix(strings.TrimPrefix(entry.Run, "^("), ")$"), "|")
		if len(names) > 16 {
			t.Fatalf("PR check group contains %d tests, want at most 16: %s", len(names), entry.Run)
		}
		for _, name := range names {
			selected[name]++
		}
	}
	if !selectionSelf || !repositoryArchitecture {
		t.Fatalf("selector change omitted its own behavior or repository test: %+v", matrix.Include)
	}
	if consumerGroups != 3 || len(selected) != 21 || !dedicated {
		t.Fatalf("bounded groups=%d selected=%v", consumerGroups, selected)
	}
	if endpointGroups != 3 || len(endpointSelected) != 5 {
		t.Fatalf("endpoint groups=%d selected=%v", endpointGroups, endpointSelected)
	}
	for name, count := range selected {
		if count != 1 {
			t.Errorf("test %s selected %d times", name, count)
		}
	}
	for name, count := range endpointSelected {
		if count != 1 {
			t.Errorf("endpoint test %s selected %d times", name, count)
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
		command.Env = isolatedFixtureEnvironment()
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
	powershellPath := filepath.Join(fixture, "powershell.txt")
	run("go", "run", filepath.Join(root, "scripts", "select-pr-checks.go"), filepath.Join(root, "scripts", "select-pr-check-registry.go"), "--base", base, "--head", "HEAD", "--matrix", matrixPath, "--powershell", powershellPath)
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
	powershellBody, err := os.ReadFile(powershellPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(powershellBody)) != "true" {
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
				command.Env = isolatedFixtureEnvironment()
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
			command.Env = isolatedFixtureEnvironment()
			output, err := command.CombinedOutput()
			if err == nil || !strings.Contains(string(output), test.want) {
				t.Fatalf("stale registry result = %v\n%s; want %q", err, output, test.want)
			}
		})
	}
}

func TestPRSelectionDeletedPackageAndDependencyErrors(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"broken deletion", "valid deletion", "missing dependency"} {
		t.Run(mode, func(t *testing.T) {
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
			run := func(name string, args ...string) []byte {
				t.Helper()
				c := exec.Command(name, args...)
				c.Dir = fixture
				c.Env = isolatedFixtureEnvironment()
				out, err := c.CombinedOutput()
				if err != nil {
					t.Fatalf("%s: %v\n%s", name, err, out)
				}
				return out
			}
			write("go.mod", "module example.com/deletion\n\ngo 1.26.8\n")
			write("docs/development/ownership.json", `{"pr_check_mappings":[]}`)
			write("cmd/tool/main.go", "package main\nfunc main(){}\n")
			write("tests/probe/probe_test.go", "package probe\nimport \"testing\"\nfunc TestUnrelated(t *testing.T){}\n")
			write("internal/architecture/architecture_test.go", "package architecture\nimport \"testing\"\nfunc TestRepositoryArchitecture(t *testing.T){}\nfunc TestUnrelated(t *testing.T){}\n")
			write("internal/source/value.go", "package source\nfunc Value() int{return 1}\n")
			write("internal/consumer/read.go", "package consumer\nimport \"example.com/deletion/internal/source\"\nfunc Read() int{return source.Value()}\n")
			write("internal/consumer/read_test.go", "package consumer\nimport \"testing\"\nfunc TestRead(t *testing.T){_=Read()}\nfunc TestUnrelated(t *testing.T){}\n")
			run("git", "init", "-q")
			run("git", "add", ".")
			commit := func() {
				run("git", "-c", "core.hooksPath="+filepath.Join(fixture, "absent-hooks"), "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "fixture")
			}
			commit()
			base := strings.TrimSpace(string(run("git", "rev-parse", "HEAD")))
			if mode == "missing dependency" {
				write("internal/consumer/read.go", "package consumer\nimport \"example.com/deletion/internal/missing\"\nfunc Read() int{return missing.Value()}\n")
			} else {
				if err := os.Remove(filepath.Join(fixture, "internal/source/value.go")); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "valid deletion" {
				write("internal/consumer/read.go", "package consumer\nfunc Read() int{return 2}\n")
			}
			run("git", "add", "-A")
			commit()
			matrixPath := filepath.Join(fixture, "matrix.json")
			c := exec.Command("go", "run", filepath.Join(root, "scripts/select-pr-checks.go"), filepath.Join(root, "scripts/select-pr-check-registry.go"), "--base", base, "--head", "HEAD", "--matrix", matrixPath)
			c.Dir = fixture
			c.Env = isolatedFixtureEnvironment()
			out, err := c.CombinedOutput()
			if mode != "valid deletion" {
				if err == nil || !strings.Contains(string(out), "load package owner") {
					t.Fatalf("broken import selection: %v\n%s", err, out)
				}
				if _, err := os.Stat(matrixPath); !os.IsNotExist(err) {
					t.Fatalf("failed selection published matrix: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("valid deletion: %v\n%s", err, out)
			}
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
			architecture, consumer := false, false
			for _, entry := range matrix.Include {
				if strings.Contains(entry.Run, "TestUnrelated") || strings.Contains(entry.Package, "/source") {
					t.Fatalf("unrelated or deleted check: %+v", entry)
				}
				architecture = architecture || strings.Contains(entry.Run, "TestRepositoryArchitecture")
				consumer = consumer || strings.Contains(entry.Run, "TestRead")
			}
			if !architecture || !consumer {
				t.Fatalf("deletion omitted static owner or affected consumer: %+v", matrix.Include)
			}
		})
	}
}

func TestPRSelectionExecutesAffectedFuzzSeeds(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []string{"dependency", "declaration"} {
		t.Run(change, func(t *testing.T) {
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
			run := func(name string, args ...string) []byte {
				t.Helper()
				c := exec.Command(name, args...)
				c.Dir, c.Env = fixture, isolatedFixtureEnvironment()
				out, err := c.CombinedOutput()
				if err != nil {
					t.Fatalf("%s: %v\n%s", name, err, out)
				}
				return out
			}
			write("go.mod", "module example.com/fuzzselection\n\ngo 1.26.8\n")
			write("docs/development/ownership.json", "{\"pr_check_mappings\":[]}\n")
			write("cmd/tool/main.go", "package main\nfunc main(){}\n")
			write("tests/probe/probe_test.go", "package probe\nimport \"testing\"\nfunc TestUnrelated(t *testing.T){}\n")
			write("internal/source/value.go", "package source\nfunc Value(v int) int {return v}\n")
			write("internal/source/stable.go", "package source\nfunc Stable(v int) int {return v}\n")
			write("internal/source/alias_test.go", "package source\nimport \"testing\"\ntype F = testing.F\n")
			seeds := "package source\nimport \"testing\"\n"
			for i := range 17 {
				seeds += fmt.Sprintf("func FuzzValue%02d(f *F){f.Add(1); f.Fuzz(func(t *testing.T,v int){if Value(v)!=v {t.Fatal(\"seed mismatch\")}})}\n", i)
			}
			seeds += "type probe struct{}\nfunc (probe) FuzzMethod(f *testing.F){_=Value(1)}\n"
			write("internal/source/value_test.go", seeds)
			write("internal/source/stable_test.go", "package source\nimport \"testing\"\nfunc FuzzUnrelated(f *testing.F){f.Add(1); f.Fuzz(func(t *testing.T,v int){_=Stable(v)})}\nfunc TestUnrelated(t *testing.T){_=Stable(1)}\n")
			run("git", "init", "-q")
			commit := func() {
				run("git", "add", ".")
				run("git", "-c", "core.hooksPath="+filepath.Join(fixture, "absent-hooks"), "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "fixture")
			}
			commit()
			base := strings.TrimSpace(string(run("git", "rev-parse", "HEAD")))
			if change == "dependency" {
				write("internal/source/value.go", "package source\nfunc Value(v int) int {return v+1}\n")
			} else {
				write("internal/source/value_test.go", strings.ReplaceAll(seeds, "Value(v)!=v", "Value(v)!=v+1"))
			}
			commit()
			matrixPath := filepath.Join(fixture, "matrix.json")
			args := []string{"run", filepath.Join(root, "scripts/select-pr-checks.go"), filepath.Join(root, "scripts/select-pr-check-registry.go"), "--base", base, "--head", "HEAD", "--matrix", matrixPath}
			out := run("go", args...)
			if !strings.Contains(string(out), "FuzzValue00") || strings.Contains(string(out), "FuzzUnrelated") || strings.Contains(string(out), "TestUnrelated") {
				t.Fatalf("incorrect direct selection:\n%s", out)
			}
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
			selected := map[string]int{}
			groups := 0
			for _, entry := range matrix.Include {
				if !strings.HasSuffix(entry.Package, "/source") {
					continue
				}
				groups++
				names := strings.Split(strings.TrimSuffix(strings.TrimPrefix(entry.Run, "^("), ")$"), "|")
				if len(names) > 16 {
					t.Fatalf("unbounded group: %s", entry.Run)
				}
				for _, name := range names {
					selected[name]++
				}
				c := exec.Command("go", "test", "-count=1", "-run", entry.Run, entry.Package)
				c.Dir, c.Env = fixture, isolatedFixtureEnvironment()
				out, err := c.CombinedOutput()
				if err == nil || !strings.Contains(string(out), "seed mismatch") {
					t.Fatalf("matrix did not execute failing seeds: %v\n%s", err, out)
				}
			}
			if groups != 2 || len(selected) != 17 {
				t.Fatalf("groups=%d selected=%v", groups, selected)
			}
			for i := range 17 {
				if selected[fmt.Sprintf("FuzzValue%02d", i)] != 1 {
					t.Fatalf("seed selected incorrectly: %v", selected)
				}
			}
			c := exec.Command("go", append(args, "--execute")...)
			c.Dir, c.Env = fixture, isolatedFixtureEnvironment()
			out, err = c.CombinedOutput()
			if err == nil || !strings.Contains(string(out), "seed mismatch") {
				t.Fatalf("execute did not run failing seeds: %v\n%s", err, out)
			}
		})
	}
}
