package architecture

import (
	"go/build"
	"os"
	"path"
	"slices"
	"strings"
	"testing"
)

// Existing Endpoint/v3 Enrollment grammar and portable behavior controls remain
// selected on both platforms. ADR-0123's separate Source artifact extension is
// Linux-only and checked against its exact genuine consumer profile below.
func TestSuccessorEnrollmentRunsPortableRulesOnWindowsAndLinux(t *testing.T) {
	directory := path.Join(repositoryRoot(t), "internal/domain/enrollment")
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
		case "infrastructure_snapshot_linux.go", "infrastructure_snapshot_linux_test.go", "infrastructure_process_linux.go":
			// Infrastructure custody requires the native Linux mechanism.
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
			"internal/domain/network/state/open.go",
			"internal/domain/network/state/retained_open_test.go",
			"internal/domain/network/state/durable/retained_open.go",
			"internal/domain/network/state/durable/retained_open_test.go",
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

func TestInfrastructureInventoryGrammarRunsOnPortableProfiles(t *testing.T) {
	directory := path.Join(repositoryRoot(t), "internal/domain/enrollment/infrastructureinventory")
	for _, target := range []string{"linux", "windows", "darwin"} {
		profile := build.Default
		profile.GOOS, profile.GOARCH, profile.CgoEnabled = target, "amd64", false
		for _, name := range []string{"doc.go", "canonical.go", "files.go", "canonical_test.go"} {
			selected, err := profile.MatchFile(directory, name)
			if err != nil {
				t.Fatal(err)
			}
			if !selected {
				t.Errorf("portable infrastructure inventory %s excluded on %s", name, target)
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
			"internal/domain/enrollment/infrastructure_snapshot_linux.go",
			"internal/domain/enrollment/infrastructure_snapshot_linux_test.go",
			"internal/domain/enrollment/infrastructure_process_linux.go",
			"internal/platform/systemd/infrastructure_prestart_linux.go",
			"internal/platform/systemd/infrastructure_prestart_linux_test.go",
			"internal/domain/release/infrastructure/admission.go",
			"internal/domain/release/infrastructure/source_release_inspection_linux.go",
			"internal/domain/release/infrastructure/preparation_linux.go",
			"internal/platform/cgroup/quiescence_linux.go",
			"internal/domain/network/state/source_inspection_linux.go",
			"internal/domain/network/state/source_preparation_linux.go",
			"internal/domain/endpoint/source_trust_preparation_linux.go",
			"internal/domain/network/state/inspection_state_linux.go",
			"internal/domain/network/state/node_inspection_linux.go",
			"internal/domain/network/state/durable/inspection_linux.go",
			"internal/domain/network/duty/source_inspection_linux.go",
			"internal/domain/network/duty/inspection_linux.go",
			"internal/domain/network/duty/node_inspection_linux.go",
			"internal/domain/node/provisioning/infrastructure/command_linux.go",
			"internal/domain/network/source/command/prepare_linux.go",
			"internal/domain/network/source/command/command_linux_test.go",
			"internal/domain/network/state/source_state_files_linux.go",
			"internal/domain/network/state/source_state_files_linux_test.go",
			"internal/platform/artifacts/protected_input_linux.go",
			"internal/platform/artifacts/protected_storage_linux.go",
			"internal/domain/network/source/command/inspect_linux.go",
			"internal/domain/network/source/inspection/inspection_linux.go",
			"internal/domain/network/source/inspection/inspection_linux_test.go",
			"tests/e2e/source/inspection_linux_test.go",
			"internal/domain/release/infrastructure/command_linux.go",
			"internal/domain/release/infrastructure/signing/command_linux.go",
			"internal/domain/release/infrastructure/signing/release_key_command_linux.go",
			"internal/domain/release/infrastructure/signing/internal/keyset/prepare_linux.go",
			"internal/domain/release/infrastructure/signing/node_release_plan_linux.go",
			"internal/domain/release/infrastructure/signing/node_release_arguments_linux.go",
			"internal/domain/release/infrastructure/signing/node_release_linux.go",
			"internal/domain/release/infrastructure/signing/node_release_predecessor_linux.go",
			"internal/domain/release/infrastructure/signing/join_release_plan_linux_test.go",
			"internal/domain/release/infrastructure/join_admission.go",
			"internal/platform/artifacts/output_linux.go",
			"internal/domain/release/infrastructure/signing/source_release_linux.go",
			"internal/domain/release/infrastructure/signing/source_release_predecessor_linux.go",
			"internal/domain/release/infrastructure/signing/source_release_plan_linux.go",
			"internal/domain/release/infrastructure/signing/source_release_plan_linux_test.go",
			"internal/domain/node/provisioning/sourcegenesis/command_linux.go",
			"internal/domain/node/provisioning/sourcegenesis/keys_linux.go",
			"internal/domain/node/provisioning/sourcegenesis/artifacts_linux.go",
			"internal/domain/node/provisioning/sourcegenesis/plan_linux_test.go",
			"internal/domain/node/provisioning/sourcegenesis/plan_linux.go",
			"internal/platform/artifacts/key_linux.go",
			"internal/platform/systemd/node_slots_linux.go",
			"internal/platform/systemd/node_slots_linux_test.go",
			"internal/platform/systemd/reference_objects_linux_test.go",
			"internal/platform/systemd/manager_properties_linux.go",
			"internal/platform/systemd/source_manager_linux.go",
			"internal/domain/release/history_binding_linux.go",
			"internal/domain/release/history_binding_linux_test.go",
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
			"internal/platform/systemd/observation/properties.go",
			"internal/platform/systemd/observation/properties_test.go",
			"internal/domain/release/infrastructure/doc.go",
			"internal/domain/release/infrastructure/artifact.go",
			"internal/domain/release/infrastructure/artifact_test.go",
			"internal/domain/release/infrastructure/artifact_identity.go",
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
			"internal/domain/release/static_metadata.go",
			"internal/domain/release/static_metadata_test.go",
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
				"internal/domain/network/state/source_inspection_native_linux_test.go",
				"internal/domain/release/infrastructure/preparation_native_linux_test.go",
				"internal/domain/release/infrastructure/signing/join_release_native_linux_test.go",
				"internal/domain/release/infrastructure/signing/join_forward_release_native_linux_test.go",
				"internal/domain/network/state/node_inspection_native_linux_test.go",
				"internal/domain/network/duty/inspection_native_linux_test.go",
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

// Cohort 0 (#850, portability design audit #848) ported the durable-store
// inspection idiom to the Unix family (Linux and macOS): the shared
// internal/durablefile inspection port and the de-tagged consumer mechanisms
// keep their exact inode-identity, root-custody, mode-bit and single-link
// semantics on both platforms. Every other platform receives the same explicit
// typed refusal; the genuine Linux ext4 storage floor and the Linux output
// composition keep their Linux boundary.
func TestCohort0UnixFamilyInspectionPortSelection(t *testing.T) {
	root := repositoryRoot(t)
	for _, target := range []string{"linux", "windows", "darwin"} {
		profile := build.Default
		profile.GOOS, profile.GOARCH, profile.CgoEnabled = target, "amd64", false
		for _, name := range []string{
			"internal/durablefile/relative_open_unix.go",
			"internal/durablefile/relative_open_unix_test.go",
			"internal/durablefile/inspection_identity_unix.go",
			"internal/durablefile/inspection_identity_unix_test.go",
			"internal/application/document/snapshot_file_unix.go",
			"internal/application/document/snapshot_file_unix_test.go",
			"internal/domain/endpoint/generation/file_unix.go",
			"internal/domain/endpoint/generation/snapshot_unix.go",
			"internal/domain/endpoint/generation/directory_unix.go",
			"internal/domain/endpoint/launch/root_ownership_unix.go",
			"internal/domain/endpoint/plan/custody_unix.go",
			"internal/domain/node/runtime/credential_unix.go",
			"internal/domain/node/runtime/credential_unix_test.go",
			"internal/domain/node/runtime/join_credential_unix.go",
			"internal/domain/node/runtime/join_credential_unix_test.go",
			"internal/platform/artifacts/input_unix.go",
			"internal/platform/artifacts/input_unix_test.go",
			"internal/platform/artifacts/directory_unix.go",
			"internal/platform/artifacts/retained_file_unix.go",
		} {
			selected, err := profile.MatchFile(path.Join(root, path.Dir(name)), path.Base(name))
			if err != nil {
				t.Fatal(err)
			}
			if want := target != "windows"; selected != want {
				t.Errorf("Unix-family inspection port %s on %s: %v, want %v", name, target, selected, want)
			}
		}
		for _, name := range []string{
			"internal/application/document/snapshot_file_other.go",
			"internal/domain/endpoint/plan/custody_other.go",
		} {
			selected, err := profile.MatchFile(path.Join(root, path.Dir(name)), path.Base(name))
			if err != nil {
				t.Fatal(err)
			}
			if want := target == "windows"; selected != want {
				t.Errorf("Unix-family inspection refusal %s on %s: %v, want %v", name, target, selected, want)
			}
		}
		for _, name := range []string{
			"internal/durablefile/inode_times_linux.go",
			"internal/platform/artifacts/protected_storage_linux.go",
			"internal/domain/node/provisioning/networkpreparation/prepare_linux.go",
		} {
			selected, err := profile.MatchFile(path.Join(root, path.Dir(name)), path.Base(name))
			if err != nil {
				t.Fatal(err)
			}
			if want := target == "linux"; selected != want {
				t.Errorf("Linux inspection boundary %s on %s: %v, want %v", name, target, selected, want)
			}
		}
		selected, err := profile.MatchFile(path.Join(root, "internal/durablefile"), "inode_times_darwin.go")
		if err != nil {
			t.Fatal(err)
		}
		if want := target == "darwin"; selected != want {
			t.Errorf("macOS inspection spelling on %s: %v, want %v", target, selected, want)
		}
	}
}

// Hosting seam 1 (#865, portability design audit #848) splits the former
// platform_linux.go by responsibility. The durable flock root lease becomes
// Unix-family (Linux and macOS) with the same exclusive-fence semantics, and its
// fixture-measure lease/storage behavior tests execute on both. Counter
// measurement stays a platform port: the genuine Linux /proc/sys measure keeps
// its Linux boundary, and the macOS measure reads the full 64-bit IFMIB
// interface-data totals (#872). The #865 staging refusal is deliberately
// superseded: the routing-socket if_data64 byte counters wrap at 2^32 and cannot
// preserve per-boot per-interface continuity, so the macOS adapter reads IFMIB
// (net.link.generic.ifdata) instead, with kern.boottime boot identity and an
// index recheck, re-proven at runtime against netstat. The macOS measure files
// keep their darwin-only boundary as a genuine platform port. Every other
// platform receives the same explicit typed refusal. The Linux-only retained
// history inspection keeps its ext4 floor and root-only lease, with a !linux
// typed refusal elsewhere. No weaker accepting path is introduced.
func TestHostingUnixLeasePortSelection(t *testing.T) {
	root := repositoryRoot(t)
	for _, target := range []string{"linux", "windows", "darwin"} {
		profile := build.Default
		profile.GOOS, profile.GOARCH, profile.CgoEnabled = target, "amd64", false
		for _, name := range []string{
			"internal/domain/hosting/period_lease_unix.go",
			"internal/domain/hosting/period_lease_unix_test.go",
			"internal/domain/hosting/budget_unix_test.go",
			"internal/domain/hosting/reservation_unix_test.go",
			"internal/domain/hosting/additional_reservation_unix_test.go",
			"internal/domain/hosting/snapshot_unix_test.go",
		} {
			selected, err := profile.MatchFile(path.Join(root, path.Dir(name)), path.Base(name))
			if err != nil {
				t.Fatal(err)
			}
			if want := target != "windows"; selected != want {
				t.Errorf("Unix-family hosting lease %s on %s: %v, want %v", name, target, selected, want)
			}
		}
		for _, name := range []string{
			"internal/domain/hosting/period_lease_unsupported.go",
			"internal/domain/hosting/counters/read_other.go",
		} {
			selected, err := profile.MatchFile(path.Join(root, path.Dir(name)), path.Base(name))
			if err != nil {
				t.Fatal(err)
			}
			if want := target == "windows"; selected != want {
				t.Errorf("hosting typed refusal %s on %s: %v, want %v", name, target, selected, want)
			}
		}
		for _, name := range []string{
			"internal/domain/hosting/counters/read_linux.go",
			"internal/domain/hosting/counters/read_linux_test.go",
			"internal/domain/hosting/reservation_process_linux_test.go",
		} {
			selected, err := profile.MatchFile(path.Join(root, path.Dir(name)), path.Base(name))
			if err != nil {
				t.Fatal(err)
			}
			if want := target == "linux"; selected != want {
				t.Errorf("Linux hosting boundary %s on %s: %v, want %v", name, target, selected, want)
			}
		}
		for _, name := range []string{
			"internal/domain/hosting/counters/read_darwin.go",
			"internal/domain/hosting/counters/read_darwin_test.go",
		} {
			selected, err := profile.MatchFile(path.Join(root, path.Dir(name)), path.Base(name))
			if err != nil {
				t.Fatal(err)
			}
			if want := target == "darwin"; selected != want {
				t.Errorf("macOS hosting measure boundary %s on %s: %v, want %v", name, target, selected, want)
			}
		}
	}
}

// Cohort 2b (#859, portability design audit #848) de-tags the universal and
// Unix-generic Release/infrastructure and Node provisioning codecs to the Unix
// family (Linux and macOS): the public identifier decoder, Node transport leaf
// codec, Node Record plan, offline Network signing declaration/certificate/
// profile codecs, the public report writer, the release-key receipt and the
// pure option-grammar and closed-profile signing-transcript leaves they compose.
// The genuine Linux boundaries stay Linux with the same explicit typed refusal
// elsewhere: the ext4 storage floor and protected input/output composition
// (artifacts output/protected_input/protected_storage, network_preparation),
// the Enrollment infrastructure inventory and its /proc/self/exe running-program
// proof (a macOS port needs a security-equivalent proc_pidpath point capability,
// not os.Executable), and the systemd Release plans. The report writer's refusal
// stub shifts to every non-Unix-family platform (ADR-0129, #859).
func TestCohort2bReleaseProvisioningUnixPortSelection(t *testing.T) {
	root := repositoryRoot(t)
	for _, target := range []string{"linux", "windows", "darwin"} {
		profile := build.Default
		profile.GOOS, profile.GOARCH, profile.CgoEnabled = target, "amd64", false
		for _, name := range []string{
			"internal/domain/node/provisioning/infrastructure/node_provisioning_role_unix.go",
			"internal/domain/node/provisioning/infrastructure/public_identifier_unix.go",
			"internal/domain/node/provisioning/infrastructure/public_identifier_unix_test.go",
			"internal/domain/node/provisioning/networksigning/certificate_unix.go",
			"internal/domain/node/provisioning/networksigning/certificate_unix_test.go",
			"internal/domain/node/provisioning/network_report_unix.go",
			"internal/domain/node/provisioning/infrastructure/node_record_unix.go",
			"internal/domain/node/provisioning/infrastructure/forwarding_node_record_unix_test.go",
			"internal/domain/node/provisioning/networksigning/declaration_unix.go",
			"internal/domain/node/provisioning/networksigning/certificate_operation_unix.go",
			"internal/domain/node/provisioning/networksigning/certificate_operation_unix_test.go",
			"internal/domain/node/provisioning/networksigning/profile_unix.go",
			"internal/domain/node/provisioning/networksigning/profile_unix_test.go",
			"internal/domain/release/infrastructure/signing/release_key_receipt_unix.go",
			"internal/platform/artifacts/flag_options_unix.go",
			"internal/domain/network/closedprofile/signing_transcript_unix.go",
		} {
			selected, err := profile.MatchFile(path.Join(root, path.Dir(name)), path.Base(name))
			if err != nil {
				t.Fatal(err)
			}
			if want := target != "windows"; selected != want {
				t.Errorf("Unix-family Release/provisioning port %s on %s: %v, want %v", name, target, selected, want)
			}
		}
		for _, name := range []string{
			"internal/domain/node/provisioning/network_report_other.go",
		} {
			selected, err := profile.MatchFile(path.Join(root, path.Dir(name)), path.Base(name))
			if err != nil {
				t.Fatal(err)
			}
			if want := target == "windows"; selected != want {
				t.Errorf("Unix-family report refusal %s on %s: %v, want %v", name, target, selected, want)
			}
		}
		for _, name := range []string{
			"internal/domain/node/provisioning/networkpreparation/prepare_linux.go",
			"internal/domain/node/provisioning/infrastructure/command_linux.go",
			"internal/domain/node/provisioning/networksigning/protected_linux.go",
			"internal/domain/node/provisioning/initial_network_keys_linux.go",
			"internal/platform/artifacts/output_linux.go",
			"internal/platform/artifacts/protected_input_linux.go",
			"internal/platform/artifacts/protected_storage_linux.go",
			"internal/domain/enrollment/infrastructure_snapshot_linux.go",
			"internal/domain/enrollment/infrastructure_process_linux.go",
			"internal/domain/release/infrastructure/preparation_linux.go",
			"internal/domain/release/infrastructure/admission.go",
			"internal/domain/release/infrastructure/source_release_inspection_linux.go",
			"internal/domain/release/infrastructure/signing/node_release_linux.go",
			"internal/domain/release/infrastructure/signing/source_release_linux.go",
			"internal/domain/release/infrastructure/signing/node_release_plan_linux.go",
			"internal/domain/release/infrastructure/signing/source_release_plan_linux.go",
			"internal/domain/release/infrastructure/signing/internal/keyset/prepare_linux.go",
		} {
			selected, err := profile.MatchFile(path.Join(root, path.Dir(name)), path.Base(name))
			if err != nil {
				t.Fatal(err)
			}
			if want := target == "linux"; selected != want {
				t.Errorf("Linux Release/provisioning boundary %s on %s: %v, want %v", name, target, selected, want)
			}
		}
	}
}

// Cohort 1 N2 (#853, portability design audit #848) ported the Introduction
// slot-history root to the Unix family (Linux and macOS): the shared
// internal/durablefile directory-relative Openat/Renameat anchor every mutation
// to the held inode and the shared full-identity comparison retains the
// anti-substitution semantics on both platforms. The kernel root-move watch is
// a point capability at the Introduction boundary (Linux inotify, macOS kqueue
// EVFILT_VNODE); every other platform receives the same explicit typed refusal.
// The macOS /dev/fd spelling does not support directory traversal, so the
// fd-anchoring is realized by the directory-relative syscalls, never a weaker
// pathname anchor. The Linux amd64 ptrace crash control stays Linux-only.
func TestCohort1N2IntroductionUnixPortSelection(t *testing.T) {
	root := repositoryRoot(t)
	for _, target := range []string{"linux", "windows", "darwin"} {
		profile := build.Default
		profile.GOOS, profile.GOARCH, profile.CgoEnabled = target, "amd64", false
		for _, name := range []string{
			"internal/durablefile/relative_rename_unix.go",
			"internal/durablefile/relative_rename_unix_test.go",
			"internal/domain/route/introduction/slot_history_unix.go",
			"internal/domain/route/introduction/slot_history_unix_test.go",
			"internal/domain/route/introduction/receiving_history_unix.go",
			"internal/domain/route/introduction/receiving_history_unix_test.go",
			"internal/domain/route/introduction/history_custody_unix_test.go",
			"internal/domain/route/introduction/history_write_unix_test.go",
			"internal/domain/route/introduction/registry_unix_test.go",
		} {
			selected, err := profile.MatchFile(path.Join(root, path.Dir(name)), path.Base(name))
			if err != nil {
				t.Fatal(err)
			}
			if want := target != "windows"; selected != want {
				t.Errorf("Unix-family Introduction port %s on %s: %v, want %v", name, target, selected, want)
			}
		}
		for _, name := range []string{
			"internal/domain/route/introduction/slot_history_other.go",
			"internal/domain/route/introduction/receiving_history_other.go",
		} {
			selected, err := profile.MatchFile(path.Join(root, path.Dir(name)), path.Base(name))
			if err != nil {
				t.Fatal(err)
			}
			if want := target == "windows"; selected != want {
				t.Errorf("Introduction platform refusal %s on %s: %v, want %v", name, target, selected, want)
			}
		}
		for _, name := range []string{
			"internal/domain/route/introduction/slot_history_watch_linux.go",
			"internal/domain/route/introduction/history_crash_linux_amd64_test.go",
		} {
			selected, err := profile.MatchFile(path.Join(root, path.Dir(name)), path.Base(name))
			if err != nil {
				t.Fatal(err)
			}
			if want := target == "linux"; selected != want {
				t.Errorf("Linux root-move watch boundary %s on %s: %v, want %v", name, target, selected, want)
			}
		}
		selected, err := profile.MatchFile(path.Join(root, "internal/domain/route/introduction"), "slot_history_watch_darwin.go")
		if err != nil {
			t.Fatal(err)
		}
		if want := target == "darwin"; selected != want {
			t.Errorf("macOS root-move watch on %s: %v, want %v", target, selected, want)
		}
	}
}

// Cohort 2a (#856, portability design audit #848) de-tagged the universal
// domain composition once the Cohort 0/1 leaf seams were cleared: the Node
// role lifetimes, the Route receiver and the Direct-Origin Source serving
// operation compose only portable rules and the already-ported Unix-family
// mechanisms (durablefile inspection, worker peer-credential, introduction
// root-watch, artifacts input custody). The artifacts observed-input and
// bounded-report seams they borrow are themselves Unix-generic and join the
// Unix family. Every other platform receives the same explicit typed refusal:
// serving and the Node runtime refuse outside Linux and macOS, while the
// Linux-only Source inspection (it composes state.SourcePreparationConfig)
// refuses on every non-Linux platform. The genuine Linux native role tests and
// the Source inspection mechanism keep their Linux boundary.
func TestCohort2aDomainDetagSelection(t *testing.T) {
	root := repositoryRoot(t)
	for _, target := range []string{"linux", "windows", "darwin"} {
		profile := build.Default
		profile.GOOS, profile.GOARCH, profile.CgoEnabled = target, "amd64", false
		for _, name := range []string{
			"internal/domain/node/runtime/descriptor_unix.go",
			"internal/domain/node/runtime/forwarding_unix.go",
			"internal/domain/node/runtime/introduction_unix.go",
			"internal/domain/node/runtime/issuer_unix.go",
			"internal/domain/node/runtime/join_unix.go",
			"internal/domain/node/runtime/lifetime_unix.go",
			"internal/domain/node/runtime/descriptor_startup_unix_test.go",
			"internal/domain/node/runtime/failure_join_unix_test.go",
			"internal/domain/node/runtime/introduction_startup_unix_test.go",
			"internal/domain/node/runtime/join_startup_unix_test.go",
			"internal/domain/node/runtime/lifetime_equivalence_unix_test.go",
			"internal/domain/node/runtime/retirement_unix_test.go",
			"internal/domain/route/receiver/admission_retirement_unix.go",
			"internal/domain/route/receiver/bootstrap_unix.go",
			"internal/domain/route/receiver/descriptor_unix.go",
			"internal/domain/route/receiver/issuer_unix.go",
			"internal/domain/route/receiver/outer_authority_unix.go",
			"internal/domain/route/receiver/receiver_unix.go",
			"internal/domain/route/receiver/receiving_admission_unix.go",
			"internal/domain/route/receiver/admission_retirement_unix_test.go",
			"internal/domain/route/receiver/introduction_parent_unix_test.go",
			"internal/domain/route/receiver/join_failure_unix_test.go",
			"internal/domain/route/receiver/outer_authority_unix_test.go",
			"internal/domain/route/receiver/receiver_retirement_unix_test.go",
			"internal/domain/network/source/serving/declaration_unix.go",
			"internal/domain/network/source/serving/events_unix.go",
			"internal/domain/network/source/serving/inputs_unix.go",
			"internal/domain/network/source/serving/report_unix.go",
			"internal/domain/network/source/serving/serving_unix.go",
			"internal/domain/network/source/serving/supervision_unix.go",
			"internal/domain/network/source/serving/declaration_unix_test.go",
			"internal/domain/network/source/serving/inputs_unix_test.go",
			"internal/domain/network/source/serving/supervision_unix_test.go",
			"internal/platform/artifacts/observed_inputs_unix.go",
			"internal/platform/artifacts/receipt_unix.go",
		} {
			selected, err := profile.MatchFile(path.Join(root, path.Dir(name)), path.Base(name))
			if err != nil {
				t.Fatal(err)
			}
			if want := target != "windows"; selected != want {
				t.Errorf("Unix-family domain composition %s on %s: %v, want %v", name, target, selected, want)
			}
		}
		for _, name := range []string{
			"internal/domain/network/source/serving/serving_other.go",
			"internal/domain/network/source/serving/serving_other_test.go",
			"internal/domain/node/runtime/start_other.go",
		} {
			selected, err := profile.MatchFile(path.Join(root, path.Dir(name)), path.Base(name))
			if err != nil {
				t.Fatal(err)
			}
			if want := target == "windows"; selected != want {
				t.Errorf("Unix-family domain refusal %s on %s: %v, want %v", name, target, selected, want)
			}
		}
		for _, name := range []string{
			"internal/domain/network/source/inspection/inspection_linux.go",
			"internal/domain/network/source/inspection/inspection_linux_test.go",
		} {
			selected, err := profile.MatchFile(path.Join(root, path.Dir(name)), path.Base(name))
			if err != nil {
				t.Fatal(err)
			}
			if want := target == "linux"; selected != want {
				t.Errorf("Linux Source inspection boundary %s on %s: %v, want %v", name, target, selected, want)
			}
		}
		for _, name := range []string{"inspection_other.go", "inspection_other_test.go"} {
			selected, err := profile.MatchFile(path.Join(root, "internal/domain/network/source/inspection"), name)
			if err != nil {
				t.Fatal(err)
			}
			if want := target != "linux"; selected != want {
				t.Errorf("Source inspection platform refusal %s on %s: %v, want %v", name, target, selected, want)
			}
		}
	}
}

// P1-B batch 2 (#787, #691) de-tagged verified pure production files whose
// composed symbols became portable in batch 1: the Node failure and shared Admission facts
// projections, the endpoint/funding composition and the shared reservation
// and session deadline helpers. They add no native mechanism, so every mandatory target
// must select them and their portable behavior tests. The Node first-failure
// physical-join test composes the Unix-family forwarding lifetime; Cohort 2a
// (#856) registers it with the de-tagged node composition.
func TestP1BBatch2PureProductionRunsOnEveryMandatoryTarget(t *testing.T) {
	root := repositoryRoot(t)
	for _, target := range []string{"linux", "windows", "darwin"} {
		profile := build.Default
		profile.GOOS, profile.GOARCH, profile.CgoEnabled = target, "amd64", false
		for _, name := range []string{
			"internal/domain/node/runtime/failure.go",
			"internal/domain/node/runtime/failure_test.go",
			"internal/domain/admission/authority.go",
			"internal/domain/admission/authority_test.go",
			"internal/domain/hosting/reservation.go",
			"internal/domain/endpoint/funding/issuance.go",
			"internal/domain/endpoint/funding/parents.go",
			"internal/domain/endpoint/funding/prefix.go",
			"internal/domain/endpoint/funding/preparation.go",
			"internal/domain/endpoint/funding/source.go",
			"internal/domain/endpoint/funding/observation.go",
			"internal/domain/endpoint/funding/preparation_test.go",
			"internal/domain/endpoint/session/reservation.go",
			"internal/domain/endpoint/session/reservation_test.go",
		} {
			selected, err := profile.MatchFile(path.Join(root, path.Dir(name)), path.Base(name))
			if err != nil {
				t.Fatal(err)
			}
			if !selected {
				t.Errorf("portable P1-B batch 2 file %s excluded on %s", name, target)
			}
		}
	}
}

// P1-B batch 3 (#794, #691) de-tagged the next verified pure leaves whose
// composed symbols are already portable: the Reader result projection, the
// shared root-isolation predicate, the Publisher document stream contract, and
// the shared Network presentation guard. Every mandatory target must select
// these files and their portable behavior tests.
func TestP1BBatch3PureProductionRunsOnEveryMandatoryTarget(t *testing.T) {
	root := repositoryRoot(t)
	for _, target := range []string{"linux", "windows", "darwin"} {
		profile := build.Default
		profile.GOOS, profile.GOARCH, profile.CgoEnabled = target, "amd64", false
		for _, name := range []string{
			"internal/domain/endpoint/reader/result.go",
			"internal/domain/endpoint/reader/result_test.go",
			"internal/pathcheck/absolute_path.go",
			"internal/domain/endpoint/publisher/roots_test.go",
			"internal/domain/endpoint/publisher/contract.go",
			"internal/domain/endpoint/funding/intent.go",
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

// Cohort 2d (#877, #691) shifted the Endpoint session boundary to the Unix family
// after the prerequisite seams merged: Instance custody (#867), reachability Store
// (#870), Hosting lease + IFMIB measurement (#871/#873) and the worker launch
// capability port (#875). The session Open* compositions (prefix/bootstrap/join)
// compose only selection (Unix-family roots), Hosting (Unix-family lease +
// Linux/macOS measurement) and the portable Route core; they launch no native
// worker, so on macOS they run the genuine composition and on Windows the shifted
// stub refuses before any root state. They are linux || darwin and their refusal
// narrows to Windows (!linux && !darwin).
//
// The Publisher composition (#883, #691) now shifts the same way after the durable
// lease seam port (#880/#882) made publication/durable native on Linux and macOS
// (plus its existing Windows lease). opening composes the Unix-family
// selection/Hosting/Instance/durable roots with the portable domain Publisher and
// launches no native worker itself; publisher launches the native worker
// (LaunchPublisher/LaunchByteStreamPublisher) BEFORE openPublication, so macOS
// receives the typed worker refusal before any root state; snapshot imports the
// caller input read-only (document.ReadSnapshotFile creates no filesystem state or
// authority) before the same typed launch refusal; byte_stream composes only the
// portable application/endpointstream contract, not the Refuse typed-refusal projection.
// They move to linux || darwin and their opening/snapshot refusals narrow to
// Windows (!linux && !darwin). macOS worker launch stays a typed refusal
// (execution/runtime !linux stubs); no unconfined worker is spawned and Linux
// behavior is byte-identical.
//
// The Reader coordinator (#885, #691) now shifts the same way after the pure
// typed refusal constructor application/endpointstream.Refuse was split into the
// portable terminal_outcome.go (the Linux-only setup-refusal
// setupCleanupError classifier stays with the protected Linux server admission).
// reader/operation/source/byte_stream compose only portable or Unix-family roots
// and the typed !linux LaunchReader/LaunchByteStreamReader worker refusals, so on
// macOS beginRead reaches the typed worker refusal before any Source or root
// state; they are linux || darwin. The Reader has no Windows refusal stub: its
// sole production consumer is the Linux participant, so the composition adds no
// Windows production code. The participant STAYS Linux: it composes endpoint/launch
// systemd delivery plus the declarations and permission Linux projections. The
// refusal-behavior tests pin that every unsupported host refuses before any state
// or authority. A future port of the worker launch / systemd delivery seams must
// update this assertion deliberately.
func TestCohort2dEndpointNativeBoundary(t *testing.T) {
	root := repositoryRoot(t)
	unixComposition := []string{
		"internal/domain/endpoint/session/prefix_unix.go",
		"internal/domain/endpoint/session/bootstrap_unix.go",
		"internal/domain/endpoint/session/join_unix.go",
		"internal/domain/endpoint/publisher/opening_unix.go",
		"internal/domain/endpoint/publisher/publisher_unix.go",
		"internal/domain/endpoint/publisher/snapshot_unix.go",
		"internal/domain/endpoint/publisher/byte_stream_unix.go",
		"internal/domain/endpoint/reader/reader_unix.go",
		"internal/domain/endpoint/reader/operation_unix.go",
		"internal/domain/endpoint/reader/source_unix.go",
		"internal/domain/endpoint/reader/byte_stream_unix.go",
	}
	windowsRefusal := []string{
		"internal/domain/endpoint/session/route_other.go",
		"internal/domain/endpoint/session/route_other_test.go",
		"internal/domain/endpoint/publisher/opening_other.go",
		"internal/domain/endpoint/publisher/opening_other_test.go",
		"internal/domain/endpoint/publisher/snapshot_other.go",
		"internal/domain/endpoint/publisher/snapshot_other_test.go",
	}
	linuxOnly := []string{
		"internal/domain/endpoint/participant_linux.go",
	}
	for _, target := range []string{"linux", "windows", "darwin"} {
		profile := build.Default
		profile.GOOS, profile.GOARCH, profile.CgoEnabled = target, "amd64", false
		for _, name := range unixComposition {
			selected, err := profile.MatchFile(path.Join(root, path.Dir(name)), path.Base(name))
			if err != nil {
				t.Fatal(err)
			}
			if want := target != "windows"; selected != want {
				t.Errorf("Unix-family Endpoint session composition %s on %s: %v, want %v", name, target, selected, want)
			}
		}
		for _, name := range windowsRefusal {
			selected, err := profile.MatchFile(path.Join(root, path.Dir(name)), path.Base(name))
			if err != nil {
				t.Fatal(err)
			}
			if want := target == "windows"; selected != want {
				t.Errorf("Endpoint session Windows refusal %s on %s: %v, want %v", name, target, selected, want)
			}
		}
		for _, name := range linuxOnly {
			selected, err := profile.MatchFile(path.Join(root, path.Dir(name)), path.Base(name))
			if err != nil {
				t.Fatal(err)
			}
			if want := target == "linux"; selected != want {
				t.Errorf("native Endpoint composition %s on %s: %v, want %v", name, target, selected, want)
			}
		}
	}
}

func TestLocalApplicationStreamPlatformBoundaries(t *testing.T) {
	root := repositoryRoot(t)
	cases := []struct {
		file    string
		targets string
	}{
		{"internal/application/endpointstream/server/doc.go", "linux"},
		{"internal/application/endpointstream/server/listener_linux.go", "linux"},
		{"internal/application/endpointstream/server_frames_linux.go", "linux"},
		{"internal/application/console/descriptors_linux.go", "linux"},
		{"internal/application/console/descriptors_other.go", "darwin windows"},
		{"internal/application/endpointstream/client/host_linux.go", "linux"},
		{"internal/application/endpointstream/client/host_other.go", "darwin windows"},
		{"internal/application/endpointstream/targetlink/private_file_unix.go", "linux darwin"},
		{"internal/application/endpointstream/targetlink/private_file_other.go", "windows"},
	}
	for _, target := range []string{"linux", "darwin", "windows"} {
		profile := build.Default
		profile.GOOS, profile.GOARCH, profile.CgoEnabled = target, "amd64", false
		for _, test := range cases {
			selected, err := profile.MatchFile(path.Join(root, path.Dir(test.file)), path.Base(test.file))
			if err != nil {
				t.Fatal(err)
			}
			want := slices.Contains(strings.Fields(test.targets), target)
			if selected != want {
				t.Errorf("%s on %s selected=%v, want %v", test.file, target, selected, want)
			}
		}
	}
}
