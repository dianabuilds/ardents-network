package architecture

import "testing"

const managerObservationPackage = "internal/platform/systemd/observation"

func managerObservationConsumer(source string) bool {
	switch source {
	case "internal/domain/endpoint/launch/manager_binding_linux.go",
		"internal/domain/endpoint/launch/manager_binding_platform_test.go",
		"internal/domain/endpoint/unit/contract.go",
		"internal/domain/endpoint/unit/platform_completion_linux.go",
		"internal/domain/execution/worker/manager_linux.go",
		"internal/domain/execution/worker/service_linux.go",
		"internal/domain/network/state/node_duty_roles_linux.go",
		"internal/domain/network/state/node_duty_repair_linux.go",
		"internal/domain/network/state/node_duty_repair_linux_test.go",
		"internal/domain/network/state/source_repair_linux.go",
		"internal/domain/network/state/source_repair_linux_test.go",
		"internal/platform/systemd/bus_linux.go",
		"internal/platform/systemd/descriptor_manager_linux.go",
		"internal/platform/systemd/doc.go",
		"internal/platform/systemd/forwarding_manager_linux.go",
		"internal/platform/systemd/infrastructure_prestart_linux.go",
		"internal/platform/systemd/infrastructure_prestart_linux_test.go",
		"internal/platform/systemd/introduction_manager_linux.go",
		"internal/platform/systemd/join_manager_linux.go",
		"internal/platform/systemd/manager_properties_linux.go",
		"internal/platform/systemd/platform_linux.go",
		"internal/platform/systemd/source_manager_linux.go",
		"internal/platform/systemd/startup_observation_linux.go",
		"internal/platform/systemd/startup_observation_native_linux_test.go",
		"tests/e2e/endpoint/replacement_scopes_linux.go":
		return true
	default:
		return false
	}
}

func TestManagerObservationImportIsolation(t *testing.T) {
	const observation = modulePath + "/" + managerObservationPackage
	const manager = modulePath + "/internal/platform/systemd"
	for _, edge := range []struct {
		source, dependency string
		allow              bool
	}{
		{"internal/domain/endpoint/unit/contract.go", observation, true},
		{"internal/domain/endpoint/unit/contract.go", manager, false},
		{"internal/domain/execution/worker/service_linux.go", manager, false},
		{"internal/domain/network/state/node_duty_repair_linux.go", observation, true},
		{"internal/domain/network/state/node_duty_repair_linux.go", manager, false},
		{"internal/domain/network/state/source_repair_linux.go", manager, true},
		{"internal/domain/network/state/undeclared_linux.go", observation, false},
		{"internal/platform/systemd/manager_properties_linux.go", observation, true},
		{"internal/platform/systemd/manager_properties_linux.go", "github.com/godbus/dbus/v5", true},
		{"internal/platform/systemd/observation/properties.go", manager, false},
		{"internal/platform/systemd/observation/properties.go", "github.com/godbus/dbus/v5", false},
	} {
		if actual := successorImportAllowed(edge.source, edge.dependency); actual != edge.allow {
			t.Errorf("%s imports %s: %t, want %t", edge.source, edge.dependency, actual, edge.allow)
		}
	}
}
