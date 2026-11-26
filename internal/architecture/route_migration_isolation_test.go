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
	"internal/domain/route",
	"internal/domain/route/ardp",
	"internal/domain/route/bootstrap",
	"internal/domain/route/capsule",
	"internal/domain/route/channel",
	"internal/domain/route/introduction",
	"internal/domain/route/issuer",
	"internal/domain/route/join",
	"internal/domain/route/join/pairing",
	"internal/domain/route/prefix",
	"internal/domain/route/receiver",
	"internal/domain/route/role",
	"internal/domain/route/selection",
	"internal/domain/route/transport",
	"internal/domain/route/transport/tls",
}

func routeMigrationBoundaryPermits(source, dependency string) bool {
	const retired = "internal/successor/route"
	const directory = "internal/domain/route"
	owner := path.Dir(source)
	local, localDependency := strings.CutPrefix(dependency, modulePath+"/")
	routeSource := owner == directory || strings.HasPrefix(owner, directory+"/")
	routeDependency := localDependency && (local == directory || strings.HasPrefix(local, directory+"/"))
	if strings.HasPrefix(source, retired+"/") || localDependency && (local == retired || strings.HasPrefix(local, retired+"/")) {
		return false
	}
	// These predecessor files occupied directories now owned by migrated code.
	// Reusing the directory cannot re-admit the predecessor.
	if (routeSource || routeDependency) && (source == "internal/domain/endpoint/source.go" || source == "internal/domain/route/closed_join_test.go") {
		return false
	}
	if routeDependency {
		if !slices.Contains(currentRoutePackages, local) {
			return false
		}
		if !slices.Contains(currentRoutePackages, owner) {
			switch owner {
			case "cmd/ardents-next", "internal/domain/connection", "internal/domain/endpoint", "internal/domain/endpoint/funding",
				"internal/domain/endpoint/publisher", "internal/domain/endpoint/reader", "internal/domain/endpoint/session",
				"internal/domain/node", "internal/domain/publication/instance", "internal/domain/publication/runtime":
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
			case "internal/domain/admission", "internal/domain/admission/receiving", "internal/domain/admission/spending",
				"internal/durablefile", "internal/domain/hosting", "internal/domain/network", "internal/domain/network/state", "internal/pathcheck", "internal/domain/reachability":
			case "internal/diagnostics/measurement", "internal/diagnostics/runtimeevent":
				// Explicit Product Owner decision, 2026-11-27: only the
				// registered numeric channel producers, never the SDK adapter.
				if !domainMeasurementImportAllowed(source, dependency) {
					return false
				}
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
		{"old Endpoint consumer", "internal/domain/endpoint/source.go", modulePath + "/internal/domain/route", false},
		{"old Node adapter", "internal/domain/node/authority/token.go", modulePath + "/internal/domain/route/receiver", false},
		{"old Route test consumer", "internal/domain/route/closed_join_test.go", modulePath + "/internal/domain/route", false},
		{"old command consumer", "cmd/ardents/route.go", modulePath + "/internal/domain/route", false},
		{"e2e bridge", "tests/e2e/route/scenario_test.go", modulePath + "/internal/domain/route/receiver", false},
		{"script bridge", "scripts/route_probe.go", modulePath + "/internal/domain/route", false},
		{"old Carrier dependency", "internal/domain/route/channel/frame_schedule.go", modulePath + "/internal/domain/route/carrier", false},
		{"registered channel measurement", "internal/domain/route/channel/frame_schedule.go", modulePath + "/internal/diagnostics/measurement", true},
		{"registered channel catalogue", "internal/domain/route/channel/session.go", modulePath + "/internal/diagnostics/runtimeevent", true},
		{"registered measurement test", "internal/domain/route/channel/measurement_test.go", modulePath + "/internal/diagnostics/measurement", true},
		{"unregistered channel file", "internal/domain/route/channel/other.go", modulePath + "/internal/diagnostics/measurement", false},
		{"unregistered catalogue edge", "internal/domain/route/channel/lane.go", modulePath + "/internal/diagnostics/runtimeevent", false},
		{"other Route owner", "internal/domain/route/prefix/prefix.go", modulePath + "/internal/diagnostics/measurement", false},
		{"channel SDK adapter denied", "internal/domain/route/channel/frame_schedule.go", modulePath + "/internal/diagnostics/telemetry", false},
		{"channel future measurement subpackage denied", "internal/domain/route/channel/session.go", modulePath + "/internal/diagnostics/measurement/export", false},
		{"measurement reverse Route import denied", "internal/diagnostics/measurement/capture.go", modulePath + "/internal/domain/route/channel", false},
		{"old Admission dependency", "internal/domain/route/receiving/admission.go", modulePath + "/internal/domain/admission/receiving", false},
		{"old Entry dependency", "internal/domain/route/selection.go", modulePath + "/internal/entry", false},
		{"old Node dependency in test", "internal/domain/route/role_test.go", modulePath + "/internal/domain/node/outer", false},
		{"external fixture dependency", "internal/domain/route/selection_test.go", modulePath + "/tests/epochfixture/network", false},
		{"new command boundary", "cmd/ardents-next/route.go", modulePath + "/internal/domain/route", true},
		{"retired canonical installed Descriptor role denied", "cmd/ardents-node/descriptor_installed_linux.go", modulePath + "/internal/domain/route/role", false},
		{"retired canonical Descriptor Route root denied", "cmd/ardents-node/descriptor_installed_linux.go", modulePath + "/internal/domain/route", false},
		{"new Admission boundary", "internal/domain/route/receiver/admission.go", modulePath + "/internal/domain/admission/receiving", true},
		{"new Network boundary", "internal/domain/route/prefix/prefix.go", modulePath + "/internal/domain/network/state", true},
		{"new Hosting boundary", "internal/domain/route/receiver/reservation.go", modulePath + "/internal/domain/hosting", true},
		{"migrated Endpoint consumer", "internal/domain/endpoint/history_preparation_linux.go", modulePath + "/internal/domain/route/selection", true},
		{"unregistered Endpoint subpackage", "internal/domain/endpoint/old/source_linux_test.go", modulePath + "/internal/domain/route/selection", false},
		{"unregistered Admission subpackage", "internal/domain/route/receiver/admission.go", modulePath + "/internal/domain/admission/legacy", false},
		{"retired Route package", "internal/domain/route/prefix/prefix.go", modulePath + "/internal/successor/route", false},
		{"retired Route source", "internal/successor/route/prefix/prefix.go", modulePath + "/internal/domain/route", false},
		{"Route external test", "internal/domain/route/path_test.go", modulePath + "/internal/domain/route", true},
		{"unrelated Network migration", "internal/domain/endpoint/source.go", modulePath + "/internal/domain/network/state", true},
		{"prefix is not a Route package", "internal/domain/endpoint/source.go", modulePath + "/internal/routeproposal", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := routeMigrationBoundaryPermits(test.source, test.dependency); got != test.allowed {
				t.Fatalf("boundary permits = %t, want %t", got, test.allowed)
			}
		})
	}
}
