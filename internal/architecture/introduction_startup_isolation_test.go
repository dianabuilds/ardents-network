package architecture

// Exact first-start consumers. No retained-history admission, sibling role,
// Route root/receiver, legacy bridge or additional canonical test is permitted.
func introductionStartupImportAllowed(source, dependency string) bool {
	switch source {
	case "cmd/ardents-control/introduction_artifact_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/enrollment", "github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure", "github.com/dianabuilds/ardents-network/internal/successor/installation/introduction":
			return true
		}
	case "cmd/ardents-node/introduction_initial_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/enrollment", "github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure", "github.com/dianabuilds/ardents-network/internal/successor/installation/introduction", "github.com/dianabuilds/ardents-network/internal/successor/release":
			return true
		}
	case "cmd/ardents-node/introduction_preflight_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/introduction":
			return true
		}
	case "internal/successor/installation/infrastructure/introduction_admission.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/enrollment", "github.com/dianabuilds/ardents-network/internal/successor/release":
			return true
		}
	case "internal/successor/installation/introduction/completed_initial_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure", "github.com/dianabuilds/ardents-network/internal/successor/release":
			return true
		}
	case "internal/successor/installation/introduction/loaded_policy_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/systemd":
			return true
		}
	case "cmd/ardents-control/introduction_cohort_native_linux_test.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/network/epoch", "github.com/dianabuilds/ardents-network/internal/successor/network/source":
			return true
		}
	case "cmd/ardents-control/introduction_epoch_plan_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/network/epoch":
			return true
		}
	case "cmd/ardents-control/introduction_epoch_signing_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/network/epoch", "github.com/dianabuilds/ardents-network/internal/successor/network/source":
			return true
		}
	case "cmd/ardents-control/introduction_node_certificate_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/network/epoch":
			return true
		}
	case "cmd/ardents-control/introduction_node_provision_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/network/epoch":
			return true
		}
	case "cmd/ardents-control/introduction_node_record_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/network/epoch":
			return true
		}
	case "cmd/ardents-control/introduction_profile_plan_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/admission/issuerprofile", "github.com/dianabuilds/ardents-network/internal/successor/network", "github.com/dianabuilds/ardents-network/internal/successor/network/closedprofile", "github.com/dianabuilds/ardents-network/internal/successor/network/epoch", "github.com/dianabuilds/ardents-network/internal/successor/network/source":
			return true
		}
	case "cmd/ardents-control/introduction_profile_signing_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/network/closedprofile":
			return true
		}
	case "cmd/ardents-control/introduction_start_native_linux_test.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/introduction":
			return true
		}
	case "cmd/ardents-control/introduction_state_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/introduction", "github.com/dianabuilds/ardents-network/internal/successor/network/source", "github.com/dianabuilds/ardents-network/internal/successor/network/state":
			return true
		}
	case "cmd/ardents-control/introduction_token_material_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/admission/issuance":
			return true
		}
	case "cmd/ardents-node/introduction_installed_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/introduction", "github.com/dianabuilds/ardents-network/internal/successor/network", "github.com/dianabuilds/ardents-network/internal/successor/network/state", "github.com/dianabuilds/ardents-network/internal/successor/node", "github.com/dianabuilds/ardents-network/internal/successor/route/role":
			return true
		}
	case "internal/successor/installation/introduction/start_barrier_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/completion":
			return true
		}
	case "internal/successor/installation/introduction/start_initial_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/cgroup", "github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure", "github.com/dianabuilds/ardents-network/internal/successor/installation/process", "github.com/dianabuilds/ardents-network/internal/successor/installation/systemd", "github.com/dianabuilds/ardents-network/internal/successor/release":
			return true
		}
	case "internal/successor/installation/introduction/startup_admission_linux_test.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure":
			return true
		}
	case "internal/successor/installation/introduction/startup_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/cgroup", "github.com/dianabuilds/ardents-network/internal/successor/installation/completion", "github.com/dianabuilds/ardents-network/internal/successor/installation/process", "github.com/dianabuilds/ardents-network/internal/successor/installation/systemd":
			return true
		}
	case "internal/successor/installation/systemd/introduction_control_linux.go":
		switch dependency {
		case "github.com/godbus/dbus/v5":
			return true
		}
	case "internal/successor/installation/systemd/introduction_manager_linux.go":
		switch dependency {
		case "github.com/godbus/dbus/v5":
			return true
		}
	}
	return false
}
