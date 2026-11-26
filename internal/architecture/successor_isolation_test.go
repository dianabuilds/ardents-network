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
			"internal/domain/network/state/retained_open.go",
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

func TestInfrastructureExecutableMechanismHasExactPlatformSelection(t *testing.T) {
	root := repositoryRoot(t)
	for _, target := range []string{"linux", "windows", "darwin"} {
		profile := build.Default
		profile.GOOS, profile.GOARCH, profile.CgoEnabled = target, "amd64", false
		for _, name := range []string{
			"internal/domain/enrollment/infrastructure_inventory.go",
			"internal/domain/enrollment/infrastructure_inventory_test.go",
			"internal/domain/enrollment/descriptor_inventory_linux_test.go",
			"internal/domain/enrollment/infrastructure_process_linux.go",
			"internal/platform/systemd/infrastructure_prestart_linux.go",
			"internal/platform/systemd/infrastructure_prestart_linux_test.go",
			"internal/domain/release/infrastructure/admission.go",
			"internal/domain/release/infrastructure/source_release_inspection_linux.go",
			"internal/domain/release/infrastructure/preparation_linux.go",
			"internal/platform/cgroup/source_quiescence_linux.go",
			"internal/domain/network/state/source_inspection_linux.go",
			"internal/domain/network/state/source_preparation_linux.go",
			"internal/domain/endpoint/source_trust_preparation_linux.go",
			"internal/domain/network/state/inspection_state_linux.go",
			"internal/domain/network/state/node_inspection_linux.go",
			"internal/domain/network/state/durable/inspection_linux.go",
			"internal/domain/network/duty/source_inspection_linux.go",
			"internal/domain/network/duty/inspection_linux.go",
			"internal/domain/network/duty/node_inspection_linux.go",
			"internal/admission/spending/inspection_linux.go",
			"internal/domain/hosting/inspection_linux.go",
			"internal/domain/node/provisioning/command_linux.go",
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
			"internal/domain/release/infrastructure/signing/join_release_keys_linux.go",
			"internal/domain/release/infrastructure/signing/infrastructure_release_keys_linux.go",
			"internal/domain/release/infrastructure/signing/node_release_plan_linux.go",
			"internal/domain/release/infrastructure/signing/node_release_arguments_linux.go",
			"internal/domain/release/infrastructure/signing/node_release_linux.go",
			"internal/domain/release/infrastructure/signing/node_release_predecessor_linux.go",
			"internal/domain/release/infrastructure/signing/join_release_plan_linux_test.go",
			"internal/domain/release/infrastructure/join_admission.go",
			"internal/platform/artifacts/output_linux.go",
			"internal/domain/release/infrastructure/signing/source_release_linux.go",
			"internal/domain/release/infrastructure/signing/source_release_predecessor_linux.go",
			"internal/domain/release/infrastructure/signing/source_release_keys_linux.go",
			"internal/domain/release/infrastructure/signing/source_release_plan_linux.go",
			"internal/domain/release/infrastructure/signing/source_release_plan_linux_test.go",
			"internal/domain/node/provisioning/source_initial_keys_linux.go",
			"internal/domain/node/provisioning/source_initial_network_linux.go",
			"internal/domain/node/provisioning/source_initial_network_linux_test.go",
			"internal/domain/node/provisioning/source_initial_plan_linux.go",
			"internal/platform/artifacts/key_linux.go",
			"internal/platform/systemd/node_slots_linux.go",
			"internal/platform/systemd/node_slots_linux_test.go",
			"internal/platform/systemd/source_object_linux_test.go",
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
			"internal/domain/release/infrastructure/doc.go",
			"internal/domain/release/infrastructure/artifact.go",
			"internal/domain/release/infrastructure/artifact_test.go",
			"internal/domain/release/infrastructure/descriptor_artifact.go",
			"internal/domain/release/infrastructure/forwarding_artifact.go",
			"internal/domain/release/infrastructure/join_artifact.go",
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
				"internal/admission/spending/inspection_native_linux_test.go",
				"internal/domain/hosting/inspection_native_linux_test.go",
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
			"internal/domain/node/credential_unix.go",
			"internal/domain/node/credential_unix_test.go",
			"internal/domain/node/join_credential_unix.go",
			"internal/domain/node/join_credential_unix_test.go",
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
			"internal/domain/node/provisioning/network_preparation_linux.go",
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
			"internal/domain/hosting/lease_unix.go",
			"internal/domain/hosting/lease_unix_test.go",
			"internal/domain/hosting/budget_unix_test.go",
			"internal/domain/hosting/reservation_unix_test.go",
			"internal/domain/hosting/deadline_unix_test.go",
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
			"internal/domain/hosting/lease_other.go",
			"internal/domain/hosting/measure_other.go",
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
			"internal/domain/hosting/measure_linux.go",
			"internal/domain/hosting/counter_linux_test.go",
			"internal/domain/hosting/reservation_process_linux_test.go",
			"internal/domain/hosting/inspection_linux.go",
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
			"internal/domain/hosting/measure_darwin.go",
			"internal/domain/hosting/measure_darwin_test.go",
		} {
			selected, err := profile.MatchFile(path.Join(root, path.Dir(name)), path.Base(name))
			if err != nil {
				t.Fatal(err)
			}
			if want := target == "darwin"; selected != want {
				t.Errorf("macOS hosting measure boundary %s on %s: %v, want %v", name, target, selected, want)
			}
		}
		selected, err := profile.MatchFile(path.Join(root, "internal/domain/hosting"), "inspection_other.go")
		if err != nil {
			t.Fatal(err)
		}
		if want := target != "linux"; selected != want {
			t.Errorf("hosting inspection refusal on %s: %v, want %v", target, selected, want)
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
			"internal/domain/node/provisioning/node_provisioning_role_unix.go",
			"internal/domain/node/provisioning/public_identifier_unix.go",
			"internal/domain/node/provisioning/public_identifier_unix_test.go",
			"internal/domain/node/provisioning/node_certificate_unix.go",
			"internal/domain/node/provisioning/node_certificate_unix_test.go",
			"internal/domain/node/provisioning/network_report_unix.go",
			"internal/domain/node/provisioning/node_record_unix.go",
			"internal/domain/node/provisioning/forwarding_node_record_unix_test.go",
			"internal/domain/node/provisioning/network_signing_declaration_unix.go",
			"internal/domain/node/provisioning/network_certificate_signing_unix.go",
			"internal/domain/node/provisioning/network_certificate_signing_unix_test.go",
			"internal/domain/node/provisioning/network_profile_signing_unix.go",
			"internal/domain/node/provisioning/network_profile_signing_test.go",
			"internal/domain/node/provisioning/network_signing_fixture_unix_test.go",
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
			"internal/domain/node/provisioning/network_preparation_linux.go",
			"internal/domain/node/provisioning/command_linux.go",
			"internal/domain/node/provisioning/network_signing_linux.go",
			"internal/domain/node/provisioning/initial_network_keys_linux.go",
			"internal/platform/artifacts/output_linux.go",
			"internal/platform/artifacts/protected_input_linux.go",
			"internal/platform/artifacts/protected_storage_linux.go",
			"internal/domain/enrollment/infrastructure_inventory.go",
			"internal/domain/enrollment/infrastructure_process_linux.go",
			"internal/domain/release/infrastructure/preparation_linux.go",
			"internal/domain/release/infrastructure/admission.go",
			"internal/domain/release/infrastructure/source_release_inspection_linux.go",
			"internal/domain/release/infrastructure/signing/node_release_linux.go",
			"internal/domain/release/infrastructure/signing/source_release_linux.go",
			"internal/domain/release/infrastructure/signing/node_release_plan_linux.go",
			"internal/domain/release/infrastructure/signing/source_release_plan_linux.go",
			"internal/domain/release/infrastructure/signing/infrastructure_release_keys_linux.go",
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
			"internal/route/introduction/slot_history_unix.go",
			"internal/route/introduction/slot_history_unix_test.go",
			"internal/route/introduction/receiving_history_unix.go",
			"internal/route/introduction/receiving_history_unix_test.go",
			"internal/route/introduction/history_custody_unix_test.go",
			"internal/route/introduction/history_write_unix_test.go",
			"internal/route/introduction/registry_unix_test.go",
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
			"internal/route/introduction/slot_history_other.go",
			"internal/route/introduction/receiving_history_other.go",
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
			"internal/route/introduction/slot_history_watch_linux.go",
			"internal/route/introduction/history_crash_linux_amd64_test.go",
		} {
			selected, err := profile.MatchFile(path.Join(root, path.Dir(name)), path.Base(name))
			if err != nil {
				t.Fatal(err)
			}
			if want := target == "linux"; selected != want {
				t.Errorf("Linux root-move watch boundary %s on %s: %v, want %v", name, target, selected, want)
			}
		}
		selected, err := profile.MatchFile(path.Join(root, "internal/route/introduction"), "slot_history_watch_darwin.go")
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
			"internal/domain/node/descriptor_unix.go",
			"internal/domain/node/forwarding_admission_unix.go",
			"internal/domain/node/forwarding_unix.go",
			"internal/domain/node/forwarding_observation_unix.go",
			"internal/domain/node/introduction_unix.go",
			"internal/domain/node/issuer_unix.go",
			"internal/domain/node/join_unix.go",
			"internal/domain/node/lifetime_unix.go",
			"internal/domain/node/runtime_unix.go",
			"internal/domain/node/descriptor_startup_unix_test.go",
			"internal/domain/node/failure_projection_unix_test.go",
			"internal/domain/node/introduction_startup_unix_test.go",
			"internal/domain/node/join_startup_unix_test.go",
			"internal/domain/node/lifetime_equivalence_unix_test.go",
			"internal/domain/node/retirement_unix_test.go",
			"internal/route/receiver/admission_retirement_unix.go",
			"internal/route/receiver/bootstrap_unix.go",
			"internal/route/receiver/descriptor_unix.go",
			"internal/route/receiver/issuer_unix.go",
			"internal/route/receiver/outer_authority_unix.go",
			"internal/route/receiver/receiver_unix.go",
			"internal/route/receiver/receiving_admission_unix.go",
			"internal/route/receiver/admission_retirement_unix_test.go",
			"internal/route/receiver/introduction_parent_unix_test.go",
			"internal/route/receiver/join_failure_unix_test.go",
			"internal/route/receiver/outer_authority_unix_test.go",
			"internal/route/receiver/receiver_retirement_unix_test.go",
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
			"internal/domain/node/runtime_other.go",
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
// composed symbols became portable in batch 1: the Node failure/admission
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
			"internal/domain/node/failure_projection.go",
			"internal/domain/node/failure_projection_test.go",
			"internal/domain/node/admission_projection.go",
			"internal/domain/node/admission_projection_test.go",
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
			"internal/pathgrammar/strict_path.go",
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
// portable application/connection contract, not the Refuse typed-refusal projection.
// They move to linux || darwin and their opening/snapshot refusals narrow to
// Windows (!linux && !darwin). macOS worker launch stays a typed refusal
// (execution/runtime !linux stubs); no unconfined worker is spawned and Linux
// behavior is byte-identical.
//
// The Reader coordinator (#885, #691) now shifts the same way after the pure
// typed refusal constructor application/connection.Refuse was split into the
// portable server_refusal.go (byte-identical; the Linux-only refusal
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
		"internal/admission", "internal/canonicaldigest", "internal/domain/connection", "internal/domain/custody", "internal/durablefile", "internal/domain/endpoint",
		"internal/domain/enrollment", "internal/domain/execution", "internal/domain/hosting", "internal/domain/network", "internal/domain/node",
		"internal/pathgrammar", "internal/platform/artifacts", "internal/platform/cgroup", "internal/platform/process", "internal/platform/systemd", "internal/processresource", "internal/domain/publication",
		"internal/domain/reachability", "internal/domain/release", "internal/route",
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
	if local == "internal/domain/endpoint" && (source == "internal/domain/endpoint/installed_service_linux_test.go" || source == "internal/domain/endpoint/endpoint_journey_native_linux_test.go") {
		return false
	}
	switch source {
	case "internal/domain/release/infrastructure/infrastructure_inventory_refusal_linux_test.go":
		if dependency == modulePath+"/tests/releasefixture" {
			return true
		}
	case "internal/domain/release/infrastructure/source_release_linux_test.go":
		if dependency == modulePath+"/tests/releasefixture" || dependency == "github.com/theupdateframework/go-tuf/v2/metadata" {
			return true
		}
	}
	if source == "internal/domain/release/infrastructure/platform_release_linux_test.go" {
		switch dependency {
		case modulePath + "/internal/domain/network/duty", modulePath + "/internal/domain/network/state",
			modulePath + "/internal/platform/systemd", modulePath + "/tests/epochfixture/network",
			"github.com/theupdateframework/go-tuf/v2/metadata":
			return true
		}
	}
	// Installed Endpoint tests have exact fixture edges and exercise the original composition.
	if localImport {
		switch source {
		case "internal/domain/endpoint/connection_accepted_request_installed_linux_test.go":
			switch local {
			case "internal/admission/receiving", "internal/domain/connection", "internal/domain/execution/runtime", "internal/domain/execution/worker",
				"internal/domain/hosting", "internal/route/ardp", "internal/route/receiver", "internal/route/transport",
				"tests/e2e/endpoint", "tests/e2e/node/processfixture":
				return true
			}
		case "internal/domain/endpoint/connection_authority_linux_test.go":
			switch local {
			case "internal/route/transport":
				return true
			}
		case "internal/domain/endpoint/connection_installed_linux_test.go":
			switch local {
			case "internal/admission", "internal/admission/issuer", "internal/admission/quota", "internal/admission/stock",
				"internal/admission/token", "internal/domain/connection", "internal/domain/endpoint/funding", "internal/domain/endpoint/publisher",
				"internal/domain/execution", "internal/domain/execution/runtime", "internal/domain/hosting", "internal/domain/network",
				"internal/domain/publication/runtime", "internal/domain/reachability", "internal/route", "internal/route/introduction",
				"internal/route/prefix", "internal/route/receiver", "internal/route/selection", "internal/route/transport",
				"tests/e2e/endpoint", "tests/e2e/node/processfixture":
				return true
			}
		case "internal/domain/endpoint/connection_response_loss_linux_test.go":
			switch local {
			case "internal/application/connection", "internal/domain/connection", "internal/domain/endpoint/publisher", "internal/domain/execution/runtime",
				"internal/domain/network", "internal/domain/publication/runtime", "internal/route/transport":
				return true
			}
		case "internal/domain/endpoint/document_command_installed_linux_test.go":
			switch local {
			case "internal/admission", "internal/application/administration", "internal/application/connection", "internal/application/document",
				"internal/domain/endpoint/funding", "internal/domain/endpoint/publisher", "internal/domain/endpoint/reader", "internal/domain/execution",
				"internal/domain/execution/runtime", "internal/domain/hosting", "internal/domain/reachability", "internal/route/selection",
				"internal/route/transport", "tests/e2e/endpoint":
				return true
			}
		case "internal/domain/endpoint/endpoint_journey_native_linux_test.go":
			switch local {
			case "internal/application/administration", "internal/domain/publication", "internal/route/transport", "tests/e2e/endpoint":
				return true
			}
		case "internal/domain/endpoint/endpoint_latency_native_linux_test.go":
			switch local {
			case "internal/route/transport", "tests/e2e/endpoint":
				return true
			}
		case "internal/domain/endpoint/endpoint_launch_fixture_linux_test.go":
			switch local {
			case "tests/e2e/endpoint":
				return true
			}
		case "internal/domain/endpoint/endpoint_replacement_worker_native_linux_test.go":
			switch local {
			case "internal/route/transport", "tests/e2e/endpoint":
				return true
			}
		case "internal/domain/endpoint/execution_installed_linux_test.go":
			switch local {
			case "internal/admission", "internal/domain/endpoint/funding", "internal/domain/endpoint/session", "internal/domain/hosting",
				"internal/domain/network/state", "internal/route/transport", "tests/e2e/endpoint", "tests/e2e/node/processfixture",
				"tests/epochfixture/network", "tests/networkfixture":
				return true
			}
		case "internal/domain/endpoint/installed_service_linux_test.go":
			switch local {
			case "internal/domain/reachability", "internal/route/transport", "tests/e2e/endpoint":
				return true
			}
		case "internal/domain/endpoint/network_admission_fixture_linux_test.go":
			switch local {
			case "internal/admission", "internal/admission/issuer", "internal/admission/quota", "internal/admission/receiving",
				"internal/domain/endpoint/funding",
				"internal/domain/network", "internal/domain/network/epoch", "internal/domain/network/state", "tests/e2e/node/processfixture",
				"tests/epochfixture/network", "tests/networkfixture":
				return true
			}
		case "internal/domain/endpoint/publication_ack_installed_linux_test.go":
			switch local {
			case "internal/admission", "internal/admission/stock", "internal/domain/endpoint/funding", "internal/domain/endpoint/publisher",
				"internal/domain/execution", "internal/domain/execution/runtime", "internal/domain/hosting", "internal/domain/network",
				"internal/domain/publication", "internal/domain/reachability", "internal/route/introduction", "internal/route/prefix",
				"internal/route/transport", "tests/e2e/endpoint", "tests/e2e/node/processfixture":
				return true
			}
		case "internal/domain/endpoint/publication_installed_linux_test.go":
			switch local {
			case "internal/admission", "internal/admission/issuer", "internal/admission/quota", "internal/admission/receiving",
				"internal/admission/stock", "internal/admission/token", "internal/domain/endpoint/funding", "internal/domain/endpoint/publisher",
				"internal/domain/endpoint/session", "internal/domain/hosting", "internal/domain/network", "internal/domain/network/state",
				"internal/domain/publication", "internal/domain/publication/instance", "internal/domain/reachability", "internal/route/ardp",
				"internal/route/capsule", "internal/route/introduction", "internal/route/receiver", "internal/route/role",
				"internal/route/transport", "tests/e2e/node/processfixture", "tests/epochfixture/network", "tests/networkfixture":
				return true
			}
		case "internal/domain/endpoint/route_fixture_linux_test.go":
			return local == "internal/route/transport" || local == "tests/e2e/node/processfixture"
		case "internal/domain/endpoint/installed_route_fixture_linux_test.go":
			switch local {
			case "internal/admission", "internal/admission/receiving", "internal/admission/stock", "internal/domain/endpoint/session",
				"internal/domain/endpoint/funding", "internal/domain/hosting", "internal/route/introduction", "tests/e2e/node/processfixture":
				return true
			}
		case "internal/domain/endpoint/route_issuer_fixture_linux_test.go":
			switch local {
			case "internal/admission", "internal/admission/issuer", "internal/admission/quota", "internal/admission/receiving",
				"internal/domain/hosting", "internal/route/receiver", "internal/route/role", "internal/route/transport",
				"tests/e2e/node/processfixture":
				return true
			}
		}
	}
	if localImport && (source == "tests/e2e/node/descriptor_platform_native_linux_test.go" || source == "tests/e2e/node/forwarding_platform_native_linux_test.go" || source == "tests/e2e/node/join_platform_native_linux_test.go") {
		switch local {
		case "internal/admission", "internal/domain/hosting", "internal/domain/network/duty", "internal/domain/network/state", "internal/platform/systemd",
			"internal/processresource", "internal/domain/reachability", "internal/domain/release/infrastructure", "internal/domain/release/infrastructure/launch",
			"tests/e2e/node/processfixture", "tests/epochfixture/network", "tests/networkfixture", "tests/releasefixture":
			return true
		}
	}
	if localImport && source == "tests/e2e/node/introduction_repair_platform_native_linux_test.go" {
		switch local {
		case "internal/domain/hosting", "internal/domain/network/duty", "internal/domain/network/state", "internal/platform/systemd",
			"internal/processresource", "internal/domain/release/infrastructure", "internal/domain/release/infrastructure/launch",
			"tests/e2e/node/processfixture", "tests/epochfixture/network", "tests/networkfixture", "tests/releasefixture":
			return true
		default:
			return false
		}
	}
	if dependency == modulePath+"/internal/admission/operator" {
		return source == "cmd/ardents-next/admission.go" || source == "cmd/ardents-next/issuance.go" || source == "cmd/ardents-next/command.go"
	}
	if strings.HasPrefix(source, "internal/domain/custody/command/") {
		// The package map also checks the package-wide import set.
		if dependency == modulePath+"/internal/domain/custody" {
			switch source {
			case "internal/domain/custody/command/command.go", "internal/domain/custody/command/terminal_secret.go",
				"internal/domain/custody/command/command_test.go", "internal/domain/custody/command/service_authority.go",
				"internal/domain/custody/command/admission_authority.go", "internal/domain/custody/command/verify_record.go",
				"internal/domain/custody/command/recovery_bundle.go", "internal/domain/custody/command/purge_record.go",
				"internal/domain/custody/command/admission_permission_client_linux_test.go":
				return true
			}
		}
		if source == "internal/domain/custody/command/command_test.go" && dependency == modulePath+"/internal/domain/publication" ||
			source == "internal/domain/custody/command/admission_permission_client_linux_test.go" && dependency == modulePath+"/internal/admission" {
			return true
		}
	}
	if source == "cmd/ardents-custody/main.go" && dependency == modulePath+"/internal/domain/custody/command" {
		return true
	}
	switch source {
	case "internal/domain/custody/command/terminal_secret.go":
		if dependency == "golang.org/x/term" {
			return true
		}
	case "internal/domain/custody/command/terminal_line_linux.go":
		if dependency == "golang.org/x/sys/unix" || dependency == "golang.org/x/term" {
			return true
		}
	case "internal/domain/custody/command/terminal_line_windows.go":
		if dependency == "golang.org/x/sys/windows" {
			return true
		}
	case "internal/domain/custody/command/terminal_secret_linux_test.go":
		if dependency == "github.com/aymanbagabas/go-pty" {
			return true
		}
	}
	if (source == "internal/domain/endpoint/funding/intent_network_test.go" || source == "internal/admission/receiving/registration_initialization_integration_test.go") && localImport && local == "tests/epochfixture/network" {
		return true
	}
	if source == "internal/admission/receiving/registration_initialization_integration_test.go" && localImport && local == "internal/domain/network/state" {
		return true
	}
	if localImport && source == "internal/domain/endpoint/session/issuance_recipient_test.go" {
		switch local {
		case "internal/admission", "internal/admission/issuer", "internal/admission/quota", "internal/admission/stock",
			"internal/admission/token", "internal/domain/endpoint/funding", "internal/domain/endpoint/session", "tests/e2e/node/processfixture", "tests/networkfixture":
			return true
		default:
			return false
		}
	}
	if localImport && source == "internal/admission/receiving/network_lifetime_linux_test.go" {
		return local == "internal/admission" || local == "internal/admission/receiving" ||
			local == "internal/admission/spending" || local == "tests/networkfixture"
	}
	if localImport && source == "internal/admission/receiving/registration_allowance_linux_test.go" {
		return local == "internal/admission" || local == "internal/admission/receiving" ||
			local == "tests/epochfixture/network" || local == "tests/networkfixture"
	}
	if localImport && (source == "internal/admission/receiving/network_reservation_linux_test.go" || source == "internal/admission/receiving/network_joined_work_linux_test.go") {
		switch local {
		case "internal/admission", "internal/admission/allocation", "internal/admission/issuer", "internal/admission/quota",
			"internal/admission/receiving", "internal/admission/spending", "internal/admission/token", "internal/domain/hosting", "internal/domain/network/state",
			"tests/epochfixture/network", "tests/networkfixture":
			return true
		default:
			return false
		}
	}
	if strings.HasPrefix(source, "tests/networkfixture/") && localImport {
		switch source {
		case "tests/networkfixture/genesis.go":
			return local == "internal/admission/issuance" || local == "internal/admission/issuer" || local == "internal/admission/quota" || local == "internal/domain/network/epoch" || local == "tests/epochfixture/network"
		case "tests/networkfixture/work_authority.go", "tests/networkfixture/loopback_test.go":
			return local == "internal/admission"
		case "tests/networkfixture/signed_admission.go":
			return local == "internal/admission" || local == "internal/admission/issuer" || local == "internal/domain/network" ||
				local == "internal/domain/network/state" || local == "tests/epochfixture/network"
		case "tests/networkfixture/issuer_conflict_test.go":
			switch local {
			case "internal/admission", "internal/admission/allocation", "internal/admission/issuer", "internal/admission/quota", "internal/admission/token", "internal/domain/network/state", "tests/epochfixture/network":
				return true
			default:
				return false
			}
		case "tests/networkfixture/state.go", "tests/networkfixture/independent_state.go", "tests/networkfixture/genesis_test.go", "tests/networkfixture/isolated_profile_test.go":
			return local == "internal/domain/network/state" || local == "tests/epochfixture/network"
		default:
			return false
		}
	}
	// Signed Network fixture inputs are available only to these test owners.
	if localImport && local == "tests/networkfixture" {
		switch source {
		case "tests/e2e/node/forwarding_exchange_native_linux_test.go":
			return true
		case "tests/e2e/node/hosting_work_fixture_linux_test.go", "tests/e2e/node/hosting_work_lifecycle_linux_test.go":
			return true
		case
			"internal/admission/receiving/registration_initialization_integration_test.go",
			"internal/domain/endpoint/execution_installed_linux_test.go",
			"internal/domain/endpoint/network_admission_fixture_linux_test.go",
			"internal/domain/endpoint/publication_installed_linux_test.go",
			"internal/domain/endpoint/funding/intent_network_test.go",
			"internal/domain/network/source/command/state_lifetime_linux_test.go",
			"internal/domain/node/authority_process_native_linux_test.go",
			"internal/domain/node/introduction_exchange_native_linux_test.go",
			"internal/domain/node/operation_workload_native_linux_test.go",
			"internal/domain/node/refill_process_native_linux_test.go",
			"internal/domain/node/route_descriptor_linux_test.go",
			"internal/domain/node/route_join_fixture_linux_test.go",
			"internal/domain/node/route_bootstrap_linux_test.go",
			"internal/domain/node/route_receiving_fixture_linux_test.go",
			"tests/e2e/endpoint/endpoint_history_linux_test.go",
			"tests/e2e/endpoint/launch_refusals_native_linux_test.go",
			"tests/e2e/endpoint/launch_replacement_native_linux_test.go",
			"tests/e2e/endpoint/endpoint_platform_native_linux_test.go",
			"tests/e2e/node/completion_native_linux_test.go",
			"tests/e2e/node/descriptor_exchange_native_linux_test.go",
			"tests/e2e/node/forwarding_startup_native_linux_test.go",
			"tests/e2e/node/issuer_exchange_native_linux_test.go",
			"tests/e2e/node/issuer_startup_native_linux_test.go",
			"tests/e2e/node/join_exchange_native_linux_test.go",
			"tests/e2e/node/network_commands_unix_test.go",
			"tests/e2e/node/receiving_startup_native_linux_test.go",
			"tests/e2e/node/retained_lifecycle_native_linux_test.go",
			"tests/e2e/node/route_bootstrap_linux_test.go",
			"tests/e2e/node/route_descriptor_linux_test.go",
			"tests/e2e/node/route_hello_linux_test.go",
			"tests/e2e/node/route_issuer_recovery_test.go",
			"tests/e2e/node/route_join_linux_test.go",
			"tests/e2e/node/route_quic_linux_test.go",
			"tests/e2e/node/route_registration_linux_test.go",
			"tests/e2e/node/source_installed_native_linux_test.go",
			"tests/e2e/node/source_lifecycle_native_linux_test.go",
			"tests/e2e/node/source_native_linux_test.go",
			"tests/e2e/node/source_preparation_process_linux_test.go",
			"tests/e2e/node/source_refresh_linux_test.go":
			return true
		default:
			return false
		}
	}
	if source == "tests/releasefixture/infrastructure_linux.go" && (dependency == modulePath+"/internal/domain/enrollment" || dependency == modulePath+"/internal/domain/release" || dependency == "github.com/theupdateframework/go-tuf/v2/metadata") {
		return true
	}
	if (source == "internal/domain/release/infrastructure/forwarding_artifact_linux_test.go" || source == "internal/domain/release/infrastructure/authorization_lifetime_linux_test.go") && dependency == modulePath+"/internal/domain/release/infrastructure" {
		return true
	}
	// Independent signed Release data is shared only by these test owners.
	if localImport && local == "tests/releasefixture" {
		switch source {
		case "internal/domain/release/infrastructure/authorization_lifetime_linux_test.go",
			"internal/domain/release/infrastructure/forwarding_artifact_linux_test.go",
			"tests/e2e/node/infrastructure_preparation_native_linux_test.go",
			"internal/domain/release/endpoint/authorization_integration_test.go", "internal/domain/release/endpoint/execution_artifact_test.go",
			"tests/e2e/node/descriptor_platform_native_linux_test.go", "tests/e2e/endpoint/journey_participant_linux.go",
			"tests/e2e/node/forwarding_platform_native_linux_test.go", "tests/e2e/node/join_platform_native_linux_test.go",
			"tests/e2e/endpoint/endpoint_platform_native_linux_test.go", "tests/e2e/endpoint/native_delivery_linux.go", "tests/e2e/endpoint/launch_releases_linux.go",
			"tests/e2e/endpoint/release_process_test.go",
			"internal/domain/release/infrastructure/platform_release_linux_test.go", "tests/e2e/node/source_installed_native_linux_test.go",
			"tests/e2e/node/source_lifecycle_native_linux_test.go", "tests/e2e/node/source_release_linux_test.go":
			return true
		default:
			return false
		}
	}
	// These predecessor adapters must stay retired after their directory was
	// reused. They may not inherit the current Endpoint's domain imports.
	if localImport {
		switch source {
		case "internal/domain/endpoint/runtime.go", "internal/domain/endpoint/source_state.go", "internal/domain/endpoint/worker_grant_linux.go":
			return false
		}
	}
	if source == "tests/e2e/node/network_preparation_chain_native_linux_test.go" && localImport {
		return local == "internal/domain/network/closedprofile" || local == "internal/domain/network/epoch" || local == "internal/domain/network/state" || local == "tests/epochfixture/network"
	}
	if (source == "tests/e2e/node/network_signing_chain_native_linux_test.go" || source == "tests/e2e/node/network_initial_keys_chain_native_linux_test.go" || source == "tests/e2e/node/network_signing_refusals_native_linux_test.go") && localImport {
		return local == "tests/e2e/source"
	}
	if source == "tests/e2e/node/infrastructure_preparation_native_linux_test.go" && localImport {
		return local == "tests/e2e/node" || local == "tests/e2e/source"
	}
	if source == "internal/domain/release/infrastructure/preparation_native_linux_test.go" && localImport {
		return local == "internal/platform/artifacts"
	}
	if localImport && local == "tests/e2e/node/processfixture" {
		switch source {
		case "tests/e2e/node/admission_authority_test.go", "tests/e2e/node/forwarding_exchange_native_linux_test.go", "tests/e2e/node/admission_console_input_test.go",
			"tests/e2e/node/descriptor_exchange_native_linux_test.go", "tests/e2e/node/issuer_exchange_native_linux_test.go", "tests/e2e/node/network_commands_unix_test.go",
			"tests/e2e/node/admission_cycle_test.go", "tests/e2e/node/admission_receiver_test.go", "tests/e2e/node/admission_interrupt_unix_test.go":
			return true
		case "internal/domain/node/route_bootstrap_linux_test.go", "internal/domain/node/route_token_linux_test.go":
			return true
		case "internal/domain/node/route_join_linux_test.go", "internal/domain/node/route_join_fixture_linux_test.go":
			return true
		case "internal/domain/node/route_descriptor_linux_test.go":
			return true
		case "internal/domain/node/publication_registration_linux_test.go", "tests/e2e/endpoint/service_instance_linux.go":
			return true
		case "tests/e2e/node/command_test.go", "tests/e2e/endpoint/command_test.go", "internal/domain/endpoint/connection_accepted_request_installed_linux_test.go":
			return true
		case "tests/e2e/endpoint/endpoint_history_linux_test.go", "tests/e2e/node/source_refresh_linux_test.go":
			return true
		case "tests/e2e/node/route_bootstrap_linux_test.go",
			"tests/e2e/node/route_receiver_process_linux_test.go", "tests/e2e/node/route_registration_linux_test.go",
			"tests/e2e/node/route_issuer_recovery_test.go",
			"tests/e2e/node/route_descriptor_linux_test.go", "tests/e2e/node/route_join_linux_test.go",
			"tests/e2e/node/route_hello_linux_test.go", "tests/e2e/node/route_quic_linux_test.go":
			return true
		case "internal/domain/network/source/command/state_lifetime_linux_test.go",
			"internal/domain/endpoint/connection_installed_linux_test.go",
			"internal/domain/endpoint/execution_installed_linux_test.go",
			"internal/domain/endpoint/network_admission_fixture_linux_test.go",
			"internal/domain/node/introduction_exchange_native_linux_test.go",
			"tests/e2e/node/join_exchange_native_linux_test.go",
			"internal/domain/node/operation_workload_native_linux_test.go",
			"internal/domain/endpoint/publication_ack_installed_linux_test.go",
			"internal/domain/endpoint/publication_installed_linux_test.go",
			"internal/domain/endpoint/route_issuer_fixture_linux_test.go",
			"internal/domain/endpoint/route_fixture_linux_test.go",
			"internal/domain/node/refill_process_native_linux_test.go",
			"internal/domain/node/route_receiving_fixture_linux_test.go",
			"internal/domain/node/route_refusal_linux_test.go", "internal/domain/node/route_refill_linux_test.go", "internal/domain/node/route_registration_linux_test.go", "internal/domain/node/route_submission_linux_test.go",
			"internal/domain/node/authority_process_native_linux_test.go",
			"internal/domain/node/operation_process_native_linux_test.go",
			"tests/e2e/endpoint/endpoint_platform_native_linux_test.go",
			"tests/e2e/node/completion_native_linux_test.go",
			"tests/e2e/node/forwarding_startup_native_linux_test.go",
			"tests/e2e/node/issuer_startup_native_linux_test.go",
			"tests/e2e/node/native_pressure_linux.go",
			"tests/e2e/node/provisioning_linux.go",
			"tests/e2e/node/receiving_startup_native_linux_test.go",
			"tests/e2e/node/repair_native_linux_test.go",
			"tests/e2e/node/retained_lifecycle_native_linux_test.go",
			"tests/e2e/node/source_installed_native_linux_test.go",
			"tests/e2e/node/source_lifecycle_native_linux_test.go",
			"tests/e2e/node/source_native_linux_test.go",
			"tests/e2e/node/source_preparation_process_linux_test.go":
			return true
		case "tests/e2e/node/admission_fresh_roots_unix_test.go",
			"tests/e2e/node/admission_unix_test.go",
			"tests/e2e/node/hosting_commands_linux_test.go",
			"tests/e2e/node/hosting_output_linux_test.go",
			"tests/e2e/node/issuance_commands_unix_test.go",
			"tests/e2e/node/issuance_export_unix_test.go",
			"tests/e2e/node/issuance_input_unix_test.go",
			"tests/e2e/node/issuance_results_unix_test.go",
			"tests/e2e/node/issuance_telemetry_unix_test.go",
			"tests/e2e/node/issuer_profile_input_unix_test.go",
			"tests/e2e/node/issuer_profile_unix_test.go":
			return true
		default:
			return false
		}
	}
	if strings.HasPrefix(source, "internal/domain/node/command/") && localImport {
		switch source {
		case "internal/domain/node/command/process.go":
			return local == "internal/processresource"
		case "internal/domain/node/command/input.go":
			return local == "internal/platform/artifacts"
		case "internal/domain/node/command/report.go", "internal/domain/node/command/report_test.go":
			return local == "internal/domain/node"
		default:
			return false
		}
	}
	if localImport && local == "internal/domain/node/command" {
		switch source {
		case "cmd/ardents-next/node.go", "cmd/ardents-next/node_roles.go":
			return true
		default:
			return false
		}
	}
	if strings.HasPrefix(source, "internal/domain/network/source/command/") && localImport {
		switch source {
		case "internal/domain/network/source/command/prepare_linux.go":
			return local == "internal/domain/endpoint" || local == "internal/domain/network/state" || local == "internal/pathgrammar" || local == "internal/platform/artifacts"
		case "internal/domain/network/source/command/inspect_linux.go":
			return local == "internal/domain/network/source/inspection" || local == "internal/platform/artifacts" || local == "internal/canonicaldigest"
		case "internal/domain/network/source/command/serve_linux.go":
			return local == "internal/domain/network/source/serving" || local == "internal/platform/artifacts"
		case "internal/domain/network/source/command/command_linux_test.go":
			return local == "internal/domain/network/source/serving"
		case "internal/domain/network/source/command/serving_output_linux_test.go":
			return local == "internal/domain/network/source/serving" || local == "internal/domain/network/state"
		case "internal/domain/network/source/command/state_lifetime_linux_test.go":
			return local == "internal/platform/artifacts" || local == "internal/domain/network/duty" ||
				local == "internal/domain/network/source" || local == "internal/domain/network/source/serving" ||
				local == "internal/domain/network/source/servingdeclaration" || local == "internal/domain/network/state" ||
				local == "tests/e2e/node" || local == "tests/e2e/source" || local == "tests/epochfixture/network"
		default:
			return false
		}
	}
	if localImport && local == "internal/domain/network/source/command" {
		switch source {
		case "cmd/ardents-next/source_command.go":
			return true
		default:
			return false
		}
	}
	if first, _, _ := strings.Cut(dependency, "/"); strings.Contains(first, ".") {
		switch source {
		case "tests/e2e/node/hosting_commands_linux_test.go":
			return dependency == modulePath+"/internal/domain/hosting" ||
				dependency == "go.opentelemetry.io/proto/otlp/collector/metrics/v1" ||
				dependency == "go.opentelemetry.io/proto/otlp/collector/trace/v1" || dependency == "google.golang.org/protobuf/proto"
		case "tests/e2e/node/source_lifecycle_native_linux_test.go":
			return dependency == "github.com/dianabuilds/ardents-network/internal/domain/enrollment" || dependency == "github.com/dianabuilds/ardents-network/internal/domain/network/duty" || dependency == "github.com/dianabuilds/ardents-network/internal/domain/network/epoch" || dependency == "github.com/dianabuilds/ardents-network/internal/domain/network/source" || dependency == "github.com/dianabuilds/ardents-network/internal/domain/network/source/servingdeclaration" || dependency == "github.com/dianabuilds/ardents-network/internal/domain/network/state" || dependency == "github.com/dianabuilds/ardents-network/internal/platform/systemd" || dependency == "github.com/dianabuilds/ardents-network/internal/processresource" || dependency == "github.com/dianabuilds/ardents-network/internal/domain/release" || dependency == "github.com/dianabuilds/ardents-network/internal/domain/release/infrastructure" || dependency == "github.com/dianabuilds/ardents-network/tests/epochfixture/network" || dependency == "github.com/theupdateframework/go-tuf/v2/metadata" || dependency == "github.com/dianabuilds/ardents-network/tests/e2e/node" || dependency == "github.com/dianabuilds/ardents-network/tests/e2e/source"
		case "tests/e2e/node/network_commands_unix_test.go":
			return dependency == modulePath+"/internal/admission" || dependency == modulePath+"/internal/admission/quota" ||
				dependency == modulePath+"/internal/admission/receiving" || dependency == modulePath+"/internal/admission/stock" ||
				dependency == modulePath+"/internal/admission/token" || dependency == modulePath+"/internal/domain/network" ||
				dependency == modulePath+"/internal/domain/network/state" || dependency == modulePath+"/tests/e2e/node" ||
				dependency == modulePath+"/tests/epochfixture/network"
		}
	}
	if localImport {
		switch source {
		case "tests/e2e/node/command_usage_test.go":
			return local == "internal/platform/systemd"
		case "tests/e2e/node/offline_profile_inspection_test.go":
			return local == "internal/admission/issuerprofile" || local == "internal/domain/network/closedprofile"
		case "tests/e2e/node/admission_authority_test.go", "tests/e2e/node/execution_route_refusal_test.go":
			return local == "internal/admission" || local == "internal/domain/network/state" || local == "tests/e2e/node"
		case "tests/e2e/node/admission_receiver_test.go":
			return local == "internal/admission" || local == "internal/admission/issuerprofile" ||
				local == "internal/admission/receiving" || local == "tests/e2e/node"
		case "tests/e2e/node/admission_interrupt_unix_test.go", "tests/e2e/node/admission_console_input_test.go":
			return local == "tests/e2e/node"
		case "tests/e2e/node/admission_cycle_test.go":
			return local == "internal/admission" || local == "internal/admission/issuance" ||
				local == "internal/admission/issuer" || local == "internal/admission/quota" ||
				local == "internal/admission/receiving" || local == "internal/admission/stock" ||
				local == "internal/admission/token" || local == "tests/e2e/node"
		case "tests/e2e/node/route_bootstrap_linux_test.go":
			return local == "internal/admission" || local == "internal/admission/quota" || local == "internal/admission/stock" || local == "internal/domain/endpoint/funding" || local == "internal/domain/endpoint/session" || local == "internal/domain/hosting" ||
				local == "internal/domain/network" || local == "internal/domain/network/epoch" || local == "internal/domain/network/state" || local == "tests/epochfixture/network"
		case "tests/e2e/node/route_issuer_recovery_test.go":
			return local == "internal/admission" || local == "internal/admission/quota" || local == "internal/admission/stock" ||
				local == "internal/admission/token" || local == "internal/domain/endpoint/funding" || local == "internal/domain/network/state" || local == "tests/epochfixture/network"
		case "tests/e2e/node/route_quic_linux_test.go":
			return local == "internal/admission" || local == "internal/admission/quota" || local == "internal/admission/stock" ||
				local == "internal/admission/token" || local == "internal/domain/endpoint/funding" || local == "internal/domain/hosting" ||
				local == "internal/domain/network/epoch" || local == "internal/domain/network/state" || local == "tests/epochfixture/network"
		case "tests/e2e/node/route_registration_linux_test.go":
			return local == "internal/admission" || local == "internal/admission/quota" || local == "internal/admission/stock" ||
				local == "internal/admission/token" || local == "internal/domain/endpoint/funding" || local == "internal/domain/endpoint/session" || local == "internal/domain/hosting" ||
				local == "internal/domain/network" || local == "internal/domain/network/epoch" || local == "internal/domain/network/state" || local == "tests/epochfixture/network"
		case "tests/e2e/node/route_join_linux_test.go":
			return local == "internal/admission" || local == "internal/admission/quota" || local == "internal/admission/stock" ||
				local == "internal/admission/token" || local == "internal/domain/endpoint/funding" || local == "internal/domain/endpoint/session" ||
				local == "internal/domain/hosting" || local == "internal/domain/network" || local == "internal/domain/network/epoch" || local == "internal/domain/network/state" || local == "tests/epochfixture/network"
		case "tests/e2e/node/route_descriptor_linux_test.go":
			return local == "internal/admission" || local == "internal/admission/quota" || local == "internal/admission/stock" ||
				local == "internal/admission/token" || local == "internal/domain/endpoint/funding" || local == "internal/domain/endpoint/session" || local == "internal/domain/hosting" ||
				local == "internal/domain/network/epoch" || local == "internal/domain/network/state" || local == "internal/domain/reachability" || local == "tests/epochfixture/network"
		case "tests/e2e/node/route_hello_linux_test.go":
			return local == "internal/admission" || local == "internal/admission/quota" || local == "internal/admission/stock" ||
				local == "internal/admission/token" || local == "internal/domain/endpoint/funding" || local == "internal/domain/hosting" ||
				local == "internal/domain/network/state" || local == "tests/epochfixture/network"
		case "tests/e2e/node/admission_hosting_linux_test.go", "tests/e2e/node/hosting_work_fixture_linux_test.go":
			return local == "internal/admission" || local == "internal/admission/receiving" || local == "internal/domain/hosting"
		case "tests/e2e/node/hosting_transfer_linux_test.go":
			return local == "internal/admission"
		case "tests/e2e/node/hosting_work_lifecycle_linux_test.go":
			return local == "internal/admission" || local == "internal/admission/issuerprofile" ||
				local == "internal/admission/receiving" || local == "internal/admission/token" || local == "internal/domain/hosting"
		case "tests/e2e/node/source_installed_native_linux_test.go":
			return local == "internal/domain/network/duty" || local == "internal/domain/network/epoch" ||
				local == "internal/domain/network/source" || local == "internal/domain/network/source/servingdeclaration" ||
				local == "internal/domain/network/state" || local == "internal/platform/systemd" ||
				local == "internal/domain/release/infrastructure" || local == "internal/domain/release/infrastructure/launch" ||
				local == "tests/e2e/node" || local == "tests/e2e/source" || local == "tests/epochfixture/network"
		case "tests/e2e/node/source_preparation_native_linux_test.go":
			return local == "internal/domain/endpoint" || local == "tests/e2e/source"
		case "tests/e2e/node/certificate_native_linux_test.go":
			return local == "tests/e2e/source"
		case "tests/e2e/node/processfixture/authority_inputs_linux.go":
			return local == "internal/admission/issuer" || local == "internal/domain/network/state" || local == "tests/epochfixture/network"
		case "tests/e2e/node/processfixture/issuer_roots_linux.go":
			return local == "internal/admission/issuance" || local == "internal/admission/issuer" || local == "internal/admission/quota"
		case "tests/e2e/node/processfixture/authority_observation_linux.go":
			return local == "internal/admission/spending" || local == "internal/domain/hosting" ||
				local == "internal/domain/network" || local == "internal/domain/network/duty" || local == "internal/domain/network/epoch"
		case "tests/e2e/node/processfixture/receiving_fixture_linux.go":
			return local == "tests/epochfixture/network"
		case "tests/e2e/node/processfixture/introduction_stock_linux.go":
			switch local {
			case "internal/admission", "internal/admission/issuer", "internal/admission/quota", "internal/admission/stock",
				"internal/admission/token", "internal/domain/endpoint/funding", "internal/domain/network", "tests/epochfixture/network":
				return true
			default:
				return false
			}
		case "tests/e2e/node/source_native_linux_test.go":
			return local == "internal/domain/network/epoch" || local == "internal/domain/network/source" || local == "internal/domain/network/source/servingdeclaration" || local == "internal/domain/network/state" || local == "internal/processresource" || local == "tests/epochfixture/network" || local == "tests/e2e/node" || local == "tests/e2e/source"
		case "tests/e2e/node/source_preparation_process_linux_test.go":
			return local == "internal/domain/endpoint" || local == "internal/domain/network/source" || local == "internal/domain/network/source/servingdeclaration" || local == "internal/domain/network/state" || local == "internal/platform/artifacts" || local == "tests/epochfixture/network" || local == "internal/domain/network/source/serving" || local == "tests/e2e/node" || local == "tests/e2e/source"
		}
	}
	if localImport && source == "tests/e2e/node/processfixture/descriptor_input_linux.go" {
		return local == "internal/admission"
	}
	if localImport && source == "tests/e2e/node/join_exchange_native_linux_test.go" {
		switch local {
		case "internal/admission", "internal/admission/issuer", "internal/admission/quota",
			"internal/admission/stock", "internal/admission/token", "internal/domain/endpoint/funding", "internal/domain/endpoint/session",
			"internal/domain/hosting", "internal/domain/network", "internal/domain/network/epoch", "internal/domain/network/state",
			"internal/processresource", "tests/epochfixture/network":
			return true
		default:
			return false
		}
	}
	if localImport && source == "tests/e2e/node/descriptor_exchange_native_linux_test.go" {
		switch local {
		case "internal/admission", "internal/admission/issuer", "internal/admission/quota", "internal/admission/stock",
			"internal/admission/token", "internal/domain/endpoint/funding", "internal/domain/hosting", "internal/domain/network/duty",
			"internal/domain/network/epoch", "internal/domain/network/state", "internal/processresource", "internal/domain/reachability", "tests/epochfixture/network":
			return true
		default:
			return false
		}
	}
	if localImport && source == "tests/e2e/node/forwarding_exchange_native_linux_test.go" {
		switch local {
		case "internal/admission", "internal/admission/issuer", "internal/admission/quota",
			"internal/admission/stock", "internal/admission/token", "internal/domain/endpoint/funding", "internal/domain/hosting",
			"internal/domain/network", "internal/domain/network/duty", "internal/domain/network/epoch", "internal/domain/network/state",
			"internal/processresource", "tests/epochfixture/network":
			return true
		default:
			return false
		}
	}
	if localImport && source == "tests/e2e/node/processfixture/installed_receiving_reopen_linux.go" {
		return local == "internal/admission" || local == "internal/admission/receiving" || local == "internal/domain/hosting"
	}
	if localImport && source == "tests/e2e/node/processfixture/receiving_spend_linux.go" {
		return local == "internal/admission" || local == "internal/admission/receiving"
	}
	if localImport && source == "tests/e2e/node/processfixture/route_topology_linux.go" {
		return local == "tests/epochfixture/network"
	}
	if localImport && source == "tests/e2e/node/processfixture/permitted_stock_unix.go" {
		return local == "internal/admission" || local == "internal/admission/stock"
	}
	if localImport && source == "tests/e2e/node/processfixture/admission_permission.go" {
		return local == "internal/admission" || local == "internal/admission/allocation"
	}
	if localImport && source == "tests/e2e/node/processfixture/admission_console.go" {
		return local == "internal/admission"
	}
	if localImport && source == "tests/e2e/node/provisioning_linux.go" {
		return local == "internal/platform/artifacts" || local == "tests/epochfixture/network"
	}
	if localImport && source == "tests/e2e/node/receiving_startup_native_linux_test.go" {
		return local == "internal/domain/hosting" || local == "internal/domain/network/duty" || local == "internal/domain/network/epoch" ||
			local == "internal/processresource" || local == "tests/epochfixture/network"
	}
	if localImport && (source == "tests/e2e/node/forwarding_startup_native_linux_test.go" || source == "tests/e2e/node/completion_native_linux_test.go") {
		return local == "internal/domain/hosting" || local == "internal/domain/network/duty" || local == "internal/domain/network/epoch" ||
			local == "internal/domain/network/state" || local == "internal/processresource" || local == "tests/epochfixture/network"
	}
	if localImport && source == "tests/e2e/node/issuer_exchange_native_linux_test.go" {
		return local == "internal/admission" || local == "internal/domain/endpoint/funding" || local == "internal/domain/hosting" ||
			local == "internal/domain/network/epoch" || local == "internal/domain/network/state" || local == "internal/processresource" ||
			local == "tests/epochfixture/network"
	}
	if localImport && source == "tests/e2e/node/issuer_startup_native_linux_test.go" {
		return local == "internal/admission/issuance" || local == "internal/admission/issuer" || local == "internal/admission/quota" || local == "internal/domain/hosting" ||
			local == "internal/domain/network/duty" || local == "internal/domain/network/epoch" || local == "internal/processresource" || local == "tests/epochfixture/network"
	}
	if localImport && source == "tests/e2e/node/issuer_inputs_linux.go" {
		return local == "internal/admission/issuance" || local == "internal/admission/issuer" || local == "internal/admission/quota"
	}
	if localImport && source == "tests/e2e/node/retained_lifecycle_native_linux_test.go" {
		return local == "internal/domain/hosting" || local == "internal/domain/network/duty" ||
			local == "internal/processresource" || local == "internal/domain/reachability" || local == "tests/epochfixture/network"
	}
	if localImport && (source == "tests/e2e/node/processfixture/hosting_fixture_linux.go" || source == "tests/e2e/node/processfixture/hosting_return_linux.go") {
		return local == "internal/domain/hosting"
	}
	if localImport {
		switch source {
		case "tests/e2e/endpoint/endpoint_platform_native_linux_test.go":
			return local == "internal/domain/enrollment" || local == "internal/domain/release" ||
				local == "internal/domain/release/endpoint" || local == "internal/domain/release/infrastructure/launch" ||
				local == "internal/domain/endpoint/plan" || local == "internal/domain/endpoint/unit" ||
				local == "tests/e2e/node" || local == "tests/e2e/source" || local == "tests/e2e/endpoint"
		case "tests/e2e/endpoint/native_delivery_linux.go":
			return local == "internal/domain/endpoint/plan" || local == "internal/domain/release/infrastructure/launch"
		case "tests/e2e/endpoint/journey_participant_native_linux_test.go":
			return local == "tests/e2e/endpoint" || local == "internal/application/administration" || local == "internal/application/connection" || local == "internal/application/document"
		case "tests/e2e/endpoint/journey_participant_linux.go":
			return local == "internal/domain/network/state" || local == "tests/releasefixture"
		case "tests/e2e/endpoint/connection_reopen_linux.go":
			return local == "internal/domain/network" || local == "internal/domain/publication/durable" || local == "internal/domain/publication/instance" || local == "internal/domain/reachability"
		case "tests/e2e/endpoint/connection_worker_linux.go":
			return local == "internal/domain/execution/runtime"
		case "tests/e2e/endpoint/connection_authority_linux.go":
			return local == "internal/application/connection" || local == "internal/domain/connection"
		case "tests/e2e/endpoint/document_commands_linux.go":
			return local == "internal/application/administration" || local == "internal/domain/publication"
		case "tests/e2e/endpoint/service_refresh_linux.go":
			return local == "internal/domain/publication" || local == "internal/domain/reachability"
		case "tests/e2e/endpoint/recovery_custody_instance_linux.go", "tests/e2e/endpoint/service_instance_linux.go":
			return local == "internal/domain/publication"
		case "tests/e2e/endpoint/launch_replacement_native_linux_test.go":
			return local == "tests/e2e/endpoint" || local == "tests/networkfixture"
		case "tests/e2e/endpoint/endpoint_idle_native_linux_test.go", "tests/e2e/endpoint/endpoint_enrollment_migration_native_linux_test.go":
			return local == "tests/e2e/endpoint"
		case "tests/e2e/endpoint/launch_refusals_native_linux_test.go":
			return local == "tests/e2e/endpoint" || local == "tests/networkfixture"
		case "tests/e2e/endpoint/endpoint_pair_native_linux_test.go", "tests/e2e/endpoint/release_process_test.go":
			return local == "tests/e2e/endpoint"
		case "tests/e2e/endpoint/launch_roots_linux.go":
			return local == "internal/domain/network/source"
		case "tests/e2e/endpoint/launch_sources_linux.go":
			return local == "internal/domain/network/state" || local == "tests/e2e/source"
		case "tests/e2e/endpoint/launch_stand_linux.go":
			return local == "internal/domain/network/state" || local == "tests/epochfixture/network"
		case "tests/e2e/endpoint/native_startup_linux.go":
			return local == "tests/epochfixture/network"
		}
	}
	if localImport {
		switch source {
		case "tests/e2e/endpoint/idle_observation_linux.go", "tests/e2e/endpoint/idle_observation_linux_test.go",
			"tests/e2e/endpoint/idle_projection.go", "tests/e2e/endpoint/idle_hosting_linux_test.go":
			return local == "internal/domain/hosting"
		}
	}
	if localImport && source == "tests/e2e/endpoint/permission_approval_linux_test.go" {
		return local == "internal/admission"
	}
	if localImport && source == "tests/e2e/endpoint/permission_approval_linux.go" {
		return local == "internal/admission" || local == "internal/admission/allocation"
	}
	if localImport && source == "tests/e2e/endpoint/installation_network_test.go" {
		return local == "internal/domain/endpoint/plan"
	}
	if localImport && source == "tests/e2e/endpoint/snapshot_worker_linux.go" {
		return local == "internal/application/administration"
	}
	if localImport && local == "tests/e2e/endpoint" {
		switch source {
		case "internal/domain/endpoint/execution_installed_linux_test.go", "internal/domain/endpoint/publication_ack_installed_linux_test.go":
			return true
		case "internal/domain/endpoint/connection_installed_linux_test.go", "internal/domain/endpoint/connection_accepted_request_installed_linux_test.go", "internal/domain/endpoint/document_command_installed_linux_test.go",
			"internal/domain/endpoint/installed_service_linux_test.go", "internal/domain/endpoint/endpoint_latency_native_linux_test.go", "internal/domain/endpoint/endpoint_replacement_worker_native_linux_test.go",
			"internal/domain/endpoint/publication_installed_linux_test.go",
			"internal/domain/endpoint/endpoint_launch_fixture_linux_test.go", "internal/domain/endpoint/endpoint_journey_native_linux_test.go":
			return true
		default:
			return false
		}
	}
	// These command tests consume only the registered process and signed fixture owners.
	if localImport && source == "internal/domain/node/introduction_exchange_native_linux_test.go" {
		switch local {
		case "internal/domain/endpoint/funding", "internal/domain/endpoint/session", "internal/domain/hosting",
			"internal/domain/network/epoch", "internal/domain/network/state", "internal/processresource", "internal/route/capsule",
			"internal/route/introduction", "tests/epochfixture/network":
			return true
		}
	}
	if localImport && source == "internal/domain/node/operation_workload_native_linux_test.go" {
		switch local {
		case "internal/admission", "internal/admission/issuer", "internal/admission/quota",
			"internal/admission/stock", "internal/admission/token", "internal/domain/endpoint/funding",
			"internal/domain/endpoint/session", "internal/domain/hosting", "internal/domain/network/epoch",
			"internal/domain/network/state", "internal/processresource", "internal/domain/reachability",
			"internal/route/introduction", "tests/epochfixture/network":
			return true
		}
	}
	if localImport && source == "internal/domain/node/operation_process_native_linux_test.go" {
		switch local {
		case "internal/admission", "internal/admission/issuance", "internal/admission/issuer",
			"internal/admission/quota", "internal/admission/spending", "internal/domain/endpoint/funding",
			"internal/domain/hosting", "internal/domain/network", "internal/domain/network/state",
			"internal/domain/node", "internal/route/role":
			return true
		}
	}
	if localImport && source == "internal/domain/node/authority_process_native_linux_test.go" {
		switch local {
		case "internal/admission/issuance", "internal/admission/issuer", "internal/admission/spending", "internal/domain/hosting",
			"internal/domain/network", "internal/domain/network/duty", "internal/domain/network/state", "internal/domain/node", "internal/domain/reachability",
			"internal/route/introduction", "internal/route/role", "internal/route/transport", "tests/epochfixture/network":
			return true
		}
	}
	if localImport {
		switch source {
		case "internal/domain/node/route_refusal_linux_test.go":
			switch local {
			case "internal/admission", "internal/admission/receiving", "internal/domain/endpoint/funding", "internal/domain/endpoint/session",
				"internal/domain/hosting", "internal/route/receiver", "internal/route/role", "internal/route/selection":
				return true
			}
		case "internal/domain/node/route_receiving_fixture_linux_test.go":
			switch local {
			case "internal/admission", "internal/admission/issuer", "internal/admission/quota", "internal/admission/stock", "internal/admission/token",
				"internal/domain/endpoint/funding", "internal/domain/network/state", "internal/route/transport", "tests/epochfixture/network":
				return true
			}
		case "internal/domain/node/route_join_fixture_linux_test.go":
			switch local {
			case "internal/route/receiver", "tests/e2e/node/processfixture", "internal/admission", "internal/admission/allocation",
				"internal/admission/issuer", "internal/admission/quota", "internal/admission/receiving", "internal/admission/stock",
				"internal/admission/token", "internal/domain/hosting", "internal/domain/endpoint/funding", "internal/domain/network",
				"tests/epochfixture/network", "tests/networkfixture", "internal/route/role", "internal/route/transport":
				return true
			}
		case "internal/domain/node/route_join_lifecycle_linux_test.go":
			switch local {
			case "internal/route/join", "internal/route/prefix", "internal/route/transport", "internal/admission",
				"internal/admission/allocation", "internal/admission/issuer", "internal/admission/quota", "internal/admission/stock",
				"internal/admission/token", "internal/domain/hosting", "internal/route/ardp", "internal/route/selection":
				return true
			}
		case "internal/domain/node/route_join_linux_test.go":
			switch local {
			case "internal/admission", "internal/admission/stock", "internal/domain/endpoint/funding", "internal/domain/hosting",
				"internal/route/prefix", "internal/route/selection", "internal/route/transport", "tests/e2e/node/processfixture":
				return true
			}
		case "internal/domain/node/route_token_linux_test.go":
			switch local {
			case "internal/admission", "internal/admission/stock", "internal/admission/token", "internal/domain/endpoint/funding",
				"internal/domain/hosting", "internal/route/ardp", "internal/route/prefix", "internal/route/selection",
				"internal/route/transport", "tests/e2e/node/processfixture":
				return true
			}
		case "internal/domain/node/route_bootstrap_linux_test.go":
			switch local {
			case "internal/domain/endpoint/funding", "internal/domain/endpoint/session", "internal/admission", "internal/admission/issuer",
				"internal/admission/quota", "internal/admission/receiving", "internal/admission/stock", "internal/domain/hosting",
				"internal/domain/network", "tests/epochfixture/network", "tests/networkfixture", "internal/route/receiver",
				"internal/route/role", "internal/route/transport", "tests/e2e/node/processfixture":
				return true
			}
		case "internal/domain/node/route_bootstrap_capacity_linux_test.go":
			switch local {
			case "internal/admission", "internal/admission/receiving", "internal/admission/stock", "internal/domain/hosting",
				"internal/route/ardp", "internal/route/channel", "internal/route/receiver", "internal/route/role",
				"internal/route/transport", "internal/route/transport/tls":
				return true
			}
		case "internal/domain/node/route_descriptor_linux_test.go":
			switch local {
			case "internal/admission", "internal/admission/issuer", "internal/admission/quota", "internal/admission/receiving",
				"internal/admission/stock", "internal/admission/token", "internal/domain/endpoint/funding", "internal/domain/endpoint/session", "internal/domain/hosting",
				"internal/domain/network", "internal/domain/reachability", "internal/route/channel", "internal/route/receiver", "internal/route/role",
				"internal/route/transport", "tests/epochfixture/network":
				return true
			}
		case "internal/domain/node/route_submission_linux_test.go":
			switch local {
			case "internal/admission", "internal/admission/receiving", "internal/domain/endpoint/funding", "internal/domain/endpoint/session",
				"internal/domain/hosting", "internal/route/capsule", "internal/route/introduction", "internal/route/prefix",
				"internal/route/receiver", "internal/route/role", "internal/route/selection":
				return true
			}
		case "internal/domain/node/route_registration_linux_test.go":
			switch local {
			case "internal/admission", "internal/admission/receiving", "internal/domain/endpoint/funding", "internal/domain/endpoint/session",
				"internal/domain/hosting", "internal/route/ardp", "internal/route/channel", "internal/route/introduction", "internal/route/receiver", "internal/route/role":
				return true
			}
		case "internal/domain/node/publication_registration_linux_test.go":
			switch local {
			case "internal/domain/publication", "internal/domain/publication/durable", "internal/domain/publication/instance", "internal/domain/reachability",
				"internal/route/capsule", "internal/route/introduction":
				return true
			}
		case "internal/domain/node/route_refill_linux_test.go":
			switch local {
			case "internal/admission", "internal/admission/receiving", "internal/admission/stock", "internal/domain/hosting", "internal/domain/network",
				"internal/route/ardp", "internal/route/channel", "internal/route/receiver", "internal/route/role", "internal/route/transport",
				"internal/route/transport/tls":
				return true
			}
		}
	}
	if localImport && source == "internal/domain/node/refill_process_native_linux_test.go" {
		switch local {
		case "internal/admission", "internal/admission/issuer", "internal/admission/quota", "internal/admission/receiving",
			"internal/admission/stock", "internal/admission/token", "internal/domain/endpoint/funding", "internal/domain/hosting",
			"internal/domain/network/state", "internal/processresource", "internal/route/ardp", "internal/route/transport",
			"internal/route/transport/tls", "tests/e2e/node/processfixture", "tests/epochfixture/network":
			return true
		default:
			return false
		}
	}
	if source == "tests/e2e/node/issuance_results_unix_test.go" && dependency == "github.com/cloudflare/circl/blindsign/blindrsa" {
		return true
	}
	if source == "tests/e2e/node/issuance_telemetry_unix_test.go" || source == "tests/e2e/node/admission_unix_test.go" {
		switch dependency {
		case "go.opentelemetry.io/proto/otlp/collector/trace/v1", "go.opentelemetry.io/proto/otlp/collector/metrics/v1", "google.golang.org/protobuf/proto":
			return true
		}
	}
	if localImport {
		switch source {
		case "tests/e2e/node/offline_admission_fixture_unix.go", "tests/e2e/node/issuance_unix_test.go", "tests/e2e/node/issuance_commands_unix_test.go":
			return local == "internal/admission" || local == "internal/admission/quota" || local == "internal/admission/issuerprofile" || local == "internal/admission/issuance"
		case "tests/e2e/node/admission_fresh_roots_unix_test.go":
			return local == "tests/e2e/node" || local == "internal/admission" || local == "internal/admission/quota" || local == "internal/admission/issuance"
		case "tests/e2e/node/admission_unix_test.go":
			return local == "tests/e2e/node" || local == "internal/admission/quota"
		case "tests/e2e/node/issuance_results_unix_test.go":
			return local == "tests/e2e/node" || local == "internal/admission/quota" || local == "internal/admission/issuerprofile" || local == "internal/admission/issuance"
		case "tests/e2e/node/issuer_profile_unix_test.go":
			return local == "internal/admission/issuerprofile/signing"
		}
	}
	if localImport && local == "tests/e2e/node" {
		switch source {
		case "internal/domain/endpoint/route_fixture_linux_test.go":
			return true
		case "internal/domain/endpoint/connection_installed_linux_test.go",
			"internal/domain/endpoint/publication_ack_installed_linux_test.go",
			"internal/domain/endpoint/route_issuer_fixture_linux_test.go":
			return true
		case "internal/domain/endpoint/endpoint_journey_native_linux_test.go",
			"internal/domain/endpoint/execution_installed_linux_test.go",
			"internal/domain/endpoint/network_admission_fixture_linux_test.go",
			"internal/domain/endpoint/publication_installed_linux_test.go":
			return true
		default:
			return false
		}
	}
	if localImport {
		switch source {
		case "tests/e2e/node/network_inspect_test.go":
			return local == "internal/domain/network/state"
		case "tests/e2e/node/source_refresh_linux_test.go":
			return local == "internal/domain/endpoint" || local == "internal/domain/network/state" || local == "tests/epochfixture/network" || local == "tests/e2e/source"
		case "tests/e2e/source/signing_workspace_linux.go":
			return local == "internal/platform/artifacts"
		}
	}
	// These test drivers use independent TLS or root-controlled fixture inputs.
	if localImport && source == "tests/e2e/source/initial_keys_linux.go" {
		return local == "internal/platform/artifacts"
	}
	if localImport && local == "tests/e2e/source" {
		switch source {
		case "internal/domain/endpoint/source_trust_native_linux_test.go", "tests/e2e/endpoint/endpoint_construction_linux_test.go":
			return true
		default:
			return false
		}
	}
	// Only the command composition consumes the Endpoint root from cmd.
	if strings.HasPrefix(source, "cmd/ardents-next/") && localImport && local == "internal/domain/endpoint" {
		switch source {
		case "cmd/ardents-next/command.go":
			return true
		default:
			return false
		}
	}
	switch source {
	case "tests/diagnostics/heapdump/heapdump_role_map_capture_test.go":
		if dependency == modulePath+"/internal/admission" {
			return true
		}
		fallthrough
	case "tests/diagnostics/heapdump/heapdump_parser_test.go", "tests/diagnostics/heapdump/heapdump_capture_test.go":
		first, _, _ := strings.Cut(dependency, "/")
		return dependency != "C" && dependency != "unsafe" && dependency != "" && !strings.Contains(first, ".") && path.Clean(dependency) == dependency && !strings.HasPrefix(dependency, "/")
	}
	if source == "tests/e2e/endpoint/recovery_custody_terminal_linux.go" && (dependency == "github.com/aymanbagabas/go-pty" || dependency == "golang.org/x/sys/unix") {
		return true
	}
	if source == "scripts/sign-qualification-release.go" {
		switch dependency {
		case modulePath + "/internal/domain/enrollment", modulePath + "/internal/domain/release",
			"github.com/sigstore/sigstore/pkg/signature", "github.com/theupdateframework/go-tuf/v2/metadata", "golang.org/x/sys/unix":
			return true
		}
		first, _, _ := strings.Cut(dependency, "/")
		return dependency != "C" && dependency != "unsafe" && dependency != "" && !strings.Contains(first, ".") && path.Clean(dependency) == dependency && !strings.HasPrefix(dependency, "/")
	}
	if source == "cmd/ardents-next/enrollment.go" && localImport {
		return local == "internal/domain/enrollment"
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
	if source == "internal/route/prefix/prefix.go" && dependency == modulePath+"/internal/domain/network/state" {
		return true
	}
	switch source {
	case "cmd/ardents-next/platform_endpoint_staging_linux.go":
		if local == "internal/domain/endpoint/launch" || local == "internal/domain/release" {
			return true
		}
	case "cmd/ardents-next/platform_endpoint_linux.go":
		if local == "internal/domain/endpoint/launch" || local == "internal/domain/endpoint/plan" || local == "internal/platform/artifacts" {
			return true
		}
	case "cmd/ardents-next/admission_console.go", "cmd/ardents-next/hosting_input.go",
		"internal/domain/endpoint/launch/platform_delivery_linux.go", "internal/domain/endpoint/launch/platform_staging_linux.go":
		if local == "internal/platform/artifacts" {
			return true
		}
	case "cmd/ardents-next/platform_linux.go":
		if local == "internal/domain/release/infrastructure/launch" || local == "internal/platform/artifacts" {
			return true
		}
	case "cmd/ardents-next/node.go", "cmd/ardents-next/source_command.go":
		if local == "internal/domain/release/infrastructure/launch" {
			return true
		}
	case "cmd/ardents-next/release.go":
		if local == "internal/domain/release/endpoint" {
			return true
		}
	case "cmd/ardents-next/platform.go":
		if local == "internal/platform/systemd" {
			return true
		}
	case "internal/domain/endpoint/plan/platform_launch.go", "internal/domain/endpoint/unit/platform_rendering.go":
		if local == "internal/pathgrammar" {
			return true
		}
	}
	if source == "tests/e2e/endpoint/replacement_scopes_linux.go" && (local == "internal/platform/cgroup" || local == "internal/platform/systemd") {
		return true
	}
	if source == "tests/e2e/endpoint/launch_replacement_linux.go" && dependency == modulePath+"/internal/platform/systemd" {
		return true
	}
	if source == "internal/domain/endpoint/endpoint_journey_native_linux_test.go" && (dependency == modulePath+"/internal/route/transport" || dependency == modulePath+"/internal/domain/publication") {
		return true
	}
	if source == "internal/domain/endpoint/endpoint_replacement_worker_native_linux_test.go" && dependency == modulePath+"/internal/route/transport" {
		return true
	}
	// The cold/warm latency root driver (#650) names the selected Carrier for the
	// shared publication Route network exactly as the journey controller does.
	if source == "internal/domain/endpoint/endpoint_latency_native_linux_test.go" && dependency == modulePath+"/internal/route/transport" {
		return true
	}
	if source == "internal/domain/network/state/source_repair_linux.go" && (dependency == modulePath+"/internal/platform/systemd" || dependency == modulePath+"/internal/platform/cgroup" || dependency == modulePath+"/internal/platform/process") {
		return true
	}
	if (source == "internal/domain/network/state/introduction_repair_linux.go" || source == "internal/domain/network/state/forwarding_repair_linux.go" || source == "internal/domain/network/state/join_repair_linux.go" || source == "internal/domain/network/state/descriptor_repair_linux.go") && (dependency == modulePath+"/internal/platform/systemd" || dependency == modulePath+"/internal/platform/cgroup") {
		return true
	}
	// The one shared node-duty repair skeleton consumes the detached manager
	// property vocabulary only; each role wrapper supplies its own per-unit
	// cgroup quiescence closure (W4, #711).
	if source == "internal/domain/network/state/node_duty_repair_linux.go" && dependency == modulePath+"/internal/platform/systemd" {
		return true
	}
	// The bounded Descriptor repair additionally clears an uncertain Store stage
	// through the existing-only reachability mechanism under the same dead-unit
	// proofs and retained leases; it grants no publication or runtime authority.
	if source == "internal/domain/network/state/descriptor_repair_linux.go" && dependency == modulePath+"/internal/domain/reachability" {
		return true
	}
	if (source == "internal/domain/network/state/source_repair_linux_test.go" || source == "internal/domain/network/state/node_duty_repair_linux_test.go") && dependency == modulePath+"/internal/platform/systemd" {
		return true
	}

	// Network's physical/application adapters consume domain policy through
	// exact files. Directory nesting does not grant policy or Admission access.
	if strings.HasPrefix(source, "internal/domain/network/") {
		sourceAdapter := source
		if sourceAdapter == "internal/domain/network/closedprofile/token_spki.go" && dependency == modulePath+"/internal/admission/issuerprofile" {
			return true
		}
		if sourceAdapter == "internal/domain/network/duty/participation_policy.go" && dependency == modulePath+"/internal/domain/network" {
			return true
		}
		if dependency == modulePath+"/internal/domain/network" {
			switch sourceAdapter {
			case "internal/domain/network/state/accepted_observation.go":
				return inZone
			case "internal/domain/network/state/closed_profile_role_join_test.go", "internal/domain/network/state/closed_runtime_view_test.go":
				return inZone
			case "internal/domain/network/state/acquisition_history.go":
				return true
			case "internal/domain/network/epoch/candidate_evaluation.go", "internal/domain/network/epoch/assignment.go":
				return true
			case "internal/domain/network/state/offline_operation.go", "internal/domain/network/state/epoch_history.go", "internal/domain/network/state/membership.go", "internal/domain/network/state/closed_member.go", "internal/domain/network/state/closed_runtime_view.go", "internal/domain/network/state/runtime_observation.go", "internal/domain/network/state/runtime_witness.go", "internal/domain/network/state/clock_observation.go", "internal/domain/network/state/closed_profile_view.go", "internal/domain/network/state/closed_duty_binding.go", "internal/domain/network/state/closed_profile_accept.go":
				return true
			}
		}
	}
	if source == "internal/domain/custody/canonical_envelope.go" && dependency == "golang.org/x/crypto/argon2" {
		return true
	}
	if (source == "internal/domain/custody/vault_lock_windows.go" || source == "internal/domain/custody/vault_lock_windows_test.go" || source == "internal/domain/custody/durability_windows.go") && dependency == "golang.org/x/sys/windows" {
		return true
	}
	// The Application document snapshot import composes only the shared
	// Unix-family inspection mechanism, no Network or Route authority (#850).
	if (source == "internal/application/document/snapshot_file_unix.go" || source == "internal/application/document/snapshot_file_unix_test.go") && dependency == modulePath+"/internal/durablefile" {
		return true
	}
	// Exact owner edges for the command responsibility transfer.
	if localImport {
		switch source {
		case "tests/e2e/endpoint/endpoint_construction_linux_test.go":
			return local == "internal/domain/network/state" || local == "tests/e2e/source" || local == "tests/epochfixture/network"
		case "tests/e2e/endpoint/endpoint_history_linux_test.go":
			return local == "internal/domain/network/source" || local == "tests/e2e/node/processfixture" || local == "tests/epochfixture/network"
		case "cmd/ardents-next/document_command.go":
			if local == "internal/application/administration" {
				return true
			}
		case "cmd/ardents-next/command.go":
			if local == "internal/domain/node" || local == "internal/domain/endpoint" || local == "internal/platform/launch" {
				return true
			}
		case "cmd/ardents-next/node.go":
			if local == "internal/domain/node" {
				return true
			}
		case "cmd/ardents-next/infrastructure_operator.go":
			if local == "internal/domain/release/infrastructure" {
				return true
			}
		case "internal/admission/issuer/current_authority.go", "internal/admission/receiving/current_authority.go":
			if local == "internal/domain/network" {
				return true
			}
		case "internal/domain/release/infrastructure/command_linux.go":
			if local == "internal/platform/artifacts" {
				return true
			}
		case "internal/platform/launch/template.go":
			if local == "internal/platform/systemd" {
				return true
			}
		case "internal/platform/launch/native_linux.go":
			if local == "internal/domain/endpoint/launch" || local == "internal/platform/artifacts" || local == "internal/platform/systemd" || local == "internal/domain/release" || local == "internal/domain/release/infrastructure/launch" {
				return true
			}
		case "internal/domain/endpoint/launch/platform_pin_linux_test.go":
			if local == "internal/platform/artifacts" {
				return true
			}
		case "internal/domain/node/route_admission_linux.go":
			if local == "internal/admission/issuer" {
				return true
			}
		case "internal/domain/node/role_command.go":
			if local == "internal/admission/issuer" || local == "internal/admission/issuance" || local == "internal/domain/network/state" {
				return true
			}
		case "internal/domain/node/role_input.go":
			if local == "internal/platform/artifacts" {
				return true
			}
		case "internal/domain/node/route_receive_linux.go":
			if local == "internal/admission/issuer" || local == "internal/admission/quota" || local == "internal/domain/network/state" || local == "internal/platform/artifacts" || local == "internal/domain/reachability" || local == "internal/route/channel" || local == "internal/route/introduction" {
				return true
			}
		case "internal/domain/network/state/offline_declaration.go":
			if local == "internal/platform/artifacts" {
				return true
			}
		case "internal/domain/endpoint/session/local_holder.go":
			if local == "internal/domain/network" || local == "internal/domain/network/state" {
				return true
			}
		case "internal/domain/endpoint/session/local_configuration.go", "internal/domain/endpoint/session/local_conversation.go", "internal/domain/endpoint/session/execution_holder.go", "internal/domain/endpoint/session/execution_refusal_other_test.go":
			if local == "internal/domain/network/state" {
				return true
			}
		case "internal/domain/endpoint/funding/current_issuer.go":
			if local == "internal/admission/issuer" || local == "internal/admission/quota" {
				return true
			}
		case "internal/domain/endpoint/funding/local_receiver.go", "internal/domain/endpoint/funding/receiver_conversation.go":
			if local == "internal/admission/receiving" {
				return true
			}
		case "internal/domain/endpoint/funding/admission_authority.go":
			if local == "internal/admission/issuer" || local == "internal/admission/receiving" || local == "internal/platform/artifacts" {
				return true
			}
		case "internal/admission/issuer/profile_declaration.go":
			if local == "internal/canonicaldigest" || local == "internal/platform/artifacts" {
				return true
			}
		case "internal/domain/hosting/command_declaration.go", "internal/admission/issuer/result_operation.go":
			if local == "internal/platform/artifacts" {
				return true
			}
		}
	}
	if source == "internal/domain/endpoint/preparation_command_linux.go" && dependency == modulePath+"/internal/domain/network/source" {
		return false
	}
	if !inZone {
		if (source == "tests/e2e/service/service_custody_process_linux_test.go" || source == "tests/e2e/service/headless_instance_acquisition_process_test.go") && dependency == modulePath+"/internal/domain/publication" {
			return true
		}
		if dependency == modulePath+"/internal/domain/custody" {
			switch source {
			case "internal/domain/endpoint/permission_test.go", "internal/domain/endpoint/issuance_test.go", "internal/domain/endpoint/permission_files_linux_test.go", "internal/domain/endpoint/network_fixture_window_test.go":
				return true
			}
		}
		if (source == "tests/e2e/service/admission_custody_process_test.go" || source == "internal/domain/endpoint/permission_request_fixture_test.go" || source == "internal/admission/stock/permission_request_fixture_test.go" || source == "internal/admission/stock/permission_view_test.go") && dependency == modulePath+"/internal/admission" {
			return true
		}
		return !zoneDependency
	}
	if (source == "internal/route/introduction/receiving_history_unix.go" || source == "internal/route/introduction/receiving_history_other.go") && dependency == modulePath+"/internal/admission/receiving" {
		return true
	}
	if (source == "internal/admission/issuer/current_authority.go" || source == "internal/admission/receiving/current_authority.go") && dependency == modulePath+"/internal/domain/network" {
		return true
	}
	if source == "internal/admission/issuer/profile_declaration.go" && dependency == modulePath+"/internal/canonicaldigest" {
		return true
	}
	if (source == "internal/admission/issuance/result_declaration.go" || source == "internal/domain/hosting/command_declaration.go" || source == "internal/admission/issuer/profile_declaration.go") && dependency == modulePath+"/internal/platform/artifacts" {
		return true
	}
	if source == "internal/domain/release/infrastructure/launch/verify_linux_test.go" && dependency == modulePath+"/internal/processresource" {
		return true
	}
	if dependency == modulePath+"/internal/domain/node/provisioning" {
		switch source {
		case "cmd/ardents-next/network.go":
			return true
		}
	}
	// Match exact Go packages. A parent folder grants no child/sibling imports.
	owner := path.Dir(source)
	permitted := map[string][]string{
		"internal/processresource":                          {},
		"internal/canonicaldigest":                          {},
		"internal/durablefile":                              {},
		"internal/pathgrammar":                              {},
		"internal/domain/node":                              {"admission", "admission/receiving", "durablefile", "domain/hosting", "domain/network", "domain/network/duty", "pathgrammar", "processresource", "route/ardp", "route/receiver", "route/role"},
		"internal/domain/network":                           {},
		"internal/domain/network/canonicalcursor":           {},
		"internal/domain/network/closedprofile":             {"domain/network/canonicalcursor"},
		"internal/domain/network/duty":                      {"durablefile", "pathgrammar"},
		"internal/domain/network/epoch":                     {"domain/network/canonicalcursor"},
		"internal/domain/network/source":                    {"domain/network/canonicalcursor", "domain/network/epoch"},
		"internal/domain/network/source/serving":            {"domain/network/source/servingdeclaration", "domain/network/state", "pathgrammar", "platform/artifacts", "processresource"},
		"internal/domain/network/source/servingdeclaration": {},
		"internal/domain/network/state":                     {"domain/network/canonicalcursor", "canonicaldigest", "domain/network/closedprofile", "domain/network/duty", "domain/network/epoch", "domain/network/source", "domain/network/source/servingdeclaration", "domain/network/state/durable", "pathgrammar"},
		"internal/domain/network/state/durable":             {"durablefile", "pathgrammar"},
		"internal/admission":                                {"admission/issuerprofile"},
		"internal/admission/issuerprofile":                  {},
		"internal/admission/allocation":                     {"admission"},
		"internal/domain/custody":                           {"admission", "admission/allocation", "domain/publication"},
		"internal/admission/attempts":                       {"pathgrammar"},
		"internal/admission/spending":                       {"pathgrammar"},
		"internal/admission/token":                          {"admission", "admission/issuerprofile"},
		"internal/admission/receiving":                      {"admission", "admission/token", "admission/spending", "platform/artifacts"},
		"internal/admission/stock":                          {"admission", "admission/allocation", "admission/attempts", "admission/token"},
		"internal/admission/quota":                          {"admission", "admission/issuerprofile", "pathgrammar"},
		"internal/admission/issuance":                       {"canonicaldigest", "admission", "admission/quota", "admission/issuerprofile", "admission/issuerprofile/signing", "pathgrammar"},
		"internal/admission/operator":                       {"admission", "admission/issuance", "admission/issuer", "admission/quota", "platform/artifacts"},
		"internal/admission/issuer":                         {"admission", "admission/quota", "admission/issuance", "admission/issuerprofile/signing", "pathgrammar"},
		"internal/admission/issuerprofile/signing":          {"admission/issuerprofile", "pathgrammar"},
		"internal/domain/hosting":                           {"canonicaldigest", "pathgrammar"},
		"internal/domain/publication":                       {},
		"internal/domain/publication/durable":               {"domain/publication"},
		"internal/domain/publication/instance":              {"canonicaldigest", "domain/publication", "domain/publication/durable", "domain/reachability", "route/introduction", "route/capsule", "route/join", "domain/connection/wire", "domain/execution/runtime"},
		"internal/domain/publication/runtime":               {"domain/connection", "domain/execution/runtime", "domain/network", "domain/publication", "domain/publication/instance", "domain/reachability", "route", "route/introduction", "route/prefix", "route/capsule", "route/join"},
		"internal/domain/connection":                        {"domain/execution/runtime", "domain/publication", "domain/publication/instance", "route/capsule", "route/join", "domain/connection/wire"},
		"internal/domain/connection/wire":                   {},
		"internal/domain/reachability":                      {"domain/publication", "pathgrammar"},
		"internal/domain/enrollment":                        {"canonicaldigest", "pathgrammar"},
		"internal/domain/release":                           {"canonicaldigest", "durablefile", "pathgrammar"},
		"internal/domain/execution":                         {},
		"internal/domain/execution/runtime":                 {"domain/execution", "domain/execution/worker"},
		"internal/domain/execution/worker":                  {"platform/systemd", "domain/endpoint/unit", "canonicaldigest", "pathgrammar"},
		"internal/domain/release/endpoint":                  {"domain/enrollment", "domain/release"},
		"internal/domain/release/infrastructure":            {"canonicaldigest", "domain/enrollment", "domain/release", "pathgrammar"},
		"internal/domain/release/infrastructure/launch":     {"admission/issuance", "canonicaldigest", "domain/node", "domain/enrollment", "domain/network/source/serving", "domain/network/source/servingdeclaration", "domain/network/state", "pathgrammar", "platform/artifacts", "platform/process", "platform/systemd", "domain/release", "domain/release/infrastructure"},
		"internal/domain/endpoint/launch":                   {"domain/release/endpoint", "canonicaldigest", "durablefile", "domain/enrollment", "domain/release", "platform/cgroup", "platform/systemd", "platform/process", "domain/endpoint/generation", "domain/endpoint/unit", "domain/endpoint/plan"},
		"internal/platform/cgroup":                          {"pathgrammar"},
		"internal/platform/artifacts":                       {"durablefile", "pathgrammar"},
		"internal/domain/endpoint/plan":                     {"canonicaldigest", "durablefile"},

		"internal/platform/systemd":           {},
		"internal/platform/process":           {"canonicaldigest", "pathgrammar"},
		"internal/domain/endpoint/generation": {"canonicaldigest", "durablefile", "pathgrammar"},
		"internal/domain/endpoint/unit":       {"canonicaldigest", "platform/systemd"},
		"internal/route":                      {},
		"internal/route/ardp":                 {},
		"internal/route/issuer":               {"route/ardp", "admission"},
		"internal/route/transport":            {},
		"internal/route/channel":              {"route/ardp", "route/transport"},
		"internal/route/bootstrap":            {"route/ardp", "route/channel"},
		"internal/route/transport/tls":        {"route/transport"},

		"internal/route/introduction":        {"admission/spending", "durablefile", "route", "route/ardp", "route/role", "route/prefix", "route/capsule", "domain/network", "admission", "pathgrammar"},
		"internal/route/capsule":             {},
		"internal/route/selection":           {"route", "domain/network", "durablefile"},
		"internal/route/role":                {"route", "route/ardp", "route/channel", "route/transport", "domain/network", "admission"},
		"internal/route/join":                {"route/prefix", "route", "route/selection", "route/ardp", "route/channel", "route/role", "route/transport", "domain/network", "admission"},
		"internal/route/join/pairing":        {"route/ardp", "route/channel", "route/role"},
		"internal/route/prefix":              {"route/issuer", "route", "route/selection", "route/ardp", "route/bootstrap", "route/channel", "route/role", "route/transport", "route/transport/tls", "domain/network", "admission"},
		"internal/route/receiver":            {"route/issuer", "route/join/pairing", "route", "route/role", "route/transport", "route/transport/tls", "route/channel", "route/bootstrap", "route/ardp", "route/introduction", "domain/network", "admission", "admission/receiving"},
		"internal/domain/endpoint/session":   {"admission", "admission/stock", "domain/endpoint/funding", "domain/endpoint/publisher", "domain/execution", "domain/execution/runtime", "domain/hosting", "domain/reachability", "route/ardp", "route/bootstrap", "route/capsule", "route/channel", "route/introduction", "route/join", "route/prefix", "route/role", "route/selection", "pathgrammar", "platform/artifacts"},
		"internal/domain/endpoint":           {"domain/endpoint/funding", "domain/endpoint/reader", "domain/endpoint/publisher", "admission", "admission/stock", "canonicaldigest", "domain/execution", "domain/execution/runtime", "domain/hosting", "domain/endpoint/launch", "domain/network", "domain/network/source", "domain/network/state", "pathgrammar"},
		"internal/domain/endpoint/publisher": {"domain/endpoint/funding", "admission", "admission/stock", "domain/execution/runtime", "domain/hosting", "domain/network", "domain/publication/durable", "domain/publication/instance", "domain/publication/runtime", "route/prefix", "route/selection", "pathgrammar"},
		"internal/domain/endpoint/reader":    {"domain/endpoint/funding", "admission", "admission/stock", "domain/connection", "domain/execution/runtime", "domain/hosting", "domain/network", "domain/reachability", "route/prefix", "route/selection"},
		"internal/domain/endpoint/funding":   {"admission", "admission/stock", "admission/token", "domain/hosting", "domain/network", "domain/network/state", "route", "route/ardp", "route/bootstrap", "route/prefix", "route/role", "route/selection", "pathgrammar"},
		"cmd/ardents-next":                   {"canonicaldigest", "pathgrammar", "domain/endpoint/session", "domain/endpoint", "domain/endpoint/publisher", "domain/endpoint/reader", "domain/endpoint/funding", "route/introduction", "route/prefix", "domain/network", "domain/network/state", "admission/stock", "admission/receiving", "admission/allocation", "admission", "admission/quota", "admission/issuerprofile", "admission/issuance", "admission/issuer", "admission/issuerprofile/signing", "domain/hosting", "route", "route/role", "route/selection", "route/join", "route/receiver", "route/channel", "route/ardp"},
	}
	if zoneDependency {
		if (source == "internal/domain/endpoint/history_preparation_linux.go" || source == "internal/domain/endpoint/history_preparation_linux_test.go") && dependency == modulePath+"/internal/route/selection" {
			return true
		}
		if (source == "internal/domain/endpoint/history_preparation_linux.go" || source == "internal/domain/endpoint/history_preparation_linux_test.go") && dependency == modulePath+"/internal/domain/network/duty" {
			return true
		}
		if (source == "internal/domain/endpoint/preparation_files_linux.go" || source == "internal/domain/endpoint/source_trust_native_linux_test.go") && dependency == modulePath+"/internal/platform/artifacts" {
			return true
		}
		if source == "internal/domain/endpoint/history_preparation_linux_test.go" && dependency == modulePath+"/internal/admission/attempts" {
			return true
		}

		if source == "internal/domain/network/source/command/serve_linux.go" && dependency == modulePath+"/internal/processresource" {
			return true
		}
		if dependency == modulePath+"/internal/platform/artifacts" {
			switch source {
			case "cmd/ardents-next/inputs.go", "cmd/ardents-next/publication.go", "cmd/ardents-next/route_linux.go",
				"internal/domain/network/source/command/serve_linux.go", "internal/domain/network/source/command/prepare_linux.go",
				"internal/domain/network/state/source_state_files_linux_test.go":
				return true
			}
		}

		if source == "cmd/ardents-next/node_roles.go" && (dependency == modulePath+"/internal/domain/node" || dependency == modulePath+"/internal/processresource") {
			return true
		}
		if (source == "internal/domain/node/runtime_unix.go" || source == "internal/domain/node/runtime_other.go") && dependency == modulePath+"/internal/admission/issuer" {
			return true
		}
		if source == "internal/domain/node/issuer_unix.go" && (dependency == modulePath+"/internal/admission/issuer" || dependency == modulePath+"/internal/admission/quota" || dependency == modulePath+"/internal/route/bootstrap") {
			return true
		}
		if source == "internal/domain/node/introduction_unix.go" && dependency == modulePath+"/internal/route/introduction" {
			return true
		}
		if source == "internal/domain/node/descriptor_unix.go" && dependency == modulePath+"/internal/domain/reachability" {
			return true
		}
		if source == "cmd/ardents-next/route_linux.go" && dependency == modulePath+"/internal/domain/node" {
			return true
		}
		if source == "internal/domain/endpoint/document_command_installed_linux_test.go" {
			switch dependency {
			case modulePath + "/internal/domain/execution",
				modulePath + "/internal/domain/execution/runtime",
				modulePath + "/internal/domain/publication",
				modulePath + "/internal/domain/reachability",
				modulePath + "/internal/route/transport":
				return true
			}
		}
		if source == "internal/domain/endpoint/connection_accepted_request_installed_linux_test.go" && (dependency == modulePath+"/internal/domain/connection" || dependency == modulePath+"/internal/domain/execution/runtime" || dependency == modulePath+"/internal/domain/execution/worker" || dependency == modulePath+"/internal/route/transport") {
			return true
		}
		if source == "internal/domain/endpoint/connection_response_loss_linux_test.go" && (dependency == modulePath+"/internal/domain/connection" || dependency == modulePath+"/internal/domain/execution/runtime" || dependency == modulePath+"/internal/domain/publication/runtime" || dependency == modulePath+"/internal/route/transport") {
			return true
		}
		if source == "internal/domain/publication/runtime/publisher_streams.go" && dependency == modulePath+"/internal/domain/connection/wire" {
			return true
		}
		if source == "internal/domain/endpoint/connection_installed_linux_test.go" && (dependency == modulePath+"/internal/domain/connection" || dependency == modulePath+"/internal/domain/execution" || dependency == modulePath+"/internal/domain/execution/runtime" || dependency == modulePath+"/internal/domain/publication/runtime" || dependency == modulePath+"/internal/domain/reachability" || dependency == modulePath+"/internal/route/transport") {
			return true
		}

		if source == "internal/domain/endpoint/connection_authority_linux_test.go" && (dependency == modulePath+"/internal/domain/connection" || dependency == modulePath+"/internal/domain/execution/runtime" || dependency == modulePath+"/internal/route/transport") {
			return true
		}
		if source == "internal/domain/endpoint/publication_ack_installed_linux_test.go" && (dependency == modulePath+"/internal/domain/execution" || dependency == modulePath+"/internal/domain/execution/runtime" || dependency == modulePath+"/internal/domain/publication" || dependency == modulePath+"/internal/domain/reachability" || dependency == modulePath+"/internal/route/transport") {
			return true
		}
		if source == "internal/domain/endpoint/installed_service_linux_test.go" && (dependency == modulePath+"/internal/domain/reachability" || dependency == modulePath+"/internal/route/transport") {
			return true
		}
		if source == "internal/domain/endpoint/publication_installed_linux_test.go" && (dependency == modulePath+"/internal/domain/publication" || dependency == modulePath+"/internal/domain/publication/instance" || dependency == modulePath+"/internal/domain/reachability" || dependency == modulePath+"/internal/route/transport" || dependency == modulePath+"/internal/route/capsule") {
			return true
		}

		if source == "cmd/ardents-next/route_linux.go" && dependency == modulePath+"/internal/route/capsule" {
			return true
		}
		if source == "cmd/ardents-next/publication.go" && (dependency == modulePath+"/internal/domain/publication/durable" || dependency == modulePath+"/internal/domain/publication/instance") {
			return true
		}
		if source == "internal/domain/endpoint/participant_linux.go" && (dependency == modulePath+"/internal/domain/endpoint/launch" || dependency == modulePath+"/internal/domain/execution" || dependency == modulePath+"/internal/domain/execution/runtime") {
			return true
		}
		if source == "cmd/ardents-next/node.go" && dependency == modulePath+"/internal/domain/node/provisioning" {
			return true
		}
		if source == "internal/domain/endpoint/declarations_linux.go" && (dependency == modulePath+"/internal/domain/execution" || dependency == modulePath+"/internal/domain/network/source") {
			return true
		}
		if (source == "cmd/ardents-next/execution_holder.go" || source == "cmd/ardents-next/admission_holder.go") && dependency == modulePath+"/internal/domain/execution/runtime" {
			return true
		}
		if source == "cmd/ardents-next/execution_holder.go" && dependency == modulePath+"/internal/domain/execution" {
			return true
		}

		if source == "cmd/ardents-next/infrastructure_linux.go" {
			return dependency == modulePath+"/internal/domain/release/infrastructure" || dependency == modulePath+"/internal/domain/release" || dependency == modulePath+"/internal/platform/artifacts"
		}
		if dependency == modulePath+"/internal/domain/enrollment" && source == "cmd/ardents-next/enrollment.go" {
			return true
		}

		if dependency == modulePath+"/internal/route/transport" {
			if source == "internal/domain/endpoint/execution_installed_linux_test.go" {
				return true
			}
			if source == "internal/domain/endpoint/route_issuer_fixture_linux_test.go" {
				return true
			}
			switch source {
			case "internal/domain/endpoint/route_fixture_linux_test.go":
				return true
			}
		}
		if source == "internal/domain/endpoint/network_admission_fixture_linux_test.go" && dependency == modulePath+"/internal/domain/network/epoch" {
			return true
		}
		if source == "cmd/ardents-next/route_issuer_linux.go" && dependency == modulePath+"/internal/route/bootstrap" {
			return true
		}
		if dependency == modulePath+"/internal/domain/reachability" && (source == "internal/route/prefix/descriptor_exchange.go" || source == "internal/route/receiver/receiver_unix.go" || source == "internal/route/receiver/descriptor_unix.go" || source == "cmd/ardents-next/route_linux.go" || source == "cmd/ardents-next/route.go" || source == "cmd/ardents-next/admission_holder.go") {
			return true
		}
		if (source == "internal/admission/stock/issuance_fixture_test.go" || source == "internal/admission/stock/observation_retirement_linux_test.go" || source == "internal/admission/stock/lifecycle_test.go" || source == "internal/admission/stock/receiving_cycle_test.go") && (dependency == modulePath+"/internal/admission/issuer" || dependency == modulePath+"/internal/admission/issuance" || dependency == modulePath+"/internal/admission/quota") {
			return true
		}
		if (source == "internal/admission/stock/cycle_test.go" || source == "internal/admission/stock/receiving_cycle_test.go") && (dependency == modulePath+"/internal/admission/receiving" || dependency == modulePath+"/internal/admission/spending") {
			return true
		}
		if source == "internal/domain/connection/publisher_recovery.go" && dependency == modulePath+"/internal/route/prefix" {
			return true
		}
		if source == "internal/domain/connection/initiator_source.go" {
			for _, owned := range []string{"domain/network", "domain/reachability", "route", "route/prefix", "route/selection", "route/introduction"} {
				if dependency == modulePath+"/internal/"+owned {
					return true
				}
			}
		}
		// The Service TLS owners borrow only the closed key-exchange group list
		// from the Route TLS contract (#903). Peer identity, certificate and
		// proof checks stay with each Service owner; no other file gains the edge.
		if dependency == modulePath+"/internal/route/transport" {
			switch source {
			case "internal/domain/connection/initiator_authentication.go", "internal/domain/connection/initiator_replacement.go",
				"internal/domain/publication/instance/service_attachment.go":
				return true
			}
		}
		if source == "internal/domain/release/infrastructure/source_plan_linux.go" && dependency == modulePath+"/internal/domain/network/source" {
			return true
		}
		if source == "internal/domain/release/infrastructure/source_startup_linux.go" && (dependency == modulePath+"/internal/platform/process" || dependency == modulePath+"/internal/platform/cgroup" || dependency == modulePath+"/internal/platform/systemd") {
			return true
		}
		if source == "internal/domain/release/infrastructure/source_start_linux.go" && (dependency == modulePath+"/internal/platform/process" || dependency == modulePath+"/internal/platform/cgroup" || dependency == modulePath+"/internal/platform/systemd") {
			return true
		}
		if source == "internal/domain/release/infrastructure/source_replace_linux.go" && (dependency == modulePath+"/internal/domain/network/state" || dependency == modulePath+"/internal/platform/cgroup" || dependency == modulePath+"/internal/platform/systemd") {
			return true
		}
		if (source == "internal/domain/release/infrastructure/source_recover_linux.go" || source == "internal/domain/release/infrastructure/source_initial_linux.go") && dependency == modulePath+"/internal/platform/systemd" {
			return true
		}
		if source == "internal/domain/release/infrastructure/source_recover_linux.go" && dependency == modulePath+"/internal/domain/network/state" {
			return true
		}
		if source == "internal/domain/release/infrastructure/source_recover_replacement_linux.go" && (dependency == modulePath+"/internal/domain/release" || dependency == modulePath+"/internal/domain/network/state" || dependency == modulePath+"/internal/platform/systemd") {
			return true
		}
		if source == "internal/domain/release/infrastructure/source_recovery_completed_linux.go" && (dependency == modulePath+"/internal/platform/cgroup" || dependency == modulePath+"/internal/platform/systemd") {
			return true
		}
		if source == "internal/domain/release/infrastructure/source_retirement_history_linux.go" && (dependency == modulePath+"/internal/platform/systemd") {
			return true
		}
		if source == "internal/domain/release/infrastructure/source_stop_linux.go" && (dependency == modulePath+"/internal/platform/process" || dependency == modulePath+"/internal/platform/cgroup" || dependency == modulePath+"/internal/platform/systemd") {
			return true
		}
		if (source == "internal/domain/release/infrastructure/source_observation_linux.go" || source == "internal/domain/release/infrastructure/source_install_linux.go" || source == "internal/domain/release/infrastructure/source_loaded_linux.go" || source == "internal/domain/release/infrastructure/source_policy_linux_test.go" || source == "internal/domain/release/infrastructure/source_running_linux_test.go") && dependency == modulePath+"/internal/platform/systemd" {
			return true
		}
		if dependency == modulePath+"/internal/domain/network/source/servingdeclaration" &&
			source == "internal/domain/network/state/source_state_files_linux_test.go" {
			return true
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
	if (source == "internal/domain/endpoint/reader/operation_unix.go" || source == "internal/domain/endpoint/reader/byte_stream_unix.go") && dependency == modulePath+"/internal/application/document" {
		return true
	}
	if (source == "internal/application/administration/byte_stream_linux.go" || source == "internal/application/administration/published_link_client_linux.go") && dependency == modulePath+"/internal/application/connection" {
		return true
	}
	if (source == "internal/application/document/local_publication.go" || source == "internal/application/document/local_link_linux.go") && dependency == modulePath+"/internal/application/administration" {
		return true
	}
	if (source == "internal/domain/endpoint/reader/reader_unix.go" || source == "internal/domain/endpoint/reader/operation_unix.go" || source == "internal/domain/endpoint/reader/byte_stream_unix.go" || source == "internal/domain/endpoint/participant_linux.go") && dependency == modulePath+"/internal/application/connection" {
		return true
	}
	if source == "cmd/ardents-next/diagnostic_timeline.go" && local == "internal/diagnostics/timeline" {
		return true
	}
	if (source == "internal/domain/endpoint/participant_linux.go" || source == "cmd/ardents-next/endpoint_administration_client.go" || source == "internal/domain/endpoint/installed_service_linux_test.go" || source == "internal/domain/endpoint/endpoint_replacement_worker_native_linux_test.go") && dependency == modulePath+"/internal/application/administration" {
		return true
	}
	if strings.HasPrefix(dependency, modulePath+"/internal/application/") {
		switch source {
		case "tests/e2e/endpoint/document_publisher_linux_test.go", "tests/e2e/endpoint/document_published_link_linux_test.go":
			return dependency == modulePath+"/internal/application/administration"
		case "tests/e2e/endpoint/document_reader_peer_linux_test.go", "tests/e2e/endpoint/document_reader_socket_linux_test.go", "tests/e2e/endpoint/stream_exchange_linux_test.go":
			return dependency == modulePath+"/internal/application/connection"
		case "tests/e2e/endpoint/document_reader_process_linux_test.go", "tests/e2e/endpoint/document_reader_linux_test.go":
			return dependency == modulePath+"/internal/application/connection" || dependency == modulePath+"/internal/application/document"
		case "cmd/ardents-next/stream_command.go":
			return dependency == modulePath+"/internal/application/connection"
		case "cmd/ardents-next/document_command.go":
			return dependency == modulePath+"/internal/application/connection" || dependency == modulePath+"/internal/application/document"
		}
	}
	if source == "internal/domain/endpoint/endpoint_journey_native_linux_test.go" || source == "internal/domain/endpoint/document_command_installed_linux_test.go" {
		switch dependency {
		case modulePath + "/internal/application/administration", modulePath + "/internal/application/connection", modulePath + "/internal/application/document":
			return true
		}
	}
	// The cold/warm latency participant (#650) runs as the installed Endpoint
	// MainPID and measures the ordinary Reader composition directly: it opens the
	// public Connection request and reads the validated text response exactly as
	// the installed reader adapter does. It owns no Application server.
	if source == "internal/domain/endpoint/latency_native_linux_test.go" {
		switch dependency {
		case modulePath + "/internal/application/connection", modulePath + "/internal/application/document":
			return true
		}
	}
	if source == "internal/domain/execution/runtime/initialization_linux.go" && dependency == modulePath+"/internal/application/document" {
		return true
	}
	if (source == "internal/domain/execution/runtime/publisher_worker_linux.go" || source == "internal/domain/execution/runtime/publisher_worker_linux_test.go") && dependency == modulePath+"/internal/application/document" {
		return true
	}
	if (source == "internal/domain/execution/runtime/byte_stream.go" || source == "internal/domain/execution/runtime/byte_stream_linux.go") && dependency == modulePath+"/internal/application/document" {
		return true
	}
	if (source == "internal/domain/execution/runtime/stream_contract.go" || source == "internal/domain/execution/runtime/stream_contract_test.go") && dependency == modulePath+"/internal/application/connection" {
		return true
	}
	if source == "internal/domain/execution/runtime/reader_worker_linux.go" && dependency == modulePath+"/internal/application/document" {
		return true
	}
	if (source == "internal/domain/execution/runtime/reader_worker_linux.go" || source == "internal/domain/execution/runtime/reader_stream.go") && dependency == modulePath+"/internal/application/connection" {
		return true
	}
	if (source == "internal/domain/execution/runtime/publisher_worker_linux.go" || source == "internal/domain/execution/runtime/publisher_stream.go" || source == "internal/domain/execution/runtime/hostile_snapshot_installed_linux_test.go") && dependency == modulePath+"/internal/application/connection" {
		return true
	}
	if source == "internal/domain/execution/worker/stream_activation_installed_linux_test.go" && dependency == modulePath+"/internal/application/streamqualification" {
		return true
	}
	if (source == "internal/domain/execution/runtime/stream_qualification_linux.go" || source == "internal/domain/execution/runtime/stream_qualification_other.go" || source == "internal/domain/execution/runtime/stream_qualification_installed_linux_test.go") && dependency == modulePath+"/internal/application/streamqualification" {
		return true
	}
	if (source == "internal/domain/connection/stream.go" || source == "internal/domain/connection/stream_test.go") && dependency == modulePath+"/internal/application/connection" {
		return true
	}
	if (source == "internal/domain/endpoint/connection_response_loss_linux_test.go" || source == "internal/domain/endpoint/connection_authority_linux_test.go") && dependency == modulePath+"/internal/application/connection" {
		return true
	}
	if source == "internal/domain/endpoint/publisher/snapshot_unix.go" && dependency == modulePath+"/internal/application/document" {
		return true
	}
	if (owner == "internal/admission/issuance" || owner == "internal/admission/token") && dependency == "github.com/cloudflare/circl/blindsign/blindrsa" {
		return true
	}
	if (source == "internal/platform/systemd/manager_reference_linux.go" || source == "internal/platform/systemd/node_slots_linux.go" || source == "internal/platform/systemd/forwarding_manager_linux.go" || source == "internal/platform/systemd/introduction_manager_linux.go" || source == "internal/platform/systemd/endpoint_reference_linux.go" || source == "internal/platform/systemd/startup_observation_linux.go" || source == "internal/platform/systemd/source_preflight_linux.go" || source == "internal/platform/systemd/source_stopped_linux.go" || source == "internal/platform/systemd/source_manager_linux.go" || source == "internal/platform/systemd/source_control_linux.go" || source == "internal/platform/systemd/source_reference_native_linux_test.go") && dependency == "github.com/godbus/dbus/v5" {
		return true
	}
	if (owner == "internal/admission/spending" || owner == "internal/domain/network/duty" || owner == "internal/domain/network/state/durable") && dependency == "golang.org/x/sys/windows" {
		return true
	}
	if (source == "internal/durablefile/owner_root_windows.go" || source == "internal/durablefile/owner_root_windows_test.go" || source == "internal/route/selection/root_permissions_windows_test.go") && dependency == "golang.org/x/sys/windows" {
		return true
	}
	// The Unix-family inspection port admits the reviewed x/sys/unix
	// directory-relative open and atomic rename only at its mechanism boundary
	// (#850; rename #853).
	if (source == "internal/durablefile/relative_open_unix.go" || source == "internal/durablefile/relative_rename_unix.go") && dependency == "golang.org/x/sys/unix" {
		return true
	}
	// The macOS hosting measure port admits the reviewed x/sys/unix IFMIB
	// interface-data sysctl read and kern.boottime boot identity only at its
	// mechanism boundary (#872). The routing-socket if_data64 byte counters wrap
	// at 2^32 and are deliberately not used; the IFMIB totals preserve per-boot
	// per-interface continuity.
	if source == "internal/domain/hosting/measure_darwin.go" && dependency == "golang.org/x/sys/unix" {
		return true
	}
	// The macOS peer-credential refusal evidence test admits the reviewed
	// x/sys/unix connect-time LOCAL_PEERCRED observation only at that boundary
	// (#852); the production macOS port refuses and imports no native mechanism.
	if source == "internal/domain/execution/worker/peercred_darwin_test.go" && dependency == "golang.org/x/sys/unix" {
		return true
	}
	if source == "internal/domain/reachability/store_platform_windows.go" && dependency == "golang.org/x/sys/windows" {
		return true
	}
	if source == "internal/domain/publication/durable/lease_windows.go" && dependency == "golang.org/x/sys/windows" {
		return true
	}
	if source == "internal/domain/release/history_native_windows.go" && dependency == "golang.org/x/sys/windows" {
		return true
	}
	if owner == "internal/domain/release" && dependency == "github.com/theupdateframework/go-tuf/v2/metadata" {
		switch source {
		case "internal/domain/release/metadata_authentication.go", "internal/domain/release/target_identity.go", "internal/domain/release/safety_policy.go", "internal/domain/release/floor_encoding.go":
			return true
		}
	}
	if source == "internal/domain/release/metadata_authentication.go" && dependency == "github.com/theupdateframework/go-tuf/v2/metadata/trustedmetadata" {
		return true
	}
	if strings.HasSuffix(source, "_test.go") {
		if source == "internal/domain/release/generation_authorization_test.go" && dependency == "github.com/theupdateframework/go-tuf/v2/metadata" {
			return true
		}
		if source == "internal/domain/release/rotation_test.go" && (dependency == "github.com/sigstore/sigstore/pkg/signature" || dependency == "github.com/theupdateframework/go-tuf/v2/metadata") {
			return true
		}
		if source == "internal/domain/release/infrastructure/forwarding_artifact_linux_test.go" && dependency == "github.com/theupdateframework/go-tuf/v2/metadata" {
			return true
		}
		if (owner == "internal/domain/network/epoch" || owner == "internal/domain/network/state" || owner == "cmd/ardents-next" || source == "internal/domain/endpoint/history_preparation_linux_test.go" || source == "internal/domain/endpoint/plan_construction_linux_test.go") && dependency == modulePath+"/tests/epochfixture/network" {
			return true
		}
		if owner == "internal/domain/network/epoch" && dependency == modulePath+"/tests/epochfixture/assignment" {
			return true
		}
	}

	if source == "internal/domain/release/infrastructure/source_transition_scope_linux.go" && dependency == "golang.org/x/sys/unix" {
		return true
	}
	if source == "internal/domain/endpoint/reader/result.go" && (dependency == modulePath+"/internal/application/connection" || dependency == modulePath+"/internal/application/document") {
		return true
	}
	if (source == "internal/domain/endpoint/publisher/publisher_unix.go" || source == "internal/domain/endpoint/publisher/opening_unix.go" || source == "internal/domain/endpoint/publisher/byte_stream_unix.go") && dependency == modulePath+"/internal/application/document" {
		return true
	}
	if source == "internal/domain/endpoint/publisher/byte_stream_unix.go" && dependency == modulePath+"/internal/application/connection" {
		return true
	}
	if source == "internal/domain/endpoint/publisher/contract.go" && dependency == modulePath+"/internal/application/document" {
		return true
	}
	if dependency == modulePath+"/internal/diagnostics/telemetry" {
		switch source {
		case "cmd/ardents-next/hosting.go", "internal/admission/operator/admission.go", "internal/admission/operator/issuance.go",
			"internal/admission/operator/input_report.go":
			return true
		}
	}
	if source == "internal/diagnostics/telemetry/observation.go" {
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
		{"public issuer inspection excludes Network authority", "cmd/ardents-next/offline_profile_inspection.go", modulePath + "/internal/domain/network/state", false},
		{"public issuer inspection excludes Route", "cmd/ardents-next/offline_profile_inspection.go", modulePath + "/internal/route/role", false},
		{"adjacent Control has no inherited codec access", "internal/domain/node/provisioning/unregistered.go", modulePath + "/internal/admission/issuerprofile", false},
		{"Endpoint release producer uses new Enrollment", "scripts/sign-qualification-release.go", modulePath + "/internal/domain/enrollment", true},
		{"Endpoint release producer uses new Release", "scripts/sign-qualification-release.go", modulePath + "/internal/domain/release", true},
		{"Endpoint release producer excludes retired Enrollment path", "scripts/sign-qualification-release.go", modulePath + "/internal/successor/enrollment", false},
		{"Endpoint release producer excludes retired Release path", "scripts/sign-qualification-release.go", modulePath + "/internal/successor/release", false},
		{"Endpoint release producer has no Network authority", "scripts/sign-qualification-release.go", modulePath + "/internal/domain/network", false},
		{"Endpoint release producer has no Route runtime", "scripts/sign-qualification-release.go", modulePath + "/internal/route/role", false},
		{"Endpoint release producer retains TUF mechanism", "scripts/sign-qualification-release.go", "github.com/theupdateframework/go-tuf/v2/metadata", true},
		{"Endpoint release producer retains signing mechanism", "scripts/sign-qualification-release.go", "github.com/sigstore/sigstore/pkg/signature", true},
		{"retained heap map validates the public Permission framing", "tests/diagnostics/heapdump/heapdump_role_map_capture_test.go", modulePath + "/internal/admission", true},
		{"retained heap map cannot open a spending root", "tests/diagnostics/heapdump/heapdump_role_map_capture_test.go", modulePath + "/internal/admission/spending", false},
		{"retained heap map does not import predecessor Endpoint", "tests/diagnostics/heapdump/heapdump_role_map_capture_test.go", modulePath + "/internal/domain/endpoint", false},
		{"Endpoint release producer retains native files", "scripts/sign-qualification-release.go", "golang.org/x/sys/unix", true},
		{"Endpoint release producer excludes foreign library", "scripts/sign-qualification-release.go", "example.org/foreign", false},
		{"packaging uses new Enrollment through the product command", "cmd/ardents-next/enrollment.go", modulePath + "/internal/domain/enrollment", true},
		{"packaging commands exclude retired Enrollment", "cmd/ardents-next/enrollment.go", modulePath + "/internal/successor/enrollment", false},
		{"standalone packaging helpers cannot consume Enrollment", "scripts/packaging-check.go", modulePath + "/internal/domain/enrollment", false},
		{"packaging commands exclude Release authority", "cmd/ardents-next/enrollment.go", modulePath + "/internal/domain/release", false},
		{"adjacent script has no inherited Enrollment access", "scripts/another-tool.go", modulePath + "/internal/domain/enrollment", false},
		{"Fixed Node Release signing uses exact artifact resources", "internal/domain/release/infrastructure/signing/node_release_linux.go", modulePath + "/internal/domain/release/infrastructure", true},
		{"Fixed Node Release predecessor verifies genuine Release", "internal/domain/release/infrastructure/signing/node_release_predecessor_linux.go", modulePath + "/internal/domain/release", true},
		{"JOIN key birth grants no Release observation", "internal/domain/release/infrastructure/signing/join_release_keys_linux.go", modulePath + "/internal/domain/release", false},
		{"Fixed Node Release signing grants no Route consumer", "internal/domain/release/infrastructure/signing/node_release_linux.go", modulePath + "/internal/route/role", false},
		{"Fixed Node Release plan grants no State owner", "internal/domain/release/infrastructure/signing/node_release_plan_linux.go", modulePath + "/internal/domain/network/state", false},
		{"delayed ACK fixture retains genuine installed operation", "internal/domain/endpoint/publication_ack_installed_linux_test.go", modulePath + "/internal/domain/execution/runtime", true},
		{"receiving Network fixture opens genuine State", "tests/networkfixture/independent_state.go", modulePath + "/internal/domain/network/state", true},
		{"receiving Network fixture has no Carrier dependency", "tests/networkfixture/independent_state.go", modulePath + "/internal/route/transport", false},
		{"receiving Network fixture has no concrete TLS adapter", "tests/networkfixture/independent_state.go", modulePath + "/internal/route/transport/tls", false},
		{"receiving Network fixture has no concrete QUIC adapter", "tests/networkfixture/independent_state.go", modulePath + "/internal/route/transport/quic", false},
		{"receiving Network fixture grants no Publisher runtime", "tests/networkfixture/independent_state.go", modulePath + "/internal/domain/publication/runtime", false},
		{"delayed ACK fixture invokes command composition", "internal/domain/endpoint/publication_ack_installed_linux_test.go", modulePath + "/internal/domain/publication/runtime", false},
		{"delayed ACK fixture cannot reserve durable authority", "internal/domain/endpoint/publication_ack_installed_linux_test.go", modulePath + "/internal/domain/publication/durable", false},
		{"installed Service scenario checks public credential codec", "tests/e2e/endpoint/service_instance_linux.go", modulePath + "/internal/domain/publication", true},
		{"installed Service scenario uses public Administration client", "internal/domain/endpoint/installed_service_linux_test.go", modulePath + "/internal/application/administration", true},
		{"installed Service scenario cannot use old Endpoint", "internal/domain/endpoint/installed_service_linux_test.go", modulePath + "/internal/domain/endpoint", false},
		{"installed Service scenario owns receiving Store fixture", "internal/domain/endpoint/installed_service_linux_test.go", modulePath + "/internal/domain/reachability", true},
		{"installed Service scenario selects exact Carrier", "internal/domain/endpoint/installed_service_linux_test.go", modulePath + "/internal/route/transport", true},
		{"installed Service scenario cannot supply private Instance", "internal/domain/endpoint/installed_service_linux_test.go", modulePath + "/internal/domain/publication/instance", false},
		{"installed Service scenario cannot bypass runtime command", "internal/domain/endpoint/installed_service_linux_test.go", modulePath + "/internal/domain/publication/runtime", false},
		{"native Publisher scenario independently checks public proof", "internal/domain/endpoint/publication_installed_linux_test.go", modulePath + "/internal/domain/publication", true},
		{"native Publisher scenario prepares genuine host Instance", "internal/domain/endpoint/publication_installed_linux_test.go", modulePath + "/internal/domain/publication/instance", true},
		{"native Publisher scenario reopens real receiving Store", "internal/domain/endpoint/publication_installed_linux_test.go", modulePath + "/internal/domain/reachability", true},
		{"native Publisher scenario names selected Carrier", "internal/domain/endpoint/publication_installed_linux_test.go", modulePath + "/internal/route/transport", true},
		{"native Publisher scenario cannot bypass command lifecycle", "internal/domain/endpoint/publication_installed_linux_test.go", modulePath + "/internal/domain/publication/runtime", false},
		{"native Publisher scenario has no direct durable reservation", "internal/domain/endpoint/publication_installed_linux_test.go", modulePath + "/internal/domain/publication/durable", false},
		{"native Publisher scenario excludes concrete TLS adapter", "internal/domain/endpoint/publication_installed_linux_test.go", modulePath + "/internal/route/transport/tls", false},
		{"native Publisher scenario excludes concrete QUIC adapter", "internal/domain/endpoint/publication_installed_linux_test.go", modulePath + "/internal/route/transport/quic", false},
		{"another native test has no Publisher exception", "cmd/ardents-next/unregistered_installed_linux_test.go", modulePath + "/internal/domain/publication/instance", false},
		{"installed Reader retains original worker", "internal/domain/endpoint/reader/reader_unix.go", modulePath + "/internal/domain/execution/runtime", true},
		{"installed Reader consumes authenticated Source", "internal/domain/endpoint/reader/reader_unix.go", modulePath + "/internal/domain/connection", true},
		{"installed Reader local byte contract", "internal/domain/endpoint/reader/reader_unix.go", modulePath + "/internal/application/connection", true},
		{"installed Reader local Text projection", "internal/domain/endpoint/reader/result.go", modulePath + "/internal/application/document", true},
		{"installed participant owns Reader listener", "internal/domain/endpoint/participant_linux.go", modulePath + "/internal/application/connection", true},
		{"first participant preparation initializes participation", "internal/domain/endpoint/history_preparation_linux.go", modulePath + "/internal/domain/network/duty", true},
		{"first participant preparation initializes selection history", "internal/domain/endpoint/history_preparation_linux.go", modulePath + "/internal/route/selection", true},
		{"participant runtime cannot initialize participation directly", "internal/domain/endpoint/participant_linux.go", modulePath + "/internal/domain/network/duty", false},
		{"first participant preparation uses Stock owner", "internal/domain/endpoint/history_preparation_linux.go", modulePath + "/internal/admission/stock", true},
		{"first participant preparation cannot bypass Stock", "internal/domain/endpoint/history_preparation_linux.go", modulePath + "/internal/admission/attempts", false},
		{"first participant preparation has no old backend", "internal/domain/endpoint/history_preparation_linux.go", modulePath + "/internal/domain/endpoint", false},
		{"plan constructor authenticates retained State", "internal/domain/endpoint/plan_construction_linux.go", modulePath + "/internal/domain/network/state", true},
		{"plan constructor observes authenticated facts", "internal/domain/endpoint/plan_construction_linux.go", modulePath + "/internal/domain/endpoint/funding", true},
		{"plan constructor cannot initialize participation", "internal/domain/endpoint/plan_construction_linux.go", modulePath + "/internal/domain/network/duty", false},
		{"plan constructor cannot initialize selection history", "internal/domain/endpoint/plan_construction_linux.go", modulePath + "/internal/route/selection", false},
		{"plan constructor has no old backend", "internal/domain/endpoint/plan_construction_linux.go", modulePath + "/internal/domain/endpoint", false},
		{"installed endpoint command uses endpoint owner", "cmd/ardents-next/command.go", modulePath + "/internal/domain/endpoint", true},
		{"plan constructor command has no direct Source codec", "internal/domain/endpoint/preparation_command_linux.go", modulePath + "/internal/domain/network/source", false},
		{"installed participant owns Administration listener", "internal/domain/endpoint/participant_linux.go", modulePath + "/internal/application/administration", true},
		{"installed Publisher retains original worker", "internal/domain/endpoint/publisher/publisher_unix.go", modulePath + "/internal/domain/execution/runtime", true},
		{"other installed adapter has no Publisher authority", "cmd/ardents-next/installed_other_linux.go", modulePath + "/internal/domain/execution/runtime", false},
		{"installed Publisher cannot use old Endpoint", "internal/domain/endpoint/publisher/publisher_unix.go", modulePath + "/internal/domain/endpoint", false},
		{"unregistered installed adapter has no local byte exception", "cmd/ardents-next/other_installed_linux.go", modulePath + "/internal/application/connection", false},
		{"installed Reader cannot use old Connection", "internal/domain/endpoint/reader/reader_unix.go", modulePath + "/internal/service/connection", false},
		{"qualified Publisher uses only native Text protocol", "internal/domain/execution/runtime/publisher_worker_linux.go", modulePath + "/internal/application/document", true},
		{"qualified Reader uses only native Text protocol", "internal/domain/execution/runtime/reader_worker_linux.go", modulePath + "/internal/application/document", true},
		{"qualified Reader input uses only byte-stream boundary", "internal/domain/execution/runtime/reader_stream.go", modulePath + "/internal/application/connection", true},
		{"qualified Publisher uses Application stream grammar", "internal/domain/execution/runtime/publisher_worker_linux.go", modulePath + "/internal/application/connection", true},
		{"selected Publisher input uses Text import", "internal/domain/endpoint/publisher/snapshot_unix.go", modulePath + "/internal/application/document", true},
		{"Execution launch borrows only Text INIT grammar", "internal/domain/execution/runtime/initialization_linux.go", modulePath + "/internal/application/document", true},
		{"Execution rules cannot borrow Text runtime", "internal/domain/execution/job.go", modulePath + "/internal/application/document", false},
		{"common Execution launch has no native Text exception", "internal/domain/execution/runtime/launch.go", modulePath + "/internal/application/document", false},
		{"other Execution composition has no Text exception", "internal/domain/execution/runtime/other_linux.go", modulePath + "/internal/application/document", false},
		{"installed Stream activation test uses fixed qualification protocol", "internal/domain/execution/worker/stream_activation_installed_linux_test.go", modulePath + "/internal/application/streamqualification", true},
		{"ordinary worker activation has no qualification protocol edge", "internal/domain/execution/worker/activate.go", modulePath + "/internal/application/streamqualification", false},
		{"ordinary Text INIT has no qualification protocol edge", "internal/domain/execution/runtime/initialization_linux.go", modulePath + "/internal/application/streamqualification", false},
		{"trusted Stream qualification seam uses fixed qualification protocol", "internal/domain/execution/runtime/stream_qualification_linux.go", modulePath + "/internal/application/streamqualification", true},
		{"common Execution launch has no qualification protocol edge", "internal/domain/execution/runtime/launch.go", modulePath + "/internal/application/streamqualification", false},
		{"ordinary Text protocol has no qualification protocol edge", "internal/domain/execution/runtime/protocol.go", modulePath + "/internal/application/streamqualification", false},
		{"Execution cannot borrow old Endpoint authority", "internal/domain/execution/runtime/initialization_linux.go", modulePath + "/internal/domain/endpoint", false},
		{"Execution cannot borrow old Broker", "internal/domain/execution/authority.go", modulePath + "/internal/application/broker", false},
		{"old Endpoint cannot consume new Execution", "internal/domain/endpoint/worker_grant_linux.go", modulePath + "/internal/domain/execution", false},
		{"native original manager reference owns bus client", "internal/platform/systemd/endpoint_reference_linux.go", "github.com/godbus/dbus/v5", true},
		{"fresh Startup manager observation owns bus client", "internal/platform/systemd/startup_observation_linux.go", "github.com/godbus/dbus/v5", true},
		{"read-only Node slot observer consumes physical D-Bus", "internal/platform/systemd/node_slots_linux.go", "github.com/godbus/dbus/v5", true},
		{"read-only Node slot observer cannot authorize Release", "internal/platform/systemd/node_slots_linux.go", modulePath + "/internal/domain/release", false},
		{"native Source reference regression checks genuine bus refusal", "internal/platform/systemd/source_reference_native_linux_test.go", "github.com/godbus/dbus/v5", true},
		{"manager value validation excludes bus client", "internal/platform/systemd/manager_values_linux.go", "github.com/godbus/dbus/v5", false},
		{"manager transport excludes bus client", "internal/platform/systemd/reference_transport_linux.go", "github.com/godbus/dbus/v5", false},
		{"Installation decisions exclude bus client", "internal/domain/endpoint/launch/predecessor_linux.go", "github.com/godbus/dbus/v5", false},
		{"portable authorization cannot own installed Endpoint", "internal/domain/release/endpoint/generation_authentication.go", modulePath + "/internal/domain/endpoint/launch", false},
		{"portable authorization cannot own physical root", "internal/domain/release/endpoint/generation_authentication.go", modulePath + "/internal/platform/cgroup", false},
		{"installed Endpoint consumes opaque authorization", "internal/domain/endpoint/launch/generation_authorization.go", modulePath + "/internal/domain/release/endpoint", true},
		{"unregistered manager file excludes bus client", "internal/platform/systemd/other_linux.go", "github.com/godbus/dbus/v5", false},
		{"Reachability verifies public delegation", "internal/domain/reachability/descriptor.go", modulePath + "/internal/domain/publication", true},
		{"Prefix completes genuine private lookup", "internal/route/prefix/descriptor_exchange.go", modulePath + "/internal/domain/reachability", true},
		{"Receiver uses genuine Descriptor Store", "internal/route/receiver/descriptor_unix.go", modulePath + "/internal/domain/reachability", true},
		{"Carrier cannot acquire Descriptor authority", "internal/route/transport/tls/node.go", modulePath + "/internal/domain/reachability", false},
		{"Reachability cannot own physical Prefix", "internal/domain/reachability/lookup_history.go", modulePath + "/internal/route/prefix", false},
		{"Shared owner-only root DACL enforcement", "internal/durablefile/owner_root_windows.go", "golang.org/x/sys/windows", true},
		{"Unix-family inspection port admits reviewed Openat", "internal/durablefile/relative_open_unix.go", "golang.org/x/sys/unix", true},
		{"macOS peer-credential refusal evidence uses reviewed LOCAL_PEERCRED", "internal/domain/execution/worker/peercred_darwin_test.go", "golang.org/x/sys/unix", true},
		{"Unix-family inspection port admits reviewed Renameat", "internal/durablefile/relative_rename_unix.go", "golang.org/x/sys/unix", true},
		{"Introduction slot-history port composes the shared durable mechanism", "internal/route/introduction/slot_history_unix.go", modulePath + "/internal/durablefile", true},
		{"document snapshot import composes the shared inspection mechanism", "internal/application/document/snapshot_file_unix.go", modulePath + "/internal/durablefile", true},
		{"document snapshot import has no Network authority", "internal/application/document/snapshot_file_unix.go", modulePath + "/internal/domain/network/state", false},
		{"Owner-only root mechanism test verifies the physical DACL", "internal/durablefile/owner_root_windows_test.go", "golang.org/x/sys/windows", true},
		{"Selection Windows floor consumes the shared mechanism", "internal/route/selection/root_permissions_windows.go", "golang.org/x/sys/windows", false},
		{"Selection Windows floor test verifies the physical DACL", "internal/route/selection/root_permissions_windows_test.go", "golang.org/x/sys/windows", true},
		{"Store native Windows lease and sync", "internal/domain/reachability/store_platform_windows.go", "golang.org/x/sys/windows", true},
		{"Descriptor rules cannot use native Windows mechanisms", "internal/domain/reachability/descriptor.go", "golang.org/x/sys/windows", false},
		{"Store rules cannot use native Windows mechanisms", "internal/domain/reachability/store.go", "golang.org/x/sys/windows", false},
		{"public proof cannot acquire Route transport", "internal/domain/publication/public_proof.go", modulePath + "/internal/route/ardp", false},
		{"public proof cannot acquire Store", "internal/domain/publication/public_proof.go", modulePath + "/internal/domain/reachability", false},
		{"Reachability cannot delegate to old verifier", "internal/domain/reachability/descriptor.go", modulePath + "/internal/service/reachability", false},
		{"public proof cannot delegate to old Publication", "internal/domain/publication/public_proof.go", modulePath + "/internal/service/publication", false},
		{"physical implementation uses shared contract", "internal/route/transport/tls/node.go", modulePath + "/internal/route/transport", true},
		{"Prefix uses shared contract", "internal/route/prefix/prefix.go", modulePath + "/internal/route/transport", true},
		{"Prefix uses portable framing", "internal/route/prefix/prefix.go", modulePath + "/internal/route/channel", true},
		{"channel uses grammar", "internal/route/channel/session.go", modulePath + "/internal/route/ardp", true},
		{"bootstrap consumes framing capacity", "internal/route/bootstrap/budget.go", modulePath + "/internal/route/channel", true},
		{"bootstrap describes canonical restriction", "internal/route/bootstrap/budget.go", modulePath + "/internal/route/ardp", true},
		{"receiver reserves bootstrap capacity", "internal/route/receiver/receiver_unix.go", modulePath + "/internal/route/bootstrap", true},
		{"channel cannot acquire bootstrap policy", "internal/route/channel/session.go", modulePath + "/internal/route/bootstrap", false},
		{"bootstrap cannot spend Admission", "internal/route/bootstrap/budget.go", modulePath + "/internal/admission/receiving", false},
		{"bootstrap cannot import receiving composition", "internal/route/bootstrap/budget.go", modulePath + "/internal/route/receiver", false},
		{"old receiver cannot import new bootstrap", "internal/domain/node/forwarding/link.go", modulePath + "/internal/route/bootstrap", false},
		{"channel uses ordered transport", "internal/route/channel/session.go", modulePath + "/internal/route/transport", true},
		{"channel cannot import TLS adapter", "internal/route/channel/session.go", modulePath + "/internal/route/transport/tls", false},
		{"channel cannot import QUIC adapter", "internal/route/channel/session_test.go", modulePath + "/internal/route/transport/quic", false},
		{"holder JOIN cannot import Receiver", "internal/route/join/acquisition.go", modulePath + "/internal/route/receiver", false},
		{"Receiver cannot import holder Prefix", "internal/route/receiver/receiver_unix.go", modulePath + "/internal/route/prefix", false},
		{"actual receiver dispatches JOIN pairing", "internal/route/receiver/receiver_unix.go", modulePath + "/internal/route/join/pairing", true},
		{"receiver does not import holder JOIN", "internal/route/receiver/receiver_unix.go", modulePath + "/internal/route/join", false},
		{"holder JOIN consumes Prefix", "internal/route/join/acquisition.go", modulePath + "/internal/route/prefix", true},
		{"former operation cannot return", "cmd/ardents-next/route_linux.go", modulePath + "/internal/route/operation", false},
		{"Prefix cannot import JOIN consumer", "internal/route/prefix/borrowing.go", modulePath + "/internal/route/join", false},
		{"Prefix cannot import Introduction consumer", "internal/route/prefix/introduction_channel.go", modulePath + "/internal/route/introduction", false},
		{"Reader initial factory retains actual Source", "internal/domain/connection/initiator_source.go", modulePath + "/internal/route/prefix", true},
		{"Publisher recovery retains original Source and Responder", "internal/domain/connection/publisher_recovery.go", modulePath + "/internal/route/prefix", true},
		{"Publisher dispatch uses canonical full logical tuple", "internal/domain/publication/runtime/publisher_streams.go", modulePath + "/internal/domain/connection/wire", true},
		{"Publication scheduler cannot construct wire authority", "internal/domain/publication/runtime/scheduler.go", modulePath + "/internal/domain/connection/wire", false},
		{"Connection record actors cannot choose Source", "internal/domain/connection/stream_records.go", modulePath + "/internal/route/prefix", false},
		{"Reader initial factory verifies independent Descriptor", "internal/domain/connection/initiator_source.go", modulePath + "/internal/domain/reachability", true},
		{"Connection wire cannot borrow Source", "internal/domain/connection/wire/continuity.go", modulePath + "/internal/route/prefix", false},
		{"Connection ledger cannot choose Source", "internal/domain/connection/receive_ledger.go", modulePath + "/internal/route/prefix", false},
		{"Prefix cannot import receiving composition", "internal/route/prefix/prefix.go", modulePath + "/internal/route/receiver", false},
		{"Prefix calls actual role owner", "internal/route/prefix/prefix.go", modulePath + "/internal/route/role", true},
		{"holder consumes actual Prefix", "internal/domain/endpoint/session/prefix_unix.go", modulePath + "/internal/route/prefix", true},
		{"channel cannot import receiving composition", "internal/route/channel/session.go", modulePath + "/internal/route/receiver", false},
		{"channel cannot import Network", "internal/route/channel/session_test.go", modulePath + "/internal/domain/network", false},
		{"channel cannot import role authority", "internal/route/channel/session.go", modulePath + "/internal/route/role", false},
		{"role cannot import receiving composition", "internal/route/role/authority.go", modulePath + "/internal/route/receiver", false},
		{"role cannot spend Admission", "internal/route/role/authority.go", modulePath + "/internal/admission/receiving", false},
		{"role consumes Network observations", "internal/route/role/authority.go", modulePath + "/internal/domain/network", true},
		{"Introduction exchange uses role binding", "internal/route/introduction/receiving.go", modulePath + "/internal/route/role", true},
		{"Source command parses sealed capsule", "internal/domain/endpoint/session/prefix_unix.go", modulePath + "/internal/route/capsule", true},
		{"submission fixture parses sealed capsule", "internal/domain/node/route_submission_linux_test.go", modulePath + "/internal/route/capsule", true},
		{"other commands cannot acquire capsule consumer", "cmd/ardents-next/admission_holder.go", modulePath + "/internal/route/capsule", false},
		{"submission consumes retained exclusions", "internal/route/introduction/submission.go", modulePath + "/internal/route", true},
		{"Introduction history birth consumes only its genuine receiving owner", "internal/route/introduction/receiving_history_unix.go", modulePath + "/internal/admission/receiving", true},
		{"Introduction cannot own receiving Grant", "internal/route/introduction/receiving.go", modulePath + "/internal/admission/receiving", false},
		{"Introduction cannot call listener composition", "internal/route/introduction/receiving.go", modulePath + "/internal/route/receiver", false},
		{"channel cannot import Admission", "internal/route/channel/session.go", modulePath + "/internal/admission", false},
		{"channel cannot import Hosting", "internal/route/channel/session.go", modulePath + "/internal/domain/hosting", false},
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
		{"bootstrap scenario uses shared contract", "internal/domain/endpoint/route_issuer_fixture_linux_test.go", modulePath + "/internal/route/transport", true},
		{"session consumes genuine stock", "internal/domain/endpoint/session/session.go", modulePath + "/internal/admission/stock", true},
		{"session consumes original Execution permission", "internal/domain/endpoint/session/session.go", modulePath + "/internal/domain/execution/runtime", true},
		{"session cannot call command implementation", "internal/domain/endpoint/session/session.go", modulePath + "/cmd/ardents-next", false},
		{"session cannot use old Endpoint", "internal/domain/endpoint/session/session.go", modulePath + "/internal/domain/endpoint", false},
		{"funding cannot depend on session", "internal/domain/endpoint/funding/source.go", modulePath + "/internal/domain/endpoint/session", false},
		{"Reader cannot own installed participant", "internal/domain/endpoint/reader/reader_unix.go", modulePath + "/internal/domain/endpoint", false},
		{"other binaries cannot acquire holder session", "cmd/ardents/main.go", modulePath + "/internal/domain/endpoint/session", false},
		{"funding cannot depend on Reader", "internal/domain/endpoint/funding/source.go", modulePath + "/internal/domain/endpoint/reader", false},
		{"Reader cannot own Publication", "internal/domain/endpoint/reader/reader_unix.go", modulePath + "/internal/domain/publication/runtime", false},
		{"Publisher cannot import command composition", "internal/domain/endpoint/publisher/publisher_unix.go", modulePath + "/cmd/ardents-next", false},
		{"other binaries cannot acquire new funding", "cmd/ardents/main.go", modulePath + "/internal/domain/endpoint/funding", false},
		{"enrollment verification uses Enrollment", "cmd/ardents-next/enrollment.go", modulePath + "/internal/domain/enrollment", true},
		{"diagnostics uses the local timeline projector", "cmd/ardents-next/diagnostic_timeline.go", modulePath + "/internal/diagnostics/timeline", true},
		{"diagnostics cannot consume Route", "cmd/ardents-next/diagnostic_timeline.go", modulePath + "/internal/route/transport", false},
		{"withdraw uses the Application Administration client", "cmd/ardents-next/endpoint_administration_client.go", modulePath + "/internal/application/administration", true},
		{"retired command has no Enrollment grant", "cmd/ardents/endpoint.go", modulePath + "/internal/domain/enrollment", false},
		{"funded Source uses real token grammar", "internal/domain/endpoint/funding/source.go", modulePath + "/internal/admission/token", true},
		{"funded Source uses finite bootstrap", "internal/domain/endpoint/funding/source.go", modulePath + "/internal/route/bootstrap", true},
		{"funded Source behavior selects real Carrier", "internal/domain/node/route_token_linux_test.go", modulePath + "/internal/route/transport", true},
		{"other funding adapter has no bootstrap exception", "cmd/ardents-next/route_funding_linux.go", modulePath + "/internal/route/bootstrap", false},
		{"installed Execution scenario uses shared Carrier identity", "internal/domain/endpoint/execution_installed_linux_test.go", modulePath + "/internal/route/transport", true},
		{"installed Execution scenario excludes direct TLS construction", "internal/domain/endpoint/execution_installed_linux_test.go", modulePath + "/internal/route/transport/tls", false},
		{"installed Execution scenario excludes direct QUIC construction", "internal/domain/endpoint/execution_installed_linux_test.go", modulePath + "/internal/route/transport/quic", false},
		{"installed Service scenario consumes real Connection", "internal/domain/endpoint/connection_installed_linux_test.go", modulePath + "/internal/domain/connection", true},
		{"hostile Publisher sibling tests the Application stream", "internal/domain/execution/runtime/hostile_snapshot_installed_linux_test.go", modulePath + "/internal/application/connection", true},
		{"hostile Publisher sibling cannot acquire legacy Service authority", "internal/domain/execution/runtime/hostile_snapshot_installed_linux_test.go", modulePath + "/internal/service/connection", false},
		{"recovery Custody process fixture has a terminal", "tests/e2e/endpoint/recovery_custody_terminal_linux.go", "github.com/aymanbagabas/go-pty", true},
		{"installed Reader cannot open a fixture terminal", "internal/domain/endpoint/reader/operation_unix.go", "github.com/aymanbagabas/go-pty", false},
		{"recovery issuance proof consumes the public verifier", "tests/e2e/endpoint/recovery_custody_instance_linux.go", modulePath + "/internal/domain/publication", true},
		{"recovery issuance proof cannot open private Instance authority", "tests/e2e/endpoint/recovery_custody_instance_linux.go", modulePath + "/internal/domain/publication/instance", false},
		{"accepted-request recovery observes genuine Connection", "internal/domain/endpoint/connection_accepted_request_installed_linux_test.go", modulePath + "/internal/domain/connection", true},
		{"accepted-request recovery cannot construct direct TLS", "internal/domain/endpoint/connection_accepted_request_installed_linux_test.go", modulePath + "/internal/route/transport/tls", false},
		{"response loss observes original Connection", "internal/domain/endpoint/connection_response_loss_linux_test.go", modulePath + "/internal/domain/connection", true},
		{"response loss retains original Execution", "internal/domain/endpoint/connection_response_loss_linux_test.go", modulePath + "/internal/domain/execution/runtime", true},
		{"withdrawal invokes genuine original Publisher", "internal/domain/endpoint/connection_response_loss_linux_test.go", modulePath + "/internal/domain/publication/runtime", true},
		{"withdrawal cannot acquire private Instance authority", "internal/domain/endpoint/connection_response_loss_linux_test.go", modulePath + "/internal/domain/publication/instance", false},
		{"response loss uses shared Carrier identity", "internal/domain/endpoint/connection_response_loss_linux_test.go", modulePath + "/internal/route/transport", true},
		{"caller loss checks exact Application outcome", "internal/domain/endpoint/connection_response_loss_linux_test.go", modulePath + "/internal/application/connection", true},
		{"response loss excludes direct TLS construction", "internal/domain/endpoint/connection_response_loss_linux_test.go", modulePath + "/internal/route/transport/tls", false},
		{"response loss excludes direct QUIC construction", "internal/domain/endpoint/connection_response_loss_linux_test.go", modulePath + "/internal/route/transport/quic", false},
		{"ordinary Text Reader consumes genuine Connection", "internal/domain/endpoint/reader/source_unix.go", modulePath + "/internal/domain/connection", true},
		{"installed Reader consumes qualified original worker", "internal/domain/endpoint/reader/operation_unix.go", modulePath + "/internal/domain/execution/runtime", true},
		{"another command cannot borrow Reader composition", "cmd/ardents-next/route.go", modulePath + "/internal/domain/connection", false},
		{"installed Service scenario consumes real Publisher", "internal/domain/endpoint/connection_installed_linux_test.go", modulePath + "/internal/domain/publication/runtime", true},
		{"installed Service scenario excludes direct TLS construction", "internal/domain/endpoint/connection_installed_linux_test.go", modulePath + "/internal/route/transport/tls", false},
		{"installed Service scenario excludes direct QUIC construction", "internal/domain/endpoint/connection_installed_linux_test.go", modulePath + "/internal/route/transport/quic", false},
		{"bootstrap scenario excludes TLS construction", "internal/domain/endpoint/route_issuer_fixture_linux_test.go", modulePath + "/internal/route/transport/tls", false},
		{"bootstrap scenario excludes QUIC construction", "internal/domain/endpoint/route_issuer_fixture_linux_test.go", modulePath + "/internal/route/transport/quic", false},
		{"unregistered scenario excludes lower contract", "cmd/ardents-next/route_other_linux_test.go", modulePath + "/internal/route/transport", false},
		{"production bootstrap excludes TLS construction", "cmd/ardents-next/route_bootstrap_linux.go", modulePath + "/internal/route/transport/tls", false},
		{"former selector cannot return in caller test", "cmd/ardents-next/route_prefix_caller_linux_test.go", modulePath + "/internal/route/carrier", false},
		{"no production Carrier allowance", "cmd/ardents-next/route_prefix_caller_linux.go", modulePath + "/internal/route/carrier", false},
		{"caller test cannot reach old Carrier", "cmd/ardents-next/route_prefix_caller_linux_test.go", modulePath + "/internal/route/carrier", false},
		{"JOIN cannot retain former selector", "internal/domain/node/route_join_linux_test.go", modulePath + "/internal/route/carrier", false},
		{"JOIN production excludes Carrier adapter", "cmd/ardents-next/route_join_linux.go", modulePath + "/internal/route/carrier", false},
		{"JOIN test excludes old Carrier", "internal/domain/node/route_join_linux_test.go", modulePath + "/internal/route/carrier", false},
		{"external public-contract test", "internal/admission/issuerprofile/provenance_test.go", modulePath + "/internal/admission/issuerprofile", true},
		{"no production self import", "internal/admission/issuerprofile/profile.go", modulePath + "/internal/admission/issuerprofile", false},
		{"no unregistered test self import", "internal/successor/future/file_test.go", modulePath + "/internal/successor/future", false},
		{"public profile is leaf", "internal/admission/issuerprofile/profile.go", modulePath + "/internal/admission", false},
		{"profile cannot borrow signing", "internal/admission/issuerprofile/profile.go", modulePath + "/internal/admission/issuerprofile/signing", false},
		{"profile tests cannot borrow signing", "internal/admission/issuerprofile/profile_test.go", modulePath + "/internal/admission/issuerprofile/signing", false},
		{"command cannot restore retired signer", "cmd/ardents-next/issuance.go", modulePath + "/internal/successor/nodeidentity", false},
		{"retired signer cannot restore facade", "internal/successor/nodeidentity/store.go", modulePath + "/internal/admission/issuerprofile/signing", false},
		{"identity cannot borrow ledger", "internal/admission/issuerprofile/signing/store.go", modulePath + "/internal/admission", false},
		{"ledger public grammar", "internal/admission/binding.go", modulePath + "/internal/admission/issuerprofile", true},
		{"issuance public grammar", "internal/admission/issuance/profile.go", modulePath + "/internal/admission/issuerprofile", true},
		{"no inherited nested imports", "internal/admission/issuance/nested/file.go", modulePath + "/internal/admission", false},
		{"no inherited nested third party", "internal/admission/issuance/nested/file.go", "github.com/cloudflare/circl/blindsign/blindrsa", false},
		{"custody allocation owner", "internal/domain/custody/admission_authority.go", modulePath + "/internal/admission/allocation", true},
		{"custody excludes old backend", "internal/domain/custody/vault.go", modulePath + "/internal/domain/custody", false},
		{"custody excludes live Instance parser", "internal/domain/custody/authority_state.go", modulePath + "/internal/domain/publication/instance", false},
		{"custody excludes Route", "internal/domain/custody/vault.go", modulePath + "/internal/route", false},
		{"Service command uses genuine new custody", "internal/domain/custody/command/service_authority.go", modulePath + "/internal/domain/custody", true},
		{"unregistered caller", "internal/successor/future/file.go", modulePath + "/internal/admission", false},
		{"unregistered dependency", "cmd/ardents-next/main.go", modulePath + "/internal/successor/future", false},
		{"standard", "internal/admission/check.go", "context", true},
		{"zone", "cmd/ardents-next/main.go", modulePath + "/internal/admission", true},
		{"hosting caller", "cmd/ardents-next/hosting.go", modulePath + "/internal/domain/hosting", true},
		{"independent domains", "internal/domain/hosting/budget.go", modulePath + "/internal/admission", false},
		{"network input adapter", "internal/domain/network/state/membership.go", modulePath + "/internal/domain/network", true},
		{"network adapter is exact", "internal/domain/network/state/other.go", modulePath + "/internal/domain/network", false},
		{"application observation", "internal/domain/network/state/runtime_observation.go", modulePath + "/internal/domain/network", true},
		{"original observation witness", "internal/domain/network/state/runtime_witness.go", modulePath + "/internal/domain/network", true},
		{"retained Route witness", "internal/route/prefix/prefix.go", modulePath + "/internal/domain/network/state", true},
		{"Route witness import is exact", "internal/route/prefix/other.go", modulePath + "/internal/domain/network/state", false},
		{"legacy consumer has no new State", "internal/domain/endpoint/source_state.go", modulePath + "/internal/domain/network/state", false},
		{"State consumer has no domain access", "internal/domain/endpoint/source_state.go", modulePath + "/internal/domain/network", false},
		{"State consumer has no physical root access", "internal/domain/endpoint/source_state.go", modulePath + "/internal/domain/network/state/durable", false},
		{"unregistered State consumer", "internal/service/publication/publication.go", modulePath + "/internal/domain/network/state", false},
		{"State excludes debit", "internal/domain/network/state/open.go", modulePath + "/internal/admission/quota", false},
		{"State excludes old backend", "internal/domain/network/state/open.go", modulePath + "/internal/domain/network/state", false},
		{"legacy admission consumer isolated", "internal/domain/node/authority/admission_observation.go", modulePath + "/internal/admission/receiving", false},
		{"legacy token consumer isolated", "internal/domain/node/authority/token.go", modulePath + "/internal/admission/receiving", false},
		{"observation excludes spending root", "internal/domain/node/authority/admission_observation.go", modulePath + "/internal/admission/spending", false},
		{"observation bridge is exact", "internal/domain/node/authority/receiver.go", modulePath + "/internal/admission/receiving", false},
		{"acquisition policy adapter", "internal/domain/network/state/acquisition_history.go", modulePath + "/internal/domain/network", true},
		{"acquisition adapter is exact", "internal/domain/network/state/attempts.go", modulePath + "/internal/domain/network", false},
		{"participation policy adapter", "internal/domain/network/duty/participation_policy.go", modulePath + "/internal/domain/network", true},
		{"participation adapter is exact", "internal/domain/network/duty/store.go", modulePath + "/internal/domain/network", false},
		{"candidate policy adapter", "internal/domain/network/epoch/candidate_evaluation.go", modulePath + "/internal/domain/network", true},
		{"assignment adapter", "internal/domain/network/epoch/assignment.go", modulePath + "/internal/domain/network", true},
		{"candidate adapter is exact", "internal/domain/network/epoch/record.go", modulePath + "/internal/domain/network", false},
		{"public token grammar adapter", "internal/domain/network/closedprofile/token_spki.go", modulePath + "/internal/admission/issuerprofile", true},
		{"public token adapter is exact", "internal/domain/network/closedprofile/profile.go", modulePath + "/internal/admission/issuerprofile", false},
		{"public token adapter excludes signing", "internal/domain/network/closedprofile/token_spki.go", modulePath + "/internal/admission/issuance", false},
		{"network excludes old state", "internal/domain/network/membership.go", modulePath + "/internal/domain/network/state", false},
		{"network excludes execution", "internal/domain/network/membership.go", modulePath + "/internal/domain/node", false},
		{"network excludes debit", "internal/domain/network/membership.go", modulePath + "/internal/admission", false},
		{"network excludes reservations", "internal/domain/network/membership.go", modulePath + "/internal/domain/hosting", false},
		{"issuance confirmed debit", "internal/admission/issuance/store.go", modulePath + "/internal/admission", true},
		{"operation owners", "internal/admission/issuer/operation.go", modulePath + "/internal/admission/issuance", true},
		{"operation excludes hosting", "internal/admission/issuer/operation.go", modulePath + "/internal/domain/hosting", false},
		{"issuance excludes hosting", "internal/admission/issuance/store.go", modulePath + "/internal/domain/hosting", false},
		{"identity purpose", "internal/admission/issuerprofile/signing/store.go", modulePath + "/internal/admission/issuerprofile", true},
		{"identity cannot borrow issuance", "internal/admission/issuerprofile/signing/store.go", modulePath + "/internal/admission/issuance", false},
		{"admission cannot borrow identity", "internal/admission/issuer_profile.go", modulePath + "/internal/admission/issuerprofile/signing", false},
		{"admission excludes composition", "internal/admission/ledger.go", modulePath + "/internal/admission/issuer", false},
		{"admission cannot borrow keys", "internal/admission/batch.go", modulePath + "/internal/admission/issuance", false},
		{"admission cannot borrow budget", "internal/admission/check.go", modulePath + "/internal/domain/hosting", false},
		{"legacy", "internal/admission/check.go", modulePath + "/internal/admission", false},
		{"legacy test fixture", "internal/admission/check_test.go", modulePath + "/tests/fixtures", false},
		{"third party bridge", "internal/admission/check.go", "example.org/bridge", false},
		{"reverse", "internal/domain/endpoint/runtime.go", modulePath + "/internal/admission", false},
		{"prefix collision", "internal/admission/check.go", modulePath + "/internal/successorold", false},
		{"unrelated", "internal/domain/endpoint/runtime.go", "context", true},
		{"relative", "internal/admission/check.go", "../admission", false},
		{"Source publication control excludes State authority", "internal/domain/release/infrastructure/source_canonical_publication_profile_native_linux_test.go", modulePath + "/internal/domain/network/state", false},
		{"Source startup native prefix excludes State authority", "internal/domain/release/infrastructure/source_startup_physical_prefix_profile_native_linux_test.go", modulePath + "/internal/domain/network/state", false},
		{"Source startup observer excludes State authority", "internal/domain/release/infrastructure/source_startup_physical_observer_native_linux_test.go", modulePath + "/internal/domain/network/state", false},
		{"Source retirement observer excludes State authority", "internal/domain/release/infrastructure/source_retirement_physical_observer_native_linux_test.go", modulePath + "/internal/domain/network/state", false},
		{"Endpoint platform native qualification uses new Enrollment", "tests/e2e/endpoint/endpoint_platform_native_linux_test.go", modulePath + "/internal/domain/enrollment", true},
		{"Endpoint platform native qualification uses fresh Release", "tests/e2e/endpoint/endpoint_platform_native_linux_test.go", modulePath + "/internal/domain/release", true},
		{"Endpoint platform native qualification uses generation authorization", "tests/e2e/endpoint/endpoint_platform_native_linux_test.go", modulePath + "/internal/domain/release/endpoint", true},
		{"Endpoint platform native qualification uses request grammar", "tests/e2e/endpoint/endpoint_platform_native_linux_test.go", modulePath + "/internal/domain/endpoint/plan", true},
		{"Endpoint platform native qualification uses unit contract", "tests/e2e/endpoint/endpoint_platform_native_linux_test.go", modulePath + "/internal/domain/endpoint/unit", true},
		{"Endpoint platform native qualification excludes old Endpoint", "tests/e2e/endpoint/endpoint_platform_native_linux_test.go", modulePath + "/internal/domain/endpoint", false},
		{"Endpoint platform native qualification excludes retired Release path", "tests/e2e/endpoint/endpoint_platform_native_linux_test.go", modulePath + "/internal/successor/release", false},
		{"Endpoint platform native qualification has no Node runtime", "tests/e2e/endpoint/endpoint_platform_native_linux_test.go", modulePath + "/internal/domain/node", false},
		{"adjacent Endpoint native test has no inherited Release edge", "cmd/ardents-next/other_endpoint_platform_native_linux_test.go", modulePath + "/internal/domain/release", false},
		{"Endpoint launch delegates Enrollment to platform CLI", "tests/e2e/endpoint/launch_replacement_native_linux_test.go", modulePath + "/internal/domain/enrollment", false},
		{"Endpoint launch delegates unit rendering to platform CLI", "tests/e2e/endpoint/launch_replacement_native_linux_test.go", modulePath + "/internal/domain/endpoint/unit", false},
		{"Endpoint launch roots use the Source bundle codec", "tests/e2e/endpoint/launch_roots_linux.go", modulePath + "/internal/domain/network/source", true},
		{"Endpoint launch native qualification excludes old Endpoint", "tests/e2e/endpoint/launch_replacement_native_linux_test.go", modulePath + "/internal/domain/endpoint", false},
		{"Endpoint launch native qualification has no Node runtime", "tests/e2e/endpoint/launch_replacement_native_linux_test.go", modulePath + "/internal/domain/node", false},
		{"adjacent Endpoint launch test has no inherited Source edge", "cmd/ardents-next/other_endpoint_launch_native_linux_test.go", modulePath + "/internal/domain/network/source", false},
		{"Endpoint pair native controller has no direct Enrollment edge", "tests/e2e/endpoint/endpoint_pair_native_linux_test.go", modulePath + "/internal/domain/enrollment", false},
		{"Endpoint pair native controller excludes old Endpoint", "tests/e2e/endpoint/endpoint_pair_native_linux_test.go", modulePath + "/internal/domain/endpoint", false},
		{"Endpoint pair native controller has no Node runtime", "tests/e2e/endpoint/endpoint_pair_native_linux_test.go", modulePath + "/internal/domain/node", false},
		{"Endpoint pair native controller has no direct Source edge", "tests/e2e/endpoint/endpoint_pair_native_linux_test.go", modulePath + "/internal/domain/network/source", false},
		{"Endpoint journey delegates Enrollment to platform CLI", "internal/domain/endpoint/endpoint_journey_native_linux_test.go", modulePath + "/internal/domain/enrollment", false},
		{"Endpoint journey delegates unit rendering to platform CLI", "internal/domain/endpoint/endpoint_journey_native_linux_test.go", modulePath + "/internal/domain/endpoint/unit", false},
		{"Endpoint journey fixture opens genuine State", "tests/e2e/endpoint/journey_participant_linux.go", modulePath + "/internal/domain/network/state", true},
		{"Endpoint journey native qualification names selected Carrier", "internal/domain/endpoint/endpoint_journey_native_linux_test.go", modulePath + "/internal/route/transport", true},
		{"Endpoint journey native qualification checks canonical Link codec", "internal/domain/endpoint/endpoint_journey_native_linux_test.go", modulePath + "/internal/domain/publication", true},
		{"Endpoint journey native qualification uses public Administration client", "internal/domain/endpoint/endpoint_journey_native_linux_test.go", modulePath + "/internal/application/administration", true},
		{"Endpoint journey native qualification uses public Connection client", "tests/e2e/endpoint/journey_participant_native_linux_test.go", modulePath + "/internal/application/connection", true},
		{"Endpoint journey native qualification uses actual document exchange", "tests/e2e/endpoint/journey_participant_native_linux_test.go", modulePath + "/internal/application/document", true},
		{"Endpoint journey native qualification excludes old Endpoint", "internal/domain/endpoint/endpoint_journey_native_linux_test.go", modulePath + "/internal/domain/endpoint", false},
		{"Endpoint journey native qualification has no Node runtime", "internal/domain/endpoint/endpoint_journey_native_linux_test.go", modulePath + "/internal/domain/node", false},
		{"Initiator authentication borrows closed key-exchange groups", "internal/domain/connection/initiator_authentication.go", modulePath + "/internal/route/transport", true},
		{"Initiator replacement borrows closed key-exchange groups", "internal/domain/connection/initiator_replacement.go", modulePath + "/internal/route/transport", true},
		{"Publisher attachment borrows closed key-exchange groups", "internal/domain/publication/instance/service_attachment.go", modulePath + "/internal/route/transport", true},
		{"Connection stream has no Route TLS contract", "internal/domain/connection/stream.go", modulePath + "/internal/route/transport", false},
		{"Initiator Source seam does not widen to the Route TLS contract", "internal/domain/connection/initiator_source.go", modulePath + "/internal/route/transport", false},
		{"Connection replacement commit has no Route TLS contract", "internal/domain/connection/initiator_replacement_commit.go", modulePath + "/internal/route/transport", false},
		{"Instance capsule opening has no Route TLS contract", "internal/domain/publication/instance/capsule_opening.go", modulePath + "/internal/route/transport", false},
		{"Instance recipient has no Route TLS contract", "internal/domain/publication/instance/recipient.go", modulePath + "/internal/route/transport", false},
		{"Instance replacement has no Route TLS contract", "internal/domain/publication/instance/service_replacement.go", modulePath + "/internal/route/transport", false},
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
