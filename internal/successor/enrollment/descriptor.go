package enrollment

import "strings"

type descriptor struct{ values map[string]string }

// The accepted v3 grammar has fixed ordered fields. Retired schemas are only
// recognized after both independent manifest and RELEASE digest verification.
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
	if values["schema"] != "ardents-closed-alpha-enrollment-v3" ||
		values["control_release"] != "release.ac1" || values["control_network"] != "network.ac1" ||
		values["control_compatibility"] != "compatibility.ac1" || values["control_release_root"] != "release.pub" ||
		values["control_network_root"] != "network.pub" || values["control_compatibility_root"] != "compatibility.pub" ||
		values["corpus_authority"] != "corpus.pub" || values["control_artifact"] != artifactName("ardents-control", values["platform"]) {
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

func artifactName(command, platform string) string {
	name := command + "-" + platform
	if strings.HasPrefix(platform, "windows-") {
		name += ".exe"
	}
	return name
}

func (d descriptor) bind(files map[string][]byte, scope Scope) (Facts, error) {
	for _, key := range []string{"artifact", "trusted_root", "control_catalog", "disclosure_root", "control_release", "control_network", "control_compatibility", "control_release_root", "control_network_root", "control_compatibility_root", "corpus_authority", "control_artifact"} {
		if _, ok := files[d.values[key]]; !ok {
			return Facts{}, ErrInventory
		}
	}
	_, node := files[artifactName("ardents-node", d.values["platform"])]
	_, custody := files[artifactName("ardents-custody", d.values["platform"])]
	if node != custody || (scope == Headless && !node) {
		return Facts{}, ErrInventory
	}
	protected, err := verifyProtected(files, d.values["platform"], d.values["artifact"], d.values["release"])
	if err != nil {
		return Facts{}, err
	}
	return Facts{Cohort: d.values["cohort"], Release: d.values["release"], Platform: d.values["platform"],
		Environment: d.values["environment"], Network: d.values["network"], TargetPath: d.values["target_path"],
		Artifact: d.values["artifact"], TrustedRoot: d.values["trusted_root"], Headless: node, Protected: protected}, nil
}
