package architecture

import (
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestFuzzProfileRunsEverySelectedMutationTarget(t *testing.T) {
	root := repositoryRoot(t)
	makefile := string(readProjectFile(t, root, "Makefile"))
	if !strings.Contains(makefile, "fuzz:\n\tgo run ./scripts/run-fuzz-targets.go") {
		t.Fatal("fuzz Make target does not delegate to the selected-target runner")
	}

	command := exec.Command("go", "run", "./scripts/run-fuzz-targets.go", "-list")
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		t.Fatalf("list selected fuzz targets: %v", err)
	}
	want := []string{
		"./internal/network/state", "FuzzCanonicalParsers", "30s",
		"./internal/contributor", "FuzzContributorJSONDecoders", "30s",
	}
	if got := strings.Fields(string(output)); !reflect.DeepEqual(got, want) {
		t.Errorf("selected fuzz targets = %v, want %v", got, want)
	}

	command = exec.Command("go", "run", "./scripts/run-fuzz-targets.go", "-verify")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("verify selected fuzz targets: %v\n%s", err, output)
	}

	command = exec.Command("go", "run", "./scripts/run-fuzz-targets.go", "-commands")
	command.Dir = root
	output, err = command.CombinedOutput()
	if err != nil {
		t.Fatalf("list selected mutation-fuzz commands: %v\n%s", err, output)
	}
	wantCommands := []string{
		"go\ttest\t./internal/network/state\t-run\t^$\t-fuzz\t^FuzzCanonicalParsers$\t-fuzztime=30s",
		"go\ttest\t./internal/contributor\t-run\t^$\t-fuzz\t^FuzzContributorJSONDecoders$\t-fuzztime=30s",
	}
	if got := strings.Split(strings.TrimSpace(string(output)), "\n"); !reflect.DeepEqual(got, wantCommands) {
		t.Errorf("selected mutation-fuzz commands = %v, want %v", got, wantCommands)
	}
}
