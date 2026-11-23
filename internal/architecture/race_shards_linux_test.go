package architecture

import (
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

func TestRaceShardsCoverTheUnitInventoryOnce(t *testing.T) {
	root := repositoryRoot(t)
	expected := listedProfilePackages(t, root, "tests/profiles/deterministic-packages.txt")
	for name := range listedProfilePackages(t, root, "tests/profiles/deterministic-linux-packages.txt") {
		expected[name] = true
	}
	seen := make(map[string]int)
	for shard := range 4 {
		command := exec.Command("make", "--no-print-directory", "race-shard-packages", "RACE_SHARD="+strconv.Itoa(shard))
		command.Dir = root
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("list race shard %d: %v\n%s", shard, err, output)
		}
		packages := strings.Fields(string(output))
		if len(packages) == 0 {
			t.Fatalf("race shard %d is empty", shard)
		}
		for _, name := range packages {
			if !expected[name] {
				t.Errorf("race shard %d contains unexpected package %s", shard, name)
			}
			seen[name]++
		}
	}
	for name := range expected {
		if seen[name] != 1 {
			t.Errorf("race inventory contains %s %d times", name, seen[name])
		}
	}
	invalid := exec.Command("make", "--no-print-directory", "race-shard-packages", "RACE_SHARD=4")
	invalid.Dir = root
	if output, err := invalid.CombinedOutput(); err == nil {
		t.Fatalf("invalid race shard accepted: %s", output)
	}
}
