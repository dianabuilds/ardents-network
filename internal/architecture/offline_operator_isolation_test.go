package architecture

import (
	"path"
	"strings"
)

func offlineOperatorFile(source string) bool {
	if strings.HasPrefix(source, "internal/platform/artifacts/") ||
		strings.HasPrefix(source, "internal/domain/network/source/inspection/") ||
		strings.HasPrefix(source, "internal/domain/node/provisioning/") ||
		strings.HasPrefix(source, "internal/domain/release/infrastructure/signing/") {
		return true
	}
	switch source {
	case "cmd/ardents-next/infrastructure_operator.go",
		"internal/domain/admission/issuance/artifact_export_unix.go",
		"internal/domain/release/infrastructure/preparation_linux.go",
		"internal/domain/release/infrastructure/preparation_native_linux_test.go",
		"internal/domain/endpoint/source_trust_preparation_linux.go",
		"internal/domain/network/state/source_state_files_linux.go",
		"cmd/ardents-next/source_inspection_linux.go", "tests/e2e/source/inspection_linux_test.go",
		"cmd/ardents-next/offline_profile_inspection.go", "tests/e2e/node/offline_profile_inspection_test.go":
		return true
	}
	return false
}

// These offline producers can prepare artifacts but cannot open live Route or
// take authority from a retained runtime root. Each foreign dependency is tied
// to the particular source responsibility that uses it.
func offlineOperatorImportAllowed(source, dependency string) bool {
	if allowedStandardImport(dependency) {
		return true
	}
	if dependency == modulePath+"/internal/domain/release/infrastructure/signing/internal/keyset" {
		switch source {
		case "internal/domain/release/infrastructure/signing/release_key_command_linux.go",
			"internal/domain/release/infrastructure/signing/release_key_receipt_unix.go",
			"internal/domain/release/infrastructure/signing/release_key_receipt_unix_test.go":
			return true
		}
		return false
	}
	if path.Dir(source) == "internal/domain/release/infrastructure/signing/internal/keyset" {
		return source == "internal/domain/release/infrastructure/signing/internal/keyset/prepare_linux.go" &&
			dependency == modulePath+"/internal/platform/artifacts"
	}
	if dependency == modulePath+"/internal/domain/node/provisioning/sourcegenesis" {
		return source == "internal/domain/node/provisioning/infrastructure/command_linux.go"
	}
	if dependency == modulePath+"/internal/domain/node/provisioning/networkpreparation" {
		return source == "internal/domain/node/provisioning/network_commands.go" ||
			source == "internal/domain/node/provisioning/network_signing_custody_native_linux_test.go"
	}
	if dependency == modulePath+"/internal/domain/node/provisioning/networksigning" {
		switch source {
		case "internal/domain/node/provisioning/network_commands.go",
			"internal/domain/node/provisioning/network_inventory_native_linux_test.go",
			"internal/domain/node/provisioning/network_profile_native_linux_test.go",
			"internal/domain/node/provisioning/network_signing_custody_native_linux_test.go",
			"internal/domain/node/provisioning/infrastructure/node_provision_linux.go",
			"internal/domain/node/provisioning/infrastructure/node_provision_native_linux_test.go":
			return true
		}
		return false
	}
	if dependency == modulePath+"/tests/networkfixture" {
		switch source {
		case "internal/domain/node/provisioning/infrastructure/epoch_provision_native_linux_test.go",
			"internal/domain/node/provisioning/infrastructure/forwarding_state_preparation_native_linux_test.go",
			"internal/domain/node/provisioning/infrastructure/forwarding_token_authority_native_linux_test.go",
			"internal/domain/node/provisioning/infrastructure/node_provision_native_linux_test.go",
			"internal/domain/node/provisioning/infrastructure/profile_native_linux_test.go",
			"internal/domain/node/provisioning/infrastructure/provisioning_key_native_linux_test.go",
			"internal/domain/node/provisioning/infrastructure/token_material_native_linux_test.go",
			"internal/domain/node/provisioning/initial_network_keys_native_linux_test.go",
			"internal/domain/node/provisioning/source_initial_network_native_linux_test.go",
			"internal/domain/node/provisioning/network_inventory_native_linux_test.go",
			"internal/domain/node/provisioning/networksigning/inventory_native_linux_test.go",
			"internal/domain/node/provisioning/network_profile_native_linux_test.go",
			"internal/domain/node/provisioning/network_public_preparation_test.go",
			"internal/domain/node/provisioning/network_record_preparation_test.go",
			"internal/domain/node/provisioning/network_signing_custody_native_linux_test.go",
			"internal/domain/node/provisioning/networksigning/certificate_operation_unix_test.go",
			"internal/domain/node/provisioning/networksigning/declaration_linux_test.go",
			"internal/domain/node/provisioning/networksigning/profile_unix_test.go":
			return true
		}
		return false
	}
	if dependency == modulePath+"/internal/domain/node/provisioning/networkdraft" {
		switch source {
		case "internal/domain/node/provisioning/networksigning/inventory_linux.go",
			"internal/domain/node/provisioning/networksigning/certificate_operation_unix.go",
			"internal/domain/node/provisioning/networksigning/certificate_operation_unix_test.go",
			"internal/domain/node/provisioning/network_commands.go",
			"internal/domain/node/provisioning/network_inventory_native_linux_test.go",
			"internal/domain/node/provisioning/network_signing_custody_native_linux_test.go",
			"internal/domain/node/provisioning/networkpreparation/output_linux.go",
			"internal/domain/node/provisioning/networkpreparation/prepare_linux.go",
			"internal/domain/node/provisioning/networkpreparation/prepare_other.go",
			"internal/domain/node/provisioning/network_profile_native_linux_test.go",
			"internal/domain/node/provisioning/networksigning/profile_unix.go",
			"internal/domain/node/provisioning/network_public_preparation_test.go",
			"internal/domain/node/provisioning/network_record_preparation_test.go",
			"internal/domain/node/provisioning/networksigning/declaration_unix.go",
			"internal/domain/node/provisioning/network_signing_fixture_unix_test.go",
			"internal/domain/node/provisioning/networksigning/protected_linux.go",
			"internal/domain/node/provisioning/networksigning/declaration_linux_test.go":
			return true
		}
		return false
	}

	switch source {
	case "internal/domain/network/state/source_state_files_linux.go":
		switch dependency {
		case modulePath + "/internal/domain/network/source", modulePath + "/internal/domain/network/source/servingdeclaration", modulePath + "/internal/platform/artifacts", modulePath + "/internal/platform/process", modulePath + "/internal/platform/processguard":
			return true
		}
	case "internal/domain/endpoint/source_trust_preparation_linux.go":
		switch dependency {
		case modulePath + "/internal/domain/network/source", modulePath + "/internal/domain/network/source/servingdeclaration", modulePath + "/internal/platform/artifacts", modulePath + "/internal/pathcheck", modulePath + "/internal/platform/processguard":
			return true
		}
	case "internal/platform/artifacts/directory_unix.go", "internal/platform/artifacts/input_unix.go",
		"internal/platform/artifacts/retained_file_unix.go", "internal/platform/artifacts/protected_input_linux.go":
		return dependency == modulePath+"/internal/pathcheck" || dependency == modulePath+"/internal/durablefile"
	case "internal/platform/artifacts/private_file_publication_unix.go", "internal/platform/artifacts/platform_inputs_linux.go":
		return dependency == modulePath+"/internal/pathcheck"
	case "internal/domain/release/infrastructure/preparation_linux.go":
		return dependency == modulePath+"/internal/platform/artifacts" || dependency == modulePath+"/internal/domain/enrollment"
	case "internal/domain/release/infrastructure/preparation_native_linux_test.go":
		return dependency == modulePath+"/internal/platform/artifacts"
	case "internal/domain/node/provisioning/network_report_unix.go":
		return dependency == modulePath+"/internal/platform/artifacts"
	case "cmd/ardents-next/offline_profile_inspection.go":
		if dependency == modulePath+"/internal/canonicaldigest" {
			return true
		}
		fallthrough
	case "tests/e2e/node/offline_profile_inspection_test.go":
		switch dependency {
		case modulePath + "/internal/domain/admission/issuerprofile", modulePath + "/internal/domain/network/closedprofile":
			return true
		}
	case "cmd/ardents-next/source_inspection_linux.go":
		return dependency == modulePath+"/internal/domain/network/source/inspection" || dependency == modulePath+"/internal/canonicaldigest" || dependency == modulePath+"/internal/platform/artifacts"
	case "internal/domain/network/source/inspection/inspection_linux_test.go":
		return dependency == modulePath+"/internal/platform/artifacts"
	case "internal/domain/network/source/inspection/inspection_linux.go":
		switch dependency {
		case modulePath + "/internal/domain/network/source", modulePath + "/internal/domain/network/source/servingdeclaration", modulePath + "/internal/domain/network/state", modulePath + "/internal/platform/artifacts", modulePath + "/internal/platform/processguard":
			return true
		}
	case "internal/domain/network/source/inspection/inspection_other.go", "internal/domain/network/source/inspection/inspection_other_test.go":
		// The typed platform refusal mirrors the contact result type only; it
		// retains no input, credential or live authority (ADR-0129, #856).
		return dependency == modulePath+"/internal/domain/network/source"
	case "tests/e2e/source/inspection_linux_test.go",
		"internal/domain/node/provisioning/source_initial_network_native_linux_test.go":
		switch dependency {
		case modulePath + "/internal/domain/network/source", modulePath + "/internal/domain/network/source/servingdeclaration":
			return true
		}
	case "internal/domain/node/provisioning/runtimeplan/command_test.go":
		return dependency == modulePath+"/internal/domain/admission/issuance" || dependency == modulePath+"/internal/domain/network/state" || dependency == modulePath+"/internal/domain/node/plan" || dependency == modulePath+"/internal/platform/processguard"
	case "internal/domain/node/provisioning/runtimeplan/prepare.go":
		switch dependency {
		case modulePath + "/internal/domain/admission/issuance", modulePath + "/internal/canonicaldigest", modulePath + "/internal/domain/hosting", modulePath + "/internal/domain/network/state",
			modulePath + "/internal/domain/node/plan", modulePath + "/internal/pathcheck", modulePath + "/internal/platform/artifacts", modulePath + "/internal/platform/processguard":
			return true
		}
	case "internal/domain/node/provisioning/infrastructure/epoch_plan_linux.go",
		"internal/domain/node/provisioning/networksigning/certificate_unix.go",
		"internal/domain/node/provisioning/infrastructure/node_provision_linux.go",
		"internal/domain/node/provisioning/infrastructure/node_record_unix.go":
		switch dependency {
		case modulePath + "/internal/canonicaldigest", modulePath + "/internal/domain/network/epoch", modulePath + "/internal/platform/artifacts":
			return true
		}
	case "internal/domain/node/provisioning/initial_network_keys_linux.go",
		"internal/domain/node/provisioning/initial_network_keys_native_linux_test.go",
		"internal/domain/node/provisioning/infrastructure/epoch_provision_linux.go",
		"internal/domain/node/provisioning/infrastructure/forwarding_epoch_plan_linux_test.go",
		"internal/domain/node/provisioning/infrastructure/node_provisioning_role_unix.go",
		"internal/domain/node/provisioning/infrastructure/provisioning_key_linux.go",
		"internal/domain/node/provisioning/infrastructure/public_identifier_unix.go",
		"internal/domain/node/provisioning/infrastructure/public_identifier_unix_test.go",
		"internal/domain/node/provisioning/sourcegenesis/keys_linux.go",
		"internal/domain/node/provisioning/sourcegenesis/plan_linux.go",
		"internal/domain/release/infrastructure/signing/node_release_arguments_linux.go",
		"internal/domain/release/infrastructure/signing/node_release_arguments_linux_test.go",
		"internal/domain/release/infrastructure/signing/release_key_command_linux.go",
		"internal/domain/release/infrastructure/signing/release_key_receipt_unix.go",
		"internal/domain/release/infrastructure/signing/source_release_plan_linux_test.go":
		switch dependency {
		case modulePath + "/internal/canonicaldigest", modulePath + "/internal/platform/artifacts":
			return true
		}
	case "internal/domain/node/provisioning/infrastructure/epoch_provision_native_linux_test.go":
		switch dependency {
		case modulePath + "/internal/domain/network/epoch", modulePath + "/internal/domain/network/source":
			return true
		}
	case "internal/domain/node/provisioning/infrastructure/epoch_signing_linux.go",
		"internal/domain/node/provisioning/sourcegenesis/artifacts_linux.go":
		switch dependency {
		case modulePath + "/internal/canonicaldigest", modulePath + "/internal/domain/network/epoch", modulePath + "/internal/domain/network/source", modulePath + "/internal/platform/artifacts":
			return true
		}
	case "internal/domain/node/provisioning/infrastructure/forwarding_token_authority_native_linux_test.go":
		switch dependency {
		case modulePath + "/internal/domain/admission", modulePath + "/internal/domain/admission/issuer", modulePath + "/internal/domain/admission/quota", modulePath + "/internal/domain/admission/receiving", modulePath + "/internal/domain/admission/stock", modulePath + "/internal/domain/admission/token", modulePath + "/internal/domain/custody", modulePath + "/internal/domain/network", modulePath + "/internal/domain/network/source", modulePath + "/internal/domain/network/state":
			return true
		}
	case "internal/domain/node/provisioning/networksigning/certificate_unix_test.go",
		"internal/domain/node/provisioning/infrastructure/node_provision_native_linux_test.go":
		switch dependency {
		case modulePath + "/internal/domain/network/epoch":
			return true
		}
	case "internal/domain/node/provisioning/infrastructure/profile_native_linux_test.go":
		switch dependency {
		case modulePath + "/internal/domain/network/closedprofile", modulePath + "/internal/domain/network/epoch", modulePath + "/internal/domain/network/source":
			return true
		}
	case "internal/domain/node/provisioning/infrastructure/profile_plan_linux.go":
		switch dependency {
		case modulePath + "/internal/domain/admission/issuerprofile", modulePath + "/internal/domain/network", modulePath + "/internal/domain/network/closedprofile", modulePath + "/internal/domain/network/epoch", modulePath + "/internal/domain/network/source", modulePath + "/internal/platform/artifacts":
			return true
		}
	case "internal/domain/node/provisioning/infrastructure/profile_signing_linux.go":
		switch dependency {
		case modulePath + "/internal/domain/network/closedprofile", modulePath + "/internal/platform/artifacts":
			return true
		}
	case "internal/domain/node/provisioning/infrastructure/token_material_linux.go":
		switch dependency {
		case modulePath + "/internal/domain/admission/issuance", modulePath + "/internal/platform/artifacts":
			return true
		}
	case "internal/domain/node/provisioning/infrastructure/token_material_native_linux_test.go":
		switch dependency {
		case modulePath + "/internal/domain/admission/issuance", modulePath + "/internal/domain/admission/issuerprofile":
			return true
		}
	case "internal/domain/release/infrastructure/signing/descriptor_forward_release_native_linux_test.go",
		"internal/domain/release/infrastructure/signing/forwarding_forward_release_native_linux_test.go",
		"internal/domain/release/infrastructure/signing/join_forward_release_native_linux_test.go":
		switch dependency {
		case modulePath + "/internal/platform/systemd":
			return true
		}
	case "internal/domain/release/infrastructure/signing/descriptor_release_native_linux_test.go",
		"internal/domain/release/infrastructure/signing/forwarding_release_native_linux_test.go",
		"internal/domain/release/infrastructure/signing/join_release_native_linux_test.go",
		"internal/domain/release/infrastructure/signing/source_release_native_linux_test.go":
		switch dependency {
		case modulePath + "/internal/platform/systemd", modulePath + "/internal/domain/release/infrastructure", "github.com/theupdateframework/go-tuf/v2/metadata":
			return true
		}
	case "internal/domain/release/infrastructure/signing/node_release_linux.go",
		"internal/domain/release/infrastructure/signing/source_release_linux.go":
		switch dependency {
		case modulePath + "/internal/platform/artifacts", modulePath + "/internal/domain/release", modulePath + "/internal/domain/release/infrastructure", "github.com/sigstore/sigstore/pkg/signature", "github.com/theupdateframework/go-tuf/v2/metadata":
			return true
		}
	case "internal/domain/release/infrastructure/signing/node_release_plan_linux.go",
		"internal/domain/release/infrastructure/signing/source_release_plan_linux.go":
		switch dependency {
		case modulePath + "/internal/platform/artifacts", modulePath + "/internal/platform/systemd", modulePath + "/internal/domain/release/infrastructure":
			return true
		}
	case "internal/domain/release/infrastructure/signing/node_release_predecessor_linux.go",
		"internal/domain/release/infrastructure/signing/source_release_predecessor_linux.go":
		switch dependency {
		case modulePath + "/internal/domain/enrollment", modulePath + "/internal/platform/artifacts", modulePath + "/internal/domain/release", modulePath + "/internal/domain/release/infrastructure", "github.com/theupdateframework/go-tuf/v2/metadata":
			return true
		}
	case "internal/domain/release/infrastructure/signing/source_release_coherence_native_linux_test.go":
		switch dependency {
		case modulePath + "/internal/domain/enrollment", modulePath + "/internal/platform/artifacts", modulePath + "/internal/domain/release", modulePath + "/internal/domain/release/infrastructure", "github.com/sigstore/sigstore/pkg/signature", "github.com/theupdateframework/go-tuf/v2/metadata":
			return true
		}
	case "internal/domain/release/infrastructure/signing/source_release_timestamp_native_linux_test.go":
		switch dependency {
		case modulePath + "/internal/domain/release", modulePath + "/internal/domain/release/infrastructure", "github.com/theupdateframework/go-tuf/v2/metadata":
			return true
		}
	case "internal/domain/node/provisioning/networkdraft/declarations.go":
		switch dependency {
		case modulePath + "/internal/canonicaldigest", modulePath + "/internal/platform/artifacts", modulePath + "/internal/domain/network/epoch", modulePath + "/internal/domain/network/closedprofile":
			return true
		}
	case "internal/domain/node/provisioning/networksigning/declaration_unix.go":
		switch dependency {
		case modulePath + "/internal/domain/network/epoch", modulePath + "/internal/domain/network/closedprofile", modulePath + "/internal/platform/artifacts":
			return true
		}
	case "internal/domain/node/provisioning/networksigning/certificate_operation_unix.go",
		"internal/domain/node/provisioning/networksigning/certificate_operation_unix_test.go":
		return dependency == modulePath+"/internal/domain/network/epoch"
	case "internal/domain/node/provisioning/networksigning/profile_unix.go":
		return dependency == modulePath+"/internal/canonicaldigest" || dependency == modulePath+"/internal/domain/network/closedprofile"
	case "internal/domain/node/provisioning/networkpreparation/output_linux.go", "internal/domain/node/provisioning/networksigning/protected_linux.go",
		"internal/domain/node/provisioning/networksigning/inventory_linux.go",
		"internal/domain/node/provisioning/network_inventory_native_linux_test.go",
		"internal/domain/node/provisioning/network_profile_native_linux_test.go",
		"internal/domain/node/provisioning/network_signing_custody_native_linux_test.go",
		"internal/domain/node/provisioning/networkpreparation/publication_native_linux_test.go",
		"internal/domain/node/provisioning/networksigning/inventory_native_linux_test.go":
		return dependency == modulePath+"/internal/platform/artifacts"
	case "internal/domain/node/provisioning/networkpreparation/prepare_linux.go":
		return dependency == modulePath+"/internal/platform/artifacts" || dependency == modulePath+"/internal/durablefile"
	case "internal/domain/node/provisioning/networksigning/declaration_linux_test.go", "internal/domain/node/provisioning/network_record_preparation_test.go":
		return dependency == modulePath+"/tests/epochfixture/network"
	case "internal/domain/node/provisioning/networksigning/profile_unix_test.go", "internal/domain/node/provisioning/network_public_preparation_test.go":
		return dependency == modulePath+"/internal/domain/network/closedprofile" || dependency == modulePath+"/tests/epochfixture/network"
	case "internal/domain/node/provisioning/network_commands.go":
		return dependency == modulePath+"/internal/canonicaldigest" || dependency == modulePath+"/internal/pathcheck"
	case "internal/domain/admission/issuance/artifact_export_unix.go":
		return dependency == modulePath+"/internal/platform/artifacts"
	case "cmd/ardents-next/infrastructure_operator.go":
		return dependency == modulePath+"/internal/domain/node/provisioning/infrastructure" || dependency == modulePath+"/internal/domain/release/infrastructure" ||
			dependency == modulePath+"/internal/domain/release/infrastructure/signing"
	}
	return false
}
