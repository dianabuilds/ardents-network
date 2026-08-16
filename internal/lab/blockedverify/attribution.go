package blockedverify

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

type attributionRecord struct {
	Schema          string `json:"schema"`
	EventID         string `json:"event_id"`
	ProcessIdentity string `json:"process_identity"`
	CgroupIdentity  string `json:"cgroup_identity"`
	Observer        string `json:"observer"`
}

type attributionFact struct {
	owner, commitment string
}

func verifyAttributions(root string, artifacts []artifactCommitment, manifestValue manifest) (map[string]attributionFact, []string) {
	expected := expectedEventIDs()
	byPath := make(map[string]artifactCommitment, len(artifacts))
	var reasons []string
	for _, artifact := range artifacts {
		if _, duplicate := byPath[artifact.Path]; duplicate {
			reasons = append(reasons, "attribution artifact path is duplicated")
		}
		byPath[artifact.Path] = artifact
	}
	facts := make(map[string]attributionFact, len(expected))
	for eventID := range expected {
		digest := sha256.Sum256([]byte(eventID))
		relative := "attribution/" + hex.EncodeToString(digest[:]) + ".json"
		artifact, found := byPath[relative]
		path, safe := safeArtifactPath(root, relative)
		if !found || !safe || artifact.Bytes < 1 {
			reasons = append(reasons, "attribution artifact is absent or unsafe: "+eventID)
			continue
		}
		var record attributionRecord
		raw, err := decodeStrict(path, &record)
		owner := expectedFixtureOwner(manifestValue.FixtureMode, eventID)
		source, mapped := sourceForOwner(manifestValue.AttributionSources, owner)
		if err != nil || digestBytes(raw) != artifact.SHA256 || int64(len(raw)) != artifact.Bytes || !mapped ||
			record.Schema != "ardents-h3-attribution-v1" || record.EventID != eventID ||
			record.ProcessIdentity != source.ProcessIdentity || record.CgroupIdentity != source.CgroupIdentity ||
			record.Observer != "external-boundary-observer" {
			reasons = append(reasons, "attribution artifact is invalid: "+eventID)
			continue
		}
		facts[eventID] = attributionFact{owner: owner, commitment: artifact.SHA256}
	}
	if len(artifacts) != len(expected) || len(byPath) != len(expected) {
		reasons = append(reasons, "attribution artifact cardinality is invalid")
	}
	return facts, reasons
}

func sourceForOwner(sources []attributionSource, owner string) (attributionSource, bool) {
	for _, source := range sources {
		if source.Owner == owner {
			return source, true
		}
	}
	return attributionSource{}, false
}

func expectedFixtureOwner(mode, eventID string) string {
	first := strings.HasSuffix(eventID, "/0") && strings.HasPrefix(eventID, "G1-invite/malformed/")
	switch {
	case first && (mode == "candidate-fail" || mode == "candidate-fail-harness-invalid" ||
		mode == "candidate-residual" || mode == "candidate-forbidden"):
		return "candidate"
	case first && (mode == "harness-invalid" || mode == "cell-inventory-missing"):
		return "harness"
	case strings.HasSuffix(eventID, "/0"):
		variant, owner, ok := canaryModeVariant(mode)
		if ok && owner == "candidate" && strings.Contains(eventID, "/"+variant+"/") {
			return "candidate"
		}
		return "none"
	default:
		return "none"
	}
}

func digestBytes(raw []byte) string {
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}
