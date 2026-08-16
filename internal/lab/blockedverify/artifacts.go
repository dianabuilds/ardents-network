package blockedverify

import (
	"path/filepath"
	"reflect"
	"strings"
)

func verifyArtifacts(root string, manifestArtifacts, evidenceArtifacts []artifactCommitment) []string {
	if !reflect.DeepEqual(manifestArtifacts, evidenceArtifacts) {
		return []string{"evidence secret commitments differ from the manifest"}
	}
	seen := make(map[string]bool, len(manifestArtifacts))
	var reasons []string
	for _, artifact := range manifestArtifacts {
		path, safe := safeArtifactPath(root, artifact.Path)
		if !safe || seen[artifact.Path] || artifact.Bytes < 1 {
			reasons = append(reasons, "secret artifact path or cardinality is invalid")
			continue
		}
		seen[artifact.Path] = true
		hash, size, err := hashFile(path)
		if err != nil || hash != artifact.SHA256 || size != artifact.Bytes {
			reasons = append(reasons, "secret artifact commitment mismatch: "+artifact.Path)
		}
	}
	if len(seen) != 7 || !seen["canaries.json"] || !seen["candidate/client.stderr"] ||
		!seen["candidate/server.stderr"] || !seen["capture/packets.bin"] {
		reasons = append(reasons, "secret artifact inventory is incomplete")
	}
	return reasons
}

func verifySupplyArtifacts(value manifest) []string {
	wanted := map[string]string{"runner": value.RunnerSHA256, "client": value.ClientSHA256,
		"server": value.ServerSHA256}
	seen := make(map[string]bool, 3)
	var reasons []string
	for _, artifact := range value.SecretArtifacts {
		for name, hash := range wanted {
			if strings.HasPrefix(artifact.Path, "supply/"+name) {
				if seen[name] || artifact.SHA256 != hash {
					reasons = append(reasons, "frozen supply commitment is duplicated or mismatched: "+name)
				}
				seen[name] = true
			}
		}
	}
	if len(seen) != len(wanted) {
		reasons = append(reasons, "frozen runner/client/server commitments are incomplete")
	}
	return reasons
}

func verifySupplementalArtifacts(root string, value manifest, observed evidence, canaries canaryCorpus) []string {
	if !reflect.DeepEqual(value.SupplementalArtifacts, observed.SupplementalArtifacts) {
		return []string{"publishable supplemental commitments differ from the manifest"}
	}
	variant, owner, canaryMode := canaryModeVariant(value.FixtureMode)
	wantsPipelineNote := canaryMode && owner == "pipeline"
	if wantsPipelineNote != (len(value.SupplementalArtifacts) == 1) {
		return []string{"publishable supplemental inventory differs from the fixture mode"}
	}
	var reasons []string
	for _, artifact := range value.SupplementalArtifacts {
		if artifact.Path != "pipeline-note.bin" || artifact.Bytes < 1 {
			reasons = append(reasons, "publishable supplemental artifact identity is invalid")
			continue
		}
		path := filepath.Join(root, artifact.Path)
		hash, size, err := hashFile(path)
		if err != nil || hash != artifact.SHA256 || size != artifact.Bytes {
			reasons = append(reasons, "publishable supplemental artifact commitment mismatch")
		}
		if wantsPipelineNote {
			set, found := canarySetForVariant(canaries, variant)
			expected := encodedCanarySet(set)
			if !found || artifact.SHA256 != digest(expected) || artifact.Bytes != int64(len(expected)) {
				reasons = append(reasons, "pipeline canary fixture did not exercise all four bound secrets")
			}
		}
	}
	return reasons
}
