package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
)

const schema = "ardents-h3-service-evidence-v1"

// Verdict is the independent terminal machine result.
type Verdict struct {
	Schema         string `json:"schema"`
	Verdict        string `json:"verdict"`
	Reason         string `json:"reason"`
	EvidenceDigest string `json:"evidence_digest"`
}

// Verify decodes and independently checks one bounded Stage 3 evidence value.
func Verify(raw []byte) Verdict {
	invalid := func(reason string) Verdict {
		return Verdict{Schema: "ardents-h3-service-verdict-v1", Verdict: "invalid", Reason: reason}
	}
	if len(raw) == 0 || len(raw) > 4<<20 {
		return invalid("evidence is empty or exceeds 4 MiB")
	}
	var input candidate
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return invalid("evidence JSON is malformed or non-canonical")
	}
	claimed, err := hex.DecodeString(input.EvidenceDigest)
	if err != nil || len(claimed) != sha256.Size {
		return invalid("evidence digest is malformed")
	}
	input.EvidenceDigest = ""
	canonical, err := json.Marshal(input)
	if err != nil {
		return invalid("evidence cannot be canonicalized")
	}
	digest := sha256.Sum256(canonical)
	if !bytes.Equal(claimed, digest[:]) {
		return invalid("evidence digest does not match")
	}
	if err := validateEvidence(input); err != nil {
		return invalid(err.Error())
	}
	if err := validateCandidate(input); err != nil {
		return Verdict{Schema: "ardents-h3-service-verdict-v1", Verdict: "fail", Reason: err.Error(),
			EvidenceDigest: hex.EncodeToString(digest[:])}
	}
	return Verdict{Schema: "ardents-h3-service-verdict-v1", Verdict: "pass",
		Reason:         "bounded Stage 3 Service Connection evidence passed every conjunct",
		EvidenceDigest: hex.EncodeToString(digest[:])}
}

func validateEvidence(input candidate) error {
	if input.Schema != schema || input.SourceCommit == "" || input.ImageID == "" || input.ManifestDigest == "" ||
		input.NetworkID == [32]byte{} || input.AuthorityPublic == [32]byte{} || input.Target == [32]byte{} ||
		len(input.Generations) != 2 || !input.PrivateMaterialAbsent {
		return errors.New("identity, schema, generation, or secret-handling evidence is incomplete")
	}
	for _, required := range requiredNegatives {
		if _, ok := input.Negatives[required]; !ok {
			return errors.New("mandatory negative evidence is missing: " + required)
		}
	}
	for _, required := range requiredShortcuts {
		if _, ok := input.ShortcutsAbsent[required]; !ok {
			return errors.New("forbidden-path evidence is missing: " + required)
		}
	}
	for _, required := range requiredCleanup {
		if _, ok := input.Cleanup[required]; !ok {
			return errors.New("cleanup evidence is missing: " + required)
		}
	}
	for _, generation := range input.Generations {
		if generation.IntroductionAcknowledgement == [32]byte{} || !generation.PublicationReady ||
			len(generation.Roles) != 6 || len(generation.ContainerIDs) != 12 {
			return errors.New("publication or process evidence is incomplete")
		}
		roles, runtimes := map[string]bool{}, map[string]bool{}
		for _, role := range generation.Roles {
			if role.Role == "" || role.PID < 1 || role.RuntimeID == "" || roles[role.Role] || runtimes[role.RuntimeID] {
				return errors.New("route process separation evidence is contradictory")
			}
			roles[role.Role], runtimes[role.RuntimeID] = true, true
		}
		for _, required := range routeRoles {
			if !roles[required] {
				return errors.New("route process evidence is missing: " + required)
			}
		}
		seen := map[string]bool{}
		for _, identity := range generation.ContainerIDs {
			if identity == "" || seen[identity] {
				return errors.New("container separation evidence is incomplete or repeated")
			}
			seen[identity] = true
		}
	}
	return nil
}
