package architecture

import (
	"path"
	"strings"
)

func offlineOperatorFile(source string) bool {
	if strings.HasPrefix(source, "internal/platform/artifacts/") ||
		strings.HasPrefix(source, "internal/network/source/inspection/") ||
		strings.HasPrefix(source, "internal/node/provisioning/") ||
		strings.HasPrefix(source, "internal/release/infrastructure/signing/") {
		return true
	}
	switch source {
	case "cmd/ardents-next/infrastructure_operator.go",
		"internal/admission/issuance/artifact_export_linux.go",
		"cmd/ardents-next/network_preparation.go", "cmd/ardents-next/network_keys_native_linux_test.go",
		"internal/release/infrastructure/preparation_linux.go",
		"internal/endpoint/source_trust_preparation_linux.go",
		"internal/network/state/source_state_files_linux.go",
		"cmd/ardents-next/network_preparation_linux.go",
		"cmd/ardents-next/source_inspection_linux.go", "tests/e2e/source/inspection_linux_test.go",
		"cmd/ardents-next/offline_profile_inspection.go", "cmd/ardents-next/offline_profile_inspection_test.go":
		return true
	}
	return false
}

// These offline producers can prepare artifacts but cannot open live Route or
// take authority from a retained runtime root. Each foreign dependency is tied
// to the particular source responsibility that uses it.
func offlineOperatorImportAllowed(source, dependency string) bool {
	first, _, _ := strings.Cut(dependency, "/")
	if dependency != "" && dependency != "C" && dependency != "unsafe" &&
		!strings.Contains(first, ".") && path.Clean(dependency) == dependency && !strings.HasPrefix(dependency, "/") {
		return true
	}
	switch source {
	case "internal/network/state/source_state_files_linux.go":
		switch dependency {
		case modulePath + "/internal/network/source", modulePath + "/internal/network/source/servingdeclaration", modulePath + "/internal/platform/artifacts", modulePath + "/internal/platform/process", modulePath + "/internal/processresource":
			return true
		}
	case "internal/endpoint/source_trust_preparation_linux.go":
		switch dependency {
		case modulePath + "/internal/network/source", modulePath + "/internal/network/source/servingdeclaration", modulePath + "/internal/platform/artifacts", modulePath + "/internal/pathgrammar", modulePath + "/internal/processresource":
			return true
		}
	case "internal/platform/artifacts/directory_linux.go", "internal/platform/artifacts/input_linux.go",
		"internal/platform/artifacts/private_file_publication_linux.go", "internal/platform/artifacts/platform_inputs_linux.go":
		return dependency == modulePath+"/internal/pathgrammar"
	case "internal/release/infrastructure/preparation_linux.go":
		return dependency == modulePath+"/internal/platform/artifacts" || dependency == modulePath+"/internal/enrollment"
	case "cmd/ardents-next/network_preparation_linux.go":
		return dependency == modulePath+"/internal/platform/artifacts"
	case "cmd/ardents-next/offline_profile_inspection.go":
		if dependency == modulePath+"/internal/canonicaldigest" {
			return true
		}
		fallthrough
	case "cmd/ardents-next/offline_profile_inspection_test.go":
		switch dependency {
		case modulePath + "/internal/admission/issuerprofile", modulePath + "/internal/network/closedprofile":
			return true
		}
	case "cmd/ardents-next/source_inspection_linux.go":
		return dependency == modulePath+"/internal/network/source/inspection" || dependency == modulePath+"/internal/canonicaldigest" || dependency == modulePath+"/internal/platform/artifacts"
	case "internal/network/source/inspection/inspection_linux_test.go":
		return dependency == modulePath+"/internal/platform/artifacts"
	case "internal/network/source/inspection/inspection_linux.go":
		switch dependency {
		case modulePath + "/internal/network/source", modulePath + "/internal/network/source/servingdeclaration", modulePath + "/internal/network/state", modulePath + "/internal/platform/artifacts", modulePath + "/internal/processresource":
			return true
		}
	case "tests/e2e/source/inspection_linux_test.go",
		"internal/node/provisioning/source_initial_network_native_linux_test.go":
		switch dependency {
		case modulePath + "/internal/network/source", modulePath + "/internal/network/source/servingdeclaration":
			return true
		}
	case "internal/node/provisioning/runtime_plan_command_test.go":
		return dependency == modulePath+"/internal/admission/issuance" || dependency == modulePath+"/internal/network/state" || dependency == modulePath+"/internal/node" || dependency == modulePath+"/internal/processresource"
	case "internal/node/provisioning/runtime_plan.go":
		switch dependency {
		case modulePath + "/internal/admission/issuance", modulePath + "/internal/canonicaldigest", modulePath + "/internal/hosting", modulePath + "/internal/network/state",
			modulePath + "/internal/node", modulePath + "/internal/pathgrammar", modulePath + "/internal/platform/artifacts", modulePath + "/internal/processresource":
			return true
		}
	case "internal/node/provisioning/epoch_plan_linux.go",
		"internal/node/provisioning/node_certificate_linux.go",
		"internal/node/provisioning/node_provision_linux.go",
		"internal/node/provisioning/node_record_linux.go":
		switch dependency {
		case modulePath + "/internal/canonicaldigest", modulePath + "/internal/network/epoch", modulePath + "/internal/platform/artifacts":
			return true
		}
	case "internal/node/provisioning/initial_network_keys_linux.go",
		"internal/node/provisioning/initial_network_keys_native_linux_test.go",
		"internal/node/provisioning/epoch_provision_linux.go",
		"internal/node/provisioning/forwarding_epoch_plan_linux_test.go",
		"internal/node/provisioning/node_provisioning_role_linux.go",
		"internal/node/provisioning/provisioning_key_linux.go",
		"internal/node/provisioning/public_identifier_linux.go",
		"internal/node/provisioning/public_identifier_linux_test.go",
		"internal/node/provisioning/source_initial_keys_linux.go",
		"internal/node/provisioning/source_initial_plan_linux.go",
		"internal/release/infrastructure/signing/descriptor_release_keys_linux.go",
		"internal/release/infrastructure/signing/forwarding_release_keys_linux.go",
		"internal/release/infrastructure/signing/infrastructure_release_keys_linux.go",
		"internal/release/infrastructure/signing/join_release_keys_linux.go",
		"internal/release/infrastructure/signing/node_release_arguments_linux.go",
		"internal/release/infrastructure/signing/node_release_arguments_linux_test.go",
		"internal/release/infrastructure/signing/release_key_receipt_linux.go",
		"internal/release/infrastructure/signing/source_release_keys_linux.go",
		"internal/release/infrastructure/signing/source_release_plan_linux_test.go":
		switch dependency {
		case modulePath + "/internal/canonicaldigest", modulePath + "/internal/platform/artifacts":
			return true
		}
	case "internal/node/provisioning/epoch_provision_native_linux_test.go":
		switch dependency {
		case modulePath + "/internal/network/epoch", modulePath + "/internal/network/source":
			return true
		}
	case "internal/node/provisioning/epoch_signing_linux.go",
		"internal/node/provisioning/source_initial_network_linux.go":
		switch dependency {
		case modulePath + "/internal/canonicaldigest", modulePath + "/internal/network/epoch", modulePath + "/internal/network/source", modulePath + "/internal/platform/artifacts":
			return true
		}
	case "internal/node/provisioning/forwarding_token_authority_native_linux_test.go":
		switch dependency {
		case modulePath + "/internal/admission", modulePath + "/internal/admission/issuer", modulePath + "/internal/admission/quota", modulePath + "/internal/admission/receiving", modulePath + "/internal/admission/stock", modulePath + "/internal/admission/token", modulePath + "/internal/custody", modulePath + "/internal/network", modulePath + "/internal/network/source", modulePath + "/internal/network/state":
			return true
		}
	case "internal/node/provisioning/node_certificate_linux_test.go",
		"internal/node/provisioning/node_provision_native_linux_test.go":
		switch dependency {
		case modulePath + "/internal/network/epoch":
			return true
		}
	case "internal/node/provisioning/profile_native_linux_test.go":
		switch dependency {
		case modulePath + "/internal/network/closedprofile", modulePath + "/internal/network/epoch", modulePath + "/internal/network/source":
			return true
		}
	case "internal/node/provisioning/profile_plan_linux.go":
		switch dependency {
		case modulePath + "/internal/admission/issuerprofile", modulePath + "/internal/network", modulePath + "/internal/network/closedprofile", modulePath + "/internal/network/epoch", modulePath + "/internal/network/source", modulePath + "/internal/platform/artifacts":
			return true
		}
	case "internal/node/provisioning/profile_signing_linux.go":
		switch dependency {
		case modulePath + "/internal/network/closedprofile", modulePath + "/internal/platform/artifacts":
			return true
		}
	case "internal/node/provisioning/token_material_linux.go":
		switch dependency {
		case modulePath + "/internal/admission/issuance", modulePath + "/internal/platform/artifacts":
			return true
		}
	case "internal/node/provisioning/token_material_native_linux_test.go":
		switch dependency {
		case modulePath + "/internal/admission/issuance", modulePath + "/internal/admission/issuerprofile":
			return true
		}
	case "internal/release/infrastructure/signing/descriptor_forward_release_native_linux_test.go",
		"internal/release/infrastructure/signing/forwarding_forward_release_native_linux_test.go",
		"internal/release/infrastructure/signing/join_forward_release_native_linux_test.go":
		switch dependency {
		case modulePath + "/internal/platform/systemd":
			return true
		}
	case "internal/release/infrastructure/signing/descriptor_release_native_linux_test.go",
		"internal/release/infrastructure/signing/forwarding_release_native_linux_test.go",
		"internal/release/infrastructure/signing/join_release_native_linux_test.go",
		"internal/release/infrastructure/signing/source_release_native_linux_test.go":
		switch dependency {
		case modulePath + "/internal/platform/systemd", modulePath + "/internal/release/infrastructure", "github.com/theupdateframework/go-tuf/v2/metadata":
			return true
		}
	case "internal/release/infrastructure/signing/node_release_linux.go",
		"internal/release/infrastructure/signing/source_release_linux.go":
		switch dependency {
		case modulePath + "/internal/platform/artifacts", modulePath + "/internal/release", modulePath + "/internal/release/infrastructure", "github.com/sigstore/sigstore/pkg/signature", "github.com/theupdateframework/go-tuf/v2/metadata":
			return true
		}
	case "internal/release/infrastructure/signing/node_release_plan_linux.go",
		"internal/release/infrastructure/signing/source_release_plan_linux.go":
		switch dependency {
		case modulePath + "/internal/platform/artifacts", modulePath + "/internal/platform/systemd", modulePath + "/internal/release/infrastructure":
			return true
		}
	case "internal/release/infrastructure/signing/node_release_predecessor_linux.go",
		"internal/release/infrastructure/signing/source_release_predecessor_linux.go":
		switch dependency {
		case modulePath + "/internal/enrollment", modulePath + "/internal/platform/artifacts", modulePath + "/internal/release", modulePath + "/internal/release/infrastructure", "github.com/theupdateframework/go-tuf/v2/metadata":
			return true
		}
	case "internal/release/infrastructure/signing/source_release_coherence_native_linux_test.go":
		switch dependency {
		case modulePath + "/internal/enrollment", modulePath + "/internal/platform/artifacts", modulePath + "/internal/release", modulePath + "/internal/release/infrastructure", "github.com/sigstore/sigstore/pkg/signature", "github.com/theupdateframework/go-tuf/v2/metadata":
			return true
		}
	case "internal/release/infrastructure/signing/source_release_timestamp_native_linux_test.go":
		switch dependency {
		case modulePath + "/internal/release", modulePath + "/internal/release/infrastructure", "github.com/theupdateframework/go-tuf/v2/metadata":
			return true
		}
	case "internal/node/provisioning/network_artifact_preparation.go":
		switch dependency {
		case modulePath + "/internal/canonicaldigest", modulePath + "/internal/platform/artifacts", modulePath + "/internal/network/epoch", modulePath + "/internal/network/closedprofile":
			return true
		}
	case "internal/node/provisioning/network_signing_declaration_linux.go":
		switch dependency {
		case modulePath + "/internal/network/epoch", modulePath + "/internal/network/closedprofile", modulePath + "/internal/platform/artifacts":
			return true
		}
	case "internal/node/provisioning/network_certificate_signing_linux.go",
		"internal/node/provisioning/network_certificate_signing_linux_test.go":
		return dependency == modulePath+"/internal/network/epoch"
	case "internal/node/provisioning/network_profile_signing_linux.go":
		return dependency == modulePath+"/internal/canonicaldigest" || dependency == modulePath+"/internal/network/closedprofile"
	case "internal/node/provisioning/network_output_linux.go", "internal/node/provisioning/network_signing_linux.go",
		"internal/node/provisioning/network_inventory_native_linux_test.go",
		"internal/node/provisioning/network_profile_native_linux_test.go",
		"internal/node/provisioning/network_signing_custody_native_linux_test.go",
		"internal/node/provisioning/network_preparation_native_linux_test.go",
		"internal/node/provisioning/network_preparation_linux.go":
		return dependency == modulePath+"/internal/platform/artifacts"
	case "internal/node/provisioning/network_signing_test.go":
		return dependency == modulePath+"/tests/epochfixture/network"
	case "internal/node/provisioning/network_profile_signing_test.go", "internal/node/provisioning/network_public_preparation_test.go":
		return dependency == modulePath+"/internal/network/closedprofile" || dependency == modulePath+"/tests/epochfixture/network"
	case "cmd/ardents-next/network_preparation.go":
		return dependency == modulePath+"/internal/node/provisioning" || dependency == modulePath+"/internal/canonicaldigest" || dependency == modulePath+"/internal/pathgrammar"
	case "cmd/ardents-next/network_keys_native_linux_test.go":
		return dependency == modulePath+"/internal/node/provisioning" || dependency == modulePath+"/internal/platform/artifacts"
	case "internal/admission/issuance/artifact_export_linux.go":
		return dependency == modulePath+"/internal/platform/artifacts"
	case "cmd/ardents-next/infrastructure_operator.go":
		return dependency == modulePath+"/internal/node/provisioning" ||
			dependency == modulePath+"/internal/release/infrastructure/signing"
	}
	return false
}
