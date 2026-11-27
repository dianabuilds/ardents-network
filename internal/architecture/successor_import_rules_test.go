package architecture

import (
	"path"
	"slices"
	"strings"
)

// File-specific rules can make a decision or leave the edge to later owners.
type importPermission uint8

const (
	importUnspecified importPermission = iota
	importAllowed
	importDenied
)

func explicitImportResult(allowed bool) importPermission {
	if allowed {
		return importAllowed
	}
	return importDenied
}

func successorImportAllowed(source, dependency string) bool {
	if domainMeasurementImportAllowed(source, dependency) {
		return true
	}
	if source == "tests/networkfixture/provisioning_plans.go" &&
		(dependency == modulePath+"/internal/domain/network/closedprofile" || dependency == modulePath+"/tests/epochfixture/network") {
		return true
	}
	if offlineOperatorFile(source) {
		return offlineOperatorImportAllowed(source, dependency)
	}
	local, localImport := strings.CutPrefix(dependency, modulePath+"/")
	const socketFile = "internal/application/internal/socketfile"
	if local == socketFile {
		switch source {
		case "internal/application/administration/server.go",
			"internal/application/administration/server_retirement_test.go",
			"internal/application/connection/server_linux.go":
			return true
		default:
			return false
		}
	}
	if path.Dir(source) == socketFile {
		return allowedStandardImport(dependency)
	}
	const logStore = "internal/diagnostics/logstore"
	if local == logStore {
		return source == "internal/diagnostics/command/monitor.go" || source == "internal/diagnostics/command/monitor_test.go"
	}
	if path.Dir(source) == logStore {
		return allowedStandardImport(dependency)
	}
	if local == managerObservationPackage {
		return managerObservationConsumer(source)
	}
	if path.Dir(source) == managerObservationPackage {
		switch dependency {
		case "bytes", "encoding/json", "errors", "io", "reflect", "strings", "testing", "unicode/utf8":
			return true
		default:
			return false
		}
	}
	if local == "internal/domain/connection/internal/instancetls" {
		return source == "internal/domain/connection/initiator_authentication.go"
	}
	if path.Dir(source) == "internal/domain/connection/internal/instancetls" && localImport {
		return false
	}
	if local == "internal/domain/admission/console" {
		switch source {
		case "cmd/ardents-next/admission_console.go",
			"internal/domain/endpoint/session/local_conversation.go",
			"internal/domain/endpoint/funding/receiver_conversation.go":
			return true
		default:
			return false
		}
	}
	if path.Dir(source) == "internal/domain/admission/console" && localImport {
		return false
	}
	if retiredPackageImport(source, local, localImport) {
		return false
	}
	if local == "internal/domain/endpoint" && (source == "internal/domain/endpoint/installed_service_linux_test.go" || source == "internal/domain/endpoint/endpoint_journey_native_linux_test.go") {
		return false
	}
	if result := installedQualificationImport(source, dependency, local, localImport); result != importUnspecified {
		return result == importAllowed
	}
	if result := offlineAuthorityImport(source, dependency, local, localImport); result != importUnspecified {
		return result == importAllowed
	}
	if result := networkFixtureImport(source, dependency, local, localImport); result != importUnspecified {
		return result == importAllowed
	}
	if result := nodeAndSourceCommandImport(source, dependency, local, localImport); result != importUnspecified {
		return result == importAllowed
	}
	if result := processScenarioImport(source, dependency, local, localImport); result != importUnspecified {
		return result == importAllowed
	}
	// The Endpoint command retains the installed and offline effect dispatch.
	if strings.HasPrefix(source, "cmd/ardents-next/") && localImport && local == "internal/domain/endpoint" {
		return false
	}
	if local == "internal/domain/endpoint/command" {
		return source == "cmd/ardents-next/command.go"
	}
	switch source {
	case "tests/diagnostics/heapdump/heapdump_role_map_capture_test.go":
		if dependency == modulePath+"/internal/domain/admission" {
			return true
		}
		fallthrough
	case "tests/diagnostics/heapdump/heapdump_parser_test.go", "tests/diagnostics/heapdump/heapdump_capture_test.go":
		return allowedStandardImport(dependency)
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
		return allowedStandardImport(dependency)
	}
	if source == "cmd/ardents-next/enrollment.go" && localImport {
		return local == "internal/domain/enrollment"
	}
	if descriptorRetainedImportAllowed(source, dependency) {
		return true
	}
	if joinRetainedImportAllowed(source, dependency) {
		return true
	}
	inZone := isMigratedDomain(source) || strings.HasPrefix(source, "cmd/ardents-next/")
	zoneDependency := localImport && isMigratedDomain(local)
	if source == "internal/domain/route/prefix/prefix.go" && dependency == modulePath+"/internal/domain/network/state" {
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
		if local == "internal/pathcheck" {
			return true
		}
	}
	if source == "tests/e2e/endpoint/replacement_scopes_linux.go" && (local == "internal/platform/cgroup" || local == "internal/platform/systemd") {
		return true
	}
	if source == "tests/e2e/endpoint/launch_replacement_linux.go" && dependency == modulePath+"/internal/platform/systemd" {
		return true
	}
	if source == "internal/domain/endpoint/endpoint_journey_native_linux_test.go" && (dependency == modulePath+"/internal/domain/route/transport" || dependency == modulePath+"/internal/domain/publication") {
		return true
	}
	if source == "internal/domain/endpoint/endpoint_replacement_worker_native_linux_test.go" && dependency == modulePath+"/internal/domain/route/transport" {
		return true
	}
	// The cold/warm latency root driver (#650) names the selected Carrier for the
	// shared publication Route network exactly as the journey controller does.
	if source == "internal/domain/endpoint/endpoint_latency_native_linux_test.go" && dependency == modulePath+"/internal/domain/route/transport" {
		return true
	}
	if source == "internal/domain/network/state/source_repair_linux.go" && (dependency == modulePath+"/internal/platform/systemd" || dependency == modulePath+"/internal/platform/cgroup" || dependency == modulePath+"/internal/platform/process") {
		return true
	}
	if (source == "internal/domain/network/state/introduction_repair_linux.go" || source == "internal/domain/network/state/forwarding_repair_linux.go" || source == "internal/domain/network/state/join_repair_linux.go" || source == "internal/domain/network/state/descriptor_repair_linux.go") && (dependency == modulePath+"/internal/platform/systemd" || dependency == modulePath+"/internal/platform/cgroup") {
		return true
	}
	// The refusal controls pass only missing or unbound original handles.
	if (source == "internal/domain/network/state/source_repair_linux_test.go" || source == "internal/domain/network/state/node_duty_repair_linux_test.go") && dependency == modulePath+"/internal/platform/systemd" {
		return true
	}
	// The bounded Descriptor repair additionally clears an uncertain Store stage
	// through the existing-only reachability mechanism under the same dead-unit
	// proofs and retained leases; it grants no publication or runtime authority.
	if source == "internal/domain/network/state/descriptor_repair_linux.go" && dependency == modulePath+"/internal/domain/reachability" {
		return true
	}

	// Network's physical/application adapters consume domain policy through
	// exact files. Directory nesting does not grant policy or Admission access.
	if strings.HasPrefix(source, "internal/domain/network/") {
		sourceAdapter := source
		if sourceAdapter == "internal/domain/network/closedprofile/profile.go" && dependency == modulePath+"/internal/domain/admission/issuerprofile" {
			return true
		}
		if sourceAdapter == "internal/domain/network/duty/participation_policy.go" && dependency == modulePath+"/internal/domain/network" {
			return true
		}
		if dependency == modulePath+"/internal/domain/network" {
			switch sourceAdapter {
			case "internal/domain/network/state/closed_profile_role_join_test.go", "internal/domain/network/state/closed_runtime_view_test.go":
				return inZone
			case "internal/domain/network/state/acquisition_history.go":
				return true
			case "internal/domain/network/epoch/candidate_evaluation.go", "internal/domain/network/epoch/assignment.go":
				return true
			case "internal/domain/network/state/command/operation.go", "internal/domain/network/state/epoch_history.go", "internal/domain/network/state/membership.go", "internal/domain/network/state/closed_member.go", "internal/domain/network/state/closed_runtime_view.go", "internal/domain/network/state/runtime_observation.go", "internal/domain/network/state/runtime_witness.go", "internal/domain/network/state/clock_observation.go", "internal/domain/network/state/closed_profile_view.go", "internal/domain/network/state/closed_duty_binding.go", "internal/domain/network/state/closed_profile_accept.go":
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
			if local == "internal/domain/node" || local == "internal/platform/launch" {
				return true
			}
		case "cmd/ardents-next/infrastructure_operator.go":
			if local == "internal/domain/release/infrastructure" {
				return true
			}
		case "internal/domain/admission/issuer/current_authority.go", "internal/domain/admission/receiving/current_authority.go":
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
		case "internal/domain/node/role_authority.go":
			if local == "internal/domain/network/state" || local == "internal/domain/admission/issuer" {
				return true
			}
		case "internal/domain/node/route_admission_linux.go":
			if local == "internal/domain/admission/issuer" {
				return true
			}
		case "internal/domain/node/route_receive_linux.go":
			if local == "internal/domain/admission/issuer" || local == "internal/domain/admission/quota" || local == "internal/domain/network/state" || local == "internal/platform/artifacts" || local == "internal/domain/reachability" || local == "internal/domain/route/channel" || local == "internal/domain/route/introduction" {
				return true
			}
		case "internal/domain/network/state/command/declaration.go":
			return local == "internal/platform/artifacts" || local == "internal/domain/network/state"
		case "internal/domain/network/state/command/operation.go", "internal/domain/network/state/command/cancellation_test.go":
			if local == "internal/domain/network/state" {
				return true
			}
		case "internal/domain/network/state/command/command.go":
			return local == "internal/platform/artifacts" || local == "internal/domain/network"
		case "internal/domain/endpoint/session/local_holder.go":
			if local == "internal/domain/network" || local == "internal/domain/network/state" {
				return true
			}
		case "internal/domain/endpoint/session/local_configuration.go", "internal/domain/endpoint/session/local_conversation.go", "internal/domain/endpoint/session/execution_holder.go", "internal/domain/endpoint/session/execution_refusal_other_test.go":
			if local == "internal/domain/network/state" {
				return true
			}
		case "internal/domain/endpoint/funding/current_issuer.go":
			if local == "internal/domain/admission/issuer" || local == "internal/domain/admission/quota" {
				return true
			}
		case "internal/domain/endpoint/funding/local_receiver.go", "internal/domain/endpoint/funding/receiver_conversation.go":
			if local == "internal/domain/admission/receiving" {
				return true
			}
		case "internal/domain/endpoint/funding/admission_authority.go":
			if local == "internal/domain/admission/issuer" || local == "internal/domain/admission/receiving" || local == "internal/platform/artifacts" {
				return true
			}
		case "internal/domain/admission/issuer/profile_declaration.go":
			if local == "internal/canonicaldigest" || local == "internal/platform/artifacts" {
				return true
			}
		case "internal/domain/hosting/command_declaration.go", "internal/domain/admission/issuer/result_operation.go":
			if local == "internal/platform/artifacts" {
				return true
			}
		}
	}
	if source == "internal/domain/endpoint/command/preparation_command_linux.go" && dependency == modulePath+"/internal/domain/network/source" {
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
		if (source == "tests/e2e/service/admission_custody_process_test.go" || source == "internal/domain/endpoint/permission_request_fixture_test.go" || source == "internal/domain/admission/stock/permission_request_fixture_test.go" || source == "internal/domain/admission/stock/permission_view_test.go") && dependency == modulePath+"/internal/domain/admission" {
			return true
		}
		return !zoneDependency
	}
	if (source == "internal/domain/route/introduction/receiving_history_unix.go" || source == "internal/domain/route/introduction/receiving_history_other.go") && dependency == modulePath+"/internal/domain/admission/receiving" {
		return true
	}
	if (source == "internal/domain/admission/issuer/current_authority.go" || source == "internal/domain/admission/receiving/current_authority.go") && dependency == modulePath+"/internal/domain/network" {
		return true
	}
	if source == "internal/domain/admission/issuer/profile_declaration.go" && dependency == modulePath+"/internal/canonicaldigest" {
		return true
	}
	if (source == "internal/domain/admission/issuance/result_declaration.go" || source == "internal/domain/hosting/command_declaration.go" || source == "internal/domain/admission/issuer/profile_declaration.go") && dependency == modulePath+"/internal/platform/artifacts" {
		return true
	}
	if source == "internal/domain/release/infrastructure/launch/verify_linux_test.go" && dependency == modulePath+"/internal/platform/processguard" {
		return true
	}
	if dependency == modulePath+"/internal/domain/network/state/internal/distribution" {
		return path.Dir(source) == "internal/domain/network/state"
	}
	if source == "internal/domain/endpoint/command/preparation_files_linux.go" {
		switch local {
		case "internal/domain/network/source", "internal/domain/network/state",
			"internal/pathcheck", "internal/platform/artifacts":
			return true
		}
	}
	if dependency == modulePath+"/internal/domain/endpoint/internal/permission" {
		return source == "internal/domain/endpoint/participant_linux.go" ||
			source == "internal/domain/endpoint/latency_native_linux_test.go"
	}
	if dependency == modulePath+"/internal/domain/endpoint" {
		switch source {
		case "internal/domain/endpoint/command/installed_command_linux.go",
			"internal/domain/endpoint/command/preparation_files_linux.go",
			"internal/domain/endpoint/command/progress_report_linux_test.go":
			return true
		}
	}
	if dependency == modulePath+"/internal/domain/network/state/command" {
		return source == "cmd/ardents-next/network.go"
	}
	if dependency == modulePath+"/internal/domain/node/provisioning" {
		switch source {
		case "cmd/ardents-next/network.go":
			return true
		}
	}
	owner := path.Dir(source)
	if zoneDependency {
		return successorDomainPackageImport(source, dependency, owner)
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
	if (source == "internal/domain/connection/stream.go" || source == "internal/domain/connection/stream_retirement.go" || source == "internal/domain/connection/stream_test.go") && dependency == modulePath+"/internal/application/connection" {
		return true
	}
	if (source == "internal/domain/endpoint/connection_response_loss_linux_test.go" || source == "internal/domain/endpoint/connection_authority_linux_test.go") && dependency == modulePath+"/internal/application/connection" {
		return true
	}
	if source == "internal/domain/endpoint/publisher/snapshot_unix.go" && dependency == modulePath+"/internal/application/document" {
		return true
	}
	if (owner == "internal/domain/admission/issuance" || owner == "internal/domain/admission/token") && dependency == "github.com/cloudflare/circl/blindsign/blindrsa" {
		return true
	}
	if (source == "internal/platform/systemd/manager_reference_linux.go" || source == "internal/platform/systemd/node_slots_linux.go" || source == "internal/platform/systemd/forwarding_manager_linux.go" || source == "internal/platform/systemd/introduction_manager_linux.go" || source == "internal/platform/systemd/endpoint_reference_linux.go" || source == "internal/platform/systemd/startup_observation_linux.go" || source == "internal/platform/systemd/manager_properties_linux.go" || source == "internal/platform/systemd/source_manager_linux.go" || source == "internal/platform/systemd/source_reference_native_linux_test.go") && dependency == "github.com/godbus/dbus/v5" {
		return true
	}
	if (owner == "internal/domain/admission/spending" || owner == "internal/domain/network/duty" || owner == "internal/domain/network/state/durable") && dependency == "golang.org/x/sys/windows" {
		return true
	}
	if (source == "internal/durablefile/owner_root_windows.go" || source == "internal/durablefile/owner_root_windows_test.go" || source == "internal/domain/route/selection/root_permissions_windows_test.go") && dependency == "golang.org/x/sys/windows" {
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
		if (owner == "internal/domain/network/epoch" || owner == "internal/domain/network/state" || source == "internal/domain/network/state/command/cancellation_test.go" || owner == "cmd/ardents-next" || source == "internal/domain/endpoint/history_preparation_linux_test.go" || source == "internal/domain/endpoint/plan_construction_linux_test.go") && dependency == modulePath+"/tests/epochfixture/network" {
			return true
		}
		if owner == "internal/domain/network/epoch" && dependency == modulePath+"/tests/epochfixture/assignment" {
			return true
		}
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
	if dependency == modulePath+"/internal/diagnostics/runtimeevent" {
		switch source {
		case "cmd/ardents-next/command.go", "cmd/ardents-next/hosting.go",
			"internal/diagnostics/telemetry/observation.go", "internal/diagnostics/telemetry/observation_test.go",
			"internal/diagnostics/telemetry/delivery.go", "internal/diagnostics/telemetry/delivery_test.go", "internal/diagnostics/telemetry/vocabulary_test.go",
			"internal/domain/admission/operator/admission.go", "internal/domain/admission/operator/issuance.go",
			"internal/domain/custody/command/command.go", "internal/domain/endpoint/command/installed_command_linux.go",
			"internal/domain/endpoint/command/installed_command_other.go", "internal/domain/endpoint/command/preparation_command_linux.go",
			"internal/domain/endpoint/command/progress_report_linux_test.go",
			"internal/diagnostics/timeline/project.go", "internal/diagnostics/command/capture.go",
			"internal/diagnostics/command/report.go", "internal/diagnostics/command/view.go",
			"internal/diagnostics/command/monitor.go", "internal/diagnostics/command/monitor_panel.go":
			return true
		}
	}
	if dependency == modulePath+"/internal/diagnostics/telemetry" {
		switch source {
		case "cmd/ardents-next/hosting.go", "internal/domain/admission/operator/admission.go", "internal/domain/admission/operator/issuance.go",
			"internal/domain/admission/operator/input_report.go":
			return true
		}
	}
	if source == "internal/diagnostics/telemetry/vocabulary_test.go" {
		switch dependency {
		case "go.opentelemetry.io/proto/otlp/collector/trace/v1", "go.opentelemetry.io/proto/otlp/collector/metrics/v1", "google.golang.org/protobuf/proto":
			return true
		}
	}
	if source == "internal/diagnostics/telemetry/observation.go" {
		if slices.Contains([]string{
			"go.opentelemetry.io/otel/attribute",
			"go.opentelemetry.io/otel/metric",
			"go.opentelemetry.io/otel/trace",
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
	return allowedStandardImport(dependency)
}

// Go reserves first import-path elements without a dot for standard-library
// paths. Unknown paths still fail compilation. cgo, unsafe and noncanonical
// paths require a different policy.
func allowedStandardImport(dependency string) bool {
	first, _, _ := strings.Cut(dependency, "/")
	return dependency != "C" && dependency != "unsafe" && dependency != "" && !strings.Contains(first, ".") && path.Clean(dependency) == dependency && !strings.HasPrefix(dependency, "/")
}
