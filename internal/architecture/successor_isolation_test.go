package architecture

import (
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// Existing Endpoint/v3 Enrollment grammar and portable behavior controls remain
// selected on both platforms. ADR-0123's separate Source artifact extension is
// Linux-only and checked against its exact genuine consumer profile below.
func TestSuccessorEnrollmentRunsPortableRulesOnWindowsAndLinux(t *testing.T) {
	directory := path.Join(repositoryRoot(t), "internal/enrollment")
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		switch name {
		case "file_unix.go", "file_unix_test.go", "file_windows.go", "file_unsupported.go":
			continue
		case "infrastructure_inventory.go", "infrastructure_inventory_test.go", "descriptor_inventory_linux_test.go", "infrastructure_process_linux.go":
			// This distinct inventory has no Windows accepting product path.
			continue
		}
		for _, target := range []string{"windows", "linux"} {
			profile := build.Default
			profile.GOOS, profile.GOARCH, profile.CgoEnabled = target, "amd64", false
			selected, err := profile.MatchFile(directory, name)
			if err != nil {
				t.Fatal(err)
			}
			if !selected {
				t.Errorf("portable Enrollment source %s excluded on %s", name, target)
			}
		}
	}
}

// Retained State opening uses portable history rules and each platform's
// existing lease adapter; it supplies no native Source installation permission.
func TestSuccessorStateRetainedOpeningRunsPortableRules(t *testing.T) {
	root := repositoryRoot(t)
	for _, target := range []string{"linux", "windows", "darwin"} {
		profile := build.Default
		profile.GOOS, profile.GOARCH, profile.CgoEnabled = target, "amd64", false
		for _, name := range []string{
			"internal/network/state/retained_open.go",
			"internal/network/state/retained_open_test.go",
			"internal/network/state/durable/retained_open.go",
			"internal/network/state/durable/retained_open_test.go",
		} {
			selected, err := profile.MatchFile(path.Join(root, path.Dir(name)), path.Base(name))
			if err != nil {
				t.Fatal(err)
			}
			if !selected {
				t.Errorf("portable retained State source %s excluded on %s", name, target)
			}
		}
	}
}

func TestInfrastructureExecutableMechanismHasExactPlatformSelection(t *testing.T) {
	root := repositoryRoot(t)
	for _, target := range []string{"linux", "windows", "darwin"} {
		profile := build.Default
		profile.GOOS, profile.GOARCH, profile.CgoEnabled = target, "amd64", false
		for _, name := range []string{
			"internal/enrollment/infrastructure_inventory.go",
			"internal/enrollment/infrastructure_inventory_test.go",
			"internal/enrollment/descriptor_inventory_linux_test.go",
			"internal/enrollment/infrastructure_process_linux.go",
			"internal/platform/systemd/infrastructure_prestart_linux.go",
			"internal/platform/systemd/infrastructure_prestart_linux_test.go",
			"internal/release/infrastructure/admission.go",
			"internal/release/infrastructure/source_release_inspection_linux.go",
			"internal/release/infrastructure/preparation_linux.go",
			"internal/platform/cgroup/source_quiescence_linux.go",
			"internal/network/state/source_inspection_linux.go",
			"internal/network/state/source_preparation_linux.go",
			"internal/endpoint/source_trust_preparation_linux.go",
			"internal/network/state/inspection_state_linux.go",
			"internal/network/state/node_inspection_linux.go",
			"internal/network/state/durable/inspection_linux.go",
			"internal/network/duty/source_inspection_linux.go",
			"internal/network/duty/inspection_linux.go",
			"internal/network/duty/node_inspection_linux.go",
			"internal/admission/spending/inspection_linux.go",
			"internal/hosting/inspection_linux.go",
			"internal/node/provisioning/command_linux.go",
			"internal/platform/artifacts/input_linux_test.go",
			"cmd/ardents-next/source_preparation_linux.go",
			"cmd/ardents-next/source_state_preparation_linux_test.go",
			"internal/network/state/source_state_files_linux.go",
			"internal/network/state/source_state_files_linux_test.go",
			"internal/platform/artifacts/directory_linux.go",
			"internal/platform/artifacts/protected_input_linux.go",
			"internal/platform/artifacts/input_linux.go",
			"internal/platform/artifacts/receipt_linux.go",
			"cmd/ardents-next/source_inspection_linux.go",
			"tests/e2e/source/inspection_linux_test.go",
			"cmd/ardents-next/infrastructure_linux.go",
			"internal/release/infrastructure/signing/command_linux.go",
			"internal/release/infrastructure/signing/join_release_keys_linux.go",
			"internal/release/infrastructure/signing/infrastructure_release_keys_linux.go",
			"internal/release/infrastructure/signing/node_release_plan_linux.go",
			"internal/release/infrastructure/signing/node_release_arguments_linux.go",
			"internal/release/infrastructure/signing/node_release_linux.go",
			"internal/release/infrastructure/signing/node_release_predecessor_linux.go",
			"internal/release/infrastructure/signing/join_release_plan_linux_test.go",
			"internal/release/infrastructure/join_admission.go",
			"internal/platform/artifacts/output_linux.go",
			"internal/release/infrastructure/signing/source_release_linux.go",
			"internal/release/infrastructure/signing/source_release_predecessor_linux.go",
			"internal/release/infrastructure/signing/source_release_keys_linux.go",
			"internal/release/infrastructure/signing/source_release_plan_linux.go",
			"internal/release/infrastructure/signing/source_release_plan_linux_test.go",
			"internal/node/provisioning/source_initial_keys_linux.go",
			"internal/node/provisioning/source_initial_network_linux.go",
			"internal/node/provisioning/source_initial_network_linux_test.go",
			"internal/node/provisioning/source_initial_plan_linux.go",
			"internal/platform/artifacts/key_linux.go",
			"internal/platform/systemd/node_slots_linux.go",
			"internal/platform/systemd/node_slots_linux_test.go",
			"internal/platform/systemd/source_object_linux_test.go",
			"internal/platform/systemd/source_manager_linux.go",
			"internal/release/history_binding_linux.go",
			"internal/release/history_binding_linux_test.go",
		} {
			selected, err := profile.MatchFile(path.Join(root, path.Dir(name)), path.Base(name))
			if err != nil {
				t.Fatal(err)
			}
			want := target == "linux"
			if selected != want {
				t.Errorf("native executable adapter %s on %s: %v, want %v", name, target, selected, want)
			}
		}
		for _, name := range []string{
			"internal/platform/systemd/infrastructure_unit_template.go",
			"internal/release/infrastructure/doc.go",
			"internal/release/infrastructure/artifact.go",
			"internal/release/infrastructure/artifact_test.go",
			"internal/release/infrastructure/descriptor_artifact.go",
			"internal/release/infrastructure/forwarding_artifact.go",
			"internal/release/infrastructure/join_artifact.go",
		} {
			selected, err := profile.MatchFile(path.Join(root, path.Dir(name)), path.Base(name))
			if err != nil {
				t.Fatal(err)
			}
			if !selected {
				t.Errorf("portable infrastructure artifact file %s excluded on %s", name, target)
			}
		}
		// The static metadata projection is a pure JSON adapter (P1-B batch 1,
		// #781): it composes only portable Release helpers and executes its
		// behavior test on every mandatory target. The native history binding
		// above stays Linux-only.
		for _, name := range []string{
			"internal/release/static_metadata.go",
			"internal/release/static_metadata_test.go",
		} {
			selected, err := profile.MatchFile(path.Join(root, path.Dir(name)), path.Base(name))
			if err != nil {
				t.Fatal(err)
			}
			if !selected {
				t.Errorf("portable static metadata file %s excluded on %s", name, target)
			}
		}
		for _, tagged := range []bool{false, true} {
			profile.BuildTags = nil
			if tagged {
				profile.BuildTags = []string{"network_signing_native"}
			}
			for _, name := range []string{
				"internal/network/state/source_inspection_native_linux_test.go",
				"cmd/ardents-next/infrastructure_preparation_native_linux_test.go",
				"internal/release/infrastructure/signing/join_release_native_linux_test.go",
				"internal/release/infrastructure/signing/join_forward_release_native_linux_test.go",
				"internal/network/state/node_inspection_native_linux_test.go",
				"internal/network/duty/inspection_native_linux_test.go",
				"internal/admission/spending/inspection_native_linux_test.go",
				"internal/hosting/inspection_native_linux_test.go",
			} {
				selected, err := profile.MatchFile(path.Join(root, path.Dir(name)), path.Base(name))
				if err != nil {
					t.Fatal(err)
				}
				if want := target == "linux" && tagged; selected != want {
					t.Errorf("history inspection native profile %s on %s tagged=%v: %v, want %v", name, target, tagged, selected, want)
				}
			}
		}
	}
}

// P1-B batch 2 (#787, #691) de-tagged verified pure production files whose
// composed symbols became portable in batch 1: the Node failure/admission
// projections, the whole endpoint/funding composition and the session route
// reservation helpers. They add no native mechanism, so every mandatory target
// must select them and their portable behavior tests. The Node first-failure
// physical-join test still composes the Linux forwarding lifetime and stays
// Linux-only.
func TestP1BBatch2PureProductionRunsOnEveryMandatoryTarget(t *testing.T) {
	root := repositoryRoot(t)
	for _, target := range []string{"linux", "windows", "darwin"} {
		profile := build.Default
		profile.GOOS, profile.GOARCH, profile.CgoEnabled = target, "amd64", false
		for _, name := range []string{
			"internal/node/failure_projection.go",
			"internal/node/failure_projection_test.go",
			"internal/node/admission_projection.go",
			"internal/node/admission_projection_test.go",
			"internal/endpoint/funding/reservation.go",
			"internal/endpoint/funding/issuance.go",
			"internal/endpoint/funding/parents.go",
			"internal/endpoint/funding/prefix.go",
			"internal/endpoint/funding/preparation.go",
			"internal/endpoint/funding/source.go",
			"internal/endpoint/funding/observation.go",
			"internal/endpoint/funding/preparation_test.go",
			"internal/endpoint/session/reservation.go",
			"internal/endpoint/session/reservation_test.go",
		} {
			selected, err := profile.MatchFile(path.Join(root, path.Dir(name)), path.Base(name))
			if err != nil {
				t.Fatal(err)
			}
			if !selected {
				t.Errorf("portable P1-B batch 2 file %s excluded on %s", name, target)
			}
		}
		for _, name := range []string{
			"internal/node/failure_projection_linux_test.go",
		} {
			selected, err := profile.MatchFile(path.Join(root, path.Dir(name)), path.Base(name))
			if err != nil {
				t.Fatal(err)
			}
			if want := target == "linux"; selected != want {
				t.Errorf("Linux forwarding first-failure test %s on %s: %v, want %v", name, target, selected, want)
			}
		}
	}
}

// P1-B batch 3 (#794, #691) de-tagged the next verified pure leaves whose
// composed symbols are already portable: the Reader local Text result
// projection, the Publisher root-isolation predicate and fixed document stream
// contract, and the Endpoint network presentation guard. They add no native
// mechanism, so every mandatory target must select them and their portable
// behavior tests. Their genuine production consumers (reader/publisher Open and
// the installed participant) remain Linux-tagged behind unchanged !linux typed
// refusals, so the de-tagged helpers stay Windows-production-dead only.
func TestP1BBatch3PureProductionRunsOnEveryMandatoryTarget(t *testing.T) {
	root := repositoryRoot(t)
	for _, target := range []string{"linux", "windows", "darwin"} {
		profile := build.Default
		profile.GOOS, profile.GOARCH, profile.CgoEnabled = target, "amd64", false
		for _, name := range []string{
			"internal/endpoint/reader/result.go",
			"internal/endpoint/reader/result_test.go",
			"internal/endpoint/publisher/roots.go",
			"internal/endpoint/publisher/roots_test.go",
			"internal/endpoint/publisher/contract.go",
			"internal/endpoint/authority.go",
		} {
			selected, err := profile.MatchFile(path.Join(root, path.Dir(name)), path.Base(name))
			if err != nil {
				t.Fatal(err)
			}
			if !selected {
				t.Errorf("portable P1-B batch 3 file %s excluded on %s", name, target)
			}
		}
	}
}

// The shared channel has no native mechanism. Check actual Go file selection,
// including tests: a passing suite must not silently omit an inherited OS file.
func TestSuccessorRouteChannelRunsSameSourcesOnWindowsAndLinux(t *testing.T) {
	directory := path.Join(repositoryRoot(t), "internal/route/channel")
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		for _, target := range []string{"windows", "linux"} {
			profile := build.Default
			profile.GOOS, profile.GOARCH, profile.CgoEnabled = target, "amd64", false
			selected, err := profile.MatchFile(directory, entry.Name())
			if err != nil {
				t.Fatal(err)
			}
			if !selected {
				t.Errorf("portable channel source %s excluded on %s", entry.Name(), target)
			}
		}
	}
}

func TestSuccessorRouteRoleRulesRunOnWindowsAndLinux(t *testing.T) {
	directory := path.Join(repositoryRoot(t), "internal/route/role")
	for _, filename := range []string{"authority.go", "authority_test.go", "presentation.go", "presentation_test.go", "doc.go"} {
		for _, target := range []string{"windows", "linux"} {
			profile := build.Default
			profile.GOOS, profile.GOARCH, profile.CgoEnabled = target, "amd64", false
			selected, err := profile.MatchFile(directory, filename)
			if err != nil {
				t.Fatal(err)
			}
			if !selected {
				t.Errorf("portable role rule %s excluded on %s", filename, target)
			}
		}
	}
}

// Prefix consumes retained facts and actual portable adapters. Durable roots
// belong to selection/Admission, not to its physical generation or borrowers.
func TestSuccessorRoutePrefixRunsSameSourcesOnWindowsAndLinux(t *testing.T) {
	directory := path.Join(repositoryRoot(t), "internal/route/prefix")
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		for _, target := range []string{"windows", "linux"} {
			profile := build.Default
			profile.GOOS, profile.GOARCH, profile.CgoEnabled = target, "amd64", false
			selected, err := profile.MatchFile(directory, name)
			if err != nil {
				t.Fatal(err)
			}
			if !selected {
				t.Errorf("portable Prefix source %s excluded on %s", name, target)
			}
		}
	}
}

// Receiving pairing/relay opens no durable root and owns no native mechanism.
func TestSuccessorRouteReceivingJoinRunsOnWindowsAndLinux(t *testing.T) {
	directory := path.Join(repositoryRoot(t), "internal/route/join/pairing")
	for _, filename := range []string{"pair.go", "relay.go", "pair_test.go", "relay_test.go", "relay_eof_test.go", "doc.go"} {
		for _, target := range []string{"windows", "linux"} {
			profile := build.Default
			profile.GOOS, profile.GOARCH, profile.CgoEnabled = target, "amd64", false
			selected, err := profile.MatchFile(directory, filename)
			if err != nil {
				t.Fatal(err)
			}
			if !selected {
				t.Errorf("portable receiving JOIN source %s excluded on %s", filename, target)
			}
		}
	}
}

// Holder exchange and inner/lower retirement use ordered I/O and exact
// borrowed generations. Native Context selection is a separate owner.
func TestSuccessorRouteHolderJoinRunsOnWindowsAndLinux(t *testing.T) {
	directory := path.Join(repositoryRoot(t), "internal/route/join")
	for _, filename := range []string{"acquisition.go", "acquisition_test.go", "client.go", "stream.go", "stream_test.go"} {
		for _, target := range []string{"windows", "linux"} {
			profile := build.Default
			profile.GOOS, profile.GOARCH, profile.CgoEnabled = target, "amd64", false
			selected, err := profile.MatchFile(directory, filename)
			if err != nil {
				t.Fatal(err)
			}
			if !selected {
				t.Errorf("portable holder JOIN source %s excluded on %s", filename, target)
			}
		}
	}
}

// Directed-pair pool owns no native root or receiving Grant. Its actual
// physical retirement tests must execute on both platforms.
func TestSuccessorRouteReceiverPoolRunsOnWindowsAndLinux(t *testing.T) {
	directory := path.Join(repositoryRoot(t), "internal/route/receiver")
	for _, filename := range []string{"carrier_pool.go", "carrier_pool_test.go", "physical_fixture_test.go"} {
		for _, target := range []string{"windows", "linux"} {
			profile := build.Default
			profile.GOOS, profile.GOARCH, profile.CgoEnabled = target, "amd64", false
			selected, err := profile.MatchFile(directory, filename)
			if err != nil {
				t.Fatal(err)
			}
			if !selected {
				t.Errorf("portable pool source %s excluded on %s", filename, target)
			}
		}
	}
}

// Holder registration consumes a retained Prefix, not a native storage owner.
func TestSuccessorRouteRegistrationRunsOnWindowsAndLinux(t *testing.T) {
	directory := path.Join(repositoryRoot(t), "internal/route/introduction")
	for _, filename := range []string{"registration.go", "registration_test.go"} {
		for _, target := range []string{"windows", "linux"} {
			profile := build.Default
			profile.GOOS, profile.GOARCH, profile.CgoEnabled = target, "amd64", false
			selected, err := profile.MatchFile(directory, filename)
			if err != nil {
				t.Fatal(err)
			}
			if !selected {
				t.Errorf("portable registration source %s excluded on %s", filename, target)
			}
		}
	}
}

// Durable selection owns native root leases, but retained authority checks,
// store behavior and recipient choices must execute, including their tests, on
// both platforms. The physical lease-corruption and foreign-uid controls stay
// native (installation_cleanup_linux_test.go, closed_set_open_cleanup_linux_test.go,
// root_hardening_linux_test.go, root_lease_identity_unix_test.go).
func TestSuccessorRouteSelectionRulesRunOnWindowsAndLinux(t *testing.T) {
	directory := path.Join(repositoryRoot(t), "internal/route/selection")
	for _, filename := range []string{"leg.go", "leg_test.go", "entry_set.go", "entry_set_test.go", "introduction_recipient.go", "rendezvous.go", "rendezvous_test.go", "closed_sets_test.go", "closed_adjacent_domains_test.go", "generation_store_test.go", "interior_store_test.go", "interior_refusal_test.go", "installation_test.go", "installation_history_test.go", "selection_owner_test.go"} {
		for _, target := range []string{"windows", "linux"} {
			profile := build.Default
			profile.GOOS, profile.GOARCH, profile.CgoEnabled = target, "amd64", false
			selected, err := profile.MatchFile(directory, filename)
			if err != nil {
				t.Fatal(err)
			}
			if !selected {
				t.Errorf("portable selection rule %s excluded on %s", filename, target)
			}
		}
	}
}

func TestSuccessorRouteIntroductionSnapshotRunsOnWindowsAndLinux(t *testing.T) {
	directory := path.Join(repositoryRoot(t), "internal/route/introduction")
	for _, filename := range []string{"slot_snapshot.go", "slot_snapshot_test.go", "slot_history.go", "slot_history_test.go", "registry.go", "receiving.go", "receiving_test.go"} {
		for _, target := range []string{"windows", "linux"} {
			profile := build.Default
			profile.GOOS, profile.GOARCH, profile.CgoEnabled = target, "amd64", false
			selected, err := profile.MatchFile(directory, filename)
			if err != nil {
				t.Fatal(err)
			}
			if !selected {
				t.Errorf("portable Introduction snapshot %s excluded on %s", filename, target)
			}
		}
	}
}

func TestSuccessorImportIsolation(t *testing.T) {
	root := repositoryRoot(t)
	walk(t, root, func(filename string, entry os.DirEntry) {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			return
		}
		relative := relativePath(t, root, filename)
		if !strings.HasPrefix(relative, "internal/") && !strings.HasPrefix(relative, "cmd/") && !strings.HasPrefix(relative, "tests/") && !strings.HasPrefix(relative, "scripts/") {
			return
		}
		file, err := parser.ParseFile(token.NewFileSet(), filename, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imported := range file.Imports {
			dependency, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if !successorImportAllowed(relative, dependency) {
				t.Errorf("successor isolation: %s imports forbidden dependency %s", relative, dependency)
			}
		}
	})
}

// This check is independently runnable with the new-domain regression. It
// checks actual implementation imports without executing legacy consumers.
func TestNetworkIntegrationImportIsolation(t *testing.T) {
	root := repositoryRoot(t)
	walk(t, root, func(filename string, entry os.DirEntry) {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			return
		}
		relative := relativePath(t, root, filename)
		if !isMigratedDomain(relative) && !strings.HasPrefix(relative, "internal/successor/") && !strings.HasPrefix(relative, "cmd/ardents-next/") && relative != "scripts/sign-qualification-release.go" && !offlineOperatorFile(relative) {
			return
		}
		file, err := parser.ParseFile(token.NewFileSet(), filename, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imported := range file.Imports {
			dependency, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if !successorImportAllowed(relative, dependency) {
				t.Errorf("new-domain isolation: %s imports forbidden dependency %s", relative, dependency)
			}
		}
	})
}

func isMigratedDomain(name string) bool {
	for _, root := range []string{
		"internal/admission", "internal/canonicalcursor", "internal/connection", "internal/custody", "internal/durablefile", "internal/endpoint",
		"internal/enrollment", "internal/execution", "internal/hosting", "internal/network", "internal/node",
		"internal/pathgrammar", "internal/platform/artifacts", "internal/platform/cgroup", "internal/platform/process", "internal/platform/systemd", "internal/processresource", "internal/publication",
		"internal/reachability", "internal/release", "internal/route",
	} {
		if name == root || strings.HasPrefix(name, root+"/") {
			return true
		}
	}
	return false
}

func successorImportAllowed(source, dependency string) bool {
	if offlineOperatorFile(source) {
		return offlineOperatorImportAllowed(source, dependency)
	}
	const retired = "internal/successor"
	const retiredSigner = "internal/nodeidentity"
	const retiredInstallation = "internal/platform/installation"
	local, localImport := strings.CutPrefix(dependency, modulePath+"/")
	for _, directory := range []string{retired, retiredSigner, retiredInstallation} {
		if strings.HasPrefix(source, directory+"/") || localImport && (local == directory || strings.HasPrefix(local, directory+"/")) {
			return false
		}
	}
	// These predecessor adapters must stay retired after their directory was
	// reused. They may not inherit the current Endpoint's domain imports.
	if localImport {
		switch source {
		case "internal/endpoint/runtime.go", "internal/endpoint/source_state.go", "internal/endpoint/worker_grant_linux.go":
			return false
		}
	}
	// Only the actual composition and its tests consume the Endpoint root.
	if strings.HasPrefix(source, "cmd/ardents-next/") && localImport && local == "internal/endpoint" {
		switch source {
		case "cmd/ardents-next/endpoint_construction_linux.go", "cmd/ardents-next/endpoint_construction_linux_test.go",
			"cmd/ardents-next/endpoint_history_linux.go", "cmd/ardents-next/endpoint_history_linux_test.go",
			"cmd/ardents-next/installed_endpoint_linux.go", "cmd/ardents-next/source_preparation_linux.go",
			"cmd/ardents-next/source_preparation_fixture_linux_test.go", "cmd/ardents-next/source_preparation_native_linux_test.go",
			"cmd/ardents-next/installed_source_linux_test.go", "cmd/ardents-next/source_preparation_process_linux_test.go":
			return true
		default:
			return false
		}
	}
	switch source {
	case "internal/diagnostics/process/heapdump_role_map_capture_test.go":
		if dependency == modulePath+"/internal/admission" {
			return true
		}
		fallthrough
	case "internal/diagnostics/process/heapdump_parser_test.go", "internal/diagnostics/process/heapdump_capture_test.go":
		first, _, _ := strings.Cut(dependency, "/")
		return dependency != "C" && dependency != "unsafe" && dependency != "" && !strings.Contains(first, ".") && path.Clean(dependency) == dependency && !strings.HasPrefix(dependency, "/")
	}
	if source == "cmd/ardents-next/connection_custody_terminal_installed_linux_test.go" && (dependency == "github.com/aymanbagabas/go-pty" || dependency == "golang.org/x/sys/unix") {
		return true
	}
	if source == "scripts/sign-qualification-release.go" {
		switch dependency {
		case modulePath + "/internal/enrollment", modulePath + "/internal/release",
			"github.com/sigstore/sigstore/pkg/signature", "github.com/theupdateframework/go-tuf/v2/metadata", "golang.org/x/sys/unix":
			return true
		}
		first, _, _ := strings.Cut(dependency, "/")
		return dependency != "C" && dependency != "unsafe" && dependency != "" && !strings.Contains(first, ".") && path.Clean(dependency) == dependency && !strings.HasPrefix(dependency, "/")
	}
	if source == "cmd/ardents-next/enrollment_packaging.go" && localImport {
		return local == "internal/enrollment"
	}
	if source == "cmd/ardents-next/platform_release_linux_test.go" && (dependency == modulePath+"/internal/network/duty" || dependency == modulePath+"/tests/epochfixture/network" || dependency == "github.com/theupdateframework/go-tuf/v2/metadata") {
		return true
	}
	if source == "cmd/ardents-next/source_lifecycle_native_linux_test.go" && dependency == "github.com/theupdateframework/go-tuf/v2/metadata" {
		return true
	}
	if dependency == modulePath+"/internal/platform/systemd" {
		switch source {
		}
	}
	if descriptorStartupImportAllowed(source, dependency) || descriptorInitialImportAllowed(source, dependency) {
		return true
	}
	if joinRetainedImportAllowed(source, dependency) {
		return true
	}
	inZone := isMigratedDomain(source) || strings.HasPrefix(source, "cmd/ardents-next/")
	zoneDependency := localImport && isMigratedDomain(local)
	if source == "internal/route/prefix/prefix.go" && dependency == modulePath+"/internal/network/state" {
		return true
	}
	if source == "cmd/ardents-next/platform_release_linux_test.go" && (dependency == modulePath+"/internal/enrollment" || dependency == modulePath+"/internal/release" || dependency == modulePath+"/internal/release/infrastructure") {
		return true
	}
	switch source {
	case "cmd/ardents-next/platform_endpoint_staging_linux.go":
		if local == "internal/endpoint/launch" || local == "internal/release" {
			return true
		}
	case "cmd/ardents-next/platform_endpoint_linux.go":
		if local == "internal/endpoint/launch" || local == "internal/endpoint/plan" || local == "internal/platform/artifacts" {
			return true
		}
	case "cmd/ardents-next/platform_endpoint_check_test.go",
		"cmd/ardents-next/admission_console.go", "cmd/ardents-next/hosting_input.go",
		"internal/endpoint/launch/platform_delivery_linux.go", "internal/endpoint/launch/platform_staging_linux.go":
		if local == "internal/platform/artifacts" {
			return true
		}
	case "cmd/ardents-next/platform_linux.go":
		if local == "internal/release/infrastructure/launch" || local == "internal/platform/artifacts" {
			return true
		}
	case "cmd/ardents-next/platform_linux_test.go", "cmd/ardents-next/repair_linux.go":
		if local == "internal/release/infrastructure/launch" {
			return true
		}
	case "cmd/ardents-next/endpoint_platform_delivery_native_linux_test.go":
		if local == "internal/endpoint/plan" {
			return true
		}
	case "cmd/ardents-next/release.go", "cmd/ardents-next/release_process_test.go":
		if local == "internal/release/endpoint" {
			return true
		}
	case "cmd/ardents-next/platform.go":
		if local == "internal/platform/systemd" {
			return true
		}
	case "internal/endpoint/plan/platform_launch.go", "internal/endpoint/unit/platform_rendering.go":
		if local == "internal/pathgrammar" {
			return true
		}
	}
	if source == "cmd/ardents-next/endpoint_platform_native_linux_test.go" && (dependency == modulePath+"/internal/enrollment" || dependency == modulePath+"/internal/release/endpoint" || dependency == modulePath+"/internal/endpoint/plan" || dependency == modulePath+"/internal/endpoint/unit" || dependency == modulePath+"/internal/release") {
		return true
	}
	if source == "cmd/ardents-next/endpoint_launch_native_linux_test.go" && (dependency == modulePath+"/internal/network/source" || dependency == modulePath+"/internal/platform/systemd" || dependency == modulePath+"/internal/platform/cgroup") {
		return true
	}
	if source == "cmd/ardents-next/endpoint_journey_native_linux_test.go" && (dependency == modulePath+"/internal/network/source" || dependency == modulePath+"/internal/route/transport" || dependency == modulePath+"/internal/publication") {
		return true
	}
	if source == "cmd/ardents-next/endpoint_replacement_worker_native_linux_test.go" && dependency == modulePath+"/internal/route/transport" {
		return true
	}
	// The cold/warm latency root driver (#650) names the selected Carrier for the
	// shared publication Route network exactly as the journey controller does.
	if source == "cmd/ardents-next/endpoint_latency_native_linux_test.go" && dependency == modulePath+"/internal/route/transport" {
		return true
	}
	if source == "internal/network/state/source_repair_linux.go" && (dependency == modulePath+"/internal/platform/systemd" || dependency == modulePath+"/internal/platform/cgroup" || dependency == modulePath+"/internal/platform/process") {
		return true
	}
	if (source == "internal/network/state/introduction_repair_linux.go" || source == "internal/network/state/forwarding_repair_linux.go" || source == "internal/network/state/join_repair_linux.go" || source == "internal/network/state/descriptor_repair_linux.go") && (dependency == modulePath+"/internal/platform/systemd" || dependency == modulePath+"/internal/platform/cgroup") {
		return true
	}
	// The one shared node-duty repair skeleton consumes the detached manager
	// property vocabulary only; each role wrapper supplies its own per-unit
	// cgroup quiescence closure (W4, #711).
	if source == "internal/network/state/node_duty_repair_linux.go" && dependency == modulePath+"/internal/platform/systemd" {
		return true
	}
	// The bounded Descriptor repair additionally clears an uncertain Store stage
	// through the existing-only reachability mechanism under the same dead-unit
	// proofs and retained leases; it grants no publication or runtime authority.
	if source == "internal/network/state/descriptor_repair_linux.go" && dependency == modulePath+"/internal/reachability" {
		return true
	}
	if (source == "internal/network/state/source_repair_linux_test.go" || source == "internal/network/state/node_duty_repair_linux_test.go") && dependency == modulePath+"/internal/platform/systemd" {
		return true
	}

	// Network's physical/application adapters consume domain policy through
	// exact files. Directory nesting does not grant policy or Admission access.
	if strings.HasPrefix(source, "internal/network/") {
		sourceAdapter := source
		if sourceAdapter == "internal/network/closedprofile/token_spki.go" && dependency == modulePath+"/internal/admission/issuerprofile" {
			return true
		}
		if sourceAdapter == "internal/network/duty/participation_policy.go" && dependency == modulePath+"/internal/network" {
			return true
		}
		if dependency == modulePath+"/internal/network" {
			switch sourceAdapter {
			case "internal/network/state/accepted_observation.go":
				return inZone
			case "internal/network/state/closed_profile_role_join_test.go", "internal/network/state/closed_runtime_view_test.go":
				return inZone
			case "internal/network/state/acquisition_history.go":
				return true
			case "internal/network/epoch/candidate_evaluation.go", "internal/network/epoch/assignment.go":
				return true
			case "internal/network/state/epoch_history.go", "internal/network/state/membership.go", "internal/network/state/closed_member.go", "internal/network/state/closed_runtime_view.go", "internal/network/state/runtime_observation.go", "internal/network/state/runtime_witness.go", "internal/network/state/clock_observation.go", "internal/network/state/closed_profile_view.go", "internal/network/state/closed_duty_binding.go", "internal/network/state/closed_profile_accept.go":
				return true
			}
		}
	}
	if source == "internal/custody/canonical_envelope.go" && dependency == "golang.org/x/crypto/argon2" {
		return true
	}
	if (source == "internal/custody/vault_lock_windows.go" || source == "internal/custody/vault_lock_windows_test.go" || source == "internal/custody/durability_windows.go") && dependency == "golang.org/x/sys/windows" {
		return true
	}
	if !inZone {
		if (source == "cmd/ardents-custody/main_test.go" || source == "tests/e2e/service/service_custody_process_linux_test.go" || source == "tests/e2e/service/headless_instance_acquisition_process_test.go") && dependency == modulePath+"/internal/publication" {
			return true
		}
		if dependency == modulePath+"/internal/custody" {
			switch source {
			case "cmd/ardents-custody/command.go", "cmd/ardents-custody/terminal_secret.go", "cmd/ardents-custody/main_test.go", "cmd/ardents-custody/service_authority.go", "cmd/ardents-custody/admission_authority.go", "cmd/ardents-custody/verify_record.go", "cmd/ardents-custody/recovery_bundle.go", "cmd/ardents-custody/purge_record.go", "cmd/ardents-custody/admission_permission_client_linux_test.go", "internal/endpoint/permission_test.go", "internal/endpoint/issuance_test.go", "internal/endpoint/permission_files_linux_test.go", "internal/endpoint/network_fixture_window_test.go":
				return true
			}
		}
		if (source == "cmd/ardents-custody/admission_permission_client_linux_test.go" || source == "tests/e2e/service/admission_custody_process_test.go" || source == "internal/endpoint/permission_request_fixture_test.go" || source == "internal/admission/stock/permission_request_fixture_test.go" || source == "internal/admission/stock/permission_view_test.go") && dependency == modulePath+"/internal/admission" {
			return true
		}
		return !zoneDependency
	}
	if source == "internal/route/introduction/receiving_history_linux.go" && dependency == modulePath+"/internal/admission/receiving" {
		return true
	}
	if source == "internal/admission/issuance/result_declaration.go" && dependency == modulePath+"/internal/platform/artifacts" {
		return true
	}
	if source == "internal/release/infrastructure/launch/verify_linux_test.go" && dependency == modulePath+"/internal/processresource" {
		return true
	}
	if dependency == modulePath+"/internal/node/provisioning" {
		switch source {
		case "cmd/ardents-next/network.go", "cmd/ardents-next/network_preparation_test.go",
			"cmd/ardents-next/network_preparation_chain_linux_test.go", "cmd/ardents-next/network_preparation_receipt_linux_test.go",
			"cmd/ardents-next/network_signing_fixture_linux_test.go", "cmd/ardents-next/network_signing_native_linux_test.go",
			"cmd/ardents-next/network_profile_native_linux_test.go", "cmd/ardents-next/node_certificate_native_linux_test.go":
			return true
		}
	}
	if source == "cmd/ardents-next/network_signing_fixture_linux_test.go" && (dependency == modulePath+"/internal/network/closedprofile" || dependency == modulePath+"/tests/epochfixture/network") {
		return true
	}
	if (source == "cmd/ardents-next/source_preparation_process_linux_test.go" || source == "cmd/ardents-next/node_certificate_process_linux_test.go") && dependency == modulePath+"/internal/platform/artifacts" {
		return true
	}
	// Match exact Go packages. A parent folder grants no child/sibling imports.
	owner := path.Dir(source)
	permitted := map[string][]string{
		"internal/processresource":                   {},
		"internal/canonicalcursor":                   {},
		"internal/durablefile":                       {},
		"internal/pathgrammar":                       {},
		"internal/node":                              {"admission", "admission/receiving", "hosting", "network", "network/duty", "pathgrammar", "processresource", "route/ardp", "route/receiver", "route/role"},
		"internal/network":                           {},
		"internal/network/closedprofile":             {"canonicalcursor"},
		"internal/network/duty":                      {"durablefile", "pathgrammar"},
		"internal/network/epoch":                     {"canonicalcursor"},
		"internal/network/source":                    {"canonicalcursor", "network/epoch"},
		"internal/network/source/serving":            {"network/source/servingdeclaration", "network/state", "pathgrammar", "platform/artifacts", "processresource"},
		"internal/network/source/servingdeclaration": {},
		"internal/network/state":                     {"canonicalcursor", "network/closedprofile", "network/duty", "network/epoch", "network/source", "network/source/servingdeclaration", "network/state/durable", "pathgrammar"},
		"internal/network/state/durable":             {"durablefile", "pathgrammar"},
		"internal/admission":                         {"admission/issuerprofile"},
		"internal/admission/issuerprofile":           {},
		"internal/admission/allocation":              {"admission"},
		"internal/custody":                           {"admission", "admission/allocation", "publication"},
		"internal/admission/attempts":                {"pathgrammar"},
		"internal/admission/spending":                {"pathgrammar"},
		"internal/admission/token":                   {"admission", "admission/issuerprofile"},
		"internal/admission/receiving":               {"admission", "admission/token", "admission/spending"},
		"internal/admission/stock":                   {"admission", "admission/allocation", "admission/attempts", "admission/token"},
		"internal/admission/quota":                   {"admission", "admission/issuerprofile", "pathgrammar"},
		"internal/admission/issuance":                {"admission", "admission/quota", "admission/issuerprofile", "admission/issuerprofile/signing", "pathgrammar"},
		"internal/admission/issuer":                  {"admission", "admission/quota", "admission/issuance", "admission/issuerprofile/signing", "pathgrammar"},
		"internal/admission/issuerprofile/signing":   {"admission/issuerprofile", "pathgrammar"},
		"internal/hosting":                           {"pathgrammar"},
		"internal/publication":                       {},
		"internal/publication/durable":               {"publication"},
		"internal/publication/instance":              {"publication", "publication/durable", "reachability", "route/introduction", "route/capsule", "route/join", "connection/wire", "execution/runtime"},
		"internal/publication/runtime":               {"connection", "execution/runtime", "network", "publication", "publication/instance", "reachability", "route", "route/introduction", "route/prefix", "route/capsule", "route/join"},
		"internal/connection":                        {"execution/runtime", "publication", "publication/instance", "route/capsule", "route/join", "connection/wire"},
		"internal/connection/wire":                   {},
		"internal/reachability":                      {"publication", "pathgrammar"},
		"internal/enrollment":                        {"pathgrammar"},
		"internal/release":                           {"durablefile", "pathgrammar"},
		"internal/execution":                         {},
		"internal/execution/runtime":                 {"execution", "execution/worker"},
		"internal/execution/worker":                  {"platform/systemd", "endpoint/unit", "pathgrammar"},
		"internal/release/endpoint":                  {"enrollment", "release"},
		"internal/release/infrastructure":            {"enrollment", "release", "pathgrammar"},
		"internal/release/infrastructure/launch":     {"admission/issuance", "enrollment", "network/source/serving", "network/source/servingdeclaration", "network/state", "node", "pathgrammar", "platform/artifacts", "platform/process", "platform/systemd", "release", "release/infrastructure"},
		"internal/endpoint/launch":                   {"release/endpoint", "enrollment", "release", "platform/cgroup", "platform/systemd", "platform/process", "endpoint/generation", "endpoint/unit", "endpoint/plan"},
		"internal/platform/cgroup":                   {"pathgrammar"},
		"internal/platform/artifacts":                {"pathgrammar"},
		"internal/endpoint/plan":                     {},

		"internal/platform/systemd":    {},
		"internal/platform/process":    {"pathgrammar"},
		"internal/endpoint/generation": {"pathgrammar"},
		"internal/endpoint/unit":       {"platform/systemd"},
		"internal/route":               {},
		"internal/route/ardp":          {},
		"internal/route/issuer":        {"route/ardp", "admission"},
		"internal/route/transport":     {},
		"internal/route/channel":       {"route/ardp", "route/transport"},
		"internal/route/bootstrap":     {"route/ardp", "route/channel"},
		"internal/route/transport/tls": {"route/transport"},

		"internal/route/introduction": {"admission/spending", "route", "route/ardp", "route/role", "route/prefix", "route/capsule", "network", "admission", "pathgrammar"},
		"internal/route/capsule":      {},
		"internal/route/selection":    {"route", "network", "durablefile"},
		"internal/route/role":         {"route", "route/ardp", "route/channel", "route/transport", "network", "admission"},
		"internal/route/join":         {"route/prefix", "route", "route/selection", "route/ardp", "route/channel", "route/role", "route/transport", "network", "admission"},
		"internal/route/join/pairing": {"route/ardp", "route/channel", "route/role"},
		"internal/route/prefix":       {"route/issuer", "route", "route/selection", "route/ardp", "route/bootstrap", "route/channel", "route/role", "route/transport", "route/transport/tls", "network", "admission"},
		"internal/route/receiver":     {"route/issuer", "route/join/pairing", "route", "route/role", "route/transport", "route/transport/tls", "route/channel", "route/bootstrap", "route/ardp", "route/introduction", "network", "admission", "admission/receiving"},
		"internal/endpoint/session":   {"admission", "admission/stock", "endpoint/funding", "endpoint/publisher", "execution/runtime", "hosting", "reachability", "route/ardp", "route/bootstrap", "route/capsule", "route/introduction", "route/join", "route/prefix", "route/role", "route/selection", "pathgrammar"},
		"internal/endpoint":           {"endpoint/funding", "endpoint/reader", "endpoint/publisher", "admission", "admission/stock", "execution", "execution/runtime", "hosting", "endpoint/launch", "network", "network/source", "network/state", "pathgrammar"},
		"internal/endpoint/publisher": {"endpoint/funding", "admission", "admission/stock", "execution/runtime", "hosting", "network", "publication/durable", "publication/instance", "publication/runtime", "route/prefix", "route/selection", "pathgrammar"},
		"internal/endpoint/reader":    {"endpoint/funding", "admission", "admission/stock", "connection", "execution/runtime", "hosting", "network", "reachability", "route/prefix", "route/selection"},
		"internal/endpoint/funding":   {"admission", "admission/stock", "admission/token", "hosting", "network", "network/state", "route", "route/ardp", "route/bootstrap", "route/prefix", "route/role", "route/selection", "pathgrammar"},
		"cmd/ardents-next":            {"endpoint/session", "endpoint", "endpoint/publisher", "endpoint/reader", "endpoint/funding", "route/introduction", "route/prefix", "network", "network/state", "admission/stock", "admission/receiving", "admission/allocation", "admission", "admission/quota", "admission/issuerprofile", "admission/issuance", "admission/issuer", "admission/issuerprofile/signing", "hosting", "route", "route/role", "route/selection", "route/join", "route/receiver", "route/channel", "route/ardp"},
	}
	if zoneDependency {
		if (source == "internal/endpoint/history_preparation_linux.go" || source == "internal/endpoint/history_preparation_linux_test.go") && dependency == modulePath+"/internal/route/selection" {
			return true
		}
		if (source == "internal/endpoint/history_preparation_linux.go" || source == "internal/endpoint/history_preparation_linux_test.go") && dependency == modulePath+"/internal/network/duty" {
			return true
		}
		if source == "internal/endpoint/history_preparation_linux_test.go" && dependency == modulePath+"/internal/admission/attempts" {
			return true
		}
		if (source == "cmd/ardents-next/endpoint_history_linux.go" || source == "cmd/ardents-next/endpoint_history_linux_test.go" || source == "cmd/ardents-next/installed_startup_linux_test.go") && dependency == modulePath+"/internal/network/source" {
			return true
		}
		if source == "cmd/ardents-next/route_composition_linux_test.go" && dependency == modulePath+"/internal/reachability" {
			return true
		}
		if source == "cmd/ardents-next/source_state_linux_test.go" && (dependency == modulePath+"/internal/network/source" || dependency == modulePath+"/internal/network/duty") {
			return true
		}
		if (source == "cmd/ardents-next/source_linux.go" || source == "cmd/ardents-next/source_serving_fixture_linux_test.go") && dependency == modulePath+"/internal/network/source/serving" {
			return true
		}
		if source == "cmd/ardents-next/source_serving_fixture_linux_test.go" && dependency == modulePath+"/internal/network/state" {
			return true
		}
		if source == "cmd/ardents-next/source_linux.go" && dependency == modulePath+"/internal/processresource" {
			return true
		}
		if dependency == modulePath+"/internal/platform/artifacts" {
			switch source {
			case "cmd/ardents-next/inputs.go", "cmd/ardents-next/source_serving_fixture_linux_test.go", "cmd/ardents-next/route_linux.go",
				"cmd/ardents-next/source_linux.go", "cmd/ardents-next/source_preparation_linux.go",
				"internal/network/state/source_state_files_linux_test.go",
				"cmd/ardents-next/network_signing_native_linux_test.go",
				"cmd/ardents-next/infrastructure_preparation_native_linux_test.go", "cmd/ardents-next/source_preparation_native_linux_test.go":
				return true
			}
		}
		if source == "cmd/ardents-next/source_native_linux_test.go" && (dependency == modulePath+"/internal/network/source" || dependency == modulePath+"/internal/network/epoch" || dependency == modulePath+"/internal/processresource") {
			return true
		}
		if source == "cmd/ardents-next/source_lifecycle_native_linux_test.go" && (dependency == modulePath+"/internal/enrollment" || dependency == modulePath+"/internal/release/infrastructure" || dependency == modulePath+"/internal/network/duty" || dependency == modulePath+"/internal/network/epoch" || dependency == modulePath+"/internal/network/source" || dependency == modulePath+"/internal/processresource" || dependency == modulePath+"/internal/release") {
			return true
		}
		if source == "cmd/ardents-next/source_installed_native_linux_test.go" && (dependency == modulePath+"/internal/release/infrastructure" || dependency == modulePath+"/internal/network/duty" || dependency == modulePath+"/internal/network/epoch" || dependency == modulePath+"/internal/network/source") {
			return true
		}
		if source == "cmd/ardents-next/forwarding_platform_native_linux_test.go" && (dependency == modulePath+"/internal/release/infrastructure" || dependency == modulePath+"/internal/network/duty" || dependency == modulePath+"/internal/processresource" || dependency == modulePath+"/internal/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/join_platform_native_linux_test.go" && (dependency == modulePath+"/internal/release/infrastructure" || dependency == modulePath+"/internal/network/duty" || dependency == modulePath+"/internal/processresource" || dependency == modulePath+"/internal/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/descriptor_platform_native_linux_test.go" && (dependency == modulePath+"/internal/release/infrastructure" || dependency == modulePath+"/internal/network/duty" || dependency == modulePath+"/internal/processresource" || dependency == modulePath+"/internal/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/source_preparation_process_linux_test.go" && dependency == modulePath+"/internal/network/source" {
			return true
		}
		if source == "cmd/ardents-next/node_operation_native_linux_test.go" && (dependency == modulePath+"/internal/node" || dependency == modulePath+"/internal/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/node_join_startup_native_linux_test.go" && (dependency == modulePath+"/internal/processresource" || dependency == modulePath+"/internal/network/duty" || dependency == modulePath+"/internal/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/node_roles_linux.go" && (dependency == modulePath+"/internal/node" || dependency == modulePath+"/internal/processresource") {
			return true
		}
		if source == "cmd/ardents-next/node_join_native_linux_test.go" && (dependency == modulePath+"/internal/processresource" || dependency == modulePath+"/internal/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/node_authority_native_linux_test.go" && (dependency == modulePath+"/internal/node" || dependency == modulePath+"/internal/network/duty" || dependency == modulePath+"/internal/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/node_completion_native_linux_test.go" && (dependency == modulePath+"/internal/network/duty" || dependency == modulePath+"/internal/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/node_operation_native_linux_test.go" && (dependency == modulePath+"/internal/node" || dependency == modulePath+"/internal/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/node_authority_native_linux_test.go" && (dependency == modulePath+"/internal/node" || dependency == modulePath+"/internal/network/duty" || dependency == modulePath+"/internal/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/node_completion_native_linux_test.go" && (dependency == modulePath+"/internal/network/duty" || dependency == modulePath+"/internal/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/node_introduction_native_linux_test.go" && (dependency == modulePath+"/internal/network/duty" || dependency == modulePath+"/internal/processresource" || dependency == modulePath+"/internal/route/introduction" || dependency == modulePath+"/internal/route/capsule" || dependency == modulePath+"/internal/route/transport" || dependency == modulePath+"/internal/route/receiver" || dependency == modulePath+"/internal/route/role" || dependency == modulePath+"/internal/route/selection") {
			return true
		}
		if source == "cmd/ardents-next/node_completion_native_linux_test.go" && (dependency == modulePath+"/internal/network/duty" || dependency == modulePath+"/internal/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/node_operation_native_linux_test.go" && (dependency == modulePath+"/internal/node" || dependency == modulePath+"/internal/reachability" || dependency == modulePath+"/internal/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/node_authority_native_linux_test.go" && (dependency == modulePath+"/internal/node" || dependency == modulePath+"/internal/network/duty" || dependency == modulePath+"/internal/reachability" || dependency == modulePath+"/internal/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/node_descriptor_native_linux_test.go" && (dependency == modulePath+"/internal/processresource" || dependency == modulePath+"/internal/network/duty" || dependency == modulePath+"/internal/route/transport" || dependency == modulePath+"/internal/reachability") {
			return true
		}
		if source == "cmd/ardents-next/node_operation_native_linux_test.go" && (dependency == modulePath+"/internal/node" || dependency == modulePath+"/internal/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/node_completion_native_linux_test.go" && (dependency == modulePath+"/internal/network/duty" || dependency == modulePath+"/internal/processresource" || dependency == modulePath+"/internal/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/node_issuer_startup_native_linux_test.go" && (dependency == modulePath+"/internal/network/duty" || dependency == modulePath+"/internal/processresource" || dependency == modulePath+"/internal/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/node_authority_native_linux_test.go" && (dependency == modulePath+"/internal/node" || dependency == modulePath+"/internal/network/duty" || dependency == modulePath+"/internal/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/node_issuer_native_linux_test.go" && (dependency == modulePath+"/internal/processresource" || dependency == modulePath+"/internal/route/transport") {
			return true
		}
		if source == "internal/node/issuer_linux.go" && (dependency == modulePath+"/internal/admission/issuer" || dependency == modulePath+"/internal/admission/quota" || dependency == modulePath+"/internal/route/bootstrap") {
			return true
		}
		if source == "internal/node/introduction_linux.go" && dependency == modulePath+"/internal/route/introduction" {
			return true
		}
		if source == "internal/node/descriptor_linux.go" && dependency == modulePath+"/internal/reachability" {
			return true
		}
		if source == "cmd/ardents-next/node_lifecycle_linux.go" && dependency == modulePath+"/internal/node" {
			return true
		}
		if source == "cmd/ardents-next/node_failure_linux.go" && dependency == modulePath+"/internal/node" {
			return true
		}
		if source == "cmd/ardents-next/node_process.go" && dependency == modulePath+"/internal/processresource" {
			return true
		}
		if (source == "cmd/ardents-next/node_native_linux_test.go" || source == "cmd/ardents-next/forwarding_lifecycle_native_linux_test.go" || source == "cmd/ardents-next/join_lifecycle_native_linux_test.go" || source == "cmd/ardents-next/descriptor_lifecycle_native_linux_test.go") && (dependency == modulePath+"/internal/processresource" || dependency == modulePath+"/internal/network/duty" || dependency == modulePath+"/internal/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/node_refill_native_linux_test.go" && (dependency == modulePath+"/internal/processresource" || dependency == modulePath+"/internal/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/node_authority_native_linux_test.go" && (dependency == modulePath+"/internal/node" || dependency == modulePath+"/internal/network/duty" || dependency == modulePath+"/internal/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/node_completion_native_linux_test.go" && (dependency == modulePath+"/internal/processresource" || dependency == modulePath+"/internal/network/duty" || dependency == modulePath+"/internal/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/document_command_installed_linux_test.go" {
			switch dependency {
			case modulePath + "/internal/execution",
				modulePath + "/internal/execution/runtime",
				modulePath + "/internal/publication",
				modulePath + "/internal/reachability",
				modulePath + "/internal/route/transport":
				return true
			}
		}
		if source == "cmd/ardents-next/connection_custody_instance_installed_linux_test.go" && dependency == modulePath+"/internal/publication" {
			return true
		}
		if source == "cmd/ardents-next/connection_accepted_request_installed_linux_test.go" && (dependency == modulePath+"/internal/connection" || dependency == modulePath+"/internal/execution/runtime" || dependency == modulePath+"/internal/execution/worker" || dependency == modulePath+"/internal/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/connection_response_loss_linux_test.go" && (dependency == modulePath+"/internal/connection" || dependency == modulePath+"/internal/execution/runtime" || dependency == modulePath+"/internal/publication/runtime" || dependency == modulePath+"/internal/route/transport") {
			return true
		}
		if source == "internal/publication/runtime/publisher_streams.go" && dependency == modulePath+"/internal/connection/wire" {
			return true
		}
		if source == "cmd/ardents-next/connection_installed_linux_test.go" && (dependency == modulePath+"/internal/connection" || dependency == modulePath+"/internal/execution" || dependency == modulePath+"/internal/execution/runtime" || dependency == modulePath+"/internal/publication/runtime" || dependency == modulePath+"/internal/reachability" || dependency == modulePath+"/internal/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/connection_reopen_linux_test.go" && (dependency == modulePath+"/internal/publication/durable" || dependency == modulePath+"/internal/publication/instance" || dependency == modulePath+"/internal/reachability" || dependency == modulePath+"/internal/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/connection_authority_linux_test.go" && (dependency == modulePath+"/internal/connection" || dependency == modulePath+"/internal/execution/runtime" || dependency == modulePath+"/internal/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/publication_network_linux_test.go" && (dependency == modulePath+"/internal/network/epoch" || dependency == modulePath+"/internal/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/publication_ack_installed_linux_test.go" && (dependency == modulePath+"/internal/execution" || dependency == modulePath+"/internal/execution/runtime" || dependency == modulePath+"/internal/publication" || dependency == modulePath+"/internal/reachability" || dependency == modulePath+"/internal/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/installed_service_linux_test.go" && (dependency == modulePath+"/internal/publication" || dependency == modulePath+"/internal/reachability" || dependency == modulePath+"/internal/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/publication_installed_linux_test.go" && (dependency == modulePath+"/internal/publication" || dependency == modulePath+"/internal/publication/instance" || dependency == modulePath+"/internal/reachability" || dependency == modulePath+"/internal/route/transport" || dependency == modulePath+"/internal/route/capsule") {
			return true
		}
		if source == "cmd/ardents-next/publication_registration_linux_test.go" && (dependency == modulePath+"/internal/publication" || dependency == modulePath+"/internal/publication/durable" || dependency == modulePath+"/internal/publication/instance" || dependency == modulePath+"/internal/reachability" || dependency == modulePath+"/internal/route/capsule") {
			return true
		}
		if (source == "cmd/ardents-next/route_linux.go" || source == "cmd/ardents-next/route_submission_linux_test.go") && dependency == modulePath+"/internal/route/capsule" {
			return true
		}
		if source == "cmd/ardents-next/publication.go" && (dependency == modulePath+"/internal/publication/durable" || dependency == modulePath+"/internal/publication/instance") {
			return true
		}
		if source == "internal/endpoint/participant_linux.go" && (dependency == modulePath+"/internal/endpoint/launch" || dependency == modulePath+"/internal/execution" || dependency == modulePath+"/internal/execution/runtime") {
			return true
		}
		if source == "cmd/ardents-next/node.go" && dependency == modulePath+"/internal/node/provisioning" {
			return true
		}
		if source == "internal/endpoint/declarations_linux.go" && (dependency == modulePath+"/internal/execution" || dependency == modulePath+"/internal/network/source") {
			return true
		}
		if (source == "cmd/ardents-next/execution_holder.go" || source == "cmd/ardents-next/admission_holder.go") && dependency == modulePath+"/internal/execution/runtime" {
			return true
		}
		if source == "cmd/ardents-next/execution_holder.go" && dependency == modulePath+"/internal/execution" {
			return true
		}
		if source == "cmd/ardents-next/execution_artifact_linux_test.go" && (dependency == modulePath+"/internal/release/endpoint" || dependency == modulePath+"/internal/enrollment" || dependency == modulePath+"/internal/release") {
			return true
		}
		if (source == "cmd/ardents-next/installation_test.go") && (dependency == modulePath+"/internal/release/endpoint" || dependency == modulePath+"/internal/endpoint/launch" || dependency == modulePath+"/internal/enrollment" || dependency == modulePath+"/internal/release") {
			return true
		}
		if source == "cmd/ardents-next/infrastructure_linux.go" {
			return dependency == modulePath+"/internal/release/infrastructure" || dependency == modulePath+"/internal/release"
		}
		if (source == "cmd/ardents-next/infrastructure_linux_test.go" || source == "cmd/ardents-next/infrastructure_caller_linux_test.go" || source == "cmd/ardents-next/infrastructure_preflight_native_linux_test.go" || source == "cmd/ardents-next/infrastructure_stopped_native_linux_test.go") && (dependency == modulePath+"/internal/release/infrastructure" || dependency == modulePath+"/internal/enrollment" || dependency == modulePath+"/internal/release") {
			return true
		}
		if source == "cmd/ardents-next/forwarding_artifact_linux_test.go" && (dependency == modulePath+"/internal/release/infrastructure" || dependency == modulePath+"/internal/enrollment" || dependency == modulePath+"/internal/release") {
			return true
		}
		if source == "cmd/ardents-next/release_process_test.go" && (dependency == modulePath+"/internal/enrollment" || dependency == modulePath+"/internal/release") {
			return true
		}
		if dependency == modulePath+"/internal/enrollment" && (source == "cmd/ardents-next/enrollment.go" || source == "cmd/ardents-next/release.go") {
			return true
		}
		if dependency == modulePath+"/internal/release" && source == "cmd/ardents-next/release.go" {
			return true
		}
		if source == "cmd/ardents-next/route_bootstrap_linux_test.go" && (dependency == modulePath+"/internal/route/issuer" || dependency == modulePath+"/internal/route/bootstrap") {
			return true
		}
		if source == "cmd/ardents-next/route_fixture_linux_test.go" && (dependency == modulePath+"/internal/route/transport/tls") {
			return true
		}
		if dependency == modulePath+"/internal/route/transport" {
			if source == "cmd/ardents-next/route_token_linux_test.go" {
				return true
			}
			if source == "cmd/ardents-next/route_submission_linux_test.go" {
				return true
			}
			if source == "cmd/ardents-next/execution_installed_linux_test.go" {
				return true
			}
			if source == "cmd/ardents-next/route_descriptor_linux_test.go" {
				return true
			}
			if source == "cmd/ardents-next/route_bootstrap_linux_test.go" {
				return true
			}
			switch source {
			case "cmd/ardents-next/route_refill_linux_test.go", "cmd/ardents-next/route_fixture_linux_test.go", "cmd/ardents-next/route_prefix_linux_test.go", "cmd/ardents-next/route_prefix_caller_linux_test.go", "cmd/ardents-next/route_process_linux_test.go", "cmd/ardents-next/route_refusal_linux_test.go", "cmd/ardents-next/route_registration_linux_test.go", "cmd/ardents-next/route_join_fixture_linux_test.go", "cmd/ardents-next/route_join_linux_test.go", "cmd/ardents-next/route_join_process_linux_test.go", "cmd/ardents-next/route_close_failure_linux_test.go", "cmd/ardents-next/route_join_lifecycle_linux_test.go":
				return true
			}
		}
		if source == "cmd/ardents-next/network_preparation_chain_linux_test.go" && (dependency == modulePath+"/internal/network/epoch" || dependency == modulePath+"/internal/network/closedprofile") {
			return true
		}
		if source == "cmd/ardents-next/network_profile_signing_test.go" && dependency == modulePath+"/internal/network/closedprofile" {
			return true
		}
		if source == "cmd/ardents-next/network_admission_fixture_linux_test.go" && dependency == modulePath+"/internal/network/epoch" {
			return true
		}
		if owner == "cmd/ardents-next" && strings.HasSuffix(source, "_test.go") && (dependency == modulePath+"/internal/admission/token" || dependency == modulePath+"/internal/admission/spending") {
			return true
		}
		if source == "cmd/ardents-next/route_issuer_linux.go" && dependency == modulePath+"/internal/route/bootstrap" {
			return true
		}
		if dependency == modulePath+"/internal/reachability" && (source == "internal/route/prefix/descriptor_exchange.go" || source == "internal/route/receiver/receiver_linux.go" || source == "internal/route/receiver/descriptor_linux.go" || source == "cmd/ardents-next/route_linux.go" || source == "cmd/ardents-next/route.go" || source == "cmd/ardents-next/admission_holder.go" || source == "cmd/ardents-next/route_descriptor_linux_test.go" || source == "cmd/ardents-next/descriptor_lifecycle_native_linux_test.go" || source == "cmd/ardents-next/descriptor_platform_native_linux_test.go") {
			return true
		}
		if (source == "internal/admission/stock/issuance_fixture_test.go" || source == "internal/admission/stock/lifecycle_test.go" || source == "internal/admission/stock/receiving_cycle_test.go") && (dependency == modulePath+"/internal/admission/issuer" || dependency == modulePath+"/internal/admission/issuance" || dependency == modulePath+"/internal/admission/quota") {
			return true
		}
		if (source == "internal/admission/stock/cycle_test.go" || source == "internal/admission/stock/receiving_cycle_test.go") && (dependency == modulePath+"/internal/admission/receiving" || dependency == modulePath+"/internal/admission/spending") {
			return true
		}
		if source == "internal/connection/publisher_recovery.go" && dependency == modulePath+"/internal/route/prefix" {
			return true
		}
		if source == "internal/connection/initiator_source.go" {
			for _, owned := range []string{"network", "reachability", "route", "route/prefix", "route/selection", "route/introduction"} {
				if dependency == modulePath+"/internal/"+owned {
					return true
				}
			}
		}
		if source == "internal/release/infrastructure/source_plan_linux.go" && dependency == modulePath+"/internal/network/source" {
			return true
		}
		if source == "internal/release/infrastructure/source_startup_linux.go" && (dependency == modulePath+"/internal/platform/process" || dependency == modulePath+"/internal/platform/cgroup" || dependency == modulePath+"/internal/platform/systemd") {
			return true
		}
		if source == "internal/release/infrastructure/source_start_linux.go" && (dependency == modulePath+"/internal/platform/process" || dependency == modulePath+"/internal/platform/cgroup" || dependency == modulePath+"/internal/platform/systemd") {
			return true
		}
		if source == "internal/release/infrastructure/source_replace_linux.go" && (dependency == modulePath+"/internal/network/state" || dependency == modulePath+"/internal/platform/cgroup" || dependency == modulePath+"/internal/platform/systemd") {
			return true
		}
		if (source == "internal/release/infrastructure/source_recover_linux.go" || source == "internal/release/infrastructure/source_initial_linux.go") && dependency == modulePath+"/internal/platform/systemd" {
			return true
		}
		if source == "internal/release/infrastructure/source_recover_linux.go" && dependency == modulePath+"/internal/network/state" {
			return true
		}
		if source == "internal/release/infrastructure/source_recover_replacement_linux.go" && (dependency == modulePath+"/internal/release" || dependency == modulePath+"/internal/network/state" || dependency == modulePath+"/internal/platform/systemd") {
			return true
		}
		if source == "internal/release/infrastructure/source_recovery_completed_linux.go" && (dependency == modulePath+"/internal/platform/cgroup" || dependency == modulePath+"/internal/platform/systemd") {
			return true
		}
		if source == "internal/release/infrastructure/source_retirement_history_linux.go" && (dependency == modulePath+"/internal/platform/systemd") {
			return true
		}
		if source == "internal/release/infrastructure/source_stop_linux.go" && (dependency == modulePath+"/internal/platform/process" || dependency == modulePath+"/internal/platform/cgroup" || dependency == modulePath+"/internal/platform/systemd") {
			return true
		}
		if (source == "internal/release/infrastructure/source_observation_linux.go" || source == "internal/release/infrastructure/source_install_linux.go" || source == "internal/release/infrastructure/source_loaded_linux.go" || source == "internal/release/infrastructure/source_policy_linux_test.go" || source == "internal/release/infrastructure/source_running_linux_test.go") && dependency == modulePath+"/internal/platform/systemd" {
			return true
		}
		if dependency == modulePath+"/internal/network/source/servingdeclaration" {
			switch source {
			case "cmd/ardents-next/source_serving_fixture_linux_test.go",
				"cmd/ardents-next/source_installed_native_linux_test.go",
				"cmd/ardents-next/source_lifecycle_native_linux_test.go",
				"cmd/ardents-next/source_native_linux_test.go",
				"cmd/ardents-next/source_preparation_process_linux_test.go",
				"internal/network/state/source_state_files_linux_test.go",
				"cmd/ardents-next/source_state_linux_test.go":
				return true
			}
		}
		// A registered package's external behavior tests may import that package.
		// This grants neither another package nor nested tests its imports.
		if _, registered := permitted[owner]; registered && strings.HasSuffix(source, "_test.go") && dependency == modulePath+"/"+owner {
			return true
		}
		for _, allowed := range permitted[owner] {
			if dependency == modulePath+"/internal/"+allowed {
				return true
			}
		}
		return false
	}
	if (source == "internal/endpoint/reader/operation_linux.go" || source == "internal/endpoint/reader/byte_stream_linux.go") && dependency == modulePath+"/internal/application/document" {
		return true
	}
	if source == "internal/application/administration/byte_stream_linux.go" && dependency == modulePath+"/internal/application/connection" {
		return true
	}
	if (source == "internal/endpoint/reader/reader_linux.go" || source == "internal/endpoint/reader/operation_linux.go" || source == "internal/endpoint/reader/byte_stream_linux.go" || source == "internal/endpoint/participant_linux.go") && dependency == modulePath+"/internal/application/connection" {
		return true
	}
	if source == "cmd/ardents-next/diagnostic_timeline.go" && local == "internal/diagnostics/timeline" {
		return true
	}
	if (source == "internal/endpoint/participant_linux.go" || source == "cmd/ardents-next/endpoint_administration_client.go" || source == "cmd/ardents-next/installed_service_linux_test.go" || source == "cmd/ardents-next/endpoint_replacement_worker_native_linux_test.go") && dependency == modulePath+"/internal/application/administration" {
		return true
	}
	if strings.HasPrefix(dependency, modulePath+"/internal/application/") {
		switch source {
		case "cmd/ardents-next/document_command.go", "cmd/ardents-next/document_reader_peer_linux_test.go", "cmd/ardents-next/stream_command_linux.go", "cmd/ardents-next/stream_command_linux_test.go":
			return dependency == modulePath+"/internal/application/connection"
		case "cmd/ardents-next/document_reader.go", "cmd/ardents-next/document_reader_linux_test.go", "cmd/ardents-next/document_reader_process_linux_test.go":
			return dependency == modulePath+"/internal/application/connection" || dependency == modulePath+"/internal/application/document"
		case "cmd/ardents-next/document_publisher.go":
			return dependency == modulePath+"/internal/application/administration" || dependency == modulePath+"/internal/application/document"
		case "cmd/ardents-next/document_published_link_linux.go", "cmd/ardents-next/document_published_link_linux_test.go", "cmd/ardents-next/document_publisher_linux_test.go":
			return dependency == modulePath+"/internal/application/administration"
		}
	}
	if source == "cmd/ardents-next/endpoint_journey_native_linux_test.go" || source == "cmd/ardents-next/document_command_installed_linux_test.go" {
		switch dependency {
		case modulePath + "/internal/application/administration", modulePath + "/internal/application/connection", modulePath + "/internal/application/document":
			return true
		}
	}
	// The cold/warm latency participant (#650) runs as the installed Endpoint
	// MainPID and measures the ordinary Reader composition directly: it opens the
	// public Connection request and reads the validated text response exactly as
	// the installed reader adapter does. It owns no Application server.
	if source == "internal/endpoint/latency_native_linux_test.go" {
		switch dependency {
		case modulePath + "/internal/application/connection", modulePath + "/internal/application/document":
			return true
		}
	}
	if source == "internal/execution/runtime/initialization_linux.go" && dependency == modulePath+"/internal/application/document" {
		return true
	}
	if (source == "internal/execution/runtime/publisher_worker_linux.go" || source == "internal/execution/runtime/publisher_worker_linux_test.go") && dependency == modulePath+"/internal/application/document" {
		return true
	}
	if (source == "internal/execution/runtime/byte_stream.go" || source == "internal/execution/runtime/byte_stream_linux.go") && dependency == modulePath+"/internal/application/document" {
		return true
	}
	if (source == "internal/execution/runtime/stream_contract.go" || source == "internal/execution/runtime/stream_contract_test.go") && dependency == modulePath+"/internal/application/connection" {
		return true
	}
	if source == "internal/execution/runtime/reader_worker_linux.go" && dependency == modulePath+"/internal/application/document" {
		return true
	}
	if (source == "internal/execution/runtime/reader_worker_linux.go" || source == "internal/execution/runtime/reader_stream.go") && dependency == modulePath+"/internal/application/connection" {
		return true
	}
	if (source == "internal/execution/runtime/publisher_worker_linux.go" || source == "internal/execution/runtime/publisher_stream.go" || source == "internal/execution/runtime/hostile_snapshot_installed_linux_test.go") && dependency == modulePath+"/internal/application/connection" {
		return true
	}
	if source == "internal/execution/worker/stream_activation_installed_linux_test.go" && dependency == modulePath+"/internal/application/streamqualification" {
		return true
	}
	if (source == "internal/execution/runtime/stream_qualification_linux.go" || source == "internal/execution/runtime/stream_qualification_other.go" || source == "internal/execution/runtime/stream_qualification_installed_linux_test.go") && dependency == modulePath+"/internal/application/streamqualification" {
		return true
	}
	if (source == "internal/connection/stream.go" || source == "internal/connection/stream_test.go") && dependency == modulePath+"/internal/application/connection" {
		return true
	}
	if (source == "cmd/ardents-next/connection_response_loss_linux_test.go" || source == "cmd/ardents-next/connection_authority_linux_test.go") && dependency == modulePath+"/internal/application/connection" {
		return true
	}
	if source == "internal/endpoint/publisher/snapshot_linux.go" && dependency == modulePath+"/internal/application/document" {
		return true
	}
	if (owner == "internal/admission/issuance" || owner == "internal/admission/token") && dependency == "github.com/cloudflare/circl/blindsign/blindrsa" {
		return true
	}
	if (source == "internal/platform/systemd/manager_reference_linux.go" || source == "internal/platform/systemd/node_slots_linux.go" || source == "internal/platform/systemd/forwarding_manager_linux.go" || source == "internal/platform/systemd/introduction_manager_linux.go" || source == "internal/platform/systemd/endpoint_reference_linux.go" || source == "internal/platform/systemd/startup_observation_linux.go" || source == "internal/platform/systemd/source_preflight_linux.go" || source == "internal/platform/systemd/source_stopped_linux.go" || source == "internal/platform/systemd/source_manager_linux.go" || source == "internal/platform/systemd/source_control_linux.go" || source == "internal/platform/systemd/source_reference_native_linux_test.go") && dependency == "github.com/godbus/dbus/v5" {
		return true
	}
	if (owner == "internal/admission/spending" || owner == "internal/network/duty" || owner == "internal/network/state/durable") && dependency == "golang.org/x/sys/windows" {
		return true
	}
	if (source == "internal/durablefile/owner_root_windows.go" || source == "internal/durablefile/owner_root_windows_test.go" || source == "internal/route/selection/root_permissions_windows_test.go") && dependency == "golang.org/x/sys/windows" {
		return true
	}
	if source == "internal/reachability/store_platform_windows.go" && dependency == "golang.org/x/sys/windows" {
		return true
	}
	if source == "internal/publication/durable/lease_windows.go" && dependency == "golang.org/x/sys/windows" {
		return true
	}
	if source == "internal/release/history_native_windows.go" && dependency == "golang.org/x/sys/windows" {
		return true
	}
	if owner == "internal/release" && dependency == "github.com/theupdateframework/go-tuf/v2/metadata" {
		switch source {
		case "internal/release/metadata_authentication.go", "internal/release/target_identity.go", "internal/release/safety_policy.go", "internal/release/floor_encoding.go":
			return true
		}
	}
	if source == "internal/release/metadata_authentication.go" && dependency == "github.com/theupdateframework/go-tuf/v2/metadata/trustedmetadata" {
		return true
	}
	if strings.HasSuffix(source, "_test.go") {
		if source == "internal/release/generation_authorization_test.go" && dependency == "github.com/theupdateframework/go-tuf/v2/metadata" {
			return true
		}
		if source == "internal/release/rotation_test.go" && (dependency == "github.com/sigstore/sigstore/pkg/signature" || dependency == "github.com/theupdateframework/go-tuf/v2/metadata") {
			return true
		}
		if (source == "cmd/ardents-next/release_process_test.go" || source == "cmd/ardents-next/infrastructure_linux_test.go") && (dependency == "github.com/sigstore/sigstore/pkg/signature" || dependency == "github.com/theupdateframework/go-tuf/v2/metadata") {
			return true
		}
		if source == "cmd/ardents-next/forwarding_artifact_linux_test.go" && dependency == "github.com/theupdateframework/go-tuf/v2/metadata" {
			return true
		}
		if (owner == "internal/network/epoch" || owner == "internal/network/state" || owner == "cmd/ardents-next" || source == "internal/endpoint/history_preparation_linux_test.go" || source == "internal/endpoint/plan_construction_linux_test.go") && dependency == modulePath+"/tests/epochfixture/network" {
			return true
		}
		if owner == "internal/network/epoch" && dependency == modulePath+"/tests/epochfixture/assignment" {
			return true
		}
	}

	if source == "internal/release/infrastructure/source_transition_scope_linux.go" && dependency == "golang.org/x/sys/unix" {
		return true
	}
	if source == "internal/endpoint/reader/result.go" && (dependency == modulePath+"/internal/application/connection" || dependency == modulePath+"/internal/application/document") {
		return true
	}
	if (source == "internal/endpoint/publisher/publisher_linux.go" || source == "internal/endpoint/publisher/opening_linux.go" || source == "internal/endpoint/publisher/byte_stream_linux.go") && dependency == modulePath+"/internal/application/document" {
		return true
	}
	if source == "internal/endpoint/publisher/byte_stream_linux.go" && dependency == modulePath+"/internal/application/connection" {
		return true
	}
	if source == "internal/endpoint/publisher/contract.go" && dependency == modulePath+"/internal/application/document" {
		return true
	}
	// OTel composition is confined to the exact command package.
	// No arbitrary third-party bridge is permitted.
	if owner == "cmd/ardents-next" {
		if slices.Contains([]string{
			"go.opentelemetry.io/otel/attribute",
			"go.opentelemetry.io/otel/metric",
			"go.opentelemetry.io/otel/sdk/metric",
			"go.opentelemetry.io/otel/sdk/metric/exemplar",
			"go.opentelemetry.io/otel/sdk/metric/metricdata",
			"go.opentelemetry.io/otel/sdk/resource",
			"go.opentelemetry.io/otel/sdk/trace",
			"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp",
			"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp",
		}, dependency) {
			return true
		}
		if strings.HasSuffix(source, "_test.go") {
			if slices.Contains([]string{"github.com/cloudflare/circl/blindsign/blindrsa", "go.opentelemetry.io/proto/otlp/collector/trace/v1", "go.opentelemetry.io/proto/otlp/collector/metrics/v1", "google.golang.org/protobuf/proto"}, dependency) {
				return true
			}
		}
	}
	// Go reserves first import-path elements without a dot for standard-library
	// paths. Unknown paths still fail compilation; relative paths are forbidden.
	first, _, _ := strings.Cut(dependency, "/")
	return dependency != "C" && dependency != "unsafe" && dependency != "" && !strings.Contains(first, ".") && path.Clean(dependency) == dependency && !strings.HasPrefix(dependency, "/")
}

func TestSuccessorIsolationPolicy(t *testing.T) {
	for _, test := range []struct {
		name, source, dependency string
		allowed                  bool
	}{
		{"public issuer inspection uses new codec", "cmd/ardents-next/offline_profile_inspection.go", modulePath + "/internal/admission/issuerprofile", true},
		{"public issuer inspection excludes old codec", "cmd/ardents-next/offline_profile_inspection.go", modulePath + "/internal/admission", false},
		{"public issuer inspection excludes private issuer", "cmd/ardents-next/offline_profile_inspection.go", modulePath + "/internal/admission/issuer", false},
		{"public issuer inspection excludes Network authority", "cmd/ardents-next/offline_profile_inspection.go", modulePath + "/internal/network/state", false},
		{"public issuer inspection excludes Route", "cmd/ardents-next/offline_profile_inspection.go", modulePath + "/internal/route/role", false},
		{"adjacent Control has no inherited codec access", "internal/node/provisioning/unregistered.go", modulePath + "/internal/admission/issuerprofile", false},
		{"Endpoint release producer uses new Enrollment", "scripts/sign-qualification-release.go", modulePath + "/internal/enrollment", true},
		{"Endpoint release producer uses new Release", "scripts/sign-qualification-release.go", modulePath + "/internal/release", true},
		{"Endpoint release producer excludes retired Enrollment path", "scripts/sign-qualification-release.go", modulePath + "/internal/successor/enrollment", false},
		{"Endpoint release producer excludes retired Release path", "scripts/sign-qualification-release.go", modulePath + "/internal/successor/release", false},
		{"Endpoint release producer has no Network authority", "scripts/sign-qualification-release.go", modulePath + "/internal/network", false},
		{"Endpoint release producer has no Route runtime", "scripts/sign-qualification-release.go", modulePath + "/internal/route/role", false},
		{"Endpoint release producer retains TUF mechanism", "scripts/sign-qualification-release.go", "github.com/theupdateframework/go-tuf/v2/metadata", true},
		{"Endpoint release producer retains signing mechanism", "scripts/sign-qualification-release.go", "github.com/sigstore/sigstore/pkg/signature", true},
		{"retained heap map validates the public Permission framing", "internal/diagnostics/process/heapdump_role_map_capture_test.go", modulePath + "/internal/admission", true},
		{"retained heap map cannot open a spending root", "internal/diagnostics/process/heapdump_role_map_capture_test.go", modulePath + "/internal/admission/spending", false},
		{"retained heap map does not import predecessor Endpoint", "internal/diagnostics/process/heapdump_role_map_capture_test.go", modulePath + "/internal/endpoint", false},
		{"Endpoint release producer retains native files", "scripts/sign-qualification-release.go", "golang.org/x/sys/unix", true},
		{"Endpoint release producer excludes foreign library", "scripts/sign-qualification-release.go", "example.org/foreign", false},
		{"packaging uses new Enrollment through the product command", "cmd/ardents-next/enrollment_packaging.go", modulePath + "/internal/enrollment", true},
		{"packaging commands exclude retired Enrollment", "cmd/ardents-next/enrollment_packaging.go", modulePath + "/internal/successor/enrollment", false},
		{"standalone packaging helpers cannot consume Enrollment", "scripts/packaging-check.go", modulePath + "/internal/enrollment", false},
		{"packaging commands exclude Release authority", "cmd/ardents-next/enrollment_packaging.go", modulePath + "/internal/release", false},
		{"adjacent script has no inherited Enrollment access", "scripts/another-tool.go", modulePath + "/internal/enrollment", false},
		{"Fixed Node Release signing uses exact artifact resources", "internal/release/infrastructure/signing/node_release_linux.go", modulePath + "/internal/release/infrastructure", true},
		{"Fixed Node Release predecessor verifies genuine Release", "internal/release/infrastructure/signing/node_release_predecessor_linux.go", modulePath + "/internal/release", true},
		{"JOIN key birth grants no Release observation", "internal/release/infrastructure/signing/join_release_keys_linux.go", modulePath + "/internal/release", false},
		{"Fixed Node Release signing grants no Route consumer", "internal/release/infrastructure/signing/node_release_linux.go", modulePath + "/internal/route/role", false},
		{"Fixed Node Release plan grants no State owner", "internal/release/infrastructure/signing/node_release_plan_linux.go", modulePath + "/internal/network/state", false},
		{"delayed ACK fixture retains genuine installed operation", "cmd/ardents-next/publication_ack_installed_linux_test.go", modulePath + "/internal/execution/runtime", true},
		{"receiving Network fixture selects accepted profile", "cmd/ardents-next/publication_network_linux_test.go", modulePath + "/internal/network/epoch", true},
		{"receiving Network fixture names selected Carriers", "cmd/ardents-next/publication_network_linux_test.go", modulePath + "/internal/route/transport", true},
		{"receiving Network fixture has no concrete TLS adapter", "cmd/ardents-next/publication_network_linux_test.go", modulePath + "/internal/route/transport/tls", false},
		{"receiving Network fixture has no concrete QUIC adapter", "cmd/ardents-next/publication_network_linux_test.go", modulePath + "/internal/route/transport/quic", false},
		{"receiving Network fixture grants no Publisher runtime", "cmd/ardents-next/publication_network_linux_test.go", modulePath + "/internal/publication/runtime", false},
		{"delayed ACK fixture invokes command composition", "cmd/ardents-next/publication_ack_installed_linux_test.go", modulePath + "/internal/publication/runtime", false},
		{"delayed ACK fixture cannot reserve durable authority", "cmd/ardents-next/publication_ack_installed_linux_test.go", modulePath + "/internal/publication/durable", false},
		{"installed Service scenario checks public credential codec", "cmd/ardents-next/installed_service_linux_test.go", modulePath + "/internal/publication", true},
		{"installed Service scenario uses public Administration client", "cmd/ardents-next/installed_service_linux_test.go", modulePath + "/internal/application/administration", true},
		{"installed Service scenario cannot use old Endpoint", "cmd/ardents-next/installed_service_linux_test.go", modulePath + "/internal/endpoint", false},
		{"installed Service scenario owns receiving Store fixture", "cmd/ardents-next/installed_service_linux_test.go", modulePath + "/internal/reachability", true},
		{"installed Service scenario selects exact Carrier", "cmd/ardents-next/installed_service_linux_test.go", modulePath + "/internal/route/transport", true},
		{"installed Service scenario cannot supply private Instance", "cmd/ardents-next/installed_service_linux_test.go", modulePath + "/internal/publication/instance", false},
		{"installed Service scenario cannot bypass runtime command", "cmd/ardents-next/installed_service_linux_test.go", modulePath + "/internal/publication/runtime", false},
		{"native Publisher scenario independently checks public proof", "cmd/ardents-next/publication_installed_linux_test.go", modulePath + "/internal/publication", true},
		{"native Publisher scenario prepares genuine host Instance", "cmd/ardents-next/publication_installed_linux_test.go", modulePath + "/internal/publication/instance", true},
		{"native Publisher scenario reopens real receiving Store", "cmd/ardents-next/publication_installed_linux_test.go", modulePath + "/internal/reachability", true},
		{"native Publisher scenario names selected Carrier", "cmd/ardents-next/publication_installed_linux_test.go", modulePath + "/internal/route/transport", true},
		{"native Publisher scenario cannot bypass command lifecycle", "cmd/ardents-next/publication_installed_linux_test.go", modulePath + "/internal/publication/runtime", false},
		{"native Publisher scenario has no direct durable reservation", "cmd/ardents-next/publication_installed_linux_test.go", modulePath + "/internal/publication/durable", false},
		{"native Publisher scenario excludes concrete TLS adapter", "cmd/ardents-next/publication_installed_linux_test.go", modulePath + "/internal/route/transport/tls", false},
		{"native Publisher scenario excludes concrete QUIC adapter", "cmd/ardents-next/publication_installed_linux_test.go", modulePath + "/internal/route/transport/quic", false},
		{"another native test has no Publisher exception", "cmd/ardents-next/unregistered_installed_linux_test.go", modulePath + "/internal/publication/instance", false},
		{"installed Reader retains original worker", "internal/endpoint/reader/reader_linux.go", modulePath + "/internal/execution/runtime", true},
		{"installed Reader consumes authenticated Source", "internal/endpoint/reader/reader_linux.go", modulePath + "/internal/connection", true},
		{"installed Reader local byte contract", "internal/endpoint/reader/reader_linux.go", modulePath + "/internal/application/connection", true},
		{"installed Reader local Text projection", "internal/endpoint/reader/result.go", modulePath + "/internal/application/document", true},
		{"installed participant owns Reader listener", "internal/endpoint/participant_linux.go", modulePath + "/internal/application/connection", true},
		{"first participant preparation initializes participation", "internal/endpoint/history_preparation_linux.go", modulePath + "/internal/network/duty", true},
		{"first participant preparation initializes selection history", "internal/endpoint/history_preparation_linux.go", modulePath + "/internal/route/selection", true},
		{"participant runtime cannot initialize participation directly", "internal/endpoint/participant_linux.go", modulePath + "/internal/network/duty", false},
		{"first participant preparation uses Stock owner", "internal/endpoint/history_preparation_linux.go", modulePath + "/internal/admission/stock", true},
		{"first participant preparation cannot bypass Stock", "internal/endpoint/history_preparation_linux.go", modulePath + "/internal/admission/attempts", false},
		{"first participant preparation has no old backend", "internal/endpoint/history_preparation_linux.go", modulePath + "/internal/endpoint", false},
		{"plan constructor authenticates retained State", "internal/endpoint/plan_construction_linux.go", modulePath + "/internal/network/state", true},
		{"plan constructor observes authenticated facts", "internal/endpoint/plan_construction_linux.go", modulePath + "/internal/endpoint/funding", true},
		{"plan constructor cannot initialize participation", "internal/endpoint/plan_construction_linux.go", modulePath + "/internal/network/duty", false},
		{"plan constructor cannot initialize selection history", "internal/endpoint/plan_construction_linux.go", modulePath + "/internal/route/selection", false},
		{"plan constructor has no old backend", "internal/endpoint/plan_construction_linux.go", modulePath + "/internal/endpoint", false},
		{"plan constructor command uses endpoint owner", "cmd/ardents-next/endpoint_construction_linux.go", modulePath + "/internal/endpoint", true},
		{"plan constructor command has no direct Source codec", "cmd/ardents-next/endpoint_construction_linux.go", modulePath + "/internal/network/source", false},
		{"installed participant owns Administration listener", "internal/endpoint/participant_linux.go", modulePath + "/internal/application/administration", true},
		{"installed Publisher retains original worker", "internal/endpoint/publisher/publisher_linux.go", modulePath + "/internal/execution/runtime", true},
		{"other installed adapter has no Publisher authority", "cmd/ardents-next/installed_other_linux.go", modulePath + "/internal/execution/runtime", false},
		{"installed Publisher cannot use old Endpoint", "internal/endpoint/publisher/publisher_linux.go", modulePath + "/internal/endpoint", false},
		{"unregistered installed adapter has no local byte exception", "cmd/ardents-next/other_installed_linux.go", modulePath + "/internal/application/connection", false},
		{"installed Reader cannot use old Connection", "internal/endpoint/reader/reader_linux.go", modulePath + "/internal/service/connection", false},
		{"qualified Publisher uses only native Text protocol", "internal/execution/runtime/publisher_worker_linux.go", modulePath + "/internal/application/document", true},
		{"qualified Reader uses only native Text protocol", "internal/execution/runtime/reader_worker_linux.go", modulePath + "/internal/application/document", true},
		{"qualified Reader input uses only byte-stream boundary", "internal/execution/runtime/reader_stream.go", modulePath + "/internal/application/connection", true},
		{"qualified Publisher uses Application stream grammar", "internal/execution/runtime/publisher_worker_linux.go", modulePath + "/internal/application/connection", true},
		{"selected Publisher input uses Text import", "internal/endpoint/publisher/snapshot_linux.go", modulePath + "/internal/application/document", true},
		{"Execution launch borrows only Text INIT grammar", "internal/execution/runtime/initialization_linux.go", modulePath + "/internal/application/document", true},
		{"Execution rules cannot borrow Text runtime", "internal/execution/job.go", modulePath + "/internal/application/document", false},
		{"common Execution launch has no native Text exception", "internal/execution/runtime/launch.go", modulePath + "/internal/application/document", false},
		{"other Execution composition has no Text exception", "internal/execution/runtime/other_linux.go", modulePath + "/internal/application/document", false},
		{"installed Stream activation test uses fixed qualification protocol", "internal/execution/worker/stream_activation_installed_linux_test.go", modulePath + "/internal/application/streamqualification", true},
		{"ordinary worker activation has no qualification protocol edge", "internal/execution/worker/activate_linux.go", modulePath + "/internal/application/streamqualification", false},
		{"ordinary Text INIT has no qualification protocol edge", "internal/execution/runtime/initialization_linux.go", modulePath + "/internal/application/streamqualification", false},
		{"trusted Stream qualification seam uses fixed qualification protocol", "internal/execution/runtime/stream_qualification_linux.go", modulePath + "/internal/application/streamqualification", true},
		{"common Execution launch has no qualification protocol edge", "internal/execution/runtime/launch.go", modulePath + "/internal/application/streamqualification", false},
		{"ordinary Text protocol has no qualification protocol edge", "internal/execution/runtime/protocol.go", modulePath + "/internal/application/streamqualification", false},
		{"Execution cannot borrow old Endpoint authority", "internal/execution/runtime/initialization_linux.go", modulePath + "/internal/endpoint", false},
		{"Execution cannot borrow old Broker", "internal/execution/authority.go", modulePath + "/internal/application/broker", false},
		{"old Endpoint cannot consume new Execution", "internal/endpoint/worker_grant_linux.go", modulePath + "/internal/execution", false},
		{"native original manager reference owns bus client", "internal/platform/systemd/endpoint_reference_linux.go", "github.com/godbus/dbus/v5", true},
		{"fresh Startup manager observation owns bus client", "internal/platform/systemd/startup_observation_linux.go", "github.com/godbus/dbus/v5", true},
		{"read-only Node slot observer consumes physical D-Bus", "internal/platform/systemd/node_slots_linux.go", "github.com/godbus/dbus/v5", true},
		{"read-only Node slot observer cannot authorize Release", "internal/platform/systemd/node_slots_linux.go", modulePath + "/internal/release", false},
		{"native Source reference regression checks genuine bus refusal", "internal/platform/systemd/source_reference_native_linux_test.go", "github.com/godbus/dbus/v5", true},
		{"manager value validation excludes bus client", "internal/platform/systemd/manager_values_linux.go", "github.com/godbus/dbus/v5", false},
		{"manager transport excludes bus client", "internal/platform/systemd/reference_transport_linux.go", "github.com/godbus/dbus/v5", false},
		{"Installation decisions exclude bus client", "internal/endpoint/launch/predecessor_linux.go", "github.com/godbus/dbus/v5", false},
		{"portable authorization cannot own installed Endpoint", "internal/release/endpoint/generation_authentication.go", modulePath + "/internal/endpoint/launch", false},
		{"portable authorization cannot own physical root", "internal/release/endpoint/generation_authentication.go", modulePath + "/internal/platform/cgroup", false},
		{"installed Endpoint consumes opaque authorization", "internal/endpoint/launch/generation_authorization.go", modulePath + "/internal/release/endpoint", true},
		{"unregistered manager file excludes bus client", "internal/platform/systemd/other_linux.go", "github.com/godbus/dbus/v5", false},
		{"Reachability verifies public delegation", "internal/reachability/descriptor.go", modulePath + "/internal/publication", true},
		{"Prefix completes genuine private lookup", "internal/route/prefix/descriptor_exchange.go", modulePath + "/internal/reachability", true},
		{"Receiver uses genuine Descriptor Store", "internal/route/receiver/descriptor_linux.go", modulePath + "/internal/reachability", true},
		{"Carrier cannot acquire Descriptor authority", "internal/route/transport/tls/node.go", modulePath + "/internal/reachability", false},
		{"Reachability cannot own physical Prefix", "internal/reachability/lookup_history.go", modulePath + "/internal/route/prefix", false},
		{"Shared owner-only root DACL enforcement", "internal/durablefile/owner_root_windows.go", "golang.org/x/sys/windows", true},
		{"Owner-only root mechanism test verifies the physical DACL", "internal/durablefile/owner_root_windows_test.go", "golang.org/x/sys/windows", true},
		{"Selection Windows floor consumes the shared mechanism", "internal/route/selection/root_permissions_windows.go", "golang.org/x/sys/windows", false},
		{"Selection Windows floor test verifies the physical DACL", "internal/route/selection/root_permissions_windows_test.go", "golang.org/x/sys/windows", true},
		{"Store native Windows lease and sync", "internal/reachability/store_platform_windows.go", "golang.org/x/sys/windows", true},
		{"Descriptor rules cannot use native Windows mechanisms", "internal/reachability/descriptor.go", "golang.org/x/sys/windows", false},
		{"Store rules cannot use native Windows mechanisms", "internal/reachability/store.go", "golang.org/x/sys/windows", false},
		{"public proof cannot acquire Route transport", "internal/publication/public_proof.go", modulePath + "/internal/route/ardp", false},
		{"public proof cannot acquire Store", "internal/publication/public_proof.go", modulePath + "/internal/reachability", false},
		{"Reachability cannot delegate to old verifier", "internal/reachability/descriptor.go", modulePath + "/internal/service/reachability", false},
		{"public proof cannot delegate to old Publication", "internal/publication/public_proof.go", modulePath + "/internal/service/publication", false},
		{"physical implementation uses shared contract", "internal/route/transport/tls/node.go", modulePath + "/internal/route/transport", true},
		{"Prefix uses shared contract", "internal/route/prefix/prefix.go", modulePath + "/internal/route/transport", true},
		{"Prefix uses portable framing", "internal/route/prefix/prefix.go", modulePath + "/internal/route/channel", true},
		{"channel uses grammar", "internal/route/channel/session.go", modulePath + "/internal/route/ardp", true},
		{"bootstrap consumes framing capacity", "internal/route/bootstrap/budget.go", modulePath + "/internal/route/channel", true},
		{"bootstrap describes canonical restriction", "internal/route/bootstrap/budget.go", modulePath + "/internal/route/ardp", true},
		{"receiver reserves bootstrap capacity", "internal/route/receiver/receiver_linux.go", modulePath + "/internal/route/bootstrap", true},
		{"channel cannot acquire bootstrap policy", "internal/route/channel/session.go", modulePath + "/internal/route/bootstrap", false},
		{"bootstrap cannot spend Admission", "internal/route/bootstrap/budget.go", modulePath + "/internal/admission/receiving", false},
		{"bootstrap cannot import receiving composition", "internal/route/bootstrap/budget.go", modulePath + "/internal/route/receiver", false},
		{"old receiver cannot import new bootstrap", "internal/node/forwarding/link.go", modulePath + "/internal/route/bootstrap", false},
		{"channel uses ordered transport", "internal/route/channel/session.go", modulePath + "/internal/route/transport", true},
		{"channel cannot import TLS adapter", "internal/route/channel/session.go", modulePath + "/internal/route/transport/tls", false},
		{"channel cannot import QUIC adapter", "internal/route/channel/session_test.go", modulePath + "/internal/route/transport/quic", false},
		{"holder JOIN cannot import Receiver", "internal/route/join/acquisition.go", modulePath + "/internal/route/receiver", false},
		{"Receiver cannot import holder Prefix", "internal/route/receiver/receiver_linux.go", modulePath + "/internal/route/prefix", false},
		{"actual receiver dispatches JOIN pairing", "internal/route/receiver/receiver_linux.go", modulePath + "/internal/route/join/pairing", true},
		{"receiver does not import holder JOIN", "internal/route/receiver/receiver_linux.go", modulePath + "/internal/route/join", false},
		{"holder JOIN consumes Prefix", "internal/route/join/acquisition.go", modulePath + "/internal/route/prefix", true},
		{"former operation cannot return", "cmd/ardents-next/route_linux.go", modulePath + "/internal/route/operation", false},
		{"Prefix cannot import JOIN consumer", "internal/route/prefix/borrowing.go", modulePath + "/internal/route/join", false},
		{"Prefix cannot import Introduction consumer", "internal/route/prefix/introduction_channel.go", modulePath + "/internal/route/introduction", false},
		{"Reader initial factory retains actual Source", "internal/connection/initiator_source.go", modulePath + "/internal/route/prefix", true},
		{"Publisher recovery retains original Source and Responder", "internal/connection/publisher_recovery.go", modulePath + "/internal/route/prefix", true},
		{"Publisher dispatch uses canonical full logical tuple", "internal/publication/runtime/publisher_streams.go", modulePath + "/internal/connection/wire", true},
		{"Publication scheduler cannot construct wire authority", "internal/publication/runtime/scheduler.go", modulePath + "/internal/connection/wire", false},
		{"Connection record actors cannot choose Source", "internal/connection/stream_records.go", modulePath + "/internal/route/prefix", false},
		{"Reader initial factory verifies independent Descriptor", "internal/connection/initiator_source.go", modulePath + "/internal/reachability", true},
		{"Connection wire cannot borrow Source", "internal/connection/wire/continuity.go", modulePath + "/internal/route/prefix", false},
		{"Connection ledger cannot choose Source", "internal/connection/receive_ledger.go", modulePath + "/internal/route/prefix", false},
		{"Prefix cannot import receiving composition", "internal/route/prefix/prefix.go", modulePath + "/internal/route/receiver", false},
		{"Prefix calls actual role owner", "internal/route/prefix/prefix.go", modulePath + "/internal/route/role", true},
		{"holder consumes actual Prefix", "cmd/ardents-next/route_linux.go", modulePath + "/internal/route/prefix", true},
		{"channel cannot import receiving composition", "internal/route/channel/session.go", modulePath + "/internal/route/receiver", false},
		{"channel cannot import Network", "internal/route/channel/session_test.go", modulePath + "/internal/network", false},
		{"channel cannot import role authority", "internal/route/channel/session.go", modulePath + "/internal/route/role", false},
		{"role cannot import receiving composition", "internal/route/role/authority.go", modulePath + "/internal/route/receiver", false},
		{"role cannot spend Admission", "internal/route/role/authority.go", modulePath + "/internal/admission/receiving", false},
		{"role consumes Network observations", "internal/route/role/authority.go", modulePath + "/internal/network", true},
		{"Introduction exchange uses role binding", "internal/route/introduction/receiving.go", modulePath + "/internal/route/role", true},
		{"Source command parses sealed capsule", "cmd/ardents-next/route_linux.go", modulePath + "/internal/route/capsule", true},
		{"submission fixture parses sealed capsule", "cmd/ardents-next/route_submission_linux_test.go", modulePath + "/internal/route/capsule", true},
		{"other commands cannot acquire capsule consumer", "cmd/ardents-next/admission_holder.go", modulePath + "/internal/route/capsule", false},
		{"submission consumes retained exclusions", "internal/route/introduction/submission.go", modulePath + "/internal/route", true},
		{"Introduction history birth consumes only its genuine receiving owner", "internal/route/introduction/receiving_history_linux.go", modulePath + "/internal/admission/receiving", true},
		{"Introduction cannot own receiving Grant", "internal/route/introduction/receiving.go", modulePath + "/internal/admission/receiving", false},
		{"Introduction cannot call listener composition", "internal/route/introduction/receiving.go", modulePath + "/internal/route/receiver", false},
		{"channel cannot import Admission", "internal/route/channel/session.go", modulePath + "/internal/admission", false},
		{"channel cannot import Hosting", "internal/route/channel/session.go", modulePath + "/internal/hosting", false},
		{"transport cannot import framing", "internal/route/transport/stream.go", modulePath + "/internal/route/channel", false},
		{"transport cannot import concrete implementation", "internal/route/transport/stream.go", modulePath + "/internal/route/carrier", false},
		{"transport cannot import receiving composition", "internal/route/transport/stream.go", modulePath + "/internal/route/receiver", false},
		{"transport cannot import QUIC library", "internal/route/transport/stream.go", "github.com/quic-go/quic-go", false},
		{"TLS implementation consumes shared authentication", "internal/route/transport/tls/client.go", modulePath + "/internal/route/transport", true},
		{"TLS cannot import QUIC implementation", "internal/route/transport/tls/client.go", modulePath + "/internal/route/transport/quic", false},
		{"TLS cannot import former carrier", "internal/route/transport/tls/server.go", modulePath + "/internal/route/carrier", false},
		{"transport cannot import its TLS adapter", "internal/route/transport/role_authentication.go", modulePath + "/internal/route/transport/tls", false},
		{"retired QUIC implementation has no import grant", "internal/route/transport/quic/role.go", modulePath + "/internal/route/transport", false},
		{"former selector cannot import QUIC library", "internal/route/carrier/closed_node_carrier.go", "github.com/quic-go/quic-go", false},
		{"retired QUIC has no native library grant", "internal/route/transport/quic/node.go", "github.com/quic-go/quic-go", false},
		{"QUIC cannot import TLS implementation", "internal/route/transport/quic/role.go", modulePath + "/internal/route/transport/tls", false},
		{"transport exporter cannot import QUIC adapter", "internal/route/transport/role_exporter.go", modulePath + "/internal/route/transport/quic", false},
		{"production command excludes lower contract", "cmd/ardents-next/route_linux.go", modulePath + "/internal/route/transport", false},
		{"bootstrap scenario uses shared contract", "cmd/ardents-next/route_bootstrap_linux_test.go", modulePath + "/internal/route/transport", true},
		{"session consumes genuine stock", "internal/endpoint/session/session.go", modulePath + "/internal/admission/stock", true},
		{"session consumes original Execution permission", "internal/endpoint/session/session.go", modulePath + "/internal/execution/runtime", true},
		{"session cannot call command implementation", "internal/endpoint/session/session.go", modulePath + "/cmd/ardents-next", false},
		{"session cannot use old Endpoint", "internal/endpoint/session/session.go", modulePath + "/internal/endpoint", false},
		{"funding cannot depend on session", "internal/endpoint/funding/source.go", modulePath + "/internal/endpoint/session", false},
		{"Reader cannot own installed participant", "internal/endpoint/reader/reader_linux.go", modulePath + "/internal/endpoint", false},
		{"other binaries cannot acquire holder session", "cmd/ardents/main.go", modulePath + "/internal/endpoint/session", false},
		{"funding cannot depend on Reader", "internal/endpoint/funding/source.go", modulePath + "/internal/endpoint/reader", false},
		{"Reader cannot own Publication", "internal/endpoint/reader/reader_linux.go", modulePath + "/internal/publication/runtime", false},
		{"Publisher cannot import command composition", "internal/endpoint/publisher/publisher_linux.go", modulePath + "/cmd/ardents-next", false},
		{"other binaries cannot acquire new funding", "cmd/ardents/main.go", modulePath + "/internal/endpoint/funding", false},
		{"enrollment verification uses Enrollment", "cmd/ardents-next/enrollment.go", modulePath + "/internal/enrollment", true},
		{"diagnostics uses the local timeline projector", "cmd/ardents-next/diagnostic_timeline.go", modulePath + "/internal/diagnostics/timeline", true},
		{"diagnostics cannot consume Route", "cmd/ardents-next/diagnostic_timeline.go", modulePath + "/internal/route/transport", false},
		{"withdraw uses the Application Administration client", "cmd/ardents-next/endpoint_administration_client.go", modulePath + "/internal/application/administration", true},
		{"retired command has no Enrollment grant", "cmd/ardents/endpoint.go", modulePath + "/internal/enrollment", false},
		{"funded Source uses real token grammar", "internal/endpoint/funding/source.go", modulePath + "/internal/admission/token", true},
		{"funded Source uses finite bootstrap", "internal/endpoint/funding/source.go", modulePath + "/internal/route/bootstrap", true},
		{"funded Source behavior selects real Carrier", "cmd/ardents-next/route_token_linux_test.go", modulePath + "/internal/route/transport", true},
		{"other funding adapter has no bootstrap exception", "cmd/ardents-next/route_funding_linux.go", modulePath + "/internal/route/bootstrap", false},
		{"installed Execution scenario uses shared Carrier identity", "cmd/ardents-next/execution_installed_linux_test.go", modulePath + "/internal/route/transport", true},
		{"installed Execution scenario excludes direct TLS construction", "cmd/ardents-next/execution_installed_linux_test.go", modulePath + "/internal/route/transport/tls", false},
		{"installed Execution scenario excludes direct QUIC construction", "cmd/ardents-next/execution_installed_linux_test.go", modulePath + "/internal/route/transport/quic", false},
		{"installed Service scenario consumes real Connection", "cmd/ardents-next/connection_installed_linux_test.go", modulePath + "/internal/connection", true},
		{"hostile Publisher sibling tests the Application stream", "internal/execution/runtime/hostile_snapshot_installed_linux_test.go", modulePath + "/internal/application/connection", true},
		{"hostile Publisher sibling cannot acquire legacy Service authority", "internal/execution/runtime/hostile_snapshot_installed_linux_test.go", modulePath + "/internal/service/connection", false},
		{"recovery Custody process fixture has a terminal", "cmd/ardents-next/connection_custody_terminal_installed_linux_test.go", "github.com/aymanbagabas/go-pty", true},
		{"installed Reader cannot open a fixture terminal", "internal/endpoint/reader/operation_linux.go", "github.com/aymanbagabas/go-pty", false},
		{"recovery issuance proof consumes the public verifier", "cmd/ardents-next/connection_custody_instance_installed_linux_test.go", modulePath + "/internal/publication", true},
		{"recovery issuance proof cannot open private Instance authority", "cmd/ardents-next/connection_custody_instance_installed_linux_test.go", modulePath + "/internal/publication/instance", false},
		{"accepted-request recovery observes genuine Connection", "cmd/ardents-next/connection_accepted_request_installed_linux_test.go", modulePath + "/internal/connection", true},
		{"accepted-request recovery cannot construct direct TLS", "cmd/ardents-next/connection_accepted_request_installed_linux_test.go", modulePath + "/internal/route/transport/tls", false},
		{"response loss observes original Connection", "cmd/ardents-next/connection_response_loss_linux_test.go", modulePath + "/internal/connection", true},
		{"response loss retains original Execution", "cmd/ardents-next/connection_response_loss_linux_test.go", modulePath + "/internal/execution/runtime", true},
		{"withdrawal invokes genuine original Publisher", "cmd/ardents-next/connection_response_loss_linux_test.go", modulePath + "/internal/publication/runtime", true},
		{"withdrawal cannot acquire private Instance authority", "cmd/ardents-next/connection_response_loss_linux_test.go", modulePath + "/internal/publication/instance", false},
		{"response loss uses shared Carrier identity", "cmd/ardents-next/connection_response_loss_linux_test.go", modulePath + "/internal/route/transport", true},
		{"caller loss checks exact Application outcome", "cmd/ardents-next/connection_response_loss_linux_test.go", modulePath + "/internal/application/connection", true},
		{"response loss excludes direct TLS construction", "cmd/ardents-next/connection_response_loss_linux_test.go", modulePath + "/internal/route/transport/tls", false},
		{"response loss excludes direct QUIC construction", "cmd/ardents-next/connection_response_loss_linux_test.go", modulePath + "/internal/route/transport/quic", false},
		{"ordinary Text Reader consumes genuine Connection", "internal/endpoint/reader/source_linux.go", modulePath + "/internal/connection", true},
		{"installed Reader consumes qualified original worker", "internal/endpoint/reader/operation_linux.go", modulePath + "/internal/execution/runtime", true},
		{"another command cannot borrow Reader composition", "cmd/ardents-next/route.go", modulePath + "/internal/connection", false},
		{"installed Service scenario consumes real Publisher", "cmd/ardents-next/connection_installed_linux_test.go", modulePath + "/internal/publication/runtime", true},
		{"installed Service scenario excludes direct TLS construction", "cmd/ardents-next/connection_installed_linux_test.go", modulePath + "/internal/route/transport/tls", false},
		{"installed Service scenario excludes direct QUIC construction", "cmd/ardents-next/connection_installed_linux_test.go", modulePath + "/internal/route/transport/quic", false},
		{"bootstrap scenario excludes TLS construction", "cmd/ardents-next/route_bootstrap_linux_test.go", modulePath + "/internal/route/transport/tls", false},
		{"bootstrap scenario excludes QUIC construction", "cmd/ardents-next/route_bootstrap_linux_test.go", modulePath + "/internal/route/transport/quic", false},
		{"unregistered scenario excludes lower contract", "cmd/ardents-next/route_other_linux_test.go", modulePath + "/internal/route/transport", false},
		{"production bootstrap excludes TLS construction", "cmd/ardents-next/route_bootstrap_linux.go", modulePath + "/internal/route/transport/tls", false},
		{"former selector cannot return in caller test", "cmd/ardents-next/route_prefix_caller_linux_test.go", modulePath + "/internal/route/carrier", false},
		{"no production Carrier allowance", "cmd/ardents-next/route_prefix_caller_linux.go", modulePath + "/internal/route/carrier", false},
		{"caller test cannot reach old Carrier", "cmd/ardents-next/route_prefix_caller_linux_test.go", modulePath + "/internal/route/carrier", false},
		{"JOIN cannot retain former selector", "cmd/ardents-next/route_join_linux_test.go", modulePath + "/internal/route/carrier", false},
		{"JOIN production excludes Carrier adapter", "cmd/ardents-next/route_join_linux.go", modulePath + "/internal/route/carrier", false},
		{"JOIN test excludes old Carrier", "cmd/ardents-next/route_join_linux_test.go", modulePath + "/internal/route/carrier", false},
		{"external public-contract test", "internal/admission/issuerprofile/provenance_test.go", modulePath + "/internal/admission/issuerprofile", true},
		{"no production self import", "internal/admission/issuerprofile/profile.go", modulePath + "/internal/admission/issuerprofile", false},
		{"no unregistered test self import", "internal/successor/future/file_test.go", modulePath + "/internal/successor/future", false},
		{"public profile is leaf", "internal/admission/issuerprofile/profile.go", modulePath + "/internal/admission", false},
		{"profile cannot borrow signing", "internal/admission/issuerprofile/profile.go", modulePath + "/internal/admission/issuerprofile/signing", false},
		{"profile tests cannot borrow signing", "internal/admission/issuerprofile/profile_test.go", modulePath + "/internal/admission/issuerprofile/signing", false},
		{"command cannot restore retired signer", "cmd/ardents-next/issuer_profile.go", modulePath + "/internal/successor/nodeidentity", false},
		{"retired signer cannot restore facade", "internal/successor/nodeidentity/store.go", modulePath + "/internal/admission/issuerprofile/signing", false},
		{"identity cannot borrow ledger", "internal/admission/issuerprofile/signing/store.go", modulePath + "/internal/admission", false},
		{"ledger public grammar", "internal/admission/binding.go", modulePath + "/internal/admission/issuerprofile", true},
		{"issuance public grammar", "internal/admission/issuance/profile.go", modulePath + "/internal/admission/issuerprofile", true},
		{"no inherited nested imports", "internal/admission/issuance/nested/file.go", modulePath + "/internal/admission", false},
		{"no inherited nested third party", "internal/admission/issuance/nested/file.go", "github.com/cloudflare/circl/blindsign/blindrsa", false},
		{"custody allocation owner", "internal/custody/admission_authority.go", modulePath + "/internal/admission/allocation", true},
		{"custody excludes old backend", "internal/custody/vault.go", modulePath + "/internal/custody", false},
		{"custody excludes live Instance parser", "internal/custody/authority_state.go", modulePath + "/internal/publication/instance", false},
		{"custody excludes Route", "internal/custody/vault.go", modulePath + "/internal/route", false},
		{"Service command uses genuine new custody", "cmd/ardents-custody/service_authority.go", modulePath + "/internal/custody", true},
		{"unregistered caller", "internal/successor/future/file.go", modulePath + "/internal/admission", false},
		{"unregistered dependency", "cmd/ardents-next/main.go", modulePath + "/internal/successor/future", false},
		{"standard", "internal/admission/check.go", "context", true},
		{"zone", "cmd/ardents-next/main.go", modulePath + "/internal/admission", true},
		{"hosting caller", "cmd/ardents-next/hosting.go", modulePath + "/internal/hosting", true},
		{"independent domains", "internal/hosting/budget.go", modulePath + "/internal/admission", false},
		{"network input adapter", "internal/network/state/membership.go", modulePath + "/internal/network", true},
		{"network adapter is exact", "internal/network/state/other.go", modulePath + "/internal/network", false},
		{"application observation", "internal/network/state/runtime_observation.go", modulePath + "/internal/network", true},
		{"original observation witness", "internal/network/state/runtime_witness.go", modulePath + "/internal/network", true},
		{"retained Route witness", "internal/route/prefix/prefix.go", modulePath + "/internal/network/state", true},
		{"Route witness import is exact", "internal/route/prefix/other.go", modulePath + "/internal/network/state", false},
		{"legacy consumer has no new State", "internal/endpoint/source_state.go", modulePath + "/internal/network/state", false},
		{"State consumer has no domain access", "internal/endpoint/source_state.go", modulePath + "/internal/network", false},
		{"State consumer has no physical root access", "internal/endpoint/source_state.go", modulePath + "/internal/network/state/durable", false},
		{"unregistered State consumer", "internal/service/publication/publication.go", modulePath + "/internal/network/state", false},
		{"State excludes debit", "internal/network/state/open.go", modulePath + "/internal/admission/quota", false},
		{"State excludes old backend", "internal/network/state/open.go", modulePath + "/internal/network/state", false},
		{"legacy admission consumer isolated", "internal/node/authority/admission_observation.go", modulePath + "/internal/admission/receiving", false},
		{"legacy token consumer isolated", "internal/node/authority/token.go", modulePath + "/internal/admission/receiving", false},
		{"observation excludes spending root", "internal/node/authority/admission_observation.go", modulePath + "/internal/admission/spending", false},
		{"observation bridge is exact", "internal/node/authority/receiver.go", modulePath + "/internal/admission/receiving", false},
		{"acquisition policy adapter", "internal/network/state/acquisition_history.go", modulePath + "/internal/network", true},
		{"acquisition adapter is exact", "internal/network/state/attempts.go", modulePath + "/internal/network", false},
		{"participation policy adapter", "internal/network/duty/participation_policy.go", modulePath + "/internal/network", true},
		{"participation adapter is exact", "internal/network/duty/store.go", modulePath + "/internal/network", false},
		{"candidate policy adapter", "internal/network/epoch/candidate_evaluation.go", modulePath + "/internal/network", true},
		{"assignment adapter", "internal/network/epoch/assignment.go", modulePath + "/internal/network", true},
		{"candidate adapter is exact", "internal/network/epoch/record.go", modulePath + "/internal/network", false},
		{"public token grammar adapter", "internal/network/closedprofile/token_spki.go", modulePath + "/internal/admission/issuerprofile", true},
		{"public token adapter is exact", "internal/network/closedprofile/profile.go", modulePath + "/internal/admission/issuerprofile", false},
		{"public token adapter excludes signing", "internal/network/closedprofile/token_spki.go", modulePath + "/internal/admission/issuance", false},
		{"network excludes old state", "internal/network/membership.go", modulePath + "/internal/network/state", false},
		{"network excludes execution", "internal/network/membership.go", modulePath + "/internal/node", false},
		{"network excludes debit", "internal/network/membership.go", modulePath + "/internal/admission", false},
		{"network excludes reservations", "internal/network/membership.go", modulePath + "/internal/hosting", false},
		{"issuance confirmed debit", "internal/admission/issuance/store.go", modulePath + "/internal/admission", true},
		{"operation owners", "internal/admission/issuer/operation.go", modulePath + "/internal/admission/issuance", true},
		{"operation excludes hosting", "internal/admission/issuer/operation.go", modulePath + "/internal/hosting", false},
		{"issuance excludes hosting", "internal/admission/issuance/store.go", modulePath + "/internal/hosting", false},
		{"identity purpose", "internal/admission/issuerprofile/signing/store.go", modulePath + "/internal/admission/issuerprofile", true},
		{"identity cannot borrow issuance", "internal/admission/issuerprofile/signing/store.go", modulePath + "/internal/admission/issuance", false},
		{"admission cannot borrow identity", "internal/admission/issuer_profile.go", modulePath + "/internal/admission/issuerprofile/signing", false},
		{"admission excludes composition", "internal/admission/ledger.go", modulePath + "/internal/admission/issuer", false},
		{"admission cannot borrow keys", "internal/admission/batch.go", modulePath + "/internal/admission/issuance", false},
		{"admission cannot borrow budget", "internal/admission/check.go", modulePath + "/internal/hosting", false},
		{"legacy", "internal/admission/check.go", modulePath + "/internal/admission", false},
		{"legacy test fixture", "internal/admission/check_test.go", modulePath + "/tests/fixtures", false},
		{"third party bridge", "internal/admission/check.go", "example.org/bridge", false},
		{"reverse", "internal/endpoint/runtime.go", modulePath + "/internal/admission", false},
		{"prefix collision", "internal/admission/check.go", modulePath + "/internal/successorold", false},
		{"unrelated", "internal/endpoint/runtime.go", "context", true},
		{"relative", "internal/admission/check.go", "../admission", false},
		{"Source publication control excludes State authority", "internal/release/infrastructure/source_canonical_publication_profile_native_linux_test.go", modulePath + "/internal/network/state", false},
		{"Source startup native prefix excludes State authority", "internal/release/infrastructure/source_startup_physical_prefix_profile_native_linux_test.go", modulePath + "/internal/network/state", false},
		{"Source startup observer excludes State authority", "internal/release/infrastructure/source_startup_physical_observer_native_linux_test.go", modulePath + "/internal/network/state", false},
		{"Source retirement observer excludes State authority", "internal/release/infrastructure/source_retirement_physical_observer_native_linux_test.go", modulePath + "/internal/network/state", false},
		{"Endpoint platform native qualification uses new Enrollment", "cmd/ardents-next/endpoint_platform_native_linux_test.go", modulePath + "/internal/enrollment", true},
		{"Endpoint platform native qualification uses fresh Release", "cmd/ardents-next/endpoint_platform_native_linux_test.go", modulePath + "/internal/release", true},
		{"Endpoint platform native qualification uses generation authorization", "cmd/ardents-next/endpoint_platform_native_linux_test.go", modulePath + "/internal/release/endpoint", true},
		{"Endpoint platform native qualification uses request grammar", "cmd/ardents-next/endpoint_platform_native_linux_test.go", modulePath + "/internal/endpoint/plan", true},
		{"Endpoint platform native qualification uses unit contract", "cmd/ardents-next/endpoint_platform_native_linux_test.go", modulePath + "/internal/endpoint/unit", true},
		{"Endpoint platform native qualification excludes old Endpoint", "cmd/ardents-next/endpoint_platform_native_linux_test.go", modulePath + "/internal/endpoint", false},
		{"Endpoint platform native qualification excludes retired Release path", "cmd/ardents-next/endpoint_platform_native_linux_test.go", modulePath + "/internal/successor/release", false},
		{"Endpoint platform native qualification has no Node runtime", "cmd/ardents-next/endpoint_platform_native_linux_test.go", modulePath + "/internal/node", false},
		{"adjacent Endpoint native test has no inherited Release edge", "cmd/ardents-next/other_endpoint_platform_native_linux_test.go", modulePath + "/internal/release", false},
		{"Endpoint launch delegates Enrollment to platform CLI", "cmd/ardents-next/endpoint_launch_native_linux_test.go", modulePath + "/internal/enrollment", false},
		{"Endpoint launch delegates unit rendering to platform CLI", "cmd/ardents-next/endpoint_launch_native_linux_test.go", modulePath + "/internal/endpoint/unit", false},
		{"Endpoint launch native qualification uses live Source", "cmd/ardents-next/endpoint_launch_native_linux_test.go", modulePath + "/internal/network/source", true},
		{"Endpoint launch native qualification excludes old Endpoint", "cmd/ardents-next/endpoint_launch_native_linux_test.go", modulePath + "/internal/endpoint", false},
		{"Endpoint launch native qualification has no Node runtime", "cmd/ardents-next/endpoint_launch_native_linux_test.go", modulePath + "/internal/node", false},
		{"adjacent Endpoint launch test has no inherited Source edge", "cmd/ardents-next/other_endpoint_launch_native_linux_test.go", modulePath + "/internal/network/source", false},
		{"Endpoint pair native controller is a stdlib-only environment orchestrator", "cmd/ardents-next/endpoint_pair_native_linux_test.go", modulePath + "/internal/enrollment", false},
		{"Endpoint pair native controller excludes old Endpoint", "cmd/ardents-next/endpoint_pair_native_linux_test.go", modulePath + "/internal/endpoint", false},
		{"Endpoint pair native controller has no Node runtime", "cmd/ardents-next/endpoint_pair_native_linux_test.go", modulePath + "/internal/node", false},
		{"Endpoint pair native controller has no direct Source edge", "cmd/ardents-next/endpoint_pair_native_linux_test.go", modulePath + "/internal/network/source", false},
		{"Endpoint journey delegates Enrollment to platform CLI", "cmd/ardents-next/endpoint_journey_native_linux_test.go", modulePath + "/internal/enrollment", false},
		{"Endpoint journey delegates unit rendering to platform CLI", "cmd/ardents-next/endpoint_journey_native_linux_test.go", modulePath + "/internal/endpoint/unit", false},
		{"Endpoint journey native qualification uses live Source", "cmd/ardents-next/endpoint_journey_native_linux_test.go", modulePath + "/internal/network/source", true},
		{"Endpoint journey native qualification names selected Carrier", "cmd/ardents-next/endpoint_journey_native_linux_test.go", modulePath + "/internal/route/transport", true},
		{"Endpoint journey native qualification checks canonical Link codec", "cmd/ardents-next/endpoint_journey_native_linux_test.go", modulePath + "/internal/publication", true},
		{"Endpoint journey native qualification uses public Administration client", "cmd/ardents-next/endpoint_journey_native_linux_test.go", modulePath + "/internal/application/administration", true},
		{"Endpoint journey native qualification uses public Connection client", "cmd/ardents-next/endpoint_journey_native_linux_test.go", modulePath + "/internal/application/connection", true},
		{"Endpoint journey native qualification uses actual text exchange", "cmd/ardents-next/endpoint_journey_native_linux_test.go", modulePath + "/internal/application/document", true},
		{"Endpoint journey native qualification excludes old Endpoint", "cmd/ardents-next/endpoint_journey_native_linux_test.go", modulePath + "/internal/endpoint", false},
		{"Endpoint journey native qualification has no Node runtime", "cmd/ardents-next/endpoint_journey_native_linux_test.go", modulePath + "/internal/node", false},
		{"unsafe", "internal/admission/check.go", "unsafe", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := successorImportAllowed(test.source, test.dependency); got != test.allowed {
				t.Fatalf("allowed = %v, want %v", got, test.allowed)
			}
		})
	}
}

// The retained read-only JOIN repair runtime uses the platform manager. The
// platform-led JOIN lifecycle owns installed delivery (ADR-0125).
func joinRetainedImportAllowed(source, dependency string) bool {
	switch source {
	case "internal/platform/systemd/join_manager_linux.go":
		switch dependency {
		case "github.com/godbus/dbus/v5":
			return true
		}
	}
	return false
}
