package architecture

// A receiving Store or the independent signed fixture has its existing exact callers.
func reachabilityImport(source, dependency string) importPermission {
	switch dependency {
	case modulePath + "/internal/domain/reachability/store":
		switch source {
		case "internal/domain/endpoint/connection_installed_linux_test.go", "internal/domain/endpoint/installed_service_linux_test.go",
			"internal/domain/endpoint/publication_ack_installed_linux_test.go", "internal/domain/endpoint/publication_installed_linux_test.go",
			"internal/domain/network/state/descriptor_repair_linux.go", "internal/domain/node/receive/command_linux.go",
			"internal/domain/node/receive/descriptor_retirement_linux_test.go", "internal/domain/node/runtime/authority_process_native_linux_test.go",
			"internal/domain/node/runtime/descriptor_unix.go", "internal/domain/node/runtime/operation_workload_native_linux_test.go",
			"internal/domain/route/receiver/receiver_unix.go", "tests/e2e/endpoint/connection_reopen_linux.go",
			"tests/e2e/endpoint/service_refresh_linux.go", "tests/e2e/node/descriptor_exchange_native_linux_test.go",
			"tests/e2e/nodeinstalled/descriptor_platform_native_linux_test.go", "tests/e2e/node/retained_lifecycle_native_linux_test.go",
			"tests/e2e/routeprocess/route_descriptor_linux_test.go",
			"internal/domain/reachability/store/persistence_crash_linux_amd64_test.go",
			"internal/domain/reachability/store/inspection_native_linux_test.go",
			"internal/domain/reachability/store/lifetime_native_test.go",
			"internal/domain/reachability/store/publication_native_test.go",
			"internal/domain/reachability/store/repair_native_test.go",
			"internal/domain/reachability/store/retained_open_native_test.go",
			"internal/domain/reachability/store/partial_write_unix_test.go":
			return importAllowed
		default:
			return importDenied
		}
	case modulePath + "/tests/descriptorfixture":
		switch source {
		case "internal/domain/reachability/descriptor_draft_test.go", "internal/domain/reachability/descriptor_test.go",
			"internal/domain/reachability/lookup_history_test.go", "internal/domain/reachability/store/persistence_crash_linux_amd64_test.go",
			"internal/domain/reachability/store/inspection_native_linux_test.go", "internal/domain/reachability/store/lifetime_native_test.go",
			"internal/domain/reachability/store/publication_native_test.go", "internal/domain/reachability/store/repair_native_test.go",
			"internal/domain/reachability/store/retained_open_native_test.go", "internal/domain/reachability/store/partial_write_unix_test.go",
			"tests/e2e/node/processfixture/descriptor_input_linux.go":
			return importAllowed
		default:
			return importDenied
		}
	default:
		return importUnspecified
	}
}
