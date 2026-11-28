package architecture

import "strings"

// Original Node credentials and role commands use the declared owners in each direction.
func nodeAndSourceCommandImport(source, dependency, local string, localImport bool) importPermission {
	if localImport {
		switch source {
		case "internal/domain/node/runtime/credential_unix.go", "internal/domain/node/runtime/join_credential_unix.go":
			return explicitImportResult(local == "internal/durablefile" || local == "internal/pathcheck")
		}
		switch local {
		case "internal/domain/node", "internal/domain/node/internal/credential":
			return importDenied
		case "internal/domain/node/admission":
			switch source {
			case "internal/domain/node/runtime/lifetime_unix.go", "internal/domain/node/runtime/forwarding_unix.go",
				"internal/domain/node/runtime/issuer_unix.go", "internal/domain/node/receive/authority_linux.go", "internal/domain/node/receive/command_linux.go":
				return importAllowed
			default:
				return importDenied
			}
		case "internal/domain/node/receive":
			return explicitImportResult(source == "cmd/ardents-next/command.go")
		}
	}
	if strings.HasPrefix(source, "internal/domain/node/command/") && localImport {
		switch source {
		case "internal/domain/node/command/role_command.go":
			switch local {
			case "internal/domain/node/runtime", "internal/domain/node/plan", "internal/domain/admission/issuer", "internal/domain/admission/issuance",
				"internal/domain/network/state":
				return importAllowed
			}
			return importDenied
		case "internal/domain/node/command/process.go":
			return explicitImportResult(local == "internal/platform/processguard")
		case "internal/domain/node/command/input.go", "internal/domain/node/command/role_input.go":
			return explicitImportResult(local == "internal/platform/artifacts")
		case "internal/domain/node/command/report.go", "internal/domain/node/command/report_test.go",
			"internal/domain/node/command/role_report.go", "internal/domain/node/command/role_report_test.go":
			return explicitImportResult(local == "internal/domain/node/runtime")
		default:
			return importDenied
		}
	}
	if localImport && local == "internal/domain/node/command" {
		switch source {
		case "cmd/ardents-next/node.go", "cmd/ardents-next/node_roles.go":
			return importAllowed
		default:
			return importDenied
		}
	}
	if strings.HasPrefix(source, "internal/domain/network/source/command/") && localImport {
		switch source {
		case "internal/domain/network/source/command/prepare_linux.go":
			return explicitImportResult(local == "internal/domain/endpoint" || local == "internal/domain/network/source/preparation" || local == "internal/pathcheck" || local == "internal/platform/artifacts")
		case "internal/domain/network/source/command/inspect_linux.go":
			return explicitImportResult(local == "internal/domain/network/source/inspection" || local == "internal/platform/artifacts" || local == "internal/canonicaldigest")
		case "internal/domain/network/source/command/serve_linux.go":
			return explicitImportResult(local == "internal/domain/network/source/serving" || local == "internal/platform/artifacts")
		case "internal/domain/network/source/command/command_linux_test.go":
			return explicitImportResult(local == "internal/domain/network/source/serving")
		case "internal/domain/network/source/command/serving_output_linux_test.go":
			return explicitImportResult(local == "internal/domain/network/source/serving" || local == "internal/domain/network/state")
		case "internal/domain/network/source/command/state_lifetime_linux_test.go":
			return explicitImportResult(local == "internal/platform/artifacts" || local == "internal/domain/network/duty" ||
				local == "internal/domain/network/source" || local == "internal/domain/network/source/serving" ||
				local == "internal/domain/network/source/servingdeclaration" || local == "internal/domain/network/state" ||
				local == "tests/e2e/node" || local == "tests/e2e/source" || local == "tests/epochfixture/network")
		default:
			return importDenied
		}
	}
	if localImport && local == "internal/domain/network/source/command" {
		switch source {
		case "cmd/ardents-next/source_command.go":
			return importAllowed
		default:
			return importDenied
		}
	}
	return importUnspecified
}
