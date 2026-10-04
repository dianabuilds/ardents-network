package architecture

import (
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

// This boundary deliberately precedes any package-map migration exceptions.
// Registering a Route package must not grant an old runtime consumer access.
func routeMigrationBoundaryPermits(source, dependency string) bool {
	const directory = "internal/successor/route"
	const imported = modulePath + "/" + directory
	routeSource := strings.HasPrefix(source, directory+"/")
	routeDependency := dependency == imported || strings.HasPrefix(dependency, imported+"/")
	newSource := strings.HasPrefix(source, "internal/successor/") || strings.HasPrefix(source, "cmd/ardents-next/")
	if routeDependency && !newSource {
		return false
	}
	if routeSource && strings.HasPrefix(dependency, modulePath+"/") && !strings.HasPrefix(dependency, modulePath+"/internal/successor/") {
		return false
	}
	return true
}

func TestRouteMigrationImportIsolation(t *testing.T) {
	root := repositoryRoot(t)
	walk(t, root, func(filename string, entry os.DirEntry) {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			return
		}
		source := relativePath(t, root, filename)
		file, err := parser.ParseFile(token.NewFileSet(), filename, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imported := range file.Imports {
			dependency, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if !routeMigrationBoundaryPermits(source, dependency) {
				t.Errorf("Route migration contract: %s imports forbidden dependency %s", source, dependency)
			}
		}
	})
}

func TestRouteMigrationIsolationPolicy(t *testing.T) {
	for _, test := range []struct {
		name, source, dependency string
		allowed                  bool
	}{
		{"old Endpoint consumer", "internal/endpoint/source.go", modulePath + "/internal/successor/route", false},
		{"old Node adapter", "internal/node/authority/token.go", modulePath + "/internal/successor/route/receiving", false},
		{"old Route test consumer", "internal/route/closed_join_test.go", modulePath + "/internal/successor/route", false},
		{"old command consumer", "cmd/ardents/route.go", modulePath + "/internal/successor/route", false},
		{"e2e bridge", "tests/e2e/route/scenario_test.go", modulePath + "/internal/successor/route/client", false},
		{"script bridge", "scripts/route_probe.go", modulePath + "/internal/successor/route", false},
		{"old Carrier dependency", "internal/successor/route/carrier/transport.go", modulePath + "/internal/route/carrier", false},
		{"old Admission dependency", "internal/successor/route/receiving/admission.go", modulePath + "/internal/admission/receiving", false},
		{"old Entry dependency", "internal/successor/route/selection.go", modulePath + "/internal/entry", false},
		{"old Node dependency in test", "internal/successor/route/role_test.go", modulePath + "/internal/node/outer", false},
		{"external fixture dependency", "internal/successor/route/selection_test.go", modulePath + "/tests/epochfixture/network", false},
		{"new command boundary", "cmd/ardents-next/route.go", modulePath + "/internal/successor/route", true},
		{"new Admission boundary", "internal/successor/route/receiving/admission.go", modulePath + "/internal/successor/admission/receiving", true},
		{"new Network boundary", "internal/successor/route/selection.go", modulePath + "/internal/successor/network/state", true},
		{"new Hosting boundary", "internal/successor/route/receiving/reservation.go", modulePath + "/internal/successor/hosting", true},
		{"Route external test", "internal/successor/route/path_test.go", modulePath + "/internal/successor/route", true},
		{"unrelated Network migration", "internal/endpoint/source.go", modulePath + "/internal/successor/network/state", true},
		{"prefix is not a Route package", "internal/endpoint/source.go", modulePath + "/internal/successor/routeproposal", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := routeMigrationBoundaryPermits(test.source, test.dependency); got != test.allowed {
				t.Fatalf("boundary permits = %t, want %t", got, test.allowed)
			}
		})
	}
}
