package architecture

import "strings"

// Admission and Custody commands and their dedicated signed test inputs use closed permissions.
func offlineAuthorityImport(source, dependency, local string, localImport bool) importPermission {
	if dependency == modulePath+"/internal/domain/admission/operator" {
		return explicitImportResult(source == "cmd/ardents-next/admission.go" || source == "cmd/ardents-next/issuance.go" || source == "cmd/ardents-next/command.go")
	}
	if strings.HasPrefix(source, "internal/domain/custody/command/") {
		if dependency == modulePath+"/internal/domain/custody/delivery" {
			switch source {
			case "internal/domain/custody/command/admission_authority.go", "internal/domain/custody/command/service_authority.go",
				"internal/domain/custody/command/command_test.go", "internal/domain/custody/command/private_permission_windows_test.go":
				return importAllowed
			default:
				return importDenied
			}
		}
		// The package map also checks the package-wide import set.
		if dependency == modulePath+"/internal/domain/custody" {
			switch source {
			case "internal/domain/custody/command/command.go", "internal/domain/custody/command/terminal_secret.go",
				"internal/domain/custody/command/command_test.go", "internal/domain/custody/command/service_authority.go",
				"internal/domain/custody/command/admission_authority.go", "internal/domain/custody/command/verify_record.go",
				"internal/domain/custody/command/recovery_bundle.go", "internal/domain/custody/command/purge_record.go",
				"internal/domain/custody/command/admission_permission_client_linux_test.go":
				return importAllowed
			}
		}
		if source == "internal/domain/custody/command/command_test.go" && dependency == modulePath+"/internal/domain/publication" ||
			source == "internal/domain/custody/command/admission_permission_client_linux_test.go" && dependency == modulePath+"/internal/domain/admission" {
			return importAllowed
		}
	}
	if source == "cmd/ardents-custody/main.go" && dependency == modulePath+"/internal/domain/custody/command" {
		return importAllowed
	}
	switch source {
	case "internal/domain/custody/command/terminal_secret.go":
		if dependency == "golang.org/x/term" {
			return importAllowed
		}
	case "internal/domain/custody/command/terminal_line_linux.go":
		if dependency == "golang.org/x/sys/unix" || dependency == "golang.org/x/term" {
			return importAllowed
		}
	case "internal/domain/custody/command/terminal_line_windows.go":
		if dependency == "golang.org/x/sys/windows" {
			return importAllowed
		}
	case "internal/domain/custody/command/terminal_secret_linux_test.go":
		if dependency == "github.com/aymanbagabas/go-pty" {
			return importAllowed
		}
	}
	return importUnspecified
}
