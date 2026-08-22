package architecture

import (
	"os/exec"
	"strings"
	"testing"
)

func TestStageEightTargetArchitectureAccountsForCurrentGoPackages(t *testing.T) {
	root := repositoryRoot(t)
	document := string(readProjectFile(t, root, "docs/development/stage-8-target-architecture.md"))
	packages := listedPackages(t, root, "./cmd/...", "./internal/...", "./tests/e2e/...")
	for packagePath := range listedLivePackages(t, root) {
		packages[packagePath] = true
	}
	for packagePath := range packages {
		relative := strings.TrimPrefix(packagePath, modulePath+"/")
		if !strings.Contains(document, "`"+relative+"`") {
			t.Errorf("target architecture lacks a disposition for current package %q", packagePath)
		}
	}
}

func listedLivePackages(t *testing.T, root string) map[string]bool {
	t.Helper()
	command := exec.Command("go", "list", "-tags=live", "./tests/live/...")
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		t.Fatalf("list live test packages: %v", err)
	}
	return packageSet(t, string(output))
}
