package architecture

import (
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path"
	"strconv"
	"strings"
	"testing"
)

// Existing Endpoint/v3 Enrollment grammar and portable behavior controls remain
// selected on both platforms. ADR-0123's separate Source artifact extension is
// Linux-only and checked against its exact genuine consumer profile below.
func TestSuccessorEnrollmentRunsPortableRulesOnWindowsAndLinux(t *testing.T) {
	directory := path.Join(repositoryRoot(t), "internal/successor/enrollment")
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
		case "infrastructure_inventory.go", "infrastructure_inventory_test.go", "infrastructure_process_linux.go":
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

func TestInfrastructureExecutableMechanismHasExactPlatformSelection(t *testing.T) {
	root := repositoryRoot(t)
	for _, target := range []string{"linux", "windows", "darwin"} {
		profile := build.Default
		profile.GOOS, profile.GOARCH, profile.CgoEnabled = target, "amd64", false
		for _, name := range []string{
			"internal/successor/enrollment/infrastructure_inventory.go",
			"internal/successor/enrollment/infrastructure_inventory_test.go",
			"internal/successor/enrollment/infrastructure_process_linux.go",
			"internal/successor/installation/infrastructure/doc.go",
			"internal/successor/installation/infrastructure/artifact.go",
			"internal/successor/installation/infrastructure/admission.go",
			"internal/successor/installation/infrastructure/artifact_test.go",
			"internal/successor/installation/infrastructure/source_request_linux.go",
			"internal/successor/installation/infrastructure/source_plan_linux.go",
			"internal/successor/installation/infrastructure/source_observation_linux.go",
			"internal/successor/installation/infrastructure/source_account_linux.go",
			"internal/successor/installation/infrastructure/source_birth_linux.go",
			"internal/successor/installation/infrastructure/source_install_linux.go",
			"internal/successor/installation/infrastructure/source_replace_linux.go",
			"internal/successor/installation/infrastructure/source_recover_linux.go",
			"internal/successor/installation/infrastructure/source_transition_channel_linux.go",
			"internal/successor/installation/infrastructure/source_transition_controller_linux.go",
			"internal/successor/installation/infrastructure/source_transition_scope_linux.go",
			"internal/successor/installation/infrastructure/source_transition_cut_linux.go",
			"internal/successor/installation/infrastructure/source_transition_process_linux.go",
			"internal/successor/installation/infrastructure/source_transition_origin_linux.go",
			"internal/successor/installation/infrastructure/source_cut_result_linux.go",
			"internal/successor/installation/infrastructure/source_cut_publication_linux.go",
			"internal/successor/installation/infrastructure/source_initial_pending_linux.go",
			"internal/successor/installation/infrastructure/source_initial_pending_records_linux.go",
			"internal/successor/installation/infrastructure/source_initial_pending_records_linux_test.go",
			"internal/successor/installation/infrastructure/source_recovery_prefix_linux.go",
			"internal/successor/installation/infrastructure/source_recovery_completion_linux.go",
			"internal/successor/installation/infrastructure/source_recovery_writing_linux.go",
			"internal/successor/installation/infrastructure/source_recovery_file_linux.go",
			"internal/successor/installation/infrastructure/source_recovery_initial_custody_linux.go",
			"internal/successor/installation/infrastructure/source_recovery_initial_grammar_linux.go",
			"internal/successor/installation/infrastructure/source_recovery_initial_grammar_linux_test.go",
			"internal/successor/installation/infrastructure/source_recovery_records_linux.go",
			"internal/successor/installation/infrastructure/source_recovery_records_linux_test.go",
			"internal/successor/installation/infrastructure/source_recovery_grammar_linux_test.go",
			"internal/successor/installation/infrastructure/source_replacement_candidate_linux.go",
			"internal/successor/installation/infrastructure/source_replacement_history_linux.go",
			"internal/successor/installation/infrastructure/source_replacement_binding_linux.go",
			"internal/successor/installation/infrastructure/source_replacement_binding_linux_test.go",
			"internal/successor/installation/infrastructure/source_replacement_records_linux.go",
			"internal/successor/installation/infrastructure/source_replacement_records_linux_test.go",
			"internal/successor/installation/infrastructure/source_replacement_plan_linux_test.go",
			"internal/successor/installation/infrastructure/source_reload_namespace_linux.go",
			"internal/successor/installation/systemd/source_replacement_linux.go",
			"internal/successor/installation/systemd/source_identity_linux.go",
			"internal/successor/installation/infrastructure/source_cut_history_linux.go",
			"internal/successor/installation/infrastructure/source_recovery_replacement_grammar_linux.go",
			"internal/successor/installation/infrastructure/source_recovery_replacement_readback_linux.go",
			"internal/successor/installation/infrastructure/source_recovery_predecessor_linux.go",
			"internal/successor/installation/infrastructure/source_recover_replacement_linux.go",
			"internal/successor/installation/infrastructure/source_recovery_completed_linux.go",
			"internal/successor/installation/infrastructure/source_binding_linux.go",
			"internal/successor/installation/infrastructure/source_binding_linux_test.go",
			"internal/successor/installation/infrastructure/source_selection_linux.go",
			"internal/successor/installation/infrastructure/source_directory_linux.go",
			"internal/successor/installation/infrastructure/source_history_linux.go",
			"internal/successor/installation/infrastructure/source_initial_linux.go",
			"internal/successor/installation/infrastructure/source_initial_records_linux.go",
			"internal/successor/installation/infrastructure/source_loaded_linux.go",
			"internal/successor/installation/infrastructure/source_policy_linux_test.go",
			"internal/successor/installation/infrastructure/source_running_linux_test.go",
			"internal/successor/installation/infrastructure/source_principal_linux.go",
			"internal/successor/installation/infrastructure/source_principal_linux_test.go",
			"internal/successor/installation/infrastructure/source_runtime_identity_linux.go",
			"internal/successor/installation/infrastructure/source_startup_linux.go",
			"internal/successor/installation/infrastructure/source_startup_linux_test.go",
			"internal/successor/installation/infrastructure/source_namespace_linux.go",
			"internal/successor/installation/infrastructure/source_start_storage_linux.go",
			"internal/successor/installation/infrastructure/source_start_storage_linux_test.go",
			"internal/successor/installation/infrastructure/source_start_records_linux.go",
			"internal/successor/installation/infrastructure/source_start_records_linux_test.go",
			"internal/successor/installation/infrastructure/source_attempt_linux.go",
			"internal/successor/installation/infrastructure/source_attempt_linux_test.go",
			"internal/successor/installation/infrastructure/source_start_history_linux.go",
			"internal/successor/installation/infrastructure/source_retirement_records_linux.go",
			"internal/successor/installation/infrastructure/source_retirement_records_linux_test.go",
			"internal/successor/installation/infrastructure/source_retirement_history_linux.go",
			"internal/successor/installation/infrastructure/source_retirement_exchange_linux.go",
			"internal/successor/installation/infrastructure/source_retirement_exchange_linux_test.go",
			"internal/successor/installation/cgroup/source_quiescence_linux.go",
			"internal/successor/network/state/source_inspection_linux.go",
			"internal/successor/network/state/durable/source_inspection_linux.go",
			"internal/successor/network/duty/source_inspection_linux.go",
			"internal/successor/network/state/retained_open_linux.go",
			"internal/successor/network/state/retained_open_linux_test.go",
			"internal/successor/network/state/durable/retained_open_linux.go",
			"internal/successor/network/state/durable/retained_open_linux_test.go",
			"internal/successor/installation/infrastructure/source_stop_linux.go",
			"internal/successor/installation/infrastructure/source_stop_storage_linux.go",
			"internal/successor/installation/infrastructure/source_stop_storage_linux_test.go",
			"internal/successor/installation/infrastructure/source_start_linux.go",
			"internal/successor/installation/infrastructure/source_start_barrier_linux.go",
			"cmd/ardents-node/source_installed_linux.go",
			"cmd/ardents-node/source_installation_linux.go",
			"cmd/ardents-node/source_output_linux.go",
			"cmd/ardents-node/source_supervision_linux_test.go",
			"cmd/ardents-control/source_state_linux.go",
			"cmd/ardents-control/source_command_linux.go",
			"cmd/ardents-control/source_state_linux_test.go",
			"cmd/ardents-control/source_input_linux.go",
			"cmd/ardents-control/source_output_linux.go",
			"cmd/ardents-control/source_inspection_linux.go",
			"cmd/ardents-control/source_inspection_linux_test.go",
			"cmd/ardents-control/source_artifact_linux.go",
			"cmd/ardents-control/forwarding_artifact_linux.go",
			"cmd/ardents-control/forwarding_request_linux.go",
			"cmd/ardents-control/forwarding_plan_linux.go",
			"cmd/ardents-control/source_artifact_output_linux.go",
			"cmd/ardents-control/source_release_linux.go",
			"cmd/ardents-control/source_release_predecessor_linux.go",
			"cmd/ardents-control/source_release_keys_linux.go",
			"cmd/ardents-control/source_release_plan_linux.go",
			"cmd/ardents-control/source_release_plan_linux_test.go",
			"cmd/ardents-control/source_initial_keys_linux.go",
			"cmd/ardents-control/source_initial_network_linux.go",
			"cmd/ardents-control/source_initial_network_linux_test.go",
			"cmd/ardents-control/source_initial_plan_linux.go",
			"cmd/ardents-control/source_install_inputs_linux.go",
			"cmd/ardents-control/source_signing_key_linux.go",
			"internal/successor/installation/process/source_invocation_linux.go",
			"internal/successor/installation/process/source_invocation_linux_test.go",
			"internal/successor/installation/cgroup/source_scope_linux.go",
			"internal/successor/installation/cgroup/source_scope_linux_test.go",
			"internal/successor/installation/systemd/source_stopped_linux.go",
			"internal/successor/installation/systemd/source_object_linux_test.go",
			"internal/successor/installation/systemd/source_manager_linux.go",
			"internal/successor/installation/systemd/source_control_linux.go",
			"internal/successor/installation/systemd/source_control_frame_linux_test.go",
			"internal/successor/installation/systemd/source_attempt_linux_test.go",
			"internal/successor/installation/infrastructure/source_request_linux_test.go",
			"internal/successor/installation/infrastructure/source_observation_linux_test.go",
			"internal/successor/release/static_metadata.go",
			"internal/successor/release/static_metadata_test.go",
			"internal/successor/release/history_binding_linux.go",
			"internal/successor/release/history_binding_linux_test.go",
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
		for _, tagged := range []bool{false, true} {
			profile.BuildTags = nil
			if tagged {
				profile.BuildTags = []string{"network_signing_native"}
			}
			name := "internal/successor/network/state/source_inspection_native_linux_test.go"
			selected, err := profile.MatchFile(path.Join(root, path.Dir(name)), path.Base(name))
			if err != nil {
				t.Fatal(err)
			}
			if want := target == "linux" && tagged; selected != want {
				t.Errorf("Source inspection native profile on %s tagged=%v: %v, want %v", target, tagged, selected, want)
			}
		}
	}
}

// The shared channel has no native mechanism. Check actual Go file selection,
// including tests: a passing suite must not silently omit an inherited OS file.
func TestSuccessorRouteChannelRunsSameSourcesOnWindowsAndLinux(t *testing.T) {
	directory := path.Join(repositoryRoot(t), "internal/successor/route/channel")
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
	directory := path.Join(repositoryRoot(t), "internal/successor/route/role")
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
	directory := path.Join(repositoryRoot(t), "internal/successor/route/prefix")
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
	directory := path.Join(repositoryRoot(t), "internal/successor/route/join")
	for _, filename := range []string{"pair.go", "relay.go", "pair_test.go", "relay_test.go", "doc.go"} {
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
	directory := path.Join(repositoryRoot(t), "internal/successor/route/join")
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
	directory := path.Join(repositoryRoot(t), "internal/successor/route/receiver")
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
	directory := path.Join(repositoryRoot(t), "internal/successor/route/introduction")
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

// Durable selection owns native root leases, but retained authority checks and
// recipient choices must execute, including their tests, on both platforms.
func TestSuccessorRouteSelectionRulesRunOnWindowsAndLinux(t *testing.T) {
	directory := path.Join(repositoryRoot(t), "internal/successor/route/selection")
	for _, filename := range []string{"leg.go", "leg_test.go", "entry_set.go", "entry_set_test.go", "introduction_recipient.go", "rendezvous.go", "rendezvous_test.go"} {
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
	directory := path.Join(repositoryRoot(t), "internal/successor/route/introduction")
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
		if !strings.HasPrefix(relative, "internal/") && !strings.HasPrefix(relative, "cmd/") && !strings.HasPrefix(relative, "tests/") {
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
		if !strings.HasPrefix(relative, "internal/successor/") && !strings.HasPrefix(relative, "cmd/ardents-next/") {
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

func successorImportAllowed(source, dependency string) bool {
	// Retired offline signer has no facade, consumers or importing source.
	retiredSigner := "internal/successor/nodeidentity"
	if strings.HasPrefix(source, retiredSigner+"/") || dependency == modulePath+"/"+retiredSigner || strings.HasPrefix(dependency, modulePath+"/"+retiredSigner+"/") {
		return false
	}
	inZone := strings.HasPrefix(source, "internal/successor/") || strings.HasPrefix(source, "cmd/ardents-next/")
	zoneDependency := dependency == modulePath+"/internal/successor" || strings.HasPrefix(dependency, modulePath+"/internal/successor/")
	if source == "internal/successor/route/prefix/prefix.go" && dependency == modulePath+"/internal/successor/network/state" {
		return true
	}
	// Network's physical/application adapters consume domain policy through
	// exact files. Directory nesting does not grant policy or Admission access.
	if strings.HasPrefix(source, "internal/successor/network/") {
		sourceAdapter := source
		if sourceAdapter == "internal/successor/network/closedprofile/token_spki.go" && dependency == modulePath+"/internal/successor/admission/issuerprofile" {
			return true
		}
		if sourceAdapter == "internal/successor/network/duty/participation_policy.go" && dependency == modulePath+"/internal/successor/network" {
			return true
		}
		if dependency == modulePath+"/internal/successor/network" {
			switch sourceAdapter {
			case "internal/successor/network/state/accepted_observation.go":
				return inZone
			case "internal/successor/network/state/closed_profile_role_join_test.go", "internal/successor/network/state/closed_runtime_view_test.go":
				return inZone
			case "internal/successor/network/state/acquisition_history.go":
				return true
			case "internal/successor/network/epoch/candidate_evaluation.go", "internal/successor/network/epoch/assignment.go":
				return true
			case "internal/successor/network/state/epoch_history.go", "internal/successor/network/state/membership.go", "internal/successor/network/state/closed_member.go", "internal/successor/network/state/closed_runtime_view.go", "internal/successor/network/state/runtime_observation.go", "internal/successor/network/state/runtime_witness.go", "internal/successor/network/state/clock_observation.go", "internal/successor/network/state/closed_profile_view.go", "internal/successor/network/state/closed_duty_binding.go", "internal/successor/network/state/closed_profile_accept.go":
				return true
			}
		}
	}
	if source == "internal/successor/custody/canonical_envelope.go" && dependency == "golang.org/x/crypto/argon2" {
		return true
	}
	if (source == "internal/successor/custody/vault_lock_windows.go" || source == "internal/successor/custody/durability_windows.go") && dependency == "golang.org/x/sys/windows" {
		return true
	}
	if !inZone {
		if source == "cmd/ardents-control/source_install_inputs_linux.go" && (dependency == modulePath+"/internal/successor/installation/infrastructure" || dependency == modulePath+"/internal/successor/network/source") {
			return true
		}
		if source == "cmd/ardents-control/source_initial_network_native_linux_test.go" && dependency == modulePath+"/internal/successor/network/source" {
			return true
		}
		if source == "cmd/ardents-control/source_initial_network_linux.go" && (dependency == modulePath+"/internal/successor/network/epoch" || dependency == modulePath+"/internal/successor/network/source") {
			return true
		}
		if source == "cmd/ardents-control/source_release_native_linux_test.go" && (dependency == modulePath+"/internal/successor/installation/infrastructure" || dependency == "github.com/theupdateframework/go-tuf/v2/metadata") {
			return true
		}
		if source == "cmd/ardents-control/source_release_coherence_native_linux_test.go" && (dependency == modulePath+"/internal/successor/enrollment" || dependency == modulePath+"/internal/successor/installation/infrastructure" || dependency == modulePath+"/internal/successor/release" || dependency == "github.com/sigstore/sigstore/pkg/signature" || dependency == "github.com/theupdateframework/go-tuf/v2/metadata") {
			return true
		}
		if source == "cmd/ardents-control/source_release_timestamp_native_linux_test.go" && (dependency == modulePath+"/internal/successor/installation/infrastructure" || dependency == modulePath+"/internal/successor/release" || dependency == "github.com/theupdateframework/go-tuf/v2/metadata") {
			return true
		}
		if source == "cmd/ardents-control/source_release_linux.go" && (dependency == modulePath+"/internal/successor/installation/infrastructure" || dependency == modulePath+"/internal/successor/release" || dependency == "github.com/sigstore/sigstore/pkg/signature" || dependency == "github.com/theupdateframework/go-tuf/v2/metadata") {
			return true
		}
		if source == "cmd/ardents-control/source_release_predecessor_linux.go" && (dependency == modulePath+"/internal/successor/enrollment" || dependency == modulePath+"/internal/successor/installation/infrastructure" || dependency == modulePath+"/internal/successor/release" || dependency == "github.com/theupdateframework/go-tuf/v2/metadata") {
			return true
		}
		if source == "cmd/ardents-control/source_release_plan_linux.go" && dependency == modulePath+"/internal/successor/installation/infrastructure" {
			return true
		}
		if source == "cmd/ardents-control/source_artifact_linux.go" && (dependency == modulePath+"/internal/successor/enrollment" || dependency == modulePath+"/internal/successor/installation/infrastructure") {
			return true
		}
		if source == "cmd/ardents-control/forwarding_artifact_linux.go" && (dependency == modulePath+"/internal/successor/enrollment" || dependency == modulePath+"/internal/successor/installation/infrastructure" || dependency == modulePath+"/internal/successor/installation/forwarding") {
			return true
		}
		if source == "cmd/ardents-control/forwarding_state_linux.go" && (dependency == modulePath+"/internal/successor/installation/forwarding" || dependency == modulePath+"/internal/successor/network/source" || dependency == modulePath+"/internal/successor/network/state") {
			return true
		}
		if source == "cmd/ardents-control/forwarding_hosting_linux.go" && (dependency == modulePath+"/internal/successor/hosting" || dependency == modulePath+"/internal/successor/installation/forwarding") {
			return true
		}
		if dependency == modulePath+"/internal/successor/network/epoch" {
			switch source {
			case "cmd/ardents-control/forwarding_node_record_linux.go", "cmd/ardents-control/forwarding_node_certificate_linux.go", "cmd/ardents-control/forwarding_node_provision_linux.go", "cmd/ardents-control/forwarding_node_certificate_linux_test.go", "cmd/ardents-control/forwarding_node_provision_native_linux_test.go":
				return true
			}
		}
		if source == "cmd/ardents-control/forwarding_epoch_plan_linux.go" && dependency == modulePath+"/internal/successor/network/epoch" {
			return true
		}
		if source == "cmd/ardents-control/forwarding_install_inputs_native_linux_test.go" {
			switch dependency {
			case modulePath + "/internal/successor/custody", modulePath + "/internal/successor/installation/forwarding", modulePath + "/internal/successor/network/closedprofile", modulePath + "/internal/successor/network/epoch", modulePath + "/internal/successor/network/source":
				return true
			}
		}
		if source == "cmd/ardents-control/forwarding_token_authority_native_linux_test.go" {
			switch dependency {
			case modulePath + "/internal/successor/admission", modulePath + "/internal/successor/admission/issuer", modulePath + "/internal/successor/admission/quota", modulePath + "/internal/successor/admission/receiving", modulePath + "/internal/successor/admission/stock", modulePath + "/internal/successor/admission/token", modulePath + "/internal/successor/custody", modulePath + "/internal/successor/network", modulePath + "/internal/successor/network/source", modulePath + "/internal/successor/network/state":
				return true
			}
		}
		if source == "cmd/ardents-control/forwarding_profile_plan_linux.go" {
			switch dependency {
			case modulePath + "/internal/successor/network", modulePath + "/internal/successor/network/closedprofile", modulePath + "/internal/successor/network/epoch", modulePath + "/internal/successor/network/source", modulePath + "/internal/successor/admission/issuerprofile":
				return true
			}
		}
		if source == "cmd/ardents-control/forwarding_profile_signing_linux.go" && dependency == modulePath+"/internal/successor/network/closedprofile" {
			return true
		}
		if source == "cmd/ardents-control/forwarding_profile_native_linux_test.go" && (dependency == modulePath+"/internal/successor/network/closedprofile" || dependency == modulePath+"/internal/successor/network/epoch" || dependency == modulePath+"/internal/successor/network/source") {
			return true
		}
		if (source == "cmd/ardents-control/forwarding_token_material_linux.go" || source == "cmd/ardents-control/forwarding_token_material_native_linux_test.go") && dependency == modulePath+"/internal/successor/admission/issuance" {
			return true
		}
		if source == "cmd/ardents-control/forwarding_token_material_native_linux_test.go" && dependency == modulePath+"/internal/successor/admission/issuerprofile" {
			return true
		}
		if (source == "cmd/ardents-control/forwarding_epoch_signing_linux.go" || source == "cmd/ardents-control/forwarding_epoch_provision_native_linux_test.go") && (dependency == modulePath+"/internal/successor/network/epoch" || dependency == modulePath+"/internal/successor/network/source") {
			return true
		}
		if source == "cmd/ardents-control/forwarding_state_linux_test.go" && dependency == modulePath+"/internal/successor/installation/forwarding" {
			return true
		}
		if source == "cmd/ardents-control/forwarding_state_native_linux_test.go" {
			switch dependency {
			case modulePath + "/internal/successor/installation/forwarding", modulePath + "/internal/successor/network/state":
				return true
			}
		}
		if (source == "cmd/ardents-control/forwarding_release_plan_linux.go" || source == "cmd/ardents-control/forwarding_release_native_linux_test.go") && (dependency == modulePath+"/internal/successor/installation/infrastructure" || dependency == modulePath+"/internal/successor/installation/forwarding") {
			return true
		}
		if source == "cmd/ardents-control/forwarding_release_linux.go" && (dependency == modulePath+"/internal/successor/installation/infrastructure" || dependency == modulePath+"/internal/successor/release" || dependency == "github.com/sigstore/sigstore/pkg/signature" || dependency == "github.com/theupdateframework/go-tuf/v2/metadata") {
			return true
		}
		if source == "cmd/ardents-control/forwarding_release_native_linux_test.go" && dependency == "github.com/theupdateframework/go-tuf/v2/metadata" {
			return true
		}
		if source == "cmd/ardents-control/forwarding_request_linux.go" && dependency == modulePath+"/internal/successor/installation/forwarding" {
			return true
		}
		if source == "cmd/ardents-control/forwarding_plan_linux.go" && dependency == modulePath+"/internal/successor/installation/forwarding" {
			return true
		}
		if source == "cmd/ardents-control/forwarding_artifact_native_linux_test.go" && dependency == modulePath+"/internal/successor/installation/forwarding" {
			return true
		}
		if (source == "cmd/ardents-control/source_inspection_linux.go" || source == "cmd/ardents-control/source_inspection_linux_test.go") && dependency == modulePath+"/internal/successor/network/source" {
			return true
		}
		if source == "cmd/ardents-control/source_state_linux.go" || source == "cmd/ardents-control/source_state_linux_test.go" {
			for _, owner := range []string{"network/source", "processresource"} {
				if dependency == modulePath+"/internal/successor/"+owner {
					return true
				}
			}
			if source == "cmd/ardents-control/source_state_linux.go" && dependency == modulePath+"/internal/successor/network/state" {
				return true
			}
		}
		if source == "cmd/ardents-node/forwarding_installed_linux.go" {
			switch dependency {
			case modulePath + "/internal/successor/installation/forwarding", modulePath + "/internal/successor/network", modulePath + "/internal/successor/network/state", modulePath + "/internal/successor/node", modulePath + "/internal/successor/route/role":
				return true
			}
		}
		if source == "cmd/ardents-node/source_supervision_linux_test.go" && dependency == modulePath+"/internal/successor/network/state" {
			return true
		}
		if source == "cmd/ardents-node/source_installed_linux.go" {
			for _, owner := range []string{"installation/infrastructure", "network/source", "network/state", "processresource"} {
				if dependency == modulePath+"/internal/successor/"+owner {
					return true
				}
			}
		}
		if source == "cmd/ardents-node/forwarding_installation_linux.go" && (dependency == modulePath+"/internal/successor/installation/forwarding" || dependency == modulePath+"/internal/successor/installation/infrastructure" || dependency == modulePath+"/internal/successor/enrollment" || dependency == modulePath+"/internal/successor/release") {
			return true
		}
		if source == "cmd/ardents-node/source_installation_linux.go" && (dependency == modulePath+"/internal/successor/network/source" || dependency == modulePath+"/internal/successor/network/state" || dependency == modulePath+"/internal/successor/enrollment" || dependency == modulePath+"/internal/successor/installation/infrastructure" || dependency == modulePath+"/internal/successor/release") {
			return true
		}
		if source == "tests/e2e/service/service_custody_process_linux_test.go" && dependency == modulePath+"/internal/successor/publication" {
			return true
		}
		if dependency == modulePath+"/internal/successor/custody" {
			switch source {
			case "cmd/ardents-custody/command.go", "cmd/ardents-custody/terminal_secret.go", "cmd/ardents-custody/main_test.go", "cmd/ardents-custody/service_authority.go", "cmd/ardents-custody/admission_authority.go", "cmd/ardents-custody/verify_record.go", "cmd/ardents-custody/recovery_bundle.go", "cmd/ardents-custody/purge_record.go", "cmd/ardents-custody/inspect_envelope.go", "cmd/ardents-custody/admission_permission_client_linux_test.go", "internal/endpoint/permission_test.go", "internal/endpoint/issuance_test.go", "internal/endpoint/permission_files_linux_test.go", "internal/endpoint/network_fixture_window_test.go", "internal/endpoint/replacement/custody_nonmutation_test.go":
				return true
			}
		}
		if (source == "cmd/ardents-custody/admission_permission_client_linux_test.go" || source == "tests/e2e/service/admission_custody_process_test.go" || source == "internal/endpoint/permission_request_fixture_test.go" || source == "internal/admission/stock/permission_request_fixture_test.go" || source == "internal/admission/stock/permission_view_test.go") && dependency == modulePath+"/internal/successor/admission" {
			return true
		}
		return !zoneDependency
	}
	if source == "internal/successor/route/introduction/receiving_history_linux.go" && dependency == modulePath+"/internal/successor/admission/receiving" {
		return true
	}
	if source == "internal/successor/installation/forwarding/initial_install_linux.go" && (dependency == modulePath+"/internal/successor/installation/infrastructure" || dependency == modulePath+"/internal/successor/installation/systemd" || dependency == modulePath+"/internal/successor/release") {
		return true
	}
	if source == "internal/successor/installation/forwarding/completed_initial_linux.go" && (dependency == modulePath+"/internal/successor/installation/infrastructure" || dependency == modulePath+"/internal/successor/release") {
		return true
	}
	if source == "internal/successor/installation/forwarding/selected_generation_linux.go" && (dependency == modulePath+"/internal/successor/enrollment" || dependency == modulePath+"/internal/successor/installation/infrastructure") {
		return true
	}
	if source == "internal/successor/installation/forwarding/startup_linux.go" {
		for _, owner := range []string{"cgroup", "completion", "process", "systemd"} {
			if dependency == modulePath+"/internal/successor/installation/"+owner {
				return true
			}
		}
	}
	if source == "internal/successor/installation/forwarding/start_initial_linux.go" {
		if dependency == modulePath+"/internal/successor/release" {
			return true
		}
		for _, owner := range []string{"cgroup", "infrastructure", "process", "systemd"} {
			if dependency == modulePath+"/internal/successor/installation/"+owner {
				return true
			}
		}
	}
	if source == "internal/successor/installation/forwarding/start_barrier_linux.go" && dependency == modulePath+"/internal/successor/installation/completion" {
		return true
	}
	if source == "internal/successor/installation/forwarding/start_initial_native_linux_test.go" && (dependency == modulePath+"/internal/successor/enrollment" || dependency == modulePath+"/internal/successor/installation/infrastructure" || dependency == modulePath+"/internal/successor/release") {
		return true
	}
	if (source == "internal/successor/installation/forwarding/start_snapshot_linux.go" || source == "internal/successor/installation/forwarding/start_snapshot_native_linux_test.go") && dependency == modulePath+"/internal/successor/installation/completion" {
		return true
	}
	if source == "internal/successor/installation/forwarding/started_linux.go" && dependency == modulePath+"/internal/successor/release" {
		return true
	}
	if source == "internal/successor/installation/forwarding/restart_linux.go" && (dependency == modulePath+"/internal/successor/release" || dependency == modulePath+"/internal/successor/installation/infrastructure") {
		return true
	}
	if source == "internal/successor/installation/forwarding/stop_linux.go" {
		switch dependency {
		case modulePath + "/internal/successor/installation/cgroup", modulePath + "/internal/successor/installation/completion", modulePath + "/internal/successor/installation/process", modulePath + "/internal/successor/installation/systemd", modulePath + "/internal/successor/release":
			return true
		}
	}
	if (source == "internal/successor/installation/forwarding/retirement_policy_linux.go" || source == "internal/successor/installation/forwarding/retirement_policy_linux_test.go") && (dependency == modulePath+"/internal/successor/installation/completion" || dependency == modulePath+"/internal/successor/installation/systemd") {
		return true
	}
	if source == "internal/successor/installation/forwarding/retirement_snapshot_native_linux_test.go" && dependency == modulePath+"/internal/successor/installation/completion" {
		return true
	}
	if source == "internal/successor/installation/forwarding/binding_linux.go" && dependency == modulePath+"/internal/successor/installation/infrastructure" {
		return true
	}
	if (source == "internal/successor/installation/forwarding/loaded_policy_linux.go" || source == "internal/successor/installation/forwarding/loaded_policy_linux_test.go") && dependency == modulePath+"/internal/successor/installation/systemd" {
		return true
	}
	if (source == "internal/successor/installation/forwarding/request_linux.go" || source == "internal/successor/installation/forwarding/unit_linux_test.go") && dependency == modulePath+"/internal/successor/installation/infrastructure" {
		return true
	}
	if source == "internal/successor/installation/forwarding/preflight_linux.go" && dependency == modulePath+"/internal/successor/installation/systemd" {
		return true
	}
	if source == "internal/successor/installation/forwarding/plan_linux.go" && dependency == modulePath+"/internal/successor/hosting" {
		return true
	}
	if source == "internal/successor/installation/forwarding/hosting_prepare_linux.go" && (dependency == modulePath+"/internal/successor/hosting" || dependency == modulePath+"/internal/successor/installation/cgroup" || dependency == modulePath+"/internal/successor/installation/systemd") {
		return true
	}
	if source == "internal/successor/installation/forwarding/hosting_prepare_native_linux_test.go" && dependency == modulePath+"/internal/successor/hosting" {
		return true
	}
	if source == "internal/successor/installation/forwarding/hosting_partial_native_linux_test.go" && dependency == modulePath+"/internal/successor/hosting" {
		return true
	}
	// Match exact Go packages. A parent folder grants no child/sibling imports.
	owner := path.Dir(source)
	permitted := map[string][]string{
		"internal/successor/processresource":                 {},
		"internal/successor/node":                            {"admission", "admission/receiving", "hosting", "network", "network/duty", "processresource", "route/ardp", "route/receiver", "route/role"},
		"internal/successor/network":                         {},
		"internal/successor/network/closedprofile":           {},
		"internal/successor/network/duty":                    {},
		"internal/successor/network/epoch":                   {},
		"internal/successor/network/source":                  {"network/epoch"},
		"internal/successor/network/state":                   {"network/closedprofile", "network/duty", "network/epoch", "network/source", "network/state/durable"},
		"internal/successor/network/state/durable":           {},
		"internal/successor/admission":                       {"admission/issuerprofile"},
		"internal/successor/admission/issuerprofile":         {},
		"internal/successor/admission/allocation":            {"admission"},
		"internal/successor/custody":                         {"admission", "admission/allocation", "publication"},
		"internal/successor/admission/attempts":              {},
		"internal/successor/admission/spending":              {},
		"internal/successor/admission/token":                 {"admission", "admission/issuerprofile"},
		"internal/successor/admission/receiving":             {"admission", "admission/token", "admission/spending"},
		"internal/successor/admission/stock":                 {"admission", "admission/allocation", "admission/attempts", "admission/token"},
		"internal/successor/admission/quota":                 {"admission", "admission/issuerprofile"},
		"internal/successor/admission/issuance":              {"admission", "admission/quota", "admission/issuerprofile", "admission/issuerprofile/signing"},
		"internal/successor/admission/issuer":                {"admission", "admission/quota", "admission/issuance", "admission/issuerprofile/signing"},
		"internal/successor/admission/issuerprofile/signing": {"admission/issuerprofile"},
		"internal/successor/hosting":                         {},
		"internal/successor/publication":                     {},
		"internal/successor/publication/durable":             {"publication"},
		"internal/successor/publication/instance":            {"publication", "publication/durable", "reachability", "route/introduction", "route/capsule", "route/join", "connection/wire", "execution/runtime"},
		"internal/successor/publication/runtime":             {"connection", "execution/runtime", "network", "publication", "publication/instance", "reachability", "route", "route/introduction", "route/prefix", "route/capsule", "route/join"},
		"internal/successor/connection":                      {"execution/runtime", "publication", "publication/instance", "route/capsule", "route/join", "connection/wire"},
		"internal/successor/connection/wire":                 {},
		"internal/successor/reachability":                    {"publication"},
		"internal/successor/enrollment":                      {},
		"internal/successor/release":                         {},
		"internal/successor/execution":                       {},
		"internal/successor/execution/runtime":               {"execution", "execution/worker"},
		"internal/successor/execution/worker":                {"installation/systemd", "installation/unit"},
		"internal/successor/installation":                    {"enrollment", "release", "installation/request"},
		"internal/successor/installation/infrastructure":     {"enrollment", "release"},
		"internal/successor/installation/forwarding":         {},
		"internal/successor/installation/endpoint":           {"installation", "enrollment", "release", "installation/cgroup", "installation/systemd", "installation/journal", "installation/process", "installation/generation", "installation/completion", "installation/fixedfile", "installation/unit", "installation/request", "installation/directory"},
		"internal/successor/installation/cgroup":             {},
		"internal/successor/installation/request":            {},
		"internal/successor/installation/directory":          {},
		"internal/successor/installation/systemd":            {},
		"internal/successor/installation/journal":            {},
		"internal/successor/installation/process":            {},
		"internal/successor/installation/generation":         {},
		"internal/successor/installation/completion":         {},
		"internal/successor/installation/fixedfile":          {},
		"internal/successor/installation/unit":               {"installation/systemd"},
		"internal/successor/route":                           {},
		"internal/successor/route/ardp":                      {},
		"internal/successor/route/issuer":                    {"route/ardp", "admission"},
		"internal/successor/route/transport":                 {},
		"internal/successor/route/channel":                   {"route/ardp", "route/transport"},
		"internal/successor/route/bootstrap":                 {"route/ardp", "route/channel"},
		"internal/successor/route/transport/tls":             {"route/transport"},

		"internal/successor/route/introduction": {"admission/spending", "route", "route/ardp", "route/role", "route/prefix", "route/capsule", "network", "admission"},
		"internal/successor/route/capsule":      {},
		"internal/successor/route/selection":    {"route", "network"},
		"internal/successor/route/role":         {"route", "route/ardp", "route/channel", "route/transport", "network", "admission"},
		"internal/successor/route/join":         {"route/prefix", "route", "route/selection", "route/ardp", "route/channel", "route/role", "route/transport", "network", "admission"},
		"internal/successor/route/prefix":       {"route/issuer", "route", "route/selection", "route/ardp", "route/bootstrap", "route/channel", "route/role", "route/transport", "route/transport/tls", "network", "admission"},
		"internal/successor/route/receiver":     {"route/issuer", "route/join", "route", "route/role", "route/transport", "route/transport/tls", "route/channel", "route/bootstrap", "route/ardp", "route/introduction", "network", "admission", "admission/receiving"},
		"cmd/ardents-next":                      {"route/introduction", "route/prefix", "network", "network/state", "admission/stock", "admission/receiving", "admission/allocation", "admission", "admission/quota", "admission/issuerprofile", "admission/issuance", "admission/issuer", "admission/issuerprofile/signing", "hosting", "route", "route/role", "route/selection", "route/join", "route/receiver", "route/channel", "route/ardp"},
	}
	if zoneDependency {
		// Existing new-zone QA registration, not another old-command R01 exception.
		if source == "cmd/ardents-next/forwarding_installed_native_linux_test.go" && (dependency == modulePath+"/internal/successor/installation/forwarding" || dependency == modulePath+"/internal/successor/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/source_state_linux_test.go" && (dependency == modulePath+"/internal/successor/network/source" || dependency == modulePath+"/internal/successor/network/duty") {
			return true
		}
		if source == "cmd/ardents-next/source_failure_linux.go" && dependency == modulePath+"/internal/successor/processresource" {
			return true
		}
		if source == "cmd/ardents-next/source_native_linux_test.go" && (dependency == modulePath+"/internal/successor/network/source" || dependency == modulePath+"/internal/successor/network/epoch" || dependency == modulePath+"/internal/successor/processresource") {
			return true
		}
		if source == "cmd/ardents-next/source_preparation_process_linux_test.go" && dependency == modulePath+"/internal/successor/network/source" {
			return true
		}
		if source == "cmd/ardents-next/node_join_operation_native_linux_test.go" && (dependency == modulePath+"/internal/successor/node" || dependency == modulePath+"/internal/successor/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/node_join_startup_native_linux_test.go" && (dependency == modulePath+"/internal/successor/processresource" || dependency == modulePath+"/internal/successor/network/duty" || dependency == modulePath+"/internal/successor/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/node_join_linux.go" && (dependency == modulePath+"/internal/successor/node" || dependency == modulePath+"/internal/successor/processresource") {
			return true
		}
		if source == "cmd/ardents-next/node_join_native_linux_test.go" && (dependency == modulePath+"/internal/successor/processresource" || dependency == modulePath+"/internal/successor/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/node_join_authority_native_linux_test.go" && (dependency == modulePath+"/internal/successor/node" || dependency == modulePath+"/internal/successor/network/duty" || dependency == modulePath+"/internal/successor/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/node_join_completion_native_linux_test.go" && (dependency == modulePath+"/internal/successor/network/duty" || dependency == modulePath+"/internal/successor/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/node_introduction_operation_native_linux_test.go" && (dependency == modulePath+"/internal/successor/node" || dependency == modulePath+"/internal/successor/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/node_introduction_authority_native_linux_test.go" && (dependency == modulePath+"/internal/successor/node" || dependency == modulePath+"/internal/successor/network/duty" || dependency == modulePath+"/internal/successor/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/node_introduction_completion_native_linux_test.go" && (dependency == modulePath+"/internal/successor/network/duty" || dependency == modulePath+"/internal/successor/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/node_introduction_native_linux_test.go" && (dependency == modulePath+"/internal/successor/network/duty" || dependency == modulePath+"/internal/successor/processresource" || dependency == modulePath+"/internal/successor/route/introduction" || dependency == modulePath+"/internal/successor/route/capsule" || dependency == modulePath+"/internal/successor/route/transport" || dependency == modulePath+"/internal/successor/route/receiver" || dependency == modulePath+"/internal/successor/route/role" || dependency == modulePath+"/internal/successor/route/selection") {
			return true
		}
		if source == "cmd/ardents-next/node_descriptor_completion_native_linux_test.go" && (dependency == modulePath+"/internal/successor/network/duty" || dependency == modulePath+"/internal/successor/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/node_descriptor_operation_native_linux_test.go" && (dependency == modulePath+"/internal/successor/node" || dependency == modulePath+"/internal/successor/reachability" || dependency == modulePath+"/internal/successor/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/node_descriptor_authority_native_linux_test.go" && (dependency == modulePath+"/internal/successor/node" || dependency == modulePath+"/internal/successor/network/duty" || dependency == modulePath+"/internal/successor/reachability" || dependency == modulePath+"/internal/successor/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/node_descriptor_native_linux_test.go" && (dependency == modulePath+"/internal/successor/processresource" || dependency == modulePath+"/internal/successor/network/duty" || dependency == modulePath+"/internal/successor/route/transport" || dependency == modulePath+"/internal/successor/reachability") {
			return true
		}
		if source == "cmd/ardents-next/node_issuer_operation_native_linux_test.go" && (dependency == modulePath+"/internal/successor/node" || dependency == modulePath+"/internal/successor/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/node_issuer_completion_native_linux_test.go" && (dependency == modulePath+"/internal/successor/network/duty" || dependency == modulePath+"/internal/successor/processresource" || dependency == modulePath+"/internal/successor/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/node_issuer_startup_native_linux_test.go" && (dependency == modulePath+"/internal/successor/network/duty" || dependency == modulePath+"/internal/successor/processresource" || dependency == modulePath+"/internal/successor/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/node_issuer_authority_native_linux_test.go" && (dependency == modulePath+"/internal/successor/node" || dependency == modulePath+"/internal/successor/network/duty" || dependency == modulePath+"/internal/successor/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/node_issuer_native_linux_test.go" && (dependency == modulePath+"/internal/successor/processresource" || dependency == modulePath+"/internal/successor/route/transport") {
			return true
		}
		if source == "internal/successor/node/issuer_linux.go" && (dependency == modulePath+"/internal/successor/admission/issuer" || dependency == modulePath+"/internal/successor/admission/quota" || dependency == modulePath+"/internal/successor/route/bootstrap") {
			return true
		}
		if source == "cmd/ardents-next/node_issuer_linux.go" && (dependency == modulePath+"/internal/successor/node" || dependency == modulePath+"/internal/successor/processresource") {
			return true
		}
		if source == "cmd/ardents-next/node_introduction_linux.go" && (dependency == modulePath+"/internal/successor/node" || dependency == modulePath+"/internal/successor/processresource") {
			return true
		}
		if source == "internal/successor/node/introduction_linux.go" && dependency == modulePath+"/internal/successor/route/introduction" {
			return true
		}
		if source == "cmd/ardents-next/node_descriptor_linux.go" && (dependency == modulePath+"/internal/successor/node" || dependency == modulePath+"/internal/successor/processresource") {
			return true
		}
		if source == "internal/successor/node/descriptor_linux.go" && dependency == modulePath+"/internal/successor/reachability" {
			return true
		}
		if source == "cmd/ardents-next/node_failure_linux.go" && dependency == modulePath+"/internal/successor/node" {
			return true
		}
		if source == "cmd/ardents-next/node_forwarding_linux.go" && (dependency == modulePath+"/internal/successor/node" || dependency == modulePath+"/internal/successor/processresource") {
			return true
		}
		if source == "cmd/ardents-next/node_process.go" && dependency == modulePath+"/internal/successor/processresource" {
			return true
		}
		if source == "cmd/ardents-next/node_native_linux_test.go" && (dependency == modulePath+"/internal/successor/processresource" || dependency == modulePath+"/internal/successor/network/duty" || dependency == modulePath+"/internal/successor/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/node_refill_native_linux_test.go" && (dependency == modulePath+"/internal/successor/processresource" || dependency == modulePath+"/internal/successor/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/node_authority_native_linux_test.go" && (dependency == modulePath+"/internal/successor/node" || dependency == modulePath+"/internal/successor/network/duty" || dependency == modulePath+"/internal/successor/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/node_completion_native_linux_test.go" && (dependency == modulePath+"/internal/successor/processresource" || dependency == modulePath+"/internal/successor/network/duty" || dependency == modulePath+"/internal/successor/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/installed_publisher_linux.go" && dependency == modulePath+"/internal/successor/execution/runtime" {
			return true
		}
		if source == "cmd/ardents-next/installed_reader_linux.go" && (dependency == modulePath+"/internal/successor/connection" || dependency == modulePath+"/internal/successor/execution/runtime") {
			return true
		}
		if source == "cmd/ardents-next/connection_response_loss_linux_test.go" && (dependency == modulePath+"/internal/successor/connection" || dependency == modulePath+"/internal/successor/execution/runtime" || dependency == modulePath+"/internal/successor/publication/runtime" || dependency == modulePath+"/internal/successor/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/connection_reader_linux.go" && (dependency == modulePath+"/internal/successor/connection" || dependency == modulePath+"/internal/successor/execution" || dependency == modulePath+"/internal/successor/execution/runtime" || dependency == modulePath+"/internal/successor/reachability") {
			return true
		}
		if source == "internal/successor/publication/runtime/publisher_streams.go" && dependency == modulePath+"/internal/successor/connection/wire" {
			return true
		}
		if source == "cmd/ardents-next/connection_installed_linux_test.go" && (dependency == modulePath+"/internal/successor/connection" || dependency == modulePath+"/internal/successor/execution" || dependency == modulePath+"/internal/successor/execution/runtime" || dependency == modulePath+"/internal/successor/publication/runtime" || dependency == modulePath+"/internal/successor/reachability" || dependency == modulePath+"/internal/successor/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/connection_reopen_linux_test.go" && (dependency == modulePath+"/internal/successor/publication/durable" || dependency == modulePath+"/internal/successor/publication/instance" || dependency == modulePath+"/internal/successor/reachability" || dependency == modulePath+"/internal/successor/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/connection_authority_linux_test.go" && (dependency == modulePath+"/internal/successor/connection" || dependency == modulePath+"/internal/successor/execution/runtime" || dependency == modulePath+"/internal/successor/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/publication_network_linux_test.go" && (dependency == modulePath+"/internal/successor/network/epoch" || dependency == modulePath+"/internal/successor/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/publication_ack_installed_linux_test.go" && (dependency == modulePath+"/internal/successor/execution" || dependency == modulePath+"/internal/successor/execution/runtime" || dependency == modulePath+"/internal/successor/publication" || dependency == modulePath+"/internal/successor/reachability" || dependency == modulePath+"/internal/successor/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/installed_service_linux_test.go" && (dependency == modulePath+"/internal/successor/publication" || dependency == modulePath+"/internal/successor/reachability" || dependency == modulePath+"/internal/successor/route/transport") {
			return true
		}
		if source == "cmd/ardents-next/publication_installed_linux_test.go" && (dependency == modulePath+"/internal/successor/publication" || dependency == modulePath+"/internal/successor/publication/instance" || dependency == modulePath+"/internal/successor/reachability" || dependency == modulePath+"/internal/successor/route/transport" || dependency == modulePath+"/internal/successor/route/capsule") {
			return true
		}
		if source == "cmd/ardents-next/publication_registration_linux_test.go" && (dependency == modulePath+"/internal/successor/publication" || dependency == modulePath+"/internal/successor/publication/durable" || dependency == modulePath+"/internal/successor/publication/instance" || dependency == modulePath+"/internal/successor/reachability" || dependency == modulePath+"/internal/successor/route/capsule") {
			return true
		}
		if (source == "cmd/ardents-next/route_linux.go" || source == "cmd/ardents-next/route_submission_linux_test.go") && dependency == modulePath+"/internal/successor/route/capsule" {
			return true
		}
		if source == "cmd/ardents-next/publication_instance.go" && dependency == modulePath+"/internal/successor/publication/instance" {
			return true
		}
		if source == "cmd/ardents-next/publication_holder_linux.go" && (dependency == modulePath+"/internal/successor/publication/durable" || dependency == modulePath+"/internal/successor/publication/instance" || dependency == modulePath+"/internal/successor/publication/runtime" || dependency == modulePath+"/internal/successor/execution/runtime") {
			return true
		}
		if source == "cmd/ardents-next/publication_holder_other.go" && dependency == modulePath+"/internal/successor/execution/runtime" {
			return true
		}
		if source == "cmd/ardents-next/publication.go" && dependency == modulePath+"/internal/successor/publication/durable" {
			return true
		}
		if (source == "cmd/ardents-next/execution_publisher_linux.go" || source == "cmd/ardents-next/execution_publisher_other.go") && dependency == modulePath+"/internal/successor/execution/runtime" {
			return true
		}
		if source == "cmd/ardents-next/installed_endpoint_linux.go" && (dependency == modulePath+"/internal/successor/installation/endpoint" || dependency == modulePath+"/internal/successor/execution" || dependency == modulePath+"/internal/successor/execution/runtime") {
			return true
		}
		if source == "cmd/ardents-next/source_linux.go" && (dependency == modulePath+"/internal/successor/processresource" || dependency == modulePath+"/internal/successor/network/state") {
			return true
		}
		if source == "cmd/ardents-next/source_preparation_linux.go" && (dependency == modulePath+"/internal/successor/network/source" || dependency == modulePath+"/internal/successor/processresource") {
			return true
		}
		if source == "cmd/ardents-next/installed_source_linux.go" && (dependency == modulePath+"/internal/successor/execution" || dependency == modulePath+"/internal/successor/network/source") {
			return true
		}
		if (source == "cmd/ardents-next/execution_holder.go" || source == "cmd/ardents-next/admission_holder.go") && dependency == modulePath+"/internal/successor/execution/runtime" {
			return true
		}
		if source == "cmd/ardents-next/execution_holder.go" && dependency == modulePath+"/internal/successor/execution" {
			return true
		}
		if (source == "cmd/ardents-next/installation.go" || source == "cmd/ardents-next/installation_test.go") && (dependency == modulePath+"/internal/successor/installation" || dependency == modulePath+"/internal/successor/installation/endpoint" || dependency == modulePath+"/internal/successor/enrollment" || dependency == modulePath+"/internal/successor/release") {
			return true
		}
		if (source == "cmd/ardents-next/infrastructure_linux.go" || source == "cmd/ardents-next/infrastructure_linux_test.go" || source == "cmd/ardents-next/infrastructure_caller_linux_test.go" || source == "cmd/ardents-next/infrastructure_preflight_native_linux_test.go" || source == "cmd/ardents-next/infrastructure_stopped_native_linux_test.go") && (dependency == modulePath+"/internal/successor/installation/infrastructure" || dependency == modulePath+"/internal/successor/enrollment" || dependency == modulePath+"/internal/successor/release") {
			return true
		}
		if source == "cmd/ardents-next/forwarding_artifact_linux_test.go" && (dependency == modulePath+"/internal/successor/installation/infrastructure" || dependency == modulePath+"/internal/successor/enrollment" || dependency == modulePath+"/internal/successor/release") {
			return true
		}
		if source == "cmd/ardents-next/release_process_test.go" && (dependency == modulePath+"/internal/successor/enrollment" || dependency == modulePath+"/internal/successor/release") {
			return true
		}
		if dependency == modulePath+"/internal/successor/enrollment" && (source == "cmd/ardents-next/enrollment.go" || source == "cmd/ardents-next/release.go") {
			return true
		}
		if dependency == modulePath+"/internal/successor/release" && source == "cmd/ardents-next/release.go" {
			return true
		}
		if source == "cmd/ardents-next/route_bootstrap_linux_test.go" && (dependency == modulePath+"/internal/successor/route/issuer" || dependency == modulePath+"/internal/successor/route/bootstrap") {
			return true
		}
		if source == "cmd/ardents-next/route_fixture_linux_test.go" && (dependency == modulePath+"/internal/successor/route/transport/tls") {
			return true
		}
		if dependency == modulePath+"/internal/successor/route/transport" {
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
		if (source == "cmd/ardents-next/network_preparation.go" || source == "cmd/ardents-next/network_preparation_chain_linux_test.go") && (dependency == modulePath+"/internal/successor/network/epoch" || dependency == modulePath+"/internal/successor/network/closedprofile") {
			return true
		}
		if source == "cmd/ardents-next/network_signing.go" && (dependency == modulePath+"/internal/successor/network/epoch" || dependency == modulePath+"/internal/successor/network/closedprofile") {
			return true
		}
		if source == "cmd/ardents-next/node_certificate_linux.go" && dependency == modulePath+"/internal/successor/network/epoch" {
			return true
		}
		if (source == "cmd/ardents-next/network_profile_signing.go" || source == "cmd/ardents-next/network_profile_signing_test.go") && dependency == modulePath+"/internal/successor/network/closedprofile" {
			return true
		}
		if source == "cmd/ardents-next/network_admission_fixture_linux_test.go" && dependency == modulePath+"/internal/successor/network/epoch" {
			return true
		}
		if owner == "cmd/ardents-next" && strings.HasSuffix(source, "_test.go") && (dependency == modulePath+"/internal/successor/admission/token" || dependency == modulePath+"/internal/successor/admission/spending") {
			return true
		}
		if source == "cmd/ardents-next/route_token_preparation_linux.go" && dependency == modulePath+"/internal/successor/admission/token" {
			return true
		}
		if (source == "cmd/ardents-next/route_issuer_linux.go" || source == "cmd/ardents-next/route_token_linux.go") && (dependency == modulePath+"/internal/successor/admission/token" || dependency == modulePath+"/internal/successor/route/bootstrap") {
			return true
		}
		if dependency == modulePath+"/internal/successor/reachability" && (source == "internal/successor/route/prefix/descriptor_exchange.go" || source == "internal/successor/route/receiver/receiver_linux.go" || source == "internal/successor/route/receiver/descriptor_linux.go" || source == "cmd/ardents-next/route_linux.go" || source == "cmd/ardents-next/route.go" || source == "cmd/ardents-next/admission_holder.go" || source == "cmd/ardents-next/route_descriptor_linux_test.go") {
			return true
		}
		if (source == "internal/successor/admission/stock/issuance_fixture_test.go" || source == "internal/successor/admission/stock/lifecycle_test.go" || source == "internal/successor/admission/stock/receiving_cycle_test.go") && (dependency == modulePath+"/internal/successor/admission/issuer" || dependency == modulePath+"/internal/successor/admission/issuance" || dependency == modulePath+"/internal/successor/admission/quota") {
			return true
		}
		if (source == "internal/successor/admission/stock/cycle_test.go" || source == "internal/successor/admission/stock/receiving_cycle_test.go") && (dependency == modulePath+"/internal/successor/admission/receiving" || dependency == modulePath+"/internal/successor/admission/spending") {
			return true
		}
		if source == "internal/successor/connection/publisher_recovery.go" && dependency == modulePath+"/internal/successor/route/prefix" {
			return true
		}
		if source == "internal/successor/connection/initiator_source.go" {
			for _, owned := range []string{"network", "reachability", "route", "route/prefix", "route/selection", "route/introduction"} {
				if dependency == modulePath+"/internal/successor/"+owned {
					return true
				}
			}
		}
		if source == "internal/successor/installation/infrastructure/source_plan_linux.go" && dependency == modulePath+"/internal/successor/network/source" {
			return true
		}
		if source == "internal/successor/installation/infrastructure/source_startup_linux.go" && (dependency == modulePath+"/internal/successor/installation/process" || dependency == modulePath+"/internal/successor/installation/cgroup" || dependency == modulePath+"/internal/successor/installation/completion" || dependency == modulePath+"/internal/successor/installation/systemd") {
			return true
		}
		if source == "internal/successor/installation/infrastructure/source_start_linux.go" && (dependency == modulePath+"/internal/successor/installation/process" || dependency == modulePath+"/internal/successor/installation/cgroup" || dependency == modulePath+"/internal/successor/installation/systemd") {
			return true
		}
		if source == "internal/successor/installation/infrastructure/source_replace_linux.go" && (dependency == modulePath+"/internal/successor/network/state" || dependency == modulePath+"/internal/successor/installation/fixedfile" || dependency == modulePath+"/internal/successor/installation/cgroup" || dependency == modulePath+"/internal/successor/installation/systemd") {
			return true
		}
		if source == "internal/successor/installation/infrastructure/source_canonical_publication_profile_native_linux_test.go" && dependency == modulePath+"/internal/successor/installation/fixedfile" {
			return true
		}
		if (source == "internal/successor/installation/infrastructure/source_recover_linux.go" || source == "internal/successor/installation/infrastructure/source_initial_linux.go") && dependency == modulePath+"/internal/successor/installation/systemd" {
			return true
		}
		if source == "internal/successor/installation/infrastructure/source_recover_linux.go" && dependency == modulePath+"/internal/successor/network/state" {
			return true
		}
		if source == "internal/successor/installation/infrastructure/source_recover_replacement_linux.go" && (dependency == modulePath+"/internal/successor/release" || dependency == modulePath+"/internal/successor/network/state" || dependency == modulePath+"/internal/successor/installation/systemd") {
			return true
		}
		if source == "internal/successor/installation/infrastructure/source_recovery_completed_linux.go" && (dependency == modulePath+"/internal/successor/installation/cgroup" || dependency == modulePath+"/internal/successor/installation/systemd") {
			return true
		}
		if source == "internal/successor/installation/infrastructure/source_start_barrier_linux.go" && dependency == modulePath+"/internal/successor/installation/completion" {
			return true
		}
		if source == "internal/successor/installation/infrastructure/source_startup_physical_prefix_profile_native_linux_test.go" && dependency == modulePath+"/internal/successor/installation/completion" {
			return true
		}
		if source == "internal/successor/installation/infrastructure/source_startup_physical_observer_native_linux_test.go" && dependency == modulePath+"/internal/successor/installation/completion" {
			return true
		}
		if source == "internal/successor/installation/infrastructure/source_retirement_physical_observer_native_linux_test.go" && dependency == modulePath+"/internal/successor/installation/completion" {
			return true
		}
		if source == "internal/successor/installation/infrastructure/source_start_history_linux.go" && dependency == modulePath+"/internal/successor/installation/completion" {
			return true
		}
		if source == "internal/successor/installation/infrastructure/source_retirement_history_linux.go" && (dependency == modulePath+"/internal/successor/installation/completion" || dependency == modulePath+"/internal/successor/installation/systemd") {
			return true
		}
		if source == "internal/successor/installation/infrastructure/source_stop_linux.go" && (dependency == modulePath+"/internal/successor/installation/process" || dependency == modulePath+"/internal/successor/installation/cgroup" || dependency == modulePath+"/internal/successor/installation/completion" || dependency == modulePath+"/internal/successor/installation/systemd") {
			return true
		}
		if (source == "internal/successor/installation/infrastructure/source_observation_linux.go" || source == "internal/successor/installation/infrastructure/source_install_linux.go" || source == "internal/successor/installation/infrastructure/source_loaded_linux.go" || source == "internal/successor/installation/infrastructure/source_policy_linux_test.go" || source == "internal/successor/installation/infrastructure/source_running_linux_test.go") && dependency == modulePath+"/internal/successor/installation/systemd" {
			return true
		}
		if source == "cmd/ardents-next/source_input_linux.go" && dependency == modulePath+"/internal/successor/network/source" {
			return true
		}
		// A registered package's external behavior tests may import that package.
		// This grants neither another package nor nested tests its imports.
		if _, registered := permitted[owner]; registered && strings.HasSuffix(source, "_test.go") && dependency == modulePath+"/"+owner {
			return true
		}
		for _, allowed := range permitted[owner] {
			if dependency == modulePath+"/internal/successor/"+allowed {
				return true
			}
		}
		return false
	}
	if (source == "cmd/ardents-next/connection_reader_linux.go" || source == "cmd/ardents-next/installed_reader_linux.go") && dependency == modulePath+"/internal/application/textdocument" {
		return true
	}
	if (source == "cmd/ardents-next/installed_reader_linux.go" || source == "cmd/ardents-next/installed_endpoint_linux.go") && dependency == modulePath+"/internal/application/connection" {
		return true
	}
	if (source == "cmd/ardents-next/installed_endpoint_linux.go" || source == "cmd/ardents-next/installed_service_linux_test.go") && dependency == modulePath+"/internal/application/administration" {
		return true
	}
	if source == "cmd/ardents-next/installed_pair_linux_test.go" {
		switch dependency {
		case modulePath + "/internal/application/administration", modulePath + "/internal/application/connection", modulePath + "/internal/application/textdocument":
			return true
		}
	}
	if source == "internal/successor/execution/runtime/initialization_linux.go" && dependency == modulePath+"/internal/application/textdocument" {
		return true
	}
	if (source == "internal/successor/execution/runtime/publisher_worker_linux.go" || source == "internal/successor/execution/runtime/publisher_worker_linux_test.go") && dependency == modulePath+"/internal/application/textdocument" {
		return true
	}
	if source == "internal/successor/execution/runtime/reader_worker_linux.go" && dependency == modulePath+"/internal/application/textdocument" {
		return true
	}
	if (source == "internal/successor/execution/runtime/reader_worker_linux.go" || source == "internal/successor/execution/runtime/reader_stream.go") && dependency == modulePath+"/internal/application/connection" {
		return true
	}
	if (source == "internal/successor/execution/runtime/publisher_worker_linux.go" || source == "internal/successor/execution/runtime/publisher_stream.go") && dependency == modulePath+"/internal/application/connection" {
		return true
	}
	if (source == "internal/successor/connection/stream.go" || source == "internal/successor/connection/stream_test.go") && dependency == modulePath+"/internal/application/connection" {
		return true
	}
	if (source == "cmd/ardents-next/connection_response_loss_linux_test.go" || source == "cmd/ardents-next/connection_authority_linux_test.go") && dependency == modulePath+"/internal/application/connection" {
		return true
	}
	if source == "cmd/ardents-next/execution_publisher_linux.go" && dependency == modulePath+"/internal/application/textdocument" {
		return true
	}
	if (owner == "internal/successor/admission/issuance" || owner == "internal/successor/admission/token") && dependency == "github.com/cloudflare/circl/blindsign/blindrsa" {
		return true
	}
	if (source == "internal/successor/installation/systemd/manager_reference_linux.go" || source == "internal/successor/installation/systemd/forwarding_stopped_linux.go" || source == "internal/successor/installation/systemd/forwarding_manager_linux.go" || source == "internal/successor/installation/systemd/forwarding_control_linux.go" || source == "internal/successor/installation/systemd/endpoint_reference_linux.go" || source == "internal/successor/installation/systemd/startup_observation_linux.go" || source == "internal/successor/installation/systemd/source_preflight_linux.go" || source == "internal/successor/installation/systemd/source_stopped_linux.go" || source == "internal/successor/installation/systemd/source_manager_linux.go" || source == "internal/successor/installation/systemd/source_control_linux.go" || source == "internal/successor/installation/systemd/source_reference_native_linux_test.go") && dependency == "github.com/godbus/dbus/v5" {
		return true
	}
	if (owner == "internal/successor/admission/spending" || owner == "internal/successor/network/duty" || owner == "internal/successor/network/state/durable") && dependency == "golang.org/x/sys/windows" {
		return true
	}
	if source == "internal/successor/reachability/store_platform_windows.go" && dependency == "golang.org/x/sys/windows" {
		return true
	}
	if source == "internal/successor/publication/durable/lease_windows.go" && dependency == "golang.org/x/sys/windows" {
		return true
	}
	if source == "internal/successor/release/history_native_windows.go" && dependency == "golang.org/x/sys/windows" {
		return true
	}
	if owner == "internal/successor/release" && dependency == "github.com/theupdateframework/go-tuf/v2/metadata" {
		switch source {
		case "internal/successor/release/metadata_authentication.go", "internal/successor/release/target_identity.go", "internal/successor/release/safety_policy.go", "internal/successor/release/floor_encoding.go":
			return true
		}
	}
	if source == "internal/successor/release/metadata_authentication.go" && dependency == "github.com/theupdateframework/go-tuf/v2/metadata/trustedmetadata" {
		return true
	}
	if strings.HasSuffix(source, "_test.go") {
		if source == "internal/successor/release/generation_authorization_test.go" && dependency == "github.com/theupdateframework/go-tuf/v2/metadata" {
			return true
		}
		if source == "internal/successor/release/rotation_test.go" && (dependency == "github.com/sigstore/sigstore/pkg/signature" || dependency == "github.com/theupdateframework/go-tuf/v2/metadata") {
			return true
		}
		if (source == "cmd/ardents-next/release_process_test.go" || source == "cmd/ardents-next/infrastructure_linux_test.go") && (dependency == "github.com/sigstore/sigstore/pkg/signature" || dependency == "github.com/theupdateframework/go-tuf/v2/metadata") {
			return true
		}
		if source == "cmd/ardents-next/forwarding_artifact_linux_test.go" && dependency == "github.com/theupdateframework/go-tuf/v2/metadata" {
			return true
		}
		if (owner == "internal/successor/network/epoch" || owner == "internal/successor/network/state" || owner == "cmd/ardents-next") && dependency == modulePath+"/tests/epochfixture/network" {
			return true
		}
		if owner == "internal/successor/network/epoch" && dependency == modulePath+"/tests/epochfixture/assignment" {
			return true
		}
	}

	if source == "internal/successor/installation/infrastructure/source_transition_scope_linux.go" && dependency == "golang.org/x/sys/unix" {
		return true
	}
	if source == "cmd/ardents-next/forwarding_installed_native_linux_test.go" && dependency == "golang.org/x/sys/unix" {
		return true
	}
	// OTel composition is confined to the exact command package.
	// No arbitrary third-party bridge is permitted.
	if owner == "cmd/ardents-next" {
		for _, allowed := range []string{
			"go.opentelemetry.io/otel/attribute",
			"go.opentelemetry.io/otel/metric",
			"go.opentelemetry.io/otel/sdk/metric",
			"go.opentelemetry.io/otel/sdk/metric/exemplar",
			"go.opentelemetry.io/otel/sdk/metric/metricdata",
			"go.opentelemetry.io/otel/sdk/resource",
			"go.opentelemetry.io/otel/sdk/trace",
			"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp",
			"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp",
		} {
			if dependency == allowed {
				return true
			}
		}
		if strings.HasSuffix(source, "_test.go") {
			for _, allowed := range []string{"github.com/cloudflare/circl/blindsign/blindrsa", "go.opentelemetry.io/proto/otlp/collector/trace/v1", "go.opentelemetry.io/proto/otlp/collector/metrics/v1", "google.golang.org/protobuf/proto"} {
				if dependency == allowed {
					return true
				}
			}
		}
	}
	// Go reserves first import-path elements without a dot for standard-library
	// paths. Unknown paths still fail compilation; relative paths are forbidden.
	first := strings.Split(dependency, "/")[0]
	return dependency != "C" && dependency != "unsafe" && dependency != "" && !strings.Contains(first, ".") && path.Clean(dependency) == dependency && !strings.HasPrefix(dependency, "/")
}

func TestSuccessorIsolationPolicy(t *testing.T) {
	for _, test := range []struct {
		name, source, dependency string
		allowed                  bool
	}{
		{"new installed Forwarding test reads pure declaration", "cmd/ardents-next/forwarding_installed_native_linux_test.go", modulePath + "/internal/successor/installation/forwarding", true},
		{"new installed Forwarding test selects shared Carrier", "cmd/ardents-next/forwarding_installed_native_linux_test.go", modulePath + "/internal/successor/route/transport", true},
		{"new installed Forwarding test observes original pidfd", "cmd/ardents-next/forwarding_installed_native_linux_test.go", "golang.org/x/sys/unix", true},
		{"unregistered new test has no installed declaration edge", "cmd/ardents-next/other_forwarding_linux_test.go", modulePath + "/internal/successor/installation/forwarding", false},
		{"unregistered new test has no pidfd edge", "cmd/ardents-next/other_forwarding_linux_test.go", "golang.org/x/sys/unix", false},
		{"old Node test remains outside new Route", "cmd/ardents-node/forwarding_installed_native_linux_test.go", modulePath + "/internal/successor/route/transport", false},
		{"new installed Forwarding test cannot bypass canonical Node", "cmd/ardents-next/forwarding_installed_native_linux_test.go", modulePath + "/internal/successor/node", false},
		{"new installed Forwarding test cannot construct concrete TLS", "cmd/ardents-next/forwarding_installed_native_linux_test.go", modulePath + "/internal/successor/route/transport/tls", false},
		{"delayed ACK fixture retains genuine installed operation", "cmd/ardents-next/publication_ack_installed_linux_test.go", modulePath + "/internal/successor/execution/runtime", true},
		{"receiving Network fixture selects accepted profile", "cmd/ardents-next/publication_network_linux_test.go", modulePath + "/internal/successor/network/epoch", true},
		{"receiving Network fixture names selected Carriers", "cmd/ardents-next/publication_network_linux_test.go", modulePath + "/internal/successor/route/transport", true},
		{"receiving Network fixture has no concrete TLS adapter", "cmd/ardents-next/publication_network_linux_test.go", modulePath + "/internal/successor/route/transport/tls", false},
		{"receiving Network fixture has no concrete QUIC adapter", "cmd/ardents-next/publication_network_linux_test.go", modulePath + "/internal/successor/route/transport/quic", false},
		{"receiving Network fixture grants no Publisher runtime", "cmd/ardents-next/publication_network_linux_test.go", modulePath + "/internal/successor/publication/runtime", false},
		{"delayed ACK fixture invokes command composition", "cmd/ardents-next/publication_ack_installed_linux_test.go", modulePath + "/internal/successor/publication/runtime", false},
		{"delayed ACK fixture cannot reserve durable authority", "cmd/ardents-next/publication_ack_installed_linux_test.go", modulePath + "/internal/successor/publication/durable", false},
		{"installed Service scenario checks public credential codec", "cmd/ardents-next/installed_service_linux_test.go", modulePath + "/internal/successor/publication", true},
		{"installed Service scenario uses public Administration client", "cmd/ardents-next/installed_service_linux_test.go", modulePath + "/internal/application/administration", true},
		{"installed pair uses public Administration client", "cmd/ardents-next/installed_pair_linux_test.go", modulePath + "/internal/application/administration", true},
		{"installed pair uses public Connection client", "cmd/ardents-next/installed_pair_linux_test.go", modulePath + "/internal/application/connection", true},
		{"installed pair uses actual text exchange", "cmd/ardents-next/installed_pair_linux_test.go", modulePath + "/internal/application/textdocument", true},
		{"installed pair cannot bypass Publication runtime", "cmd/ardents-next/installed_pair_linux_test.go", modulePath + "/internal/successor/publication/runtime", false},
		{"installed pair cannot import old Endpoint", "cmd/ardents-next/installed_pair_linux_test.go", modulePath + "/internal/endpoint", false},
		{"installed Service scenario cannot use old Endpoint", "cmd/ardents-next/installed_service_linux_test.go", modulePath + "/internal/endpoint", false},
		{"installed Service scenario owns receiving Store fixture", "cmd/ardents-next/installed_service_linux_test.go", modulePath + "/internal/successor/reachability", true},
		{"installed Service scenario selects exact Carrier", "cmd/ardents-next/installed_service_linux_test.go", modulePath + "/internal/successor/route/transport", true},
		{"installed Service scenario cannot supply private Instance", "cmd/ardents-next/installed_service_linux_test.go", modulePath + "/internal/successor/publication/instance", false},
		{"installed Service scenario cannot bypass runtime command", "cmd/ardents-next/installed_service_linux_test.go", modulePath + "/internal/successor/publication/runtime", false},
		{"native Publisher scenario independently checks public proof", "cmd/ardents-next/publication_installed_linux_test.go", modulePath + "/internal/successor/publication", true},
		{"native Publisher scenario prepares genuine host Instance", "cmd/ardents-next/publication_installed_linux_test.go", modulePath + "/internal/successor/publication/instance", true},
		{"native Publisher scenario reopens real receiving Store", "cmd/ardents-next/publication_installed_linux_test.go", modulePath + "/internal/successor/reachability", true},
		{"native Publisher scenario names selected Carrier", "cmd/ardents-next/publication_installed_linux_test.go", modulePath + "/internal/successor/route/transport", true},
		{"native Publisher scenario cannot bypass command lifecycle", "cmd/ardents-next/publication_installed_linux_test.go", modulePath + "/internal/successor/publication/runtime", false},
		{"native Publisher scenario has no direct durable reservation", "cmd/ardents-next/publication_installed_linux_test.go", modulePath + "/internal/successor/publication/durable", false},
		{"native Publisher scenario excludes concrete TLS adapter", "cmd/ardents-next/publication_installed_linux_test.go", modulePath + "/internal/successor/route/transport/tls", false},
		{"native Publisher scenario excludes concrete QUIC adapter", "cmd/ardents-next/publication_installed_linux_test.go", modulePath + "/internal/successor/route/transport/quic", false},
		{"another native test has no Publisher exception", "cmd/ardents-next/unregistered_installed_linux_test.go", modulePath + "/internal/successor/publication/instance", false},
		{"installed Reader retains original worker", "cmd/ardents-next/installed_reader_linux.go", modulePath + "/internal/successor/execution/runtime", true},
		{"installed Reader consumes authenticated Source", "cmd/ardents-next/installed_reader_linux.go", modulePath + "/internal/successor/connection", true},
		{"installed Reader local byte contract", "cmd/ardents-next/installed_reader_linux.go", modulePath + "/internal/application/connection", true},
		{"installed Reader local Text projection", "cmd/ardents-next/installed_reader_linux.go", modulePath + "/internal/application/textdocument", true},
		{"installed participant owns Reader listener", "cmd/ardents-next/installed_endpoint_linux.go", modulePath + "/internal/application/connection", true},
		{"installed participant owns Administration listener", "cmd/ardents-next/installed_endpoint_linux.go", modulePath + "/internal/application/administration", true},
		{"installed Publisher retains original worker", "cmd/ardents-next/installed_publisher_linux.go", modulePath + "/internal/successor/execution/runtime", true},
		{"other installed adapter has no Publisher authority", "cmd/ardents-next/installed_other_linux.go", modulePath + "/internal/successor/execution/runtime", false},
		{"installed Publisher cannot use old Endpoint", "cmd/ardents-next/installed_publisher_linux.go", modulePath + "/internal/endpoint", false},
		{"unregistered installed adapter has no local byte exception", "cmd/ardents-next/other_installed_linux.go", modulePath + "/internal/application/connection", false},
		{"installed Reader cannot use old Connection", "cmd/ardents-next/installed_reader_linux.go", modulePath + "/internal/service/connection", false},
		{"qualified Publisher uses only native Text protocol", "internal/successor/execution/runtime/publisher_worker_linux.go", modulePath + "/internal/application/textdocument", true},
		{"qualified Reader uses only native Text protocol", "internal/successor/execution/runtime/reader_worker_linux.go", modulePath + "/internal/application/textdocument", true},
		{"qualified Reader input uses only byte-stream boundary", "internal/successor/execution/runtime/reader_stream.go", modulePath + "/internal/application/connection", true},
		{"qualified Publisher uses Application stream grammar", "internal/successor/execution/runtime/publisher_worker_linux.go", modulePath + "/internal/application/connection", true},
		{"selected Publisher input uses Text import", "cmd/ardents-next/execution_publisher_linux.go", modulePath + "/internal/application/textdocument", true},
		{"Execution launch borrows only Text INIT grammar", "internal/successor/execution/runtime/initialization_linux.go", modulePath + "/internal/application/textdocument", true},
		{"Execution rules cannot borrow Text runtime", "internal/successor/execution/job.go", modulePath + "/internal/application/textdocument", false},
		{"common Execution launch has no native Text exception", "internal/successor/execution/runtime/launch.go", modulePath + "/internal/application/textdocument", false},
		{"other Execution composition has no Text exception", "internal/successor/execution/runtime/other_linux.go", modulePath + "/internal/application/textdocument", false},
		{"Execution cannot borrow old Endpoint authority", "internal/successor/execution/runtime/initialization_linux.go", modulePath + "/internal/endpoint", false},
		{"Execution cannot borrow old Broker", "internal/successor/execution/authority.go", modulePath + "/internal/application/broker", false},
		{"old Endpoint cannot consume new Execution", "internal/endpoint/worker_grant_linux.go", modulePath + "/internal/successor/execution", false},
		{"native original manager reference owns bus client", "internal/successor/installation/systemd/endpoint_reference_linux.go", "github.com/godbus/dbus/v5", true},
		{"fresh Startup manager observation owns bus client", "internal/successor/installation/systemd/startup_observation_linux.go", "github.com/godbus/dbus/v5", true},
		{"native Source reference regression checks genuine bus refusal", "internal/successor/installation/systemd/source_reference_native_linux_test.go", "github.com/godbus/dbus/v5", true},
		{"manager value validation excludes bus client", "internal/successor/installation/systemd/manager_values_linux.go", "github.com/godbus/dbus/v5", false},
		{"manager transport excludes bus client", "internal/successor/installation/systemd/reference_transport_linux.go", "github.com/godbus/dbus/v5", false},
		{"Installation decisions exclude bus client", "internal/successor/installation/endpoint/predecessor_linux.go", "github.com/godbus/dbus/v5", false},
		{"portable authorization cannot own installed Endpoint", "internal/successor/installation/generation_authentication.go", modulePath + "/internal/successor/installation/endpoint", false},
		{"portable authorization cannot own physical root", "internal/successor/installation/generation_authentication.go", modulePath + "/internal/successor/installation/cgroup", false},
		{"installed Endpoint consumes opaque authorization", "internal/successor/installation/endpoint/generation_authorization.go", modulePath + "/internal/successor/installation", true},
		{"unregistered manager file excludes bus client", "internal/successor/installation/systemd/other_linux.go", "github.com/godbus/dbus/v5", false},
		{"Reachability verifies public delegation", "internal/successor/reachability/descriptor.go", modulePath + "/internal/successor/publication", true},
		{"Prefix completes genuine private lookup", "internal/successor/route/prefix/descriptor_exchange.go", modulePath + "/internal/successor/reachability", true},
		{"Receiver uses genuine Descriptor Store", "internal/successor/route/receiver/descriptor_linux.go", modulePath + "/internal/successor/reachability", true},
		{"Carrier cannot acquire Descriptor authority", "internal/successor/route/transport/tls/node.go", modulePath + "/internal/successor/reachability", false},
		{"Reachability cannot own physical Prefix", "internal/successor/reachability/lookup_history.go", modulePath + "/internal/successor/route/prefix", false},
		{"Store native Windows lease and sync", "internal/successor/reachability/store_platform_windows.go", "golang.org/x/sys/windows", true},
		{"Descriptor rules cannot use native Windows mechanisms", "internal/successor/reachability/descriptor.go", "golang.org/x/sys/windows", false},
		{"Store rules cannot use native Windows mechanisms", "internal/successor/reachability/store.go", "golang.org/x/sys/windows", false},
		{"public proof cannot acquire Route transport", "internal/successor/publication/public_proof.go", modulePath + "/internal/successor/route/ardp", false},
		{"public proof cannot acquire Store", "internal/successor/publication/public_proof.go", modulePath + "/internal/successor/reachability", false},
		{"Reachability cannot delegate to old verifier", "internal/successor/reachability/descriptor.go", modulePath + "/internal/service/reachability", false},
		{"public proof cannot delegate to old Publication", "internal/successor/publication/public_proof.go", modulePath + "/internal/service/publication", false},
		{"physical implementation uses shared contract", "internal/successor/route/transport/tls/node.go", modulePath + "/internal/successor/route/transport", true},
		{"Prefix uses shared contract", "internal/successor/route/prefix/prefix.go", modulePath + "/internal/successor/route/transport", true},
		{"Prefix uses portable framing", "internal/successor/route/prefix/prefix.go", modulePath + "/internal/successor/route/channel", true},
		{"channel uses grammar", "internal/successor/route/channel/session.go", modulePath + "/internal/successor/route/ardp", true},
		{"bootstrap consumes framing capacity", "internal/successor/route/bootstrap/budget.go", modulePath + "/internal/successor/route/channel", true},
		{"bootstrap describes canonical restriction", "internal/successor/route/bootstrap/budget.go", modulePath + "/internal/successor/route/ardp", true},
		{"receiver reserves bootstrap capacity", "internal/successor/route/receiver/receiver_linux.go", modulePath + "/internal/successor/route/bootstrap", true},
		{"channel cannot acquire bootstrap policy", "internal/successor/route/channel/session.go", modulePath + "/internal/successor/route/bootstrap", false},
		{"bootstrap cannot spend Admission", "internal/successor/route/bootstrap/budget.go", modulePath + "/internal/successor/admission/receiving", false},
		{"bootstrap cannot import receiving composition", "internal/successor/route/bootstrap/budget.go", modulePath + "/internal/successor/route/receiver", false},
		{"old receiver cannot import new bootstrap", "internal/node/forwarding/link.go", modulePath + "/internal/successor/route/bootstrap", false},
		{"channel uses ordered transport", "internal/successor/route/channel/session.go", modulePath + "/internal/successor/route/transport", true},
		{"channel cannot import TLS adapter", "internal/successor/route/channel/session.go", modulePath + "/internal/successor/route/transport/tls", false},
		{"channel cannot import QUIC adapter", "internal/successor/route/channel/session_test.go", modulePath + "/internal/successor/route/transport/quic", false},
		{"holder JOIN cannot import Receiver", "internal/successor/route/join/acquisition.go", modulePath + "/internal/successor/route/receiver", false},
		{"Receiver cannot import holder Prefix", "internal/successor/route/receiver/receiver_linux.go", modulePath + "/internal/successor/route/prefix", false},
		{"actual receiver dispatches JOIN", "internal/successor/route/receiver/receiver_linux.go", modulePath + "/internal/successor/route/join", true},
		{"holder JOIN consumes Prefix", "internal/successor/route/join/acquisition.go", modulePath + "/internal/successor/route/prefix", true},
		{"former operation cannot return", "cmd/ardents-next/route_linux.go", modulePath + "/internal/successor/route/operation", false},
		{"Prefix cannot import JOIN consumer", "internal/successor/route/prefix/borrowing.go", modulePath + "/internal/successor/route/join", false},
		{"Prefix cannot import Introduction consumer", "internal/successor/route/prefix/introduction_channel.go", modulePath + "/internal/successor/route/introduction", false},
		{"Reader initial factory retains actual Source", "internal/successor/connection/initiator_source.go", modulePath + "/internal/successor/route/prefix", true},
		{"Publisher recovery retains original Source and Responder", "internal/successor/connection/publisher_recovery.go", modulePath + "/internal/successor/route/prefix", true},
		{"Publisher dispatch uses canonical full logical tuple", "internal/successor/publication/runtime/publisher_streams.go", modulePath + "/internal/successor/connection/wire", true},
		{"Publication scheduler cannot construct wire authority", "internal/successor/publication/runtime/scheduler.go", modulePath + "/internal/successor/connection/wire", false},
		{"Connection record actors cannot choose Source", "internal/successor/connection/stream_records.go", modulePath + "/internal/successor/route/prefix", false},
		{"Reader initial factory verifies independent Descriptor", "internal/successor/connection/initiator_source.go", modulePath + "/internal/successor/reachability", true},
		{"Connection wire cannot borrow Source", "internal/successor/connection/wire/continuity.go", modulePath + "/internal/successor/route/prefix", false},
		{"Connection ledger cannot choose Source", "internal/successor/connection/receive_ledger.go", modulePath + "/internal/successor/route/prefix", false},
		{"Prefix cannot import receiving composition", "internal/successor/route/prefix/prefix.go", modulePath + "/internal/successor/route/receiver", false},
		{"Prefix calls actual role owner", "internal/successor/route/prefix/prefix.go", modulePath + "/internal/successor/route/role", true},
		{"holder consumes actual Prefix", "cmd/ardents-next/route_linux.go", modulePath + "/internal/successor/route/prefix", true},
		{"channel cannot import receiving composition", "internal/successor/route/channel/session.go", modulePath + "/internal/successor/route/receiver", false},
		{"channel cannot import Network", "internal/successor/route/channel/session_test.go", modulePath + "/internal/successor/network", false},
		{"channel cannot import role authority", "internal/successor/route/channel/session.go", modulePath + "/internal/successor/route/role", false},
		{"role cannot import receiving composition", "internal/successor/route/role/authority.go", modulePath + "/internal/successor/route/receiver", false},
		{"role cannot spend Admission", "internal/successor/route/role/authority.go", modulePath + "/internal/successor/admission/receiving", false},
		{"role consumes Network observations", "internal/successor/route/role/authority.go", modulePath + "/internal/successor/network", true},
		{"Introduction exchange uses role binding", "internal/successor/route/introduction/receiving.go", modulePath + "/internal/successor/route/role", true},
		{"Source command parses sealed capsule", "cmd/ardents-next/route_linux.go", modulePath + "/internal/successor/route/capsule", true},
		{"submission fixture parses sealed capsule", "cmd/ardents-next/route_submission_linux_test.go", modulePath + "/internal/successor/route/capsule", true},
		{"other commands cannot acquire capsule consumer", "cmd/ardents-next/admission_holder.go", modulePath + "/internal/successor/route/capsule", false},
		{"submission consumes retained exclusions", "internal/successor/route/introduction/submission.go", modulePath + "/internal/successor/route", true},
		{"Introduction history birth consumes only its genuine receiving owner", "internal/successor/route/introduction/receiving_history_linux.go", modulePath + "/internal/successor/admission/receiving", true},
		{"Introduction cannot own receiving Grant", "internal/successor/route/introduction/receiving.go", modulePath + "/internal/successor/admission/receiving", false},
		{"Introduction cannot call listener composition", "internal/successor/route/introduction/receiving.go", modulePath + "/internal/successor/route/receiver", false},
		{"channel cannot import Admission", "internal/successor/route/channel/session.go", modulePath + "/internal/successor/admission", false},
		{"channel cannot import Hosting", "internal/successor/route/channel/session.go", modulePath + "/internal/successor/hosting", false},
		{"transport cannot import framing", "internal/successor/route/transport/stream.go", modulePath + "/internal/successor/route/channel", false},
		{"transport cannot import concrete implementation", "internal/successor/route/transport/stream.go", modulePath + "/internal/successor/route/carrier", false},
		{"transport cannot import receiving composition", "internal/successor/route/transport/stream.go", modulePath + "/internal/successor/route/receiver", false},
		{"transport cannot import QUIC library", "internal/successor/route/transport/stream.go", "github.com/quic-go/quic-go", false},
		{"TLS implementation consumes shared authentication", "internal/successor/route/transport/tls/client.go", modulePath + "/internal/successor/route/transport", true},
		{"TLS cannot import QUIC implementation", "internal/successor/route/transport/tls/client.go", modulePath + "/internal/successor/route/transport/quic", false},
		{"TLS cannot import former carrier", "internal/successor/route/transport/tls/server.go", modulePath + "/internal/successor/route/carrier", false},
		{"transport cannot import its TLS adapter", "internal/successor/route/transport/role_authentication.go", modulePath + "/internal/successor/route/transport/tls", false},
		{"retired QUIC implementation has no import grant", "internal/successor/route/transport/quic/role.go", modulePath + "/internal/successor/route/transport", false},
		{"former selector cannot import QUIC library", "internal/successor/route/carrier/closed_node_carrier.go", "github.com/quic-go/quic-go", false},
		{"retired QUIC has no native library grant", "internal/successor/route/transport/quic/node.go", "github.com/quic-go/quic-go", false},
		{"QUIC cannot import TLS implementation", "internal/successor/route/transport/quic/role.go", modulePath + "/internal/successor/route/transport/tls", false},
		{"transport exporter cannot import QUIC adapter", "internal/successor/route/transport/role_exporter.go", modulePath + "/internal/successor/route/transport/quic", false},
		{"production command excludes lower contract", "cmd/ardents-next/route_linux.go", modulePath + "/internal/successor/route/transport", false},
		{"bootstrap scenario uses shared contract", "cmd/ardents-next/route_bootstrap_linux_test.go", modulePath + "/internal/successor/route/transport", true},
		{"funded Source uses real token grammar", "cmd/ardents-next/route_token_linux.go", modulePath + "/internal/successor/admission/token", true},
		{"funded Source uses finite bootstrap", "cmd/ardents-next/route_token_linux.go", modulePath + "/internal/successor/route/bootstrap", true},
		{"funded Source behavior selects real Carrier", "cmd/ardents-next/route_token_linux_test.go", modulePath + "/internal/successor/route/transport", true},
		{"other funding adapter has no bootstrap exception", "cmd/ardents-next/route_funding_linux.go", modulePath + "/internal/successor/route/bootstrap", false},
		{"installed Execution scenario uses shared Carrier identity", "cmd/ardents-next/execution_installed_linux_test.go", modulePath + "/internal/successor/route/transport", true},
		{"installed Execution scenario excludes direct TLS construction", "cmd/ardents-next/execution_installed_linux_test.go", modulePath + "/internal/successor/route/transport/tls", false},
		{"installed Execution scenario excludes direct QUIC construction", "cmd/ardents-next/execution_installed_linux_test.go", modulePath + "/internal/successor/route/transport/quic", false},
		{"installed Service scenario consumes real Connection", "cmd/ardents-next/connection_installed_linux_test.go", modulePath + "/internal/successor/connection", true},
		{"response loss observes original Connection", "cmd/ardents-next/connection_response_loss_linux_test.go", modulePath + "/internal/successor/connection", true},
		{"response loss retains original Execution", "cmd/ardents-next/connection_response_loss_linux_test.go", modulePath + "/internal/successor/execution/runtime", true},
		{"withdrawal invokes genuine original Publisher", "cmd/ardents-next/connection_response_loss_linux_test.go", modulePath + "/internal/successor/publication/runtime", true},
		{"withdrawal cannot acquire private Instance authority", "cmd/ardents-next/connection_response_loss_linux_test.go", modulePath + "/internal/successor/publication/instance", false},
		{"response loss uses shared Carrier identity", "cmd/ardents-next/connection_response_loss_linux_test.go", modulePath + "/internal/successor/route/transport", true},
		{"caller loss checks exact Application outcome", "cmd/ardents-next/connection_response_loss_linux_test.go", modulePath + "/internal/application/connection", true},
		{"response loss excludes direct TLS construction", "cmd/ardents-next/connection_response_loss_linux_test.go", modulePath + "/internal/successor/route/transport/tls", false},
		{"response loss excludes direct QUIC construction", "cmd/ardents-next/connection_response_loss_linux_test.go", modulePath + "/internal/successor/route/transport/quic", false},
		{"ordinary Text Reader consumes genuine Connection", "cmd/ardents-next/connection_reader_linux.go", modulePath + "/internal/successor/connection", true},
		{"ordinary Text Reader consumes qualified original worker", "cmd/ardents-next/connection_reader_linux.go", modulePath + "/internal/successor/execution/runtime", true},
		{"another command cannot borrow Reader composition", "cmd/ardents-next/route.go", modulePath + "/internal/successor/connection", false},
		{"installed Service scenario consumes real Publisher", "cmd/ardents-next/connection_installed_linux_test.go", modulePath + "/internal/successor/publication/runtime", true},
		{"installed Service scenario excludes direct TLS construction", "cmd/ardents-next/connection_installed_linux_test.go", modulePath + "/internal/successor/route/transport/tls", false},
		{"installed Service scenario excludes direct QUIC construction", "cmd/ardents-next/connection_installed_linux_test.go", modulePath + "/internal/successor/route/transport/quic", false},
		{"bootstrap scenario excludes TLS construction", "cmd/ardents-next/route_bootstrap_linux_test.go", modulePath + "/internal/successor/route/transport/tls", false},
		{"bootstrap scenario excludes QUIC construction", "cmd/ardents-next/route_bootstrap_linux_test.go", modulePath + "/internal/successor/route/transport/quic", false},
		{"unregistered scenario excludes lower contract", "cmd/ardents-next/route_other_linux_test.go", modulePath + "/internal/successor/route/transport", false},
		{"production bootstrap excludes TLS construction", "cmd/ardents-next/route_bootstrap_linux.go", modulePath + "/internal/successor/route/transport/tls", false},
		{"former selector cannot return in caller test", "cmd/ardents-next/route_prefix_caller_linux_test.go", modulePath + "/internal/successor/route/carrier", false},
		{"no production Carrier allowance", "cmd/ardents-next/route_prefix_caller_linux.go", modulePath + "/internal/successor/route/carrier", false},
		{"caller test cannot reach old Carrier", "cmd/ardents-next/route_prefix_caller_linux_test.go", modulePath + "/internal/route/carrier", false},
		{"JOIN cannot retain former selector", "cmd/ardents-next/route_join_linux_test.go", modulePath + "/internal/successor/route/carrier", false},
		{"JOIN production excludes Carrier adapter", "cmd/ardents-next/route_join_linux.go", modulePath + "/internal/successor/route/carrier", false},
		{"JOIN test excludes old Carrier", "cmd/ardents-next/route_join_linux_test.go", modulePath + "/internal/route/carrier", false},
		{"external public-contract test", "internal/successor/admission/issuerprofile/provenance_test.go", modulePath + "/internal/successor/admission/issuerprofile", true},
		{"no production self import", "internal/successor/admission/issuerprofile/profile.go", modulePath + "/internal/successor/admission/issuerprofile", false},
		{"no unregistered test self import", "internal/successor/future/file_test.go", modulePath + "/internal/successor/future", false},
		{"public profile is leaf", "internal/successor/admission/issuerprofile/profile.go", modulePath + "/internal/successor/admission", false},
		{"profile cannot borrow signing", "internal/successor/admission/issuerprofile/profile.go", modulePath + "/internal/successor/admission/issuerprofile/signing", false},
		{"profile tests cannot borrow signing", "internal/successor/admission/issuerprofile/profile_test.go", modulePath + "/internal/successor/admission/issuerprofile/signing", false},
		{"command cannot restore retired signer", "cmd/ardents-next/issuer_profile.go", modulePath + "/internal/successor/nodeidentity", false},
		{"retired signer cannot restore facade", "internal/successor/nodeidentity/store.go", modulePath + "/internal/successor/admission/issuerprofile/signing", false},
		{"identity cannot borrow ledger", "internal/successor/admission/issuerprofile/signing/store.go", modulePath + "/internal/successor/admission", false},
		{"ledger public grammar", "internal/successor/admission/binding.go", modulePath + "/internal/successor/admission/issuerprofile", true},
		{"issuance public grammar", "internal/successor/admission/issuance/profile.go", modulePath + "/internal/successor/admission/issuerprofile", true},
		{"no inherited nested imports", "internal/successor/admission/issuance/nested/file.go", modulePath + "/internal/successor/admission", false},
		{"no inherited nested third party", "internal/successor/admission/issuance/nested/file.go", "github.com/cloudflare/circl/blindsign/blindrsa", false},
		{"custody allocation owner", "internal/successor/custody/admission_authority.go", modulePath + "/internal/successor/admission/allocation", true},
		{"custody excludes old backend", "internal/successor/custody/vault.go", modulePath + "/internal/custody", false},
		{"custody excludes live Instance parser", "internal/successor/custody/authority_state.go", modulePath + "/internal/successor/publication/instance", false},
		{"custody excludes Route", "internal/successor/custody/vault.go", modulePath + "/internal/successor/route", false},
		{"Service command uses genuine new custody", "cmd/ardents-custody/service_authority.go", modulePath + "/internal/successor/custody", true},
		{"unregistered caller", "internal/successor/future/file.go", modulePath + "/internal/successor/admission", false},
		{"unregistered dependency", "cmd/ardents-next/main.go", modulePath + "/internal/successor/future", false},
		{"standard", "internal/successor/admission/check.go", "context", true},
		{"zone", "cmd/ardents-next/main.go", modulePath + "/internal/successor/admission", true},
		{"hosting caller", "cmd/ardents-next/hosting.go", modulePath + "/internal/successor/hosting", true},
		{"independent domains", "internal/successor/hosting/budget.go", modulePath + "/internal/successor/admission", false},
		{"network input adapter", "internal/successor/network/state/membership.go", modulePath + "/internal/successor/network", true},
		{"network adapter is exact", "internal/successor/network/state/other.go", modulePath + "/internal/successor/network", false},
		{"application observation", "internal/successor/network/state/runtime_observation.go", modulePath + "/internal/successor/network", true},
		{"original observation witness", "internal/successor/network/state/runtime_witness.go", modulePath + "/internal/successor/network", true},
		{"retained Route witness", "internal/successor/route/prefix/prefix.go", modulePath + "/internal/successor/network/state", true},
		{"Route witness import is exact", "internal/successor/route/prefix/other.go", modulePath + "/internal/successor/network/state", false},
		{"legacy consumer has no new State", "internal/endpoint/source_state.go", modulePath + "/internal/successor/network/state", false},
		{"State consumer has no domain access", "internal/endpoint/source_state.go", modulePath + "/internal/successor/network", false},
		{"State consumer has no physical root access", "internal/endpoint/source_state.go", modulePath + "/internal/successor/network/state/durable", false},
		{"unregistered State consumer", "internal/service/publication/publication.go", modulePath + "/internal/successor/network/state", false},
		{"State excludes debit", "internal/successor/network/state/open.go", modulePath + "/internal/successor/admission/quota", false},
		{"State excludes old backend", "internal/successor/network/state/open.go", modulePath + "/internal/network/state", false},
		{"legacy admission consumer isolated", "internal/node/authority/admission_observation.go", modulePath + "/internal/successor/admission/receiving", false},
		{"legacy token consumer isolated", "internal/node/authority/token.go", modulePath + "/internal/successor/admission/receiving", false},
		{"observation excludes spending root", "internal/node/authority/admission_observation.go", modulePath + "/internal/successor/admission/spending", false},
		{"observation bridge is exact", "internal/node/authority/receiver.go", modulePath + "/internal/successor/admission/receiving", false},
		{"acquisition policy adapter", "internal/successor/network/state/acquisition_history.go", modulePath + "/internal/successor/network", true},
		{"acquisition adapter is exact", "internal/successor/network/state/attempts.go", modulePath + "/internal/successor/network", false},
		{"participation policy adapter", "internal/successor/network/duty/participation_policy.go", modulePath + "/internal/successor/network", true},
		{"participation adapter is exact", "internal/successor/network/duty/store.go", modulePath + "/internal/successor/network", false},
		{"candidate policy adapter", "internal/successor/network/epoch/candidate_evaluation.go", modulePath + "/internal/successor/network", true},
		{"assignment adapter", "internal/successor/network/epoch/assignment.go", modulePath + "/internal/successor/network", true},
		{"candidate adapter is exact", "internal/successor/network/epoch/record.go", modulePath + "/internal/successor/network", false},
		{"public token grammar adapter", "internal/successor/network/closedprofile/token_spki.go", modulePath + "/internal/successor/admission/issuerprofile", true},
		{"public token adapter is exact", "internal/successor/network/closedprofile/profile.go", modulePath + "/internal/successor/admission/issuerprofile", false},
		{"public token adapter excludes signing", "internal/successor/network/closedprofile/token_spki.go", modulePath + "/internal/successor/admission/issuance", false},
		{"network excludes old state", "internal/successor/network/membership.go", modulePath + "/internal/network/state", false},
		{"network excludes execution", "internal/successor/network/membership.go", modulePath + "/internal/node", false},
		{"network excludes debit", "internal/successor/network/membership.go", modulePath + "/internal/successor/admission", false},
		{"network excludes reservations", "internal/successor/network/membership.go", modulePath + "/internal/successor/hosting", false},
		{"issuance confirmed debit", "internal/successor/admission/issuance/store.go", modulePath + "/internal/successor/admission", true},
		{"operation owners", "internal/successor/admission/issuer/operation.go", modulePath + "/internal/successor/admission/issuance", true},
		{"operation excludes hosting", "internal/successor/admission/issuer/operation.go", modulePath + "/internal/successor/hosting", false},
		{"issuance excludes hosting", "internal/successor/admission/issuance/store.go", modulePath + "/internal/successor/hosting", false},
		{"identity purpose", "internal/successor/admission/issuerprofile/signing/store.go", modulePath + "/internal/successor/admission/issuerprofile", true},
		{"identity cannot borrow issuance", "internal/successor/admission/issuerprofile/signing/store.go", modulePath + "/internal/successor/admission/issuance", false},
		{"admission cannot borrow identity", "internal/successor/admission/issuer_profile.go", modulePath + "/internal/successor/admission/issuerprofile/signing", false},
		{"admission excludes composition", "internal/successor/admission/ledger.go", modulePath + "/internal/successor/admission/issuer", false},
		{"admission cannot borrow keys", "internal/successor/admission/batch.go", modulePath + "/internal/successor/admission/issuance", false},
		{"admission cannot borrow budget", "internal/successor/admission/check.go", modulePath + "/internal/successor/hosting", false},
		{"legacy", "internal/successor/admission/check.go", modulePath + "/internal/admission", false},
		{"legacy test fixture", "internal/successor/admission/check_test.go", modulePath + "/tests/fixtures", false},
		{"third party bridge", "internal/successor/admission/check.go", "example.org/bridge", false},
		{"reverse", "internal/endpoint/runtime.go", modulePath + "/internal/successor/admission", false},
		{"prefix collision", "internal/successor/admission/check.go", modulePath + "/internal/successorold", false},
		{"unrelated", "internal/endpoint/runtime.go", "context", true},
		{"relative", "internal/successor/admission/check.go", "../admission", false},
		{"Source publication native control uses actual fixed mutation", "internal/successor/installation/infrastructure/source_canonical_publication_profile_native_linux_test.go", modulePath + "/internal/successor/installation/fixedfile", true},
		{"Source seal native control cannot borrow mutation", "internal/successor/installation/infrastructure/source_canonical_seal_profile_native_linux_test.go", modulePath + "/internal/successor/installation/fixedfile", false},
		{"Source publication control excludes State authority", "internal/successor/installation/infrastructure/source_canonical_publication_profile_native_linux_test.go", modulePath + "/internal/successor/network/state", false},
		{"Source publication runtime filename has no test permission", "internal/successor/installation/infrastructure/source_canonical_publication_profile_linux.go", modulePath + "/internal/successor/installation/fixedfile", false},
		{"Source startup native prefix uses actual closed frame codec", "internal/successor/installation/infrastructure/source_startup_physical_prefix_profile_native_linux_test.go", modulePath + "/internal/successor/installation/completion", true},
		{"Source startup other native test has no frame permission", "internal/successor/installation/infrastructure/source_startup_physical_prefix_other_native_linux_test.go", modulePath + "/internal/successor/installation/completion", false},
		{"Source startup runtime filename has no frame test permission", "internal/successor/installation/infrastructure/source_startup_physical_prefix_profile_linux.go", modulePath + "/internal/successor/installation/completion", false},
		{"Source startup native prefix excludes State authority", "internal/successor/installation/infrastructure/source_startup_physical_prefix_profile_native_linux_test.go", modulePath + "/internal/successor/network/state", false},
		{"Source startup physical observer checks actual closed frame", "internal/successor/installation/infrastructure/source_startup_physical_observer_native_linux_test.go", modulePath + "/internal/successor/installation/completion", true},
		{"Source startup observer runtime spelling has no frame permission", "internal/successor/installation/infrastructure/source_startup_physical_observer_linux.go", modulePath + "/internal/successor/installation/completion", false},
		{"Source startup observer excludes State authority", "internal/successor/installation/infrastructure/source_startup_physical_observer_native_linux_test.go", modulePath + "/internal/successor/network/state", false},
		{"Source retirement observer binds original completion frame", "internal/successor/installation/infrastructure/source_retirement_physical_observer_native_linux_test.go", modulePath + "/internal/successor/installation/completion", true},
		{"Source retirement observer runtime spelling has no frame permission", "internal/successor/installation/infrastructure/source_retirement_physical_observer_linux.go", modulePath + "/internal/successor/installation/completion", false},
		{"Source retirement adjacent native file has no frame permission", "internal/successor/installation/infrastructure/source_retirement_physical_other_native_linux_test.go", modulePath + "/internal/successor/installation/completion", false},
		{"Source retirement observer excludes State authority", "internal/successor/installation/infrastructure/source_retirement_physical_observer_native_linux_test.go", modulePath + "/internal/successor/network/state", false},
		{"unsafe", "internal/successor/admission/check.go", "unsafe", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := successorImportAllowed(test.source, test.dependency); got != test.allowed {
				t.Fatalf("allowed = %v, want %v", got, test.allowed)
			}
		})
	}
}
