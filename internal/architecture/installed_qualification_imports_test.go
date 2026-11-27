package architecture

// Release and installed Endpoint/Node fixtures carry file-specific authority and input dependencies.
func installedQualificationImport(source, dependency, local string, localImport bool) importPermission {
	switch source {
	case "internal/domain/release/infrastructure/infrastructure_inventory_refusal_linux_test.go":
		if dependency == modulePath+"/tests/releasefixture" {
			return importAllowed
		}
	case "internal/domain/release/infrastructure/source_release_linux_test.go":
		if dependency == modulePath+"/tests/releasefixture" || dependency == "github.com/theupdateframework/go-tuf/v2/metadata" {
			return importAllowed
		}
	}
	if source == "internal/domain/release/infrastructure/platform_release_linux_test.go" {
		switch dependency {
		case modulePath + "/internal/domain/network/duty", modulePath + "/internal/domain/network/state",
			modulePath + "/internal/platform/systemd", modulePath + "/tests/epochfixture/network",
			"github.com/theupdateframework/go-tuf/v2/metadata":
			return importAllowed
		}
	}
	// Installed Endpoint tests have exact fixture edges and exercise the original composition.
	if localImport {
		switch source {
		case "internal/domain/endpoint/connection_accepted_request_installed_linux_test.go":
			switch local {
			case "internal/domain/admission/receiving", "internal/domain/connection", "internal/domain/execution/runtime", "internal/domain/execution/worker",
				"internal/domain/hosting", "internal/domain/route/ardp", "internal/domain/route/receiver", "internal/domain/route/transport",
				"tests/e2e/endpoint", "tests/e2e/node/processfixture":
				return importAllowed
			}
		case "internal/domain/endpoint/connection_authority_linux_test.go":
			switch local {
			case "internal/domain/route/transport":
				return importAllowed
			}
		case "internal/domain/endpoint/connection_installed_linux_test.go":
			switch local {
			case "internal/domain/admission", "internal/domain/admission/issuer", "internal/domain/admission/quota", "internal/domain/admission/stock",
				"internal/domain/admission/token", "internal/domain/connection", "internal/domain/endpoint/funding", "internal/domain/endpoint/publisher",
				"internal/domain/execution", "internal/domain/execution/runtime", "internal/domain/hosting", "internal/domain/network",
				"internal/domain/publication/runtime", "internal/domain/reachability", "internal/domain/route", "internal/domain/route/introduction",
				"internal/domain/route/prefix", "internal/domain/route/receiver", "internal/domain/route/selection", "internal/domain/route/transport",
				"tests/e2e/endpoint", "tests/e2e/node/processfixture":
				return importAllowed
			}
		case "internal/domain/endpoint/connection_response_loss_linux_test.go":
			switch local {
			case "internal/application/endpointstream", "internal/domain/connection", "internal/domain/endpoint/publisher", "internal/domain/execution/runtime",
				"internal/domain/network", "internal/domain/publication/runtime", "internal/domain/route/transport":
				return importAllowed
			}
		case "internal/domain/endpoint/document_command_installed_linux_test.go":
			switch local {
			case "internal/domain/admission", "internal/application/administration", "internal/application/endpointstream", "internal/application/document",
				"internal/domain/endpoint/funding", "internal/domain/endpoint/publisher", "internal/domain/endpoint/reader", "internal/domain/execution",
				"internal/domain/execution/runtime", "internal/domain/hosting", "internal/domain/reachability", "internal/domain/route/selection",
				"internal/domain/route/transport", "tests/e2e/endpoint":
				return importAllowed
			}
		case "internal/domain/endpoint/endpoint_journey_native_linux_test.go":
			switch local {
			case "internal/application/administration", "internal/domain/publication", "internal/domain/route/transport", "tests/e2e/endpoint":
				return importAllowed
			}
		case "internal/domain/endpoint/endpoint_latency_native_linux_test.go":
			switch local {
			case "internal/domain/route/transport", "tests/e2e/endpoint":
				return importAllowed
			}
		case "internal/domain/endpoint/endpoint_launch_fixture_linux_test.go":
			switch local {
			case "tests/e2e/endpoint":
				return importAllowed
			}
		case "internal/domain/endpoint/endpoint_replacement_worker_native_linux_test.go":
			switch local {
			case "internal/domain/route/transport", "tests/e2e/endpoint":
				return importAllowed
			}
		case "internal/domain/endpoint/execution_installed_linux_test.go":
			switch local {
			case "internal/domain/admission", "internal/domain/endpoint/funding", "internal/domain/endpoint/session", "internal/domain/hosting",
				"internal/domain/network/state", "internal/domain/route/transport", "tests/e2e/endpoint", "tests/e2e/node/processfixture",
				"tests/epochfixture/network", "tests/networkfixture":
				return importAllowed
			}
		case "internal/domain/endpoint/installed_service_linux_test.go":
			switch local {
			case "internal/domain/reachability", "internal/domain/route/transport", "tests/e2e/endpoint":
				return importAllowed
			}
		case "internal/domain/endpoint/network_admission_fixture_linux_test.go":
			switch local {
			case "internal/domain/admission", "internal/domain/admission/issuer", "internal/domain/admission/quota", "internal/domain/admission/receiving",
				"internal/domain/endpoint/funding",
				"internal/domain/network", "internal/domain/network/epoch", "internal/domain/network/state", "tests/e2e/node/processfixture",
				"tests/epochfixture/network", "tests/networkfixture":
				return importAllowed
			}
		case "internal/domain/endpoint/publication_ack_installed_linux_test.go":
			switch local {
			case "internal/domain/admission", "internal/domain/admission/stock", "internal/domain/endpoint/funding", "internal/domain/endpoint/publisher",
				"internal/domain/execution", "internal/domain/execution/runtime", "internal/domain/hosting", "internal/domain/network",
				"internal/domain/publication", "internal/domain/reachability", "internal/domain/route/introduction", "internal/domain/route/prefix",
				"internal/domain/route/transport", "tests/e2e/endpoint", "tests/e2e/node/processfixture":
				return importAllowed
			}
		case "internal/domain/endpoint/publication_installed_linux_test.go":
			switch local {
			case "internal/domain/admission", "internal/domain/admission/issuer", "internal/domain/admission/quota", "internal/domain/admission/receiving",
				"internal/domain/admission/stock", "internal/domain/admission/token", "internal/domain/endpoint/funding", "internal/domain/endpoint/publisher",
				"internal/domain/endpoint/session", "internal/domain/hosting", "internal/domain/network", "internal/domain/network/state",
				"internal/domain/publication", "internal/domain/publication/instance", "internal/domain/reachability", "internal/domain/route/ardp",
				"internal/domain/route/capsule", "internal/domain/route/introduction", "internal/domain/route/receiver", "internal/domain/route/role",
				"internal/domain/route/transport", "tests/e2e/node/processfixture", "tests/epochfixture/network", "tests/networkfixture":
				return importAllowed
			}
		case "internal/domain/endpoint/route_fixture_linux_test.go":
			return explicitImportResult(local == "internal/domain/route/transport" || local == "tests/e2e/node/processfixture")
		case "internal/domain/endpoint/installed_route_fixture_linux_test.go":
			switch local {
			case "internal/domain/admission", "internal/domain/admission/receiving", "internal/domain/admission/stock", "internal/domain/endpoint/session",
				"internal/domain/endpoint/funding", "internal/domain/hosting", "internal/domain/route/introduction", "tests/e2e/node/processfixture":
				return importAllowed
			}
		case "internal/domain/endpoint/route_issuer_fixture_linux_test.go":
			switch local {
			case "internal/domain/admission", "internal/domain/admission/issuer", "internal/domain/admission/quota", "internal/domain/admission/receiving",
				"internal/domain/hosting", "internal/domain/route/receiver", "internal/domain/route/role", "internal/domain/route/transport",
				"tests/e2e/node/processfixture":
				return importAllowed
			}
		}
	}
	// Installed workload drivers compose current Endpoint operations with their
	// own fixture authority. They have no direct Route package imports.
	if localImport {
		switch source {
		case "tests/e2e/nodeinstalled/join_workload_linux_test.go":
			return explicitImportResult(local == "internal/domain/admission" || local == "internal/domain/admission/stock" ||
				local == "internal/domain/endpoint/session" || local == "tests/e2e/node/processfixture")
		case "tests/e2e/nodeinstalled/route_workload_linux_test.go":
			switch local {
			case "internal/domain/admission", "internal/domain/admission/issuer", "internal/domain/endpoint/funding",
				"internal/domain/endpoint/session", "internal/domain/hosting", "internal/domain/network/state",
				"internal/domain/reachability", "internal/platform/processguard", "tests/e2e/node/processfixture",
				"tests/epochfixture/network", "tests/networkfixture":
				return importAllowed
			default:
				return importDenied
			}
		}
	}

	if localImport && (source == "tests/e2e/nodeinstalled/descriptor_platform_native_linux_test.go" || source == "tests/e2e/nodeinstalled/forwarding_platform_native_linux_test.go" || source == "tests/e2e/nodeinstalled/join_platform_native_linux_test.go") {
		switch local {
		case "internal/domain/admission", "internal/domain/hosting", "internal/domain/network/duty", "internal/domain/network/state", "internal/platform/systemd",
			"internal/platform/processguard", "internal/domain/reachability", "internal/domain/release/infrastructure", "internal/domain/release/infrastructure/launch",
			"tests/e2e/node/processfixture", "tests/epochfixture/network", "tests/networkfixture", "tests/releasefixture":
			return importAllowed
		}
	}
	if localImport && source == "tests/e2e/nodeinstalled/introduction_repair_platform_native_linux_test.go" {
		switch local {
		case "internal/domain/hosting", "internal/domain/network/duty", "internal/domain/network/state", "internal/platform/systemd",
			"internal/platform/processguard", "internal/domain/release/infrastructure", "internal/domain/release/infrastructure/launch",
			"tests/e2e/node/processfixture", "tests/epochfixture/network", "tests/networkfixture", "tests/releasefixture":
			return importAllowed
		default:
			return importDenied
		}
	}
	return importUnspecified
}
