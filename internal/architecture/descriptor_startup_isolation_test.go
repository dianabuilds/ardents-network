package architecture

// Exact fixed Descriptor startup/offline preparation consumers only. No role selector,
// Route root, legacy backend, callback bridge or sibling live Installation owner.
func descriptorStartupImportAllowed(source, dependency string) bool {
	switch source {
	case "cmd/ardents-control/descriptor_artifact_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/enrollment", "github.com/dianabuilds/ardents-network/internal/successor/installation/descriptor", "github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure":
			return true
		}
	case "cmd/ardents-control/descriptor_cohort_native_linux_test.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/network/epoch", "github.com/dianabuilds/ardents-network/internal/successor/network/source":
			return true
		}
	case "cmd/ardents-control/descriptor_epoch_plan_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/network/epoch":
			return true
		}
	case "cmd/ardents-control/descriptor_epoch_signing_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/network/epoch", "github.com/dianabuilds/ardents-network/internal/successor/network/source":
			return true
		}
	case "cmd/ardents-control/descriptor_hosting_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/hosting", "github.com/dianabuilds/ardents-network/internal/successor/installation/descriptor":
			return true
		}
	case "cmd/ardents-control/descriptor_node_certificate_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/network/epoch":
			return true
		}
	case "cmd/ardents-control/descriptor_node_provision_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/network/epoch":
			return true
		}
	case "cmd/ardents-control/descriptor_node_record_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/network/epoch":
			return true
		}
	case "cmd/ardents-control/descriptor_profile_plan_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/admission/issuerprofile", "github.com/dianabuilds/ardents-network/internal/successor/network", "github.com/dianabuilds/ardents-network/internal/successor/network/closedprofile", "github.com/dianabuilds/ardents-network/internal/successor/network/epoch", "github.com/dianabuilds/ardents-network/internal/successor/network/source":
			return true
		}
	case "cmd/ardents-control/descriptor_profile_signing_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/network/closedprofile":
			return true
		}
	case "cmd/ardents-control/descriptor_start_native_linux_test.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/descriptor":
			return true
		}
	case "cmd/ardents-control/descriptor_state_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/descriptor", "github.com/dianabuilds/ardents-network/internal/successor/network/source", "github.com/dianabuilds/ardents-network/internal/successor/network/state":
			return true
		}
	case "cmd/ardents-control/descriptor_token_material_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/admission/issuance":
			return true
		}
	case "cmd/ardents-node/descriptor_installation_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/enrollment", "github.com/dianabuilds/ardents-network/internal/successor/installation/descriptor", "github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure", "github.com/dianabuilds/ardents-network/internal/successor/release":
			return true
		}
	case "cmd/ardents-node/descriptor_installed_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/descriptor", "github.com/dianabuilds/ardents-network/internal/successor/network", "github.com/dianabuilds/ardents-network/internal/successor/network/state", "github.com/dianabuilds/ardents-network/internal/successor/node", "github.com/dianabuilds/ardents-network/internal/successor/route/role":
			return true
		}
	case "internal/successor/installation/descriptor/completed_initial_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure", "github.com/dianabuilds/ardents-network/internal/successor/release":
			return true
		}
	case "internal/successor/installation/descriptor/hosting_prepare_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/hosting", "github.com/dianabuilds/ardents-network/internal/successor/installation/cgroup", "github.com/dianabuilds/ardents-network/internal/successor/installation/systemd":
			return true
		}
	case "internal/successor/installation/descriptor/loaded_policy_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/systemd":
			return true
		}
	case "internal/successor/installation/descriptor/start_barrier_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/completion":
			return true
		}
	case "internal/successor/installation/descriptor/start_initial_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/cgroup", "github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure", "github.com/dianabuilds/ardents-network/internal/successor/installation/process", "github.com/dianabuilds/ardents-network/internal/successor/installation/systemd", "github.com/dianabuilds/ardents-network/internal/successor/release":
			return true
		}
	case "internal/successor/installation/descriptor/startup_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/cgroup", "github.com/dianabuilds/ardents-network/internal/successor/installation/completion", "github.com/dianabuilds/ardents-network/internal/successor/installation/process", "github.com/dianabuilds/ardents-network/internal/successor/installation/systemd":
			return true
		}
	case "internal/successor/installation/infrastructure/descriptor_admission.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/enrollment", "github.com/dianabuilds/ardents-network/internal/successor/release":
			return true
		}
	case "internal/successor/installation/systemd/descriptor_control_linux.go":
		switch dependency {
		case "github.com/godbus/dbus/v5":
			return true
		}
	case "internal/successor/installation/systemd/descriptor_manager_linux.go":
		switch dependency {
		case "github.com/godbus/dbus/v5":
			return true
		}
	case "internal/successor/installation/systemd/descriptor_stopped_linux.go":
		switch dependency {
		case "github.com/godbus/dbus/v5":
			return true
		}
	case "internal/successor/installation/systemd/manager_reference_linux.go":
		switch dependency {
		case "github.com/godbus/dbus/v5":
			return true
		}
	}
	return false
}
