package architecture

import "testing"

func TestRepositoryArchitectureQualificationBoundary(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	registry := readPackageRegistry(t, root)
	qualification, exists := registry["internal/qualification"]
	if !exists {
		t.Fatal("internal/qualification must be registered")
	}
	if len(qualification.allowedImports) != 0 {
		t.Error("internal/qualification must use only the standard library")
	}
	for owner, registration := range registry {
		if !registration.allowedImports["internal/qualification"] {
			continue
		}
		if owner != "cmd/ardents-qualify" {
			t.Errorf("runtime package %s cannot import internal/qualification", owner)
		}
	}
}
