package architecture

// Exact imports of Introduction initial admission and stopped installation.
// No runtime/Route consumer or role-wide permission is introduced.
func introductionInitialImportAllowed(source, dependency string) bool {
	switch source {
	case "cmd/ardents-control/introduction_hosting_linux.go":
		return dependency == modulePath+"/internal/successor/hosting" || dependency == modulePath+"/internal/successor/installation/introduction"
	case "internal/successor/installation/introduction/hosting_prepare_linux.go":
		return dependency == modulePath+"/internal/successor/hosting" || dependency == modulePath+"/internal/successor/installation/cgroup" || dependency == modulePath+"/internal/successor/installation/systemd"
	case "internal/successor/installation/introduction/hosting_native_linux_test.go", "internal/successor/installation/introduction/hosting_history_linux_test.go":
		return dependency == modulePath+"/internal/successor/hosting"

	case "internal/successor/installation/systemd/introduction_stopped_linux.go":
		return dependency == "github.com/godbus/dbus/v5"
	case "cmd/ardents-control/introduction_initial_stopped_native_linux_test.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/introduction":
			return true
		}
	case "cmd/ardents-control/introduction_release_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure", "github.com/dianabuilds/ardents-network/internal/successor/release", "github.com/sigstore/sigstore/pkg/signature", "github.com/theupdateframework/go-tuf/v2/metadata":
			return true
		}
	case "cmd/ardents-control/introduction_release_native_linux_test.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/introduction", "github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure", "github.com/theupdateframework/go-tuf/v2/metadata":
			return true
		}
	case "cmd/ardents-control/introduction_release_plan_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/introduction", "github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure":
			return true
		}
	case "cmd/ardents-node/introduction_initial_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/enrollment", "github.com/dianabuilds/ardents-network/internal/successor/installation/introduction", "github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure", "github.com/dianabuilds/ardents-network/internal/successor/release":
			return true
		}
	case "internal/successor/installation/introduction/initial_records_linux_test.go", "internal/successor/installation/introduction/binding_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure":
			return true
		}
	case "internal/successor/installation/introduction/completed_initial_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure", "github.com/dianabuilds/ardents-network/internal/successor/release":
			return true
		}
	case "internal/successor/installation/introduction/generation_bytes_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/enrollment", "github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure":
			return true
		}
	case "internal/successor/installation/introduction/initial_birth_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/directory", "github.com/dianabuilds/ardents-network/internal/successor/installation/fixedfile":
			return true
		}
	case "internal/successor/installation/introduction/initial_install_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure", "github.com/dianabuilds/ardents-network/internal/successor/installation/systemd", "github.com/dianabuilds/ardents-network/internal/successor/release":
			return true
		}
	case "internal/successor/installation/introduction/inputs_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/request":
			return true
		}
	case "internal/successor/installation/introduction/loaded_policy_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/systemd":
			return true
		}
	case "internal/successor/installation/introduction/plan.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/hosting":
			return true
		}
	case "internal/successor/installation/introduction/preflight_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/systemd":
			return true
		}
	case "internal/successor/installation/infrastructure/introduction_admission.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/enrollment", "github.com/dianabuilds/ardents-network/internal/successor/release":
			return true
		}
	}
	return false
}
