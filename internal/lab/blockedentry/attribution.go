package blockedentry

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
)

type attributionRecord struct {
	Schema          string `json:"schema"`
	EventID         string `json:"event_id"`
	ProcessIdentity string `json:"process_identity"`
	CgroupIdentity  string `json:"cgroup_identity"`
	Observer        string `json:"observer"`
}

type attributionSource struct {
	ProcessIdentity string `json:"process_identity"`
	CgroupIdentity  string `json:"cgroup_identity"`
	Owner           string `json:"owner"`
}

var attributionSources = []attributionSource{
	{"none", "none", "none"},
	{"bridge-adapter", "adapter-e", "candidate"},
	{"evidence-harness", "lab-harness", "harness"},
}

func attributionRoot(evidenceRoot string) string {
	return evidenceRoot + ".partial/secret/attribution"
}

func attributionRelative(eventID string) string {
	digest := sha256.Sum256([]byte(eventID))
	return "attribution/" + hex.EncodeToString(digest[:]) + ".json"
}

func writeAttribution(evidenceRoot, eventID, owner string) (string, error) {
	source := attributionSources[0]
	for _, candidate := range attributionSources {
		if candidate.Owner == owner {
			source = candidate
		}
	}
	record := attributionRecord{Schema: "ardents-h3-attribution-v1", EventID: eventID,
		ProcessIdentity: source.ProcessIdentity, CgroupIdentity: source.CgroupIdentity,
		Observer: "external-boundary-observer"}
	path := filepath.Join(evidenceRoot+".partial/secret", filepath.FromSlash(attributionRelative(eventID)))
	if err := writeJSON(path, record); err != nil {
		return "", err
	}
	hash, _, err := hashFile(path)
	return hash, err
}

func fixtureOwner(mode, eventID string) string {
	if mode == "final-campaign" {
		if strings.Contains(eventID, "/collector-loss/") || strings.Contains(eventID, "/blocker-loss/") ||
			strings.Contains(eventID, "/pipeline-contamination-") {
			return "harness"
		}
		return "candidate"
	}
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

func attributionCommitments(secretRoot string, events []event) ([]artifactCommitment, error) {
	result := make([]artifactCommitment, 0, len(events))
	for _, event := range events {
		value, err := commitment(secretRoot, attributionRelative(event.ID))
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, nil
}
