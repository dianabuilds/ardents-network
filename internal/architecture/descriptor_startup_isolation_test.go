package architecture

// Descriptor keeps exact imports for Release admission and the read-only
// platform manager after retiring the transitional startup orchestration
// (ADR-0125).
func descriptorStartupImportAllowed(source, dependency string) bool {
	switch source {
	case "internal/release/infrastructure/descriptor_admission.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/domain/enrollment", "github.com/dianabuilds/ardents-network/internal/release":
			return true
		}
	case "internal/platform/systemd/descriptor_manager_linux.go":
		switch dependency {
		case "github.com/godbus/dbus/v5":
			return true
		}
	case "internal/platform/systemd/manager_reference_linux.go":
		switch dependency {
		case "github.com/godbus/dbus/v5":
			return true
		}
	}
	return false
}
