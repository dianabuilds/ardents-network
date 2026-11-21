package enrollment

import "strings"

const (
	bundleSchemaV3 = "ardents-closed-alpha-enrollment-v3"
	bundleSchemaV4 = "ardents-closed-alpha-enrollment-v4"
)

type descriptor struct{ values map[string]string }

// Both verified grammars have the same ordered fields. The pinned schema
// selects an exact companion inventory after the RELEASE digest is verified.
func parseDescriptor(raw []byte) (descriptor, error) {
	if len(raw) == 0 || raw[len(raw)-1] != '\n' {
		return descriptor{}, ErrInventory
	}
	lines := strings.Split(string(raw[:len(raw)-1]), "\n")
	if lines[0] == "schema=ardents-closed-alpha-enrollment-v1" || lines[0] == "schema=ardents-closed-alpha-enrollment-v2" {
		return descriptor{}, ErrLegacyEnrollmentDescriptor
	}
	keys := []string{"schema", "cohort", "release", "platform", "environment", "network", "target_path", "artifact", "trusted_root", "control_catalog", "disclosure_root", "control_release", "control_network", "control_compatibility", "control_release_root", "control_network_root", "control_compatibility_root", "corpus_authority", "control_artifact"}
	if len(lines) != len(keys) {
		return descriptor{}, ErrInventory
	}
	values := make(map[string]string, len(keys))
	for i, key := range keys {
		parts := strings.SplitN(lines[i], "=", 2)
		if len(parts) != 2 || parts[0] != key || parts[1] == "" || strings.ContainsRune(parts[1], '\r') {
			return descriptor{}, ErrInventory
		}
		values[key] = parts[1]
	}
	schema := values["schema"]
	if (schema != bundleSchemaV3 && schema != bundleSchemaV4) ||
		values["control_release"] != "release.ac1" || values["control_network"] != "network.ac1" ||
		values["control_compatibility"] != "compatibility.ac1" || values["control_release_root"] != "release.pub" ||
		values["control_network_root"] != "network.pub" || values["control_compatibility_root"] != "compatibility.pub" ||
		values["corpus_authority"] != "corpus.pub" || values["control_artifact"] != ExecutableArtifactName("ardents-control", values["platform"]) {
		return descriptor{}, ErrInventory
	}
	seen := make(map[string]bool)
	for _, key := range keys[7:] {
		name := values[key]
		if !validName(name) || name == "RELEASE" || name == "SHA256SUMS" || seen[name] {
			return descriptor{}, ErrInventory
		}
		seen[name] = true
	}
	return descriptor{values: values}, nil
}

func (d descriptor) bind(files map[string][]byte, scope Scope) (Facts, error) {
	for _, key := range []string{"artifact", "trusted_root", "control_catalog", "disclosure_root", "control_release", "control_network", "control_compatibility", "control_release_root", "control_network_root", "control_compatibility_root", "corpus_authority", "control_artifact"} {
		if _, ok := files[d.values[key]]; !ok {
			return Facts{}, ErrInventory
		}
	}
	_, node := files[ExecutableArtifactName("ardents-node", d.values["platform"])]
	_, custody := files[ExecutableArtifactName("ardents-custody", d.values["platform"])]
	switch d.values["schema"] {
	case bundleSchemaV3:
		if node != custody || (scope == Headless && !node) {
			return Facts{}, ErrInventory
		}
	case bundleSchemaV4:
		if node || !custody {
			return Facts{}, ErrInventory
		}
	default:
		return Facts{}, ErrInventory
	}
	protected, err := verifyProtected(files, d.values["platform"], d.values["artifact"], d.values["release"])
	if err != nil {
		return Facts{}, err
	}
	return Facts{Cohort: d.values["cohort"], Release: d.values["release"], Platform: d.values["platform"],
		Environment: d.values["environment"], Network: d.values["network"], TargetPath: d.values["target_path"],
		Artifact: d.values["artifact"], TrustedRoot: d.values["trusted_root"], ControlCatalog: d.values["control_catalog"], DisclosureRoot: d.values["disclosure_root"], ControlArtifact: d.values["control_artifact"], Headless: custody, Protected: protected}, nil
}
