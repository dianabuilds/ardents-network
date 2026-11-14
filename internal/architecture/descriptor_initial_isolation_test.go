package architecture

// Exact imports of Descriptor initial admission and stopped installation.
// No runtime/Route consumer or role-wide permission is introduced.
func descriptorInitialImportAllowed(source, dependency string) bool {
	switch source {
	case "internal/successor/installation/systemd/descriptor_stopped_linux.go":
		return dependency == "github.com/godbus/dbus/v5"
	case "cmd/ardents-control/descriptor_initial_stopped_native_linux_test.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/descriptor":
			return true
		}
	case "cmd/ardents-control/descriptor_release_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure", "github.com/dianabuilds/ardents-network/internal/successor/release", "github.com/sigstore/sigstore/pkg/signature", "github.com/theupdateframework/go-tuf/v2/metadata":
			return true
		}
	case "cmd/ardents-control/descriptor_release_native_linux_test.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/descriptor", "github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure", "github.com/theupdateframework/go-tuf/v2/metadata":
			return true
		}
	case "cmd/ardents-control/descriptor_release_plan_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/descriptor", "github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure":
			return true
		}
	case "cmd/ardents-node/descriptor_installation_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/enrollment", "github.com/dianabuilds/ardents-network/internal/successor/installation/descriptor", "github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure", "github.com/dianabuilds/ardents-network/internal/successor/release":
			return true
		}
	case "internal/successor/installation/descriptor/binding_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure":
			return true
		}
	case "internal/successor/installation/descriptor/completed_initial_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure", "github.com/dianabuilds/ardents-network/internal/successor/release":
			return true
		}
	case "internal/successor/installation/descriptor/generation_bytes_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/enrollment", "github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure":
			return true
		}
	case "internal/successor/installation/descriptor/initial_birth_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/directory", "github.com/dianabuilds/ardents-network/internal/successor/installation/fixedfile":
			return true
		}
	case "internal/successor/installation/descriptor/initial_install_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure", "github.com/dianabuilds/ardents-network/internal/successor/installation/systemd", "github.com/dianabuilds/ardents-network/internal/successor/release":
			return true
		}
	case "internal/successor/installation/descriptor/inputs_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/request":
			return true
		}
	case "internal/successor/installation/descriptor/loaded_policy_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/systemd":
			return true
		}
	case "internal/successor/installation/descriptor/plan_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/hosting":
			return true
		}
	case "internal/successor/installation/descriptor/preflight_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/systemd":
			return true
		}
	case "internal/successor/installation/infrastructure/descriptor_admission.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/enrollment", "github.com/dianabuilds/ardents-network/internal/successor/release":
			return true
		}
	}
	return false
}
