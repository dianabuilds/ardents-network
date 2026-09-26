package architecture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQualificationFixtureTestsAreInTheLinuxCheck(t *testing.T) {
	root := repositoryRoot(t)
	const fixture = "./tests/qualification/stream-network-two-host/fixturecommand/qualification-network"
	for _, name := range []string{"main_test.go", "selection_test.go"} {
		path := filepath.Join(root, filepath.FromSlash(fixture), name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("selected qualification fixture test %s: %v", name, err)
		}
	}
	makefile := string(readProjectFile(t, root, "Makefile"))
	for _, required := range []string{
		"fixture-network-test:\n\t@test \"$(HEADLESS_GOOS)\" = linux",
		"go test " + fixture + " -count=1",
		"$(MAKE) --output-sync=target fixture-network-test",
	} {
		if !strings.Contains(makefile, required) {
			t.Errorf("Linux check does not select qualification fixture: missing %q", required)
		}
	}
}
