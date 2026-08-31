package enrollment

import (
	"errors"
	"strings"
)

type descriptor struct {
	schema                                                           string
	cohort, release, platform, environment, network, targetPath      string
	artifact, trustedRoot, controlCatalog, disclosureRoot            string
	controlRelease, controlNetwork, controlCompatibility             string
	controlReleaseRoot, controlNetworkRoot, controlCompatibilityRoot string
	corpusAuthority, controlArtifact                                 string
}

func parseDescriptor(raw []byte) (descriptor, error) {
	baseKeys := []string{"schema", "cohort", "release", "platform", "environment", "network", "target_path", "artifact", "trusted_root", "control_catalog", "disclosure_root", "control_release", "control_network", "control_compatibility", "control_release_root", "control_network_root", "control_compatibility_root"}
	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if len(lines) == 0 || len(raw) == 0 || raw[len(raw)-1] != '\n' {
		return descriptor{}, errors.New("alpha descriptor is not canonical")
	}
	keys := baseKeys
	if lines[0] == "schema=ardents-closed-alpha-enrollment-v2" || lines[0] == "schema=ardents-closed-alpha-enrollment-v3" {
		keys = append(append([]string(nil), baseKeys...), "corpus_authority")
	}
	if lines[0] == "schema=ardents-closed-alpha-enrollment-v3" {
		keys = append(keys, "control_artifact")
	}
	if len(lines) != len(keys) {
		return descriptor{}, errors.New("alpha descriptor is not canonical")
	}
	values := make(map[string]string, len(keys))
	for index, line := range lines {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 || parts[0] != keys[index] || parts[1] == "" {
			return descriptor{}, errors.New("alpha descriptor is not canonical")
		}
		values[parts[0]] = parts[1]
	}
	if !validDescriptor(values) {
		return descriptor{}, errors.New("alpha descriptor is invalid")
	}
	return descriptor{schema: values["schema"], cohort: values["cohort"], release: values["release"], platform: values["platform"], environment: values["environment"], network: values["network"], targetPath: values["target_path"], artifact: values["artifact"], trustedRoot: values["trusted_root"], controlCatalog: values["control_catalog"], disclosureRoot: values["disclosure_root"], controlRelease: values["control_release"], controlNetwork: values["control_network"], controlCompatibility: values["control_compatibility"], controlReleaseRoot: values["control_release_root"], controlNetworkRoot: values["control_network_root"], controlCompatibilityRoot: values["control_compatibility_root"], corpusAuthority: values["corpus_authority"], controlArtifact: values["control_artifact"]}, nil
}

func validDescriptor(values map[string]string) bool {
	schema := values["schema"]
	if (schema != "ardents-closed-alpha-enrollment-v1" && schema != "ardents-closed-alpha-enrollment-v2" && schema != "ardents-closed-alpha-enrollment-v3") || values["control_release"] != "release.ac1" || values["control_network"] != "network.ac1" || values["control_compatibility"] != "compatibility.ac1" || values["control_release_root"] != "release.pub" || values["control_network_root"] != "network.pub" || values["control_compatibility_root"] != "compatibility.pub" {
		return false
	}
	if (schema == "ardents-closed-alpha-enrollment-v2" || schema == "ardents-closed-alpha-enrollment-v3") && values["corpus_authority"] != "corpus.pub" {
		return false
	}
	if schema == "ardents-closed-alpha-enrollment-v3" && values["control_artifact"] != ExecutableArtifactName("ardents-control", values["platform"]) {
		return false
	}
	names := []string{values["artifact"], values["trusted_root"], values["control_catalog"], values["disclosure_root"], values["control_release"], values["control_network"], values["control_compatibility"], values["control_release_root"], values["control_network_root"], values["control_compatibility_root"]}
	if schema == "ardents-closed-alpha-enrollment-v2" || schema == "ardents-closed-alpha-enrollment-v3" {
		names = append(names, values["corpus_authority"])
	}
	if schema == "ardents-closed-alpha-enrollment-v3" {
		names = append(names, values["control_artifact"])
	}
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		if !validName(name) || name == descriptorName {
			return false
		}
		if _, found := seen[name]; found {
			return false
		}
		seen[name] = struct{}{}
	}
	return true
}

// ExecutableArtifactName returns the one enrollment-owned artifact name for a
// command on a declared platform. Callers must not reconstruct this identity.
func ExecutableArtifactName(command, platform string) string {
	name := command + "-" + platform
	if strings.HasPrefix(platform, "windows-") {
		return name + ".exe"
	}
	return name
}

func (descriptor descriptor) matches(request Request) error {
	if descriptor.cohort != request.Pin.Cohort || descriptor.release != request.Pin.Release || descriptor.platform != request.Pin.Platform || descriptor.environment != request.Environment || descriptor.network != request.Network || descriptor.targetPath != request.TargetPath {
		return errors.New("alpha descriptor does not match the enrollment request")
	}
	return nil
}
