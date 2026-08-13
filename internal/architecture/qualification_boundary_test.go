package architecture

import "testing"

func TestRepositoryArchitectureQualificationBoundary(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	registry := readPackageRegistry(t, root)
	packages := []string{"internal/qualification/byteio", "internal/qualification/epochfixture", "internal/qualification/state",
		"internal/qualification/node", "internal/qualification/node/fixture", "internal/qualification/route", "internal/qualification/routesmoke",
		"internal/qualification/service", "internal/qualification/servicesmoke"}
	for _, directory := range packages {
		qualification, exists := registry[directory]
		if !exists {
			t.Fatalf("%s must be registered", directory)
		}
		switch directory {
		case "internal/qualification/state":
			if len(qualification.allowedImports) != 1 || !qualification.allowedImports["internal/qualification/byteio"] {
				t.Errorf("%s may import only qualification byte I/O", directory)
			}
		case "internal/qualification/node":
			if len(qualification.allowedImports) != 2 || !qualification.allowedImports["internal/qualification/byteio"] ||
				!qualification.allowedImports["internal/qualification/node/fixture"] {
				t.Errorf("%s may import only qualification byte I/O and its fixture", directory)
			}
		case "internal/qualification/node/fixture":
			if len(qualification.allowedImports) != 3 || !qualification.allowedImports["internal/qualification/byteio"] ||
				!qualification.allowedImports["internal/network/epoch/assignment"] ||
				!qualification.allowedImports["internal/qualification/epochfixture"] {
				t.Errorf("%s may import only canonical Epoch rules and qualification byte I/O", directory)
			}
		case "internal/qualification/routesmoke":
			wanted := []string{"internal/network/epoch/assignment", "internal/network/state", "internal/qualification/byteio",
				"internal/qualification/epochfixture", "internal/qualification/route", "internal/route"}
			if len(qualification.allowedImports) != len(wanted) {
				t.Errorf("%s has an unexpected import count", directory)
			}
		case "internal/qualification/servicesmoke":
			wanted := []string{"internal/qualification/byteio", "internal/qualification/routesmoke", "internal/route", "internal/serviceconn"}
			if len(qualification.allowedImports) != len(wanted) {
				t.Errorf("%s has an unexpected import count", directory)
			}
			for _, dependency := range wanted {
				if !qualification.allowedImports[dependency] {
					t.Errorf("%s must permit %s", directory, dependency)
				}
			}
			for _, dependency := range wanted {
				if !qualification.allowedImports[dependency] {
					t.Errorf("%s must permit %s", directory, dependency)
				}
			}
		case "internal/qualification/route":
			if len(qualification.allowedImports) != 1 || !qualification.allowedImports["internal/route"] {
				t.Errorf("%s may import the candidate Route only from compatibility tests", directory)
			}
		case "internal/qualification/epochfixture":
			if len(qualification.allowedImports) != 3 || !qualification.allowedImports["internal/network/epoch"] ||
				!qualification.allowedImports["internal/network/epoch/assignment"] ||
				!qualification.allowedImports["internal/network/epoch/merkle"] {
				t.Errorf("%s may import only canonical Epoch rules", directory)
			}
		default:
			if len(qualification.allowedImports) != 0 {
				t.Errorf("%s must use only the standard library", directory)
			}
		}
	}
	for owner, registration := range registry {
		for _, directory := range packages {
			allowed := owner == "cmd/ardents-qualify" ||
				owner == "internal/qualification/routesmoke" ||
				owner == "internal/qualification/servicesmoke" &&
					(directory == "internal/qualification/byteio" || directory == "internal/qualification/routesmoke") ||
				owner == "cmd/ardents-route-qualify" && directory == "internal/qualification/route" ||
				owner == "cmd/ardents-service-qualify" && directory == "internal/qualification/service" ||
				owner == "internal/qualification/state" && directory == "internal/qualification/byteio" ||
				owner == "internal/qualification/node" && (directory == "internal/qualification/byteio" || directory == "internal/qualification/node/fixture") ||
				owner == "internal/qualification/node/fixture" && (directory == "internal/qualification/byteio" || directory == "internal/qualification/epochfixture")
			if registration.allowedImports[directory] && !allowed {
				t.Errorf("runtime package %s cannot import %s", owner, directory)
			}
		}
	}
}
