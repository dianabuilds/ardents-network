package architecture

// Descriptor's current permissions are limited to its retained Release
// admission, the signed offline fixtures, and the read-only native manager.
func descriptorRetainedImportAllowed(source, dependency string) bool {
	switch source {
	case "internal/domain/release/infrastructure/descriptor_admission.go":
		return dependency == modulePath+"/internal/domain/enrollment" || dependency == modulePath+"/internal/domain/release"
	case "internal/domain/release/infrastructure/signing/descriptor_release_native_linux_test.go":
		return dependency == modulePath+"/internal/domain/release/infrastructure" || dependency == "github.com/theupdateframework/go-tuf/v2/metadata"
	case "internal/domain/release/infrastructure/signing/descriptor_forward_release_native_linux_test.go":
		return dependency == modulePath+"/internal/domain/release/infrastructure"
	case "internal/platform/systemd/descriptor_manager_linux.go":
		return dependency == "github.com/godbus/dbus/v5"
	default:
		return false
	}
}

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
