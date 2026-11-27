package architecture

import "strings"

// Compiled process scenarios and their local test drivers have explicit imports.
func processScenarioImport(source, dependency, local string, localImport bool) importPermission {
	if first, _, _ := strings.Cut(dependency, "/"); strings.Contains(first, ".") {
		switch source {
		case "tests/e2e/node/hosting_commands_linux_test.go":
			return explicitImportResult(dependency == modulePath+"/internal/domain/hosting" ||
				dependency == "go.opentelemetry.io/proto/otlp/collector/metrics/v1" ||
				dependency == "go.opentelemetry.io/proto/otlp/collector/trace/v1" || dependency == "google.golang.org/protobuf/proto")
		case "tests/e2e/node/source_lifecycle_native_linux_test.go":
			return explicitImportResult(dependency == "github.com/dianabuilds/ardents-network/internal/domain/enrollment" || dependency == "github.com/dianabuilds/ardents-network/internal/domain/network/duty" || dependency == "github.com/dianabuilds/ardents-network/internal/domain/network/epoch" || dependency == "github.com/dianabuilds/ardents-network/internal/domain/network/source" || dependency == "github.com/dianabuilds/ardents-network/internal/domain/network/source/servingdeclaration" || dependency == "github.com/dianabuilds/ardents-network/internal/domain/network/state" || dependency == "github.com/dianabuilds/ardents-network/internal/platform/systemd" || dependency == "github.com/dianabuilds/ardents-network/internal/platform/processguard" || dependency == "github.com/dianabuilds/ardents-network/internal/domain/release" || dependency == "github.com/dianabuilds/ardents-network/internal/domain/release/infrastructure" || dependency == "github.com/dianabuilds/ardents-network/tests/epochfixture/network" || dependency == "github.com/theupdateframework/go-tuf/v2/metadata" || dependency == "github.com/dianabuilds/ardents-network/tests/e2e/node" || dependency == "github.com/dianabuilds/ardents-network/tests/e2e/source")
		case "tests/e2e/node/network_commands_unix_test.go":
			return explicitImportResult(dependency == modulePath+"/internal/domain/admission" || dependency == modulePath+"/internal/domain/admission/quota" ||
				dependency == modulePath+"/internal/domain/admission/receiving" || dependency == modulePath+"/internal/domain/admission/stock" ||
				dependency == modulePath+"/internal/domain/admission/token" || dependency == modulePath+"/internal/domain/network" ||
				dependency == modulePath+"/internal/domain/network/state" || dependency == modulePath+"/tests/e2e/node" ||
				dependency == modulePath+"/tests/epochfixture/network")
		}
	}
	if localImport {
		switch source {
		case "tests/e2e/node/command_usage_test.go":
			return explicitImportResult(local == "internal/platform/systemd")
		case "tests/e2e/admissioncli/offline_profile_inspection_test.go":
			return explicitImportResult(local == "internal/domain/admission/issuerprofile" || local == "internal/domain/network/closedprofile")
		case "tests/e2e/admissioncli/admission_authority_test.go", "tests/e2e/node/execution_route_refusal_test.go":
			return explicitImportResult(local == "internal/domain/admission" || local == "internal/domain/network/state" || local == "tests/e2e/node")
		case "tests/e2e/admissioncli/admission_receiver_test.go":
			return explicitImportResult(local == "internal/domain/admission" || local == "internal/domain/admission/issuerprofile" ||
				local == "internal/domain/admission/receiving" || local == "tests/e2e/node")
		case "tests/e2e/admissioncli/admission_interrupt_unix_test.go", "tests/e2e/admissioncli/admission_console_input_test.go":
			return explicitImportResult(local == "tests/e2e/node")
		case "tests/e2e/node/admission_cycle_test.go":
			return explicitImportResult(local == "internal/domain/admission" || local == "internal/domain/admission/issuance" ||
				local == "internal/domain/admission/issuer" || local == "internal/domain/admission/quota" ||
				local == "internal/domain/admission/receiving" || local == "internal/domain/admission/stock" ||
				local == "internal/domain/admission/token" || local == "tests/e2e/node")
		case "tests/e2e/routeprocess/route_bootstrap_linux_test.go":
			return explicitImportResult(local == "internal/domain/admission" || local == "internal/domain/admission/quota" || local == "internal/domain/admission/stock" || local == "internal/domain/endpoint/funding" || local == "internal/domain/endpoint/session" || local == "internal/domain/hosting" ||
				local == "internal/domain/network" || local == "internal/domain/network/epoch" || local == "internal/domain/network/state" || local == "tests/epochfixture/network")
		case "tests/e2e/routeprocess/route_issuer_recovery_test.go":
			return explicitImportResult(local == "internal/domain/admission" || local == "internal/domain/admission/quota" || local == "internal/domain/admission/stock" ||
				local == "internal/domain/admission/token" || local == "internal/domain/endpoint/funding" || local == "internal/domain/network/state" || local == "tests/epochfixture/network")
		case "tests/e2e/routeprocess/route_quic_linux_test.go":
			return explicitImportResult(local == "internal/domain/admission" || local == "internal/domain/admission/quota" || local == "internal/domain/admission/stock" ||
				local == "internal/domain/admission/token" || local == "internal/domain/endpoint/funding" || local == "internal/domain/hosting" ||
				local == "internal/domain/network/epoch" || local == "internal/domain/network/state" || local == "tests/epochfixture/network")
		case "tests/e2e/routeprocess/route_registration_linux_test.go":
			return explicitImportResult(local == "internal/domain/admission" || local == "internal/domain/admission/quota" || local == "internal/domain/admission/stock" ||
				local == "internal/domain/admission/token" || local == "internal/domain/endpoint/funding" || local == "internal/domain/endpoint/session" || local == "internal/domain/hosting" ||
				local == "internal/domain/network" || local == "internal/domain/network/epoch" || local == "internal/domain/network/state" || local == "tests/epochfixture/network")
		case "tests/e2e/routeprocess/route_join_linux_test.go":
			return explicitImportResult(local == "internal/domain/admission" || local == "internal/domain/admission/quota" || local == "internal/domain/admission/stock" ||
				local == "internal/domain/admission/token" || local == "internal/domain/endpoint/funding" || local == "internal/domain/endpoint/session" ||
				local == "internal/domain/hosting" || local == "internal/domain/network" || local == "internal/domain/network/epoch" || local == "internal/domain/network/state" || local == "tests/epochfixture/network")
		case "tests/e2e/routeprocess/route_descriptor_linux_test.go":
			return explicitImportResult(local == "internal/domain/admission" || local == "internal/domain/admission/quota" || local == "internal/domain/admission/stock" ||
				local == "internal/domain/admission/token" || local == "internal/domain/endpoint/funding" || local == "internal/domain/endpoint/session" || local == "internal/domain/hosting" ||
				local == "internal/domain/network/epoch" || local == "internal/domain/network/state" || local == "internal/domain/reachability" || local == "tests/epochfixture/network")
		case "tests/e2e/routeprocess/route_hello_linux_test.go":
			return explicitImportResult(local == "internal/domain/admission" || local == "internal/domain/admission/quota" || local == "internal/domain/admission/stock" ||
				local == "internal/domain/admission/token" || local == "internal/domain/endpoint/funding" || local == "internal/domain/hosting" ||
				local == "internal/domain/network/state" || local == "tests/epochfixture/network")
		case "tests/e2e/node/admission_hosting_linux_test.go", "tests/e2e/node/hosting_work_fixture_linux_test.go":
			return explicitImportResult(local == "internal/domain/admission" || local == "internal/domain/admission/receiving" || local == "internal/domain/hosting")
		case "tests/e2e/node/hosting_transfer_linux_test.go":
			return explicitImportResult(local == "internal/domain/admission")
		case "tests/e2e/node/hosting_work_lifecycle_linux_test.go":
			return explicitImportResult(local == "internal/domain/admission" || local == "internal/domain/admission/issuerprofile" ||
				local == "internal/domain/admission/receiving" || local == "internal/domain/admission/token" || local == "internal/domain/hosting")
		case "tests/e2e/nodeinstalled/source_installed_native_linux_test.go":
			return explicitImportResult(local == "internal/domain/network/duty" || local == "internal/domain/network/epoch" ||
				local == "internal/domain/network/source" || local == "internal/domain/network/source/servingdeclaration" ||
				local == "internal/domain/network/state" || local == "internal/platform/systemd" ||
				local == "internal/domain/release/infrastructure" || local == "internal/domain/release/infrastructure/launch" ||
				local == "tests/e2e/node" || local == "tests/e2e/source" || local == "tests/epochfixture/network")
		case "tests/e2e/node/source_preparation_native_linux_test.go":
			return explicitImportResult(local == "internal/domain/endpoint" || local == "tests/e2e/source")
		case "tests/e2e/node/certificate_native_linux_test.go":
			return explicitImportResult(local == "tests/e2e/source")
		case "tests/e2e/node/processfixture/authority_inputs_linux.go":
			return explicitImportResult(local == "internal/domain/admission/issuer" || local == "internal/domain/network/state" || local == "tests/epochfixture/network")
		case "tests/e2e/node/processfixture/issuer_roots_linux.go":
			return explicitImportResult(local == "internal/domain/admission/issuance" || local == "internal/domain/admission/issuer" || local == "internal/domain/admission/quota")
		case "tests/e2e/node/processfixture/authority_observation_linux.go":
			return explicitImportResult(local == "internal/domain/admission/spending" || local == "internal/domain/hosting" ||
				local == "internal/domain/network" || local == "internal/domain/network/duty" || local == "internal/domain/network/epoch")
		case "tests/e2e/node/processfixture/receiving_fixture_linux.go":
			return explicitImportResult(local == "tests/epochfixture/network")
		case "tests/e2e/node/processfixture/introduction_stock_linux.go", "tests/e2e/node/processfixture/route_stock_linux.go":
			switch local {
			case "internal/domain/admission", "internal/domain/admission/issuer", "internal/domain/admission/quota", "internal/domain/admission/stock",
				"internal/domain/admission/token", "internal/domain/endpoint/funding", "internal/domain/network", "tests/epochfixture/network":
				return importAllowed
			default:
				return importDenied
			}
		case "tests/e2e/node/source_native_linux_test.go":
			return explicitImportResult(local == "internal/domain/network/epoch" || local == "internal/domain/network/source" || local == "internal/domain/network/source/servingdeclaration" || local == "internal/domain/network/state" || local == "internal/platform/processguard" || local == "tests/epochfixture/network" || local == "tests/e2e/node" || local == "tests/e2e/source")
		case "tests/e2e/node/source_preparation_process_linux_test.go":
			return explicitImportResult(local == "internal/domain/endpoint" || local == "internal/domain/network/source" || local == "internal/domain/network/source/servingdeclaration" || local == "internal/domain/network/state" || local == "internal/platform/artifacts" || local == "tests/epochfixture/network" || local == "internal/domain/network/source/serving" || local == "tests/e2e/node" || local == "tests/e2e/source")
		}
	}
	if localImport && source == "tests/e2e/node/processfixture/descriptor_input_linux.go" {
		return explicitImportResult(local == "internal/domain/admission")
	}
	if localImport && source == "tests/e2e/node/join_exchange_native_linux_test.go" {
		switch local {
		case "internal/domain/admission", "internal/domain/admission/issuer", "internal/domain/admission/quota",
			"internal/domain/admission/stock", "internal/domain/admission/token", "internal/domain/endpoint/funding", "internal/domain/endpoint/session",
			"internal/domain/hosting", "internal/domain/network", "internal/domain/network/epoch", "internal/domain/network/state",
			"internal/platform/processguard", "tests/epochfixture/network":
			return importAllowed
		default:
			return importDenied
		}
	}
	if localImport && source == "tests/e2e/node/descriptor_exchange_native_linux_test.go" {
		switch local {
		case "internal/domain/admission", "internal/domain/admission/issuer", "internal/domain/admission/quota", "internal/domain/admission/stock",
			"internal/domain/admission/token", "internal/domain/endpoint/funding", "internal/domain/hosting", "internal/domain/network/duty",
			"internal/domain/network/epoch", "internal/domain/network/state", "internal/platform/processguard", "internal/domain/reachability", "tests/epochfixture/network":
			return importAllowed
		default:
			return importDenied
		}
	}
	if localImport && source == "tests/e2e/node/forwarding_exchange_native_linux_test.go" {
		switch local {
		case "internal/domain/admission", "internal/domain/admission/issuer", "internal/domain/admission/quota",
			"internal/domain/admission/stock", "internal/domain/admission/token", "internal/domain/endpoint/funding", "internal/domain/hosting",
			"internal/domain/network", "internal/domain/network/duty", "internal/domain/network/epoch", "internal/domain/network/state",
			"internal/platform/processguard", "tests/epochfixture/network":
			return importAllowed
		default:
			return importDenied
		}
	}
	if localImport && source == "tests/e2e/node/processfixture/installed_receiving_reopen_linux.go" {
		return explicitImportResult(local == "internal/domain/admission" || local == "internal/domain/admission/receiving" || local == "internal/domain/hosting")
	}
	if localImport && source == "tests/e2e/node/processfixture/receiving_spend_linux.go" {
		return explicitImportResult(local == "internal/domain/admission" || local == "internal/domain/admission/receiving")
	}
	if localImport && source == "tests/e2e/node/processfixture/route_topology_linux.go" {
		return explicitImportResult(local == "tests/epochfixture/network")
	}
	if localImport && source == "tests/e2e/node/processfixture/permitted_stock_unix.go" {
		return explicitImportResult(local == "internal/domain/admission" || local == "internal/domain/admission/stock")
	}
	if localImport && source == "tests/e2e/node/processfixture/admission_permission.go" {
		return explicitImportResult(local == "internal/domain/admission" || local == "internal/domain/admission/allocation")
	}
	if localImport && source == "tests/e2e/node/processfixture/admission_console.go" {
		return explicitImportResult(local == "internal/domain/admission" || local == "internal/domain/admission/stock")
	}
	if localImport && source == "tests/e2e/node/provisioning_linux.go" {
		return explicitImportResult(local == "internal/platform/artifacts" || local == "tests/epochfixture/network")
	}
	if localImport && source == "tests/e2e/node/receiving_startup_native_linux_test.go" {
		return explicitImportResult(local == "internal/domain/hosting" || local == "internal/domain/network/duty" || local == "internal/domain/network/epoch" ||
			local == "internal/platform/processguard" || local == "tests/epochfixture/network")
	}
	if localImport && (source == "tests/e2e/node/forwarding_startup_native_linux_test.go" || source == "tests/e2e/node/completion_native_linux_test.go") {
		return explicitImportResult(local == "internal/domain/hosting" || local == "internal/domain/network/duty" || local == "internal/domain/network/epoch" ||
			local == "internal/domain/network/state" || local == "internal/platform/processguard" || local == "tests/epochfixture/network")
	}
	if localImport && source == "tests/e2e/node/issuer_exchange_native_linux_test.go" {
		return explicitImportResult(local == "internal/domain/admission" || local == "internal/domain/endpoint/funding" || local == "internal/domain/hosting" ||
			local == "internal/domain/network/epoch" || local == "internal/domain/network/state" || local == "internal/platform/processguard" ||
			local == "tests/epochfixture/network")
	}
	if localImport && source == "tests/e2e/node/issuer_startup_native_linux_test.go" {
		return explicitImportResult(local == "internal/domain/admission/issuance" || local == "internal/domain/admission/issuer" || local == "internal/domain/admission/quota" || local == "internal/domain/hosting" ||
			local == "internal/domain/network/duty" || local == "internal/domain/network/epoch" || local == "internal/platform/processguard" || local == "tests/epochfixture/network")
	}
	if localImport && source == "tests/e2e/node/issuer_inputs_linux.go" {
		return explicitImportResult(local == "internal/domain/admission/issuance" || local == "internal/domain/admission/issuer" || local == "internal/domain/admission/quota")
	}
	if localImport && source == "tests/e2e/node/retained_lifecycle_native_linux_test.go" {
		return explicitImportResult(local == "internal/domain/hosting" || local == "internal/domain/network/duty" ||
			local == "internal/platform/processguard" || local == "internal/domain/reachability" || local == "tests/epochfixture/network")
	}
	if localImport && (source == "tests/e2e/node/processfixture/hosting_fixture_linux.go" || source == "tests/e2e/node/processfixture/hosting_return_linux.go") {
		return explicitImportResult(local == "internal/domain/hosting")
	}
	if localImport {
		switch source {
		case "tests/e2e/endpointinstalled/endpoint_platform_native_linux_test.go":
			return explicitImportResult(local == "internal/domain/enrollment" || local == "internal/domain/release" ||
				local == "internal/domain/release/endpoint" || local == "internal/domain/release/infrastructure/launch" ||
				local == "internal/domain/endpoint/plan" || local == "internal/domain/endpoint/unit" ||
				local == "tests/e2e/node" || local == "tests/e2e/source" || local == "tests/e2e/endpoint")
		case "tests/e2e/endpoint/native_delivery_linux.go":
			return explicitImportResult(local == "internal/domain/endpoint/plan" || local == "internal/domain/release/infrastructure/launch")
		case "tests/e2e/endpointinstalled/journey_participant_native_linux_test.go":
			return explicitImportResult(local == "tests/e2e/endpoint" || local == "internal/application/administration" || local == "internal/application/endpointstream" || local == "internal/application/document")
		case "tests/e2e/endpoint/journey_participant_linux.go":
			return explicitImportResult(local == "internal/domain/network/state" || local == "tests/releasefixture")
		case "tests/e2e/endpoint/connection_reopen_linux.go":
			return explicitImportResult(local == "internal/domain/network" || local == "internal/domain/publication/durable" || local == "internal/domain/publication/instance" || local == "internal/domain/reachability")
		case "tests/e2e/endpoint/connection_worker_linux.go":
			return explicitImportResult(local == "internal/domain/execution/runtime")
		case "tests/e2e/endpoint/connection_authority_linux.go":
			return explicitImportResult(local == "internal/application/endpointstream" || local == "internal/domain/connection")
		case "tests/e2e/endpoint/document_commands_linux.go":
			return explicitImportResult(local == "internal/application/administration" || local == "internal/domain/publication")
		case "tests/e2e/endpoint/service_refresh_linux.go":
			return explicitImportResult(local == "internal/domain/publication" || local == "internal/domain/reachability")
		case "tests/e2e/endpoint/recovery_custody_instance_linux.go", "tests/e2e/endpoint/service_instance_linux.go":
			return explicitImportResult(local == "internal/domain/publication")
		case "tests/e2e/endpointinstalled/launch_replacement_native_linux_test.go":
			return explicitImportResult(local == "tests/e2e/endpoint" || local == "tests/networkfixture")
		case "tests/e2e/endpointinstalled/endpoint_idle_native_linux_test.go", "tests/e2e/endpointinstalled/endpoint_enrollment_migration_native_linux_test.go":
			return explicitImportResult(local == "tests/e2e/endpoint")
		case "tests/e2e/endpointinstalled/launch_refusals_native_linux_test.go":
			return explicitImportResult(local == "tests/e2e/endpoint" || local == "tests/networkfixture")
		case "tests/e2e/endpointinstalled/endpoint_pair_native_linux_test.go", "tests/e2e/endpointcli/release_process_test.go":
			return explicitImportResult(local == "tests/e2e/endpoint")
		case "tests/e2e/endpoint/launch_roots_linux.go":
			return explicitImportResult(local == "internal/domain/network/source")
		case "tests/e2e/endpoint/launch_sources_linux.go":
			return explicitImportResult(local == "internal/domain/network/state" || local == "tests/e2e/source")
		case "tests/e2e/endpoint/launch_stand_linux.go":
			return explicitImportResult(local == "internal/domain/network/state" || local == "tests/epochfixture/network")
		case "tests/e2e/endpoint/native_startup_linux.go":
			return explicitImportResult(local == "tests/epochfixture/network")
		}
	}
	if localImport {
		switch source {
		case "tests/e2e/endpoint/idle_observation_linux.go", "tests/e2e/endpoint/idle_observation_linux_test.go",
			"tests/e2e/endpoint/idle_projection.go", "tests/e2e/endpoint/idle_hosting_linux_test.go":
			return explicitImportResult(local == "internal/domain/hosting")
		}
	}
	if localImport && source == "tests/e2e/endpoint/permission_approval_linux_test.go" {
		return explicitImportResult(local == "internal/domain/admission")
	}
	if localImport && source == "tests/e2e/endpoint/permission_approval_linux.go" {
		return explicitImportResult(local == "internal/domain/admission" || local == "internal/domain/admission/allocation")
	}
	if localImport && source == "tests/e2e/endpoint/installation_network_test.go" {
		return explicitImportResult(local == "internal/domain/endpoint/plan")
	}
	if localImport && source == "tests/e2e/endpoint/snapshot_worker_linux.go" {
		return explicitImportResult(local == "internal/application/administration")
	}
	if localImport && local == "tests/e2e/endpoint" {
		switch source {
		case "internal/domain/endpoint/execution_installed_linux_test.go", "internal/domain/endpoint/publication_ack_installed_linux_test.go":
			return importAllowed
		case "internal/domain/endpoint/connection_installed_linux_test.go", "internal/domain/endpoint/connection_accepted_request_installed_linux_test.go", "internal/domain/endpoint/document_command_installed_linux_test.go",
			"internal/domain/endpoint/installed_service_linux_test.go", "internal/domain/endpoint/endpoint_latency_native_linux_test.go", "internal/domain/endpoint/endpoint_replacement_worker_native_linux_test.go",
			"internal/domain/endpoint/publication_installed_linux_test.go",
			"internal/domain/endpoint/endpoint_launch_fixture_linux_test.go", "internal/domain/endpoint/endpoint_journey_native_linux_test.go":
			return importAllowed
		default:
			return importDenied
		}
	}
	// These command tests consume only the registered process and signed fixture owners.
	if localImport && source == "internal/domain/node/runtime/introduction_exchange_native_linux_test.go" {
		switch local {
		case "internal/domain/endpoint/funding", "internal/domain/endpoint/session", "internal/domain/hosting",
			"internal/domain/network/epoch", "internal/domain/network/state", "internal/platform/processguard", "internal/domain/route/capsule",
			"internal/domain/route/introduction", "tests/epochfixture/network":
			return importAllowed
		}
	}
	if localImport && source == "internal/domain/node/runtime/operation_workload_native_linux_test.go" {
		switch local {
		case "internal/domain/admission", "internal/domain/admission/issuer", "internal/domain/admission/quota",
			"internal/domain/admission/stock", "internal/domain/admission/token", "internal/domain/endpoint/funding",
			"internal/domain/endpoint/session", "internal/domain/hosting", "internal/domain/network/epoch",
			"internal/domain/network/state", "internal/platform/processguard", "internal/domain/reachability",
			"internal/domain/route/introduction", "tests/epochfixture/network":
			return importAllowed
		}
	}
	if localImport && source == "internal/domain/node/runtime/operation_process_native_linux_test.go" {
		switch local {
		case "internal/domain/admission", "internal/domain/admission/issuance", "internal/domain/admission/issuer",
			"internal/domain/admission/quota", "internal/domain/admission/spending", "internal/domain/endpoint/funding",
			"internal/domain/hosting", "internal/domain/network", "internal/domain/network/state",
			"internal/domain/node/runtime", "internal/domain/route/role":
			return importAllowed
		}
	}
	if localImport && source == "internal/domain/node/runtime/authority_process_native_linux_test.go" {
		switch local {
		case "internal/domain/admission/issuance", "internal/domain/admission/issuer", "internal/domain/admission/spending", "internal/domain/hosting",
			"internal/domain/network", "internal/domain/network/duty", "internal/domain/network/state", "internal/domain/node/runtime", "internal/domain/reachability",
			"internal/domain/route/introduction", "internal/domain/route/role", "internal/domain/route/transport", "tests/epochfixture/network":
			return importAllowed
		}
	}
	if localImport {
		switch source {
		case "internal/domain/node/receive/post_spend_refusal_linux_test.go":
			switch local {
			case "internal/domain/admission", "internal/domain/admission/receiving", "internal/domain/endpoint/funding", "internal/domain/endpoint/session",
				"internal/domain/hosting", "internal/domain/route/receiver", "internal/domain/route/role", "internal/domain/route/selection":
				return importAllowed
			}
		case "internal/domain/node/receive/receiver_fixture_linux_test.go":
			switch local {
			case "internal/domain/admission", "internal/domain/admission/issuer", "internal/domain/admission/quota", "internal/domain/admission/stock", "internal/domain/admission/token",
				"internal/domain/endpoint/funding", "internal/domain/network/state", "internal/domain/route/transport", "tests/epochfixture/network":
				return importAllowed
			}
		case "internal/domain/node/receive/join_fixture_linux_test.go":
			switch local {
			case "internal/domain/route/receiver", "tests/e2e/node/processfixture", "internal/domain/admission", "internal/domain/admission/allocation",
				"internal/domain/admission/issuer", "internal/domain/admission/quota", "internal/domain/admission/receiving", "internal/domain/admission/stock",
				"internal/domain/admission/token", "internal/domain/hosting", "internal/domain/endpoint/funding", "internal/domain/network",
				"tests/epochfixture/network", "tests/networkfixture", "internal/domain/route/role", "internal/domain/route/transport":
				return importAllowed
			}
		case "internal/domain/node/receive/responder_lifetime_linux_test.go":
			switch local {
			case "internal/domain/route/join", "internal/domain/route/prefix", "internal/domain/route/transport", "internal/domain/admission",
				"internal/domain/admission/allocation", "internal/domain/admission/issuer", "internal/domain/admission/quota", "internal/domain/admission/stock",
				"internal/domain/admission/token", "internal/domain/hosting", "internal/domain/route/ardp", "internal/domain/route/selection":
				return importAllowed
			}
		case "internal/domain/node/receive/responder_startup_linux_test.go":
			switch local {
			case "internal/domain/admission", "internal/domain/admission/stock", "internal/domain/endpoint/funding", "internal/domain/hosting",
				"internal/domain/route/prefix", "internal/domain/route/selection", "internal/domain/route/transport", "tests/e2e/node/processfixture":
				return importAllowed
			}
		case "internal/domain/node/receive/source_funding_linux_test.go":
			switch local {
			case "internal/domain/admission", "internal/domain/admission/stock", "internal/domain/admission/token", "internal/domain/endpoint/funding",
				"internal/domain/hosting", "internal/domain/route/ardp", "internal/domain/route/prefix", "internal/domain/route/selection",
				"internal/domain/route/transport", "tests/e2e/node/processfixture":
				return importAllowed
			}
		case "internal/domain/node/receive/issuer_control_linux_test.go":
			switch local {
			case "internal/domain/endpoint/funding", "internal/domain/endpoint/session", "internal/domain/admission", "internal/domain/admission/issuer",
				"internal/domain/admission/quota", "internal/domain/admission/receiving", "internal/domain/admission/stock", "internal/domain/hosting",
				"internal/domain/network", "tests/epochfixture/network", "tests/networkfixture", "internal/domain/route/receiver",
				"internal/domain/route/role", "internal/domain/route/transport", "tests/e2e/node/processfixture":
				return importAllowed
			}
		case "internal/domain/node/receive/bootstrap_capacity_linux_test.go":
			switch local {
			case "internal/domain/admission", "internal/domain/admission/receiving", "internal/domain/admission/stock", "internal/domain/hosting",
				"internal/domain/route/ardp", "internal/domain/route/channel", "internal/domain/route/receiver", "internal/domain/route/role",
				"internal/domain/route/transport", "internal/domain/route/transport/tls":
				return importAllowed
			}
		case "internal/domain/node/receive/descriptor_retirement_linux_test.go":
			switch local {
			case "internal/domain/admission", "internal/domain/admission/issuer", "internal/domain/admission/quota", "internal/domain/admission/receiving",
				"internal/domain/admission/stock", "internal/domain/admission/token", "internal/domain/endpoint/funding", "internal/domain/endpoint/session", "internal/domain/hosting",
				"internal/domain/network", "internal/domain/reachability", "internal/domain/route/channel", "internal/domain/route/receiver", "internal/domain/route/role",
				"internal/domain/route/transport", "tests/epochfixture/network":
				return importAllowed
			}
		case "internal/domain/node/receive/introduction_submission_linux_test.go":
			switch local {
			case "internal/domain/admission", "internal/domain/admission/receiving", "internal/domain/endpoint/funding", "internal/domain/endpoint/session",
				"internal/domain/hosting", "internal/domain/route/capsule", "internal/domain/route/introduction", "internal/domain/route/prefix",
				"internal/domain/route/receiver", "internal/domain/route/role", "internal/domain/route/selection":
				return importAllowed
			}
		case "internal/domain/node/receive/introduction_registration_linux_test.go":
			switch local {
			case "internal/domain/admission", "internal/domain/admission/receiving", "internal/domain/endpoint/funding", "internal/domain/endpoint/session",
				"internal/domain/hosting", "internal/domain/route/ardp", "internal/domain/route/channel", "internal/domain/route/introduction", "internal/domain/route/receiver", "internal/domain/route/role":
				return importAllowed
			}
		case "internal/domain/node/receive/publication_registration_linux_test.go":
			switch local {
			case "internal/domain/publication", "internal/domain/publication/durable", "internal/domain/publication/instance", "internal/domain/reachability",
				"internal/domain/route/capsule", "internal/domain/route/introduction":
				return importAllowed
			}
		case "internal/domain/node/receive/forwarding_refill_linux_test.go":
			switch local {
			case "internal/domain/admission", "internal/domain/admission/receiving", "internal/domain/admission/stock", "internal/domain/hosting", "internal/domain/network",
				"internal/domain/route/ardp", "internal/domain/route/channel", "internal/domain/route/receiver", "internal/domain/route/role", "internal/domain/route/transport",
				"internal/domain/route/transport/tls":
				return importAllowed
			}
		}
	}
	if localImport && source == "internal/domain/node/runtime/forwarding_refill_native_linux_test.go" {
		switch local {
		case "internal/domain/admission", "internal/domain/admission/issuer", "internal/domain/admission/quota", "internal/domain/admission/receiving",
			"internal/domain/admission/stock", "internal/domain/admission/token", "internal/domain/endpoint/funding", "internal/domain/hosting",
			"internal/domain/network/state", "internal/platform/processguard", "internal/domain/route/ardp", "internal/domain/route/transport",
			"internal/domain/route/transport/tls", "tests/e2e/node/processfixture", "tests/epochfixture/network":
			return importAllowed
		default:
			return importDenied
		}
	}
	if source == "tests/e2e/admissioncli/issuance_results_unix_test.go" && dependency == "github.com/cloudflare/circl/blindsign/blindrsa" {
		return importAllowed
	}
	if source == "tests/e2e/admissioncli/issuance_telemetry_unix_test.go" || source == "tests/e2e/admissioncli/admission_unix_test.go" {
		switch dependency {
		case "go.opentelemetry.io/proto/otlp/collector/trace/v1", "go.opentelemetry.io/proto/otlp/collector/metrics/v1", "google.golang.org/protobuf/proto":
			return importAllowed
		}
	}
	if localImport {
		switch source {
		case "tests/e2e/node/offline_admission_fixture_unix.go", "tests/e2e/admissioncli/issuance_unix_test.go", "tests/e2e/admissioncli/issuance_commands_unix_test.go":
			return explicitImportResult(local == "internal/domain/admission" || local == "internal/domain/admission/quota" || local == "internal/domain/admission/issuerprofile" || local == "internal/domain/admission/issuance")
		case "tests/e2e/admissioncli/admission_fresh_roots_unix_test.go":
			return explicitImportResult(local == "tests/e2e/node" || local == "internal/domain/admission" || local == "internal/domain/admission/quota" || local == "internal/domain/admission/issuance")
		case "tests/e2e/admissioncli/admission_unix_test.go":
			return explicitImportResult(local == "tests/e2e/node" || local == "internal/domain/admission/quota")
		case "tests/e2e/admissioncli/issuance_results_unix_test.go":
			return explicitImportResult(local == "tests/e2e/node" || local == "internal/domain/admission/quota" || local == "internal/domain/admission/issuerprofile" || local == "internal/domain/admission/issuance")
		case "tests/e2e/admissioncli/issuer_profile_unix_test.go":
			return explicitImportResult(local == "internal/domain/admission/issuerprofile/signing")
		}
	}
	if localImport && local == "tests/e2e/node" {
		switch source {
		case "internal/domain/endpoint/route_fixture_linux_test.go":
			return importAllowed
		case "internal/domain/endpoint/connection_installed_linux_test.go",
			"internal/domain/endpoint/publication_ack_installed_linux_test.go",
			"internal/domain/endpoint/route_issuer_fixture_linux_test.go":
			return importAllowed
		case "internal/domain/endpoint/endpoint_journey_native_linux_test.go",
			"internal/domain/endpoint/execution_installed_linux_test.go",
			"internal/domain/endpoint/network_admission_fixture_linux_test.go",
			"internal/domain/endpoint/publication_installed_linux_test.go":
			return importAllowed
		default:
			return importDenied
		}
	}
	if localImport {
		switch source {
		case "tests/e2e/node/network_inspect_test.go":
			return explicitImportResult(local == "internal/domain/network/state")
		case "tests/e2e/node/source_refresh_linux_test.go":
			return explicitImportResult(local == "internal/domain/endpoint" || local == "internal/domain/network/state" || local == "tests/epochfixture/network" || local == "tests/e2e/source")
		case "tests/e2e/source/signing_workspace_linux.go":
			return explicitImportResult(local == "internal/platform/artifacts")
		}
	}
	// These test drivers use independent TLS or root-controlled fixture inputs.
	if localImport && source == "tests/e2e/source/initial_keys_linux.go" {
		return explicitImportResult(local == "internal/platform/artifacts")
	}
	if localImport && local == "tests/e2e/source" {
		switch source {
		case "internal/domain/endpoint/source_trust_native_linux_test.go", "tests/e2e/endpointcli/endpoint_construction_linux_test.go":
			return importAllowed
		default:
			return importDenied
		}
	}
	return importUnspecified
}
