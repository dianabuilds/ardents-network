package architecture

import (
	"go/parser"
	"go/token"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// These exact package identities come from the migrated Route and its current
// owners. Package-map registration alone never grants access to Route.
var currentRoutePackages = []string{
	"internal/route",
	"internal/route/ardp",
	"internal/route/bootstrap",
	"internal/route/capsule",
	"internal/route/channel",
	"internal/route/introduction",
	"internal/route/issuer",
	"internal/route/join",
	"internal/route/join/pairing",
	"internal/route/prefix",
	"internal/route/receiver",
	"internal/route/role",
	"internal/route/selection",
	"internal/route/transport",
	"internal/route/transport/tls",
}

func routeMigrationBoundaryPermits(source, dependency string) bool {
	const retired = "internal/successor/route"
	const directory = "internal/route"
	owner := path.Dir(source)
	local, localDependency := strings.CutPrefix(dependency, modulePath+"/")
	routeSource := owner == directory || strings.HasPrefix(owner, directory+"/")
	routeDependency := localDependency && (local == directory || strings.HasPrefix(local, directory+"/"))
	if strings.HasPrefix(source, retired+"/") || localDependency && (local == retired || strings.HasPrefix(local, retired+"/")) {
		return false
	}
	// These predecessor files occupied directories now owned by migrated code.
	// Reusing the directory cannot re-admit the predecessor.
	if (routeSource || routeDependency) && (source == "internal/endpoint/source.go" || source == "internal/route/closed_join_test.go") {
		return false
	}
	if routeDependency {
		if !slices.Contains(currentRoutePackages, local) {
			return false
		}
		if !slices.Contains(currentRoutePackages, owner) {
			switch owner {
			case "cmd/ardents-next", "internal/connection", "internal/endpoint", "internal/endpoint/funding",
				"internal/endpoint/publisher", "internal/endpoint/reader", "internal/endpoint/session",
				"internal/domain/node", "internal/publication/instance", "internal/publication/runtime":
			default:
				return false
			}
		}
	}
	if routeSource && localDependency {
		if !slices.Contains(currentRoutePackages, owner) {
			return false
		}
		if !routeDependency {
			switch local {
			case "internal/admission", "internal/admission/receiving", "internal/admission/spending",
				"internal/durablefile", "internal/hosting", "internal/network", "internal/network/state", "internal/pathgrammar", "internal/reachability":
			default:
				return false
			}
		}
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
		{"old Endpoint consumer", "internal/endpoint/source.go", modulePath + "/internal/route", false},
		{"old Node adapter", "internal/domain/node/authority/token.go", modulePath + "/internal/route/receiver", false},
		{"old Route test consumer", "internal/route/closed_join_test.go", modulePath + "/internal/route", false},
		{"old command consumer", "cmd/ardents/route.go", modulePath + "/internal/route", false},
		{"e2e bridge", "tests/e2e/route/scenario_test.go", modulePath + "/internal/route/receiver", false},
		{"script bridge", "scripts/route_probe.go", modulePath + "/internal/route", false},
		{"old Carrier dependency", "internal/route/channel/frame_schedule.go", modulePath + "/internal/route/carrier", false},
		{"old Admission dependency", "internal/route/receiving/admission.go", modulePath + "/internal/admission/receiving", false},
		{"old Entry dependency", "internal/route/selection.go", modulePath + "/internal/entry", false},
		{"old Node dependency in test", "internal/route/role_test.go", modulePath + "/internal/domain/node/outer", false},
		{"external fixture dependency", "internal/route/selection_test.go", modulePath + "/tests/epochfixture/network", false},
		{"new command boundary", "cmd/ardents-next/route.go", modulePath + "/internal/route", true},
		{"retired canonical installed Descriptor role denied", "cmd/ardents-node/descriptor_installed_linux.go", modulePath + "/internal/route/role", false},
		{"retired canonical Descriptor Route root denied", "cmd/ardents-node/descriptor_installed_linux.go", modulePath + "/internal/route", false},
		{"new Admission boundary", "internal/route/receiver/admission.go", modulePath + "/internal/admission/receiving", true},
		{"new Network boundary", "internal/route/prefix/prefix.go", modulePath + "/internal/network/state", true},
		{"new Hosting boundary", "internal/route/receiver/reservation.go", modulePath + "/internal/hosting", true},
		{"migrated Endpoint consumer", "internal/endpoint/history_preparation_linux.go", modulePath + "/internal/route/selection", true},
		{"unregistered Endpoint subpackage", "internal/endpoint/old/source_linux_test.go", modulePath + "/internal/route/selection", false},
		{"unregistered Admission subpackage", "internal/route/receiver/admission.go", modulePath + "/internal/admission/legacy", false},
		{"retired Route package", "internal/route/prefix/prefix.go", modulePath + "/internal/successor/route", false},
		{"retired Route source", "internal/successor/route/prefix/prefix.go", modulePath + "/internal/route", false},
		{"Route external test", "internal/route/path_test.go", modulePath + "/internal/route", true},
		{"unrelated Network migration", "internal/endpoint/source.go", modulePath + "/internal/network/state", true},
		{"prefix is not a Route package", "internal/endpoint/source.go", modulePath + "/internal/routeproposal", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := routeMigrationBoundaryPermits(test.source, test.dependency); got != test.allowed {
				t.Fatalf("boundary permits = %t, want %t", got, test.allowed)
			}
		})
	}
}
