package architecture

import "strings"

// Signed Network, Admission, Release and process fixtures have exact consumers.
func networkFixtureImport(source, dependency, local string, localImport bool) importPermission {
	if (source == "internal/domain/endpoint/funding/intent_network_test.go" || source == "internal/domain/admission/receiving/registration_initialization_integration_test.go") && localImport && local == "tests/epochfixture/network" {
		return importAllowed
	}
	if source == "internal/domain/admission/receiving/registration_initialization_integration_test.go" && localImport && local == "internal/domain/network/state" {
		return importAllowed
	}
	if localImport && source == "internal/domain/endpoint/session/issuance_recipient_test.go" {
		switch local {
		case "internal/domain/admission", "internal/domain/admission/issuer", "internal/domain/admission/quota", "internal/domain/admission/stock",
			"internal/domain/admission/token", "internal/domain/endpoint/funding", "internal/domain/endpoint/session", "tests/e2e/node/processfixture", "tests/networkfixture":
			return importAllowed
		default:
			return importDenied
		}
	}
	if localImport && source == "internal/domain/admission/receiving/network_lifetime_linux_test.go" {
		return explicitImportResult(local == "internal/domain/admission" || local == "internal/domain/admission/receiving" ||
			local == "internal/domain/admission/spending" || local == "tests/networkfixture")
	}
	if localImport && source == "internal/domain/admission/receiving/registration_allowance_linux_test.go" {
		return explicitImportResult(local == "internal/domain/admission" || local == "internal/domain/admission/receiving" ||
			local == "tests/epochfixture/network" || local == "tests/networkfixture")
	}
	if localImport && (source == "internal/domain/admission/receiving/network_reservation_linux_test.go" || source == "internal/domain/admission/receiving/network_joined_work_linux_test.go") {
		switch local {
		case "internal/domain/admission", "internal/domain/admission/allocation", "internal/domain/admission/issuer", "internal/domain/admission/quota",
			"internal/domain/admission/receiving", "internal/domain/admission/spending", "internal/domain/admission/token", "internal/domain/hosting", "internal/domain/network/state",
			"tests/epochfixture/network", "tests/networkfixture":
			return importAllowed
		default:
			return importDenied
		}
	}
	if strings.HasPrefix(source, "tests/networkfixture/") && localImport {
		switch source {
		case "tests/networkfixture/genesis.go":
			return explicitImportResult(local == "internal/domain/admission/issuance" || local == "internal/domain/admission/issuer" || local == "internal/domain/admission/quota" || local == "internal/domain/network/epoch" || local == "tests/epochfixture/network")
		case "tests/networkfixture/work_authority.go", "tests/networkfixture/loopback_test.go":
			return explicitImportResult(local == "internal/domain/admission")
		case "tests/networkfixture/signed_admission.go":
			return explicitImportResult(local == "internal/domain/admission" || local == "internal/domain/admission/issuer" || local == "internal/domain/network" ||
				local == "internal/domain/network/state" || local == "tests/epochfixture/network")
		case "tests/networkfixture/issuer_conflict_test.go":
			switch local {
			case "internal/domain/admission", "internal/domain/admission/allocation", "internal/domain/admission/issuer", "internal/domain/admission/quota", "internal/domain/admission/token", "internal/domain/network/state", "tests/epochfixture/network":
				return importAllowed
			default:
				return importDenied
			}
		case "tests/networkfixture/state.go", "tests/networkfixture/independent_state.go", "tests/networkfixture/genesis_test.go", "tests/networkfixture/isolated_profile_test.go":
			return explicitImportResult(local == "internal/domain/network/state" || local == "tests/epochfixture/network")
		default:
			return importDenied
		}
	}
	// Signed Network fixture inputs are available only to these test owners.
	if localImport && local == "tests/networkfixture" {
		switch source {
		case "tests/e2e/node/forwarding_exchange_native_linux_test.go":
			return importAllowed
		case "tests/e2e/node/hosting_work_fixture_linux_test.go", "tests/e2e/node/hosting_work_lifecycle_linux_test.go":
			return importAllowed
		case
			"internal/domain/admission/receiving/registration_initialization_integration_test.go",
			"internal/domain/endpoint/execution_installed_linux_test.go",
			"internal/domain/endpoint/network_admission_fixture_linux_test.go",
			"internal/domain/endpoint/publication_installed_linux_test.go",
			"internal/domain/endpoint/funding/intent_network_test.go",
			"internal/domain/network/source/command/state_lifetime_linux_test.go",
			"internal/domain/node/runtime/authority_process_native_linux_test.go",
			"internal/domain/node/runtime/introduction_exchange_native_linux_test.go",
			"internal/domain/node/runtime/operation_workload_native_linux_test.go",
			"internal/domain/node/runtime/forwarding_refill_native_linux_test.go",
			"internal/domain/node/receive/descriptor_retirement_linux_test.go",
			"internal/domain/node/receive/join_fixture_linux_test.go",
			"internal/domain/node/receive/issuer_control_linux_test.go",
			"internal/domain/node/receive/receiver_fixture_linux_test.go",
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
			return importAllowed
		default:
			return importDenied
		}
	}
	if source == "tests/releasefixture/infrastructure_linux.go" && (dependency == modulePath+"/internal/domain/enrollment" || dependency == modulePath+"/internal/domain/release" || dependency == "github.com/theupdateframework/go-tuf/v2/metadata") {
		return importAllowed
	}
	if (source == "internal/domain/release/infrastructure/forwarding_artifact_linux_test.go" || source == "internal/domain/release/infrastructure/authorization_lifetime_linux_test.go") && dependency == modulePath+"/internal/domain/release/infrastructure" {
		return importAllowed
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
			return importAllowed
		default:
			return importDenied
		}
	}
	if localImport && retiredEndpointSource(source) {
		return importDenied
	}
	if source == "tests/e2e/node/network_preparation_chain_native_linux_test.go" && localImport {
		return explicitImportResult(local == "internal/domain/network/closedprofile" || local == "internal/domain/network/epoch" || local == "internal/domain/network/state" || local == "tests/epochfixture/network")
	}
	if (source == "tests/e2e/node/network_signing_chain_native_linux_test.go" || source == "tests/e2e/node/network_initial_keys_chain_native_linux_test.go" || source == "tests/e2e/node/network_signing_refusals_native_linux_test.go") && localImport {
		return explicitImportResult(local == "tests/e2e/source")
	}
	if source == "tests/e2e/node/infrastructure_preparation_native_linux_test.go" && localImport {
		return explicitImportResult(local == "tests/e2e/node" || local == "tests/e2e/source")
	}
	if source == "internal/domain/release/infrastructure/preparation_native_linux_test.go" && localImport {
		return explicitImportResult(local == "internal/platform/artifacts")
	}
	if localImport && local == "tests/e2e/node/processfixture" {
		switch source {
		case "tests/e2e/node/admission_authority_test.go", "tests/e2e/node/forwarding_exchange_native_linux_test.go", "tests/e2e/node/admission_console_input_test.go",
			"tests/e2e/node/descriptor_exchange_native_linux_test.go", "tests/e2e/node/issuer_exchange_native_linux_test.go", "tests/e2e/node/network_commands_unix_test.go",
			"tests/e2e/node/admission_cycle_test.go", "tests/e2e/node/admission_receiver_test.go", "tests/e2e/node/admission_interrupt_unix_test.go":
			return importAllowed
		case "internal/domain/node/receive/issuer_control_linux_test.go", "internal/domain/node/receive/source_funding_linux_test.go":
			return importAllowed
		case "internal/domain/node/receive/responder_startup_linux_test.go", "internal/domain/node/receive/join_fixture_linux_test.go":
			return importAllowed
		case "internal/domain/node/receive/descriptor_retirement_linux_test.go":
			return importAllowed
		case "internal/domain/node/receive/publication_registration_linux_test.go", "tests/e2e/endpoint/service_instance_linux.go":
			return importAllowed
		case "tests/e2e/node/command_test.go", "tests/e2e/endpoint/command_test.go", "internal/domain/endpoint/connection_accepted_request_installed_linux_test.go":
			return importAllowed
		case "tests/e2e/endpoint/endpoint_history_linux_test.go", "tests/e2e/node/source_refresh_linux_test.go":
			return importAllowed
		case "tests/e2e/node/route_bootstrap_linux_test.go",
			"tests/e2e/node/route_receiver_process_linux_test.go", "tests/e2e/node/route_registration_linux_test.go",
			"tests/e2e/node/route_issuer_recovery_test.go",
			"tests/e2e/node/route_descriptor_linux_test.go", "tests/e2e/node/route_join_linux_test.go",
			"tests/e2e/node/route_hello_linux_test.go", "tests/e2e/node/route_quic_linux_test.go":
			return importAllowed
		case "internal/domain/network/source/command/state_lifetime_linux_test.go",
			"internal/domain/endpoint/connection_installed_linux_test.go",
			"internal/domain/endpoint/execution_installed_linux_test.go",
			"internal/domain/endpoint/network_admission_fixture_linux_test.go",
			"internal/domain/node/runtime/introduction_exchange_native_linux_test.go",
			"tests/e2e/node/join_exchange_native_linux_test.go",
			"internal/domain/node/runtime/operation_workload_native_linux_test.go",
			"internal/domain/endpoint/publication_ack_installed_linux_test.go",
			"internal/domain/endpoint/publication_installed_linux_test.go",
			"internal/domain/endpoint/route_issuer_fixture_linux_test.go",
			"internal/domain/endpoint/route_fixture_linux_test.go",
			"internal/domain/node/runtime/forwarding_refill_native_linux_test.go",
			"internal/domain/node/receive/receiver_fixture_linux_test.go",
			"internal/domain/node/receive/post_spend_refusal_linux_test.go", "internal/domain/node/receive/forwarding_refill_linux_test.go", "internal/domain/node/receive/introduction_registration_linux_test.go", "internal/domain/node/receive/introduction_submission_linux_test.go",
			"internal/domain/node/runtime/authority_process_native_linux_test.go",
			"internal/domain/node/runtime/operation_process_native_linux_test.go",
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
			return importAllowed
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
			return importAllowed
		default:
			return importDenied
		}
	}
	return importUnspecified
}
