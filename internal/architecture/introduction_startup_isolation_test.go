package architecture

// Exact start, retained restart, Stop and independent history consumers. No sibling role,
// Route root/receiver, legacy bridge or additional canonical test is permitted.
func introductionStartupImportAllowed(source, dependency string) bool {
	if (source == "internal/successor/installation/introduction/start_attempt_linux.go" || source == "internal/successor/installation/introduction/start_records_linux.go" || source == "internal/successor/installation/introduction/start_storage_linux.go" || source == "internal/successor/installation/introduction/retirement_storage_linux.go" || source == "internal/successor/installation/introduction/retirement_snapshot_linux.go" || source == "internal/successor/installation/introduction/retirement_records_linux_test.go") && dependency == modulePath+"/internal/successor/installation/journal" {
		return true
	}
	switch source {
	case "internal/successor/installation/introduction/stopped_inspections_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/admission/spending", "github.com/dianabuilds/ardents-network/internal/successor/hosting", "github.com/dianabuilds/ardents-network/internal/successor/network/state", "github.com/dianabuilds/ardents-network/internal/successor/route/introduction":
			return true
		}
	case "internal/successor/installation/introduction/restart_policy_linux_test.go":
		return dependency == "github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
	case "internal/successor/installation/introduction/restart_linux.go":
		return dependency == "github.com/dianabuilds/ardents-network/internal/successor/release"

	case "internal/successor/installation/introduction/retirement_policy_linux.go":
		return dependency == "github.com/dianabuilds/ardents-network/internal/successor/installation/completion" || dependency == "github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
	case "internal/successor/installation/introduction/stop_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/cgroup", "github.com/dianabuilds/ardents-network/internal/successor/installation/completion", "github.com/dianabuilds/ardents-network/internal/successor/installation/process", "github.com/dianabuilds/ardents-network/internal/successor/installation/systemd", "github.com/dianabuilds/ardents-network/internal/successor/release":
			return true
		}

	case "internal/successor/installation/introduction/installed_history_native_linux_test.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/introduction", "github.com/dianabuilds/ardents-network/internal/successor/release":
			return true
		}
	case "internal/successor/installation/introduction/start_snapshot_linux.go":
		return dependency == "github.com/dianabuilds/ardents-network/internal/successor/installation/completion"
	case "internal/successor/installation/introduction/started_history_linux.go":
		return dependency == "github.com/dianabuilds/ardents-network/internal/successor/release"
	case "internal/successor/installation/introduction/installed_cohort_native_linux_test.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/network/epoch", "github.com/dianabuilds/ardents-network/internal/successor/network/source":
			return true
		}
	case "internal/successor/installation/introduction/installed_operations_native_linux_test.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/route/ardp", "github.com/dianabuilds/ardents-network/internal/successor/route/capsule", "github.com/dianabuilds/ardents-network/internal/successor/route/introduction":
			return true
		}
	case "internal/successor/installation/introduction/installed_preparation_native_linux_test.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/admission", "github.com/dianabuilds/ardents-network/internal/successor/admission/stock", "github.com/dianabuilds/ardents-network/internal/successor/custody":
			return true
		}
	case "internal/successor/installation/introduction/installed_start_native_linux_test.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/introduction":
			return true
		}
	case "internal/successor/installation/introduction/installed_stock_native_linux_test.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/admission", "github.com/dianabuilds/ardents-network/internal/successor/admission/issuance", "github.com/dianabuilds/ardents-network/internal/successor/admission/issuer", "github.com/dianabuilds/ardents-network/internal/successor/admission/quota", "github.com/dianabuilds/ardents-network/internal/successor/admission/receiving", "github.com/dianabuilds/ardents-network/internal/successor/admission/stock", "github.com/dianabuilds/ardents-network/internal/successor/admission/token", "github.com/dianabuilds/ardents-network/internal/successor/custody", "github.com/dianabuilds/ardents-network/internal/successor/network", "github.com/dianabuilds/ardents-network/internal/successor/network/source", "github.com/dianabuilds/ardents-network/internal/successor/network/state":
			return true
		}
	case "internal/successor/installation/introduction/installed_transport_native_linux_test.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/admission/stock", "github.com/dianabuilds/ardents-network/internal/successor/route/ardp", "github.com/dianabuilds/ardents-network/internal/successor/route/channel", "github.com/dianabuilds/ardents-network/internal/successor/route/role", "github.com/dianabuilds/ardents-network/internal/successor/route/transport", "github.com/dianabuilds/ardents-network/internal/successor/route/transport/tls":
			return true
		}
	case "cmd/ardents-control/introduction_artifact_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/enrollment":
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
		case "github.com/dianabuilds/ardents-network/internal/successor/release":
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
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/cgroup", "github.com/dianabuilds/ardents-network/internal/successor/installation/process", "github.com/dianabuilds/ardents-network/internal/successor/installation/systemd", "github.com/dianabuilds/ardents-network/internal/successor/release":
			return true
		}
	case "internal/successor/installation/introduction/startup_linux.go":
		switch dependency {
		case "github.com/dianabuilds/ardents-network/internal/successor/installation/cgroup", "github.com/dianabuilds/ardents-network/internal/successor/installation/completion", "github.com/dianabuilds/ardents-network/internal/successor/installation/process", "github.com/dianabuilds/ardents-network/internal/successor/installation/systemd":
			return true
		}
	}
	return false
}
