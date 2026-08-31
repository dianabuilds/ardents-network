package enrollment

import (
	"errors"
	"strings"
)

type descriptor struct {
	cohort, release, platform, environment, network, targetPath string
	artifact, browserAdapter, browserEntry, browserExtension    string
	required                                                    []string
}

func parseDescriptor(raw []byte) (descriptor, error) {
	keys := []string{"schema", "cohort", "release", "platform", "environment", "network", "target_path", "artifact", "trusted_root", "control_catalog", "disclosure_root", "control_release", "control_network", "control_compatibility", "control_release_root", "control_network_root", "control_compatibility_root", "corpus_authority", "control_artifact", "browser_adapter_artifact", "browser_entry_artifact", "browser_entry_extension"}
	if len(raw) == 0 || raw[len(raw)-1] != '\n' {
		return descriptor{}, errors.New("browser enrollment descriptor is not canonical")
	}
	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if len(lines) != len(keys) {
		return descriptor{}, errors.New("browser enrollment descriptor is not canonical")
	}
	values := make(map[string]string, len(keys))
	for index, line := range lines {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 || parts[0] != keys[index] || parts[1] == "" {
			return descriptor{}, errors.New("browser enrollment descriptor is not canonical")
		}
		values[parts[0]] = parts[1]
	}
	platform := values["platform"]
	if values["schema"] != descriptorSchema || values["control_release"] != "release.ac1" ||
		values["control_network"] != "network.ac1" || values["control_compatibility"] != "compatibility.ac1" ||
		values["control_release_root"] != "release.pub" || values["control_network_root"] != "network.pub" ||
		values["control_compatibility_root"] != "compatibility.pub" || values["corpus_authority"] != "corpus.pub" ||
		values["control_artifact"] != "ardents-control-"+platform ||
		values["browser_adapter_artifact"] != executableName("ardents-browser", platform) ||
		values["browser_entry_artifact"] != executableName("ardents-browser-entry", platform) ||
		values["browser_entry_extension"] != "ardents-alpha-browser-entry.xpi" {
		return descriptor{}, errors.New("browser enrollment descriptor is invalid")
	}
	requiredKeys := keys[7:]
	required := make([]string, 0, len(requiredKeys))
	seen := make(map[string]bool)
	for _, key := range requiredKeys {
		name := values[key]
		if !validName(name) || name == descriptorName || seen[name] {
			return descriptor{}, errors.New("browser enrollment descriptor inventory is invalid")
		}
		seen[name] = true
		required = append(required, name)
	}
	return descriptor{cohort: values["cohort"], release: values["release"], platform: platform,
		environment: values["environment"], network: values["network"], targetPath: values["target_path"],
		artifact: values["artifact"], browserAdapter: values["browser_adapter_artifact"],
		browserEntry: values["browser_entry_artifact"], browserExtension: values["browser_entry_extension"], required: required}, nil
}

func executableName(command, platform string) string {
	name := command + "-" + platform
	if strings.HasPrefix(platform, "windows-") {
		return name + ".exe"
	}
	return name
}

func (descriptor descriptor) matches(request Request) bool {
	return descriptor.cohort == request.Pin.Cohort && descriptor.release == request.Pin.Release &&
		descriptor.platform == request.Pin.Platform && descriptor.environment == request.Environment &&
		descriptor.network == request.Network && descriptor.targetPath == request.TargetPath
}
