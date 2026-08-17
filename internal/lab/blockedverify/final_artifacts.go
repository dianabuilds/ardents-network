package blockedverify

import (
	"reflect"
	"strings"
)

const emptyDockerConfigSHA256 = "ca3d163bab055381827226140568f3bef7eaac187cebd76878e0b63e9e442356"

var requiredMeasurementArtifacts = []string{
	"candidate/client.stderr",
	"candidate/server.stderr",
	"capture/packets.bin",
	"measurements/profiles.jsonl",
	"measurements/capacity.jsonl",
	"measurements/sustained.jsonl",
	"measurements/pressure.jsonl",
	"measurements/recovery.jsonl",
	"measurements/resources.jsonl",
	"measurements/traffic.jsonl",
	"measurements/host.json",
	"measurements/cells.jsonl",
}

func verifyMeasurementArtifacts(root string, summary *finalSummary) []string {
	if summary == nil || len(summary.Artifacts) != len(requiredMeasurementArtifacts) {
		return []string{"final measurement artifact inventory is incomplete"}
	}
	var reasons []string
	snapshots := make(map[string][]byte, len(requiredMeasurementArtifacts))
	for index, expected := range requiredMeasurementArtifacts {
		artifact := summary.Artifacts[index]
		path, safe := safeArtifactPath(root, artifact.Path)
		if !safe || artifact.Path != expected || artifact.Bytes < 1 || !isHexDigest(artifact.SHA256, 32) {
			reasons = append(reasons, "final measurement artifact identity is invalid")
			continue
		}
		var hash string
		var size int64
		var err error
		if strings.HasPrefix(expected, "measurements/") {
			snapshots[expected], hash, size, err = snapshotMeasurement(path)
		} else {
			hash, size, err = hashFile(path)
		}
		if err != nil || hash != artifact.SHA256 || size != artifact.Bytes {
			reasons = append(reasons, "final measurement artifact commitment mismatch: "+artifact.Path)
		}
	}
	reasons = append(reasons, verifyFinalMeasurementContent(snapshots, summary)...)
	return reasons
}

func verifyFinalInputArtifacts(values []artifactCommitment, spec finalSpec) []string {
	if len(values) != 8+len(requiredFinalConfigurations) {
		return []string{"final immutable input artifact inventory is incomplete"}
	}
	byPath := make(map[string]artifactCommitment, len(values))
	for _, value := range values {
		if byPath[value.Path].Path != "" {
			return []string{"final immutable input artifact inventory is duplicated"}
		}
		byPath[value.Path] = value
	}
	if byPath["canaries.json"].Path == "" || byPath["final-spec.json"].Path == "" {
		return []string{"final immutable input artifact inventory is incomplete"}
	}
	if !reflect.DeepEqual(byPath[spec.RuntimeCompose.Path], spec.RuntimeCompose) ||
		!reflect.DeepEqual(byPath[spec.SupplyLock.Path], spec.SupplyLock) {
		return []string{"final runtime supply artifact differs from its frozen specification"}
	}
	dockerConfig := byPath["runtime/docker-config/config.json"]
	if dockerConfig.Path == "" || dockerConfig.Bytes != 3 || dockerConfig.SHA256 != emptyDockerConfigSHA256 {
		return []string{"final Docker authority configuration is not exactly empty"}
	}
	for _, expected := range spec.Configurations {
		if !reflect.DeepEqual(byPath[expected.Path], expected) {
			return []string{"final configuration artifact differs from its frozen specification"}
		}
	}
	return nil
}

func verifyDevelopmentInputArtifacts(values []artifactCommitment) []string {
	wanted := map[string]bool{"canaries.json": true, "candidate/client.stderr": true,
		"candidate/server.stderr": true, "capture/packets.bin": true}
	if len(values) != 7 {
		return []string{"development immutable input artifact inventory is incomplete"}
	}
	for _, value := range values {
		delete(wanted, value.Path)
	}
	if len(wanted) != 0 {
		return []string{"development immutable input artifact inventory is incomplete"}
	}
	return nil
}
