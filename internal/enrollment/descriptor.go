package enrollment

import (
	"strconv"
	"strings"
)

const bundleSchema = "ardents-closed-alpha-enrollment-v5"

type descriptor struct{ values map[string]string }

// The pinned RELEASE digest authenticates the sole supported descriptor.
func parseDescriptor(raw []byte) (descriptor, error) {
	if len(raw) == 0 || raw[len(raw)-1] != '\n' {
		return descriptor{}, ErrInventory
	}
	lines := strings.Split(string(raw[:len(raw)-1]), "\n")
	keys := []string{"schema", "cohort", "release", "platform", "environment", "network", "target_path", "artifact", "trusted_root"}
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
	if values["schema"] != bundleSchema {
		return descriptor{}, ErrInventory
	}
	custody := ExecutableArtifactName("ardents-custody", values["platform"])
	seen := map[string]bool{"RELEASE": true, "SHA256SUMS": true, custody: true}
	for _, name := range []string{values["artifact"], values["trusted_root"]} {
		if !validName(name) || seen[name] {
			return descriptor{}, ErrInventory
		}
		seen[name] = true
	}
	return descriptor{values: values}, nil
}

func (d descriptor) bind(files map[string][]byte) (Facts, error) {
	platform, artifact := d.values["platform"], d.values["artifact"]
	trustedRoot, release := d.values["trusted_root"], d.values["release"]
	custody := ExecutableArtifactName("ardents-custody", platform)
	static := map[string]bool{"RELEASE": true, artifact: true, trustedRoot: true, custody: true}
	for name := range static {
		if _, ok := files[name]; !ok {
			return Facts{}, ErrInventory
		}
	}
	protected, err := verifyProtected(files, platform, artifact, release)
	if err != nil {
		return Facts{}, err
	}
	if protected {
		static["protected-endpoint.json"] = true
		for _, name := range protectedNames() {
			static[name] = true
		}
	}
	metadata := make(map[string]bool)
	for name := range files {
		if static[name] {
			continue
		}
		role := releaseMetadataRole(name)
		if role == "" || role != "root.json" && metadata[role] {
			return Facts{}, ErrInventory
		}
		metadata[role] = true
	}
	for _, role := range []string{"timestamp.json", "snapshot.json", "targets.json"} {
		if !metadata[role] {
			return Facts{}, ErrInventory
		}
	}
	return Facts{Cohort: d.values["cohort"], Release: release, Platform: platform,
		Environment: d.values["environment"], Network: d.values["network"], TargetPath: d.values["target_path"],
		Artifact: artifact, TrustedRoot: trustedRoot, Protected: protected}, nil
}

// Only direct TUF metadata names can cross from the frozen bundle to Release.
func releaseMetadataRole(name string) string {
	switch name {
	case "timestamp.json", "snapshot.json", "targets.json":
		return name
	}
	version, role, _ := strings.Cut(name, ".")
	if role != "root.json" && role != "snapshot.json" && role != "targets.json" {
		return ""
	}
	number, err := strconv.ParseInt(version, 10, 64)
	if err == nil && number > 0 && strconv.FormatInt(number, 10) == version {
		return role
	}
	return ""
}
