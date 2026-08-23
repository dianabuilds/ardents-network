package architecture

import (
	"strings"
	"testing"
)

func TestStageEightTargetArchitectureAccountsForCurrentGoPackages(t *testing.T) {
	root := repositoryRoot(t)
	document := string(readProjectFile(t, root, "docs/development/stage-8-target-architecture.md"))
	packages := listedPackages(t, root, "./cmd/...", "./internal/...", "./tests/e2e/...")
	for packagePath := range packages {
		relative := strings.TrimPrefix(packagePath, modulePath+"/")
		if !strings.Contains(document, "`"+relative+"`") {
			t.Errorf("target architecture lacks a disposition for current package %q", packagePath)
		}
	}
}
