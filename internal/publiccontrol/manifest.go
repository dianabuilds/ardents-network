package publiccontrol

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	manifestSchema = "ardents-public-control-evidence-v1"
	// MaximumEvidenceManifestSize bounds one untrusted public-control evidence
	// manifest before JSON parsing.
	MaximumEvidenceManifestSize = 1 << 20
)

// Outcome describes one public-control evidence-reader result. The reader has no
// qualified result because factual independent operation is external evidence.
type Outcome string

const (
	OutcomeForged                   Outcome = "forged"
	OutcomeReplayed                 Outcome = "replayed"
	OutcomeStale                    Outcome = "stale"
	OutcomeConflicting              Outcome = "conflicting"
	OutcomeRevoked                  Outcome = "revoked"
	OutcomeUnavailable              Outcome = "unavailable"
	OutcomeIndependenceConflict     Outcome = "independence-conflict"
	OutcomeExternalEvidenceRequired Outcome = "external-evidence-required"
)

// Finding makes one reader conclusion visible with its precise manifest cause.
type Finding struct {
	Outcome Outcome `json:"outcome"`
	Detail  string  `json:"detail"`
}

// Report is the bounded, diagnostic output for one declared evidence package.
// It is intentionally unsuitable as Endpoint, Release, Epoch, or Namespace
// authority input.
type Report struct {
	Schema    string    `json:"schema"`
	Candidate string    `json:"candidate"`
	Outcomes  []Outcome `json:"outcomes"`
	Findings  []Finding `json:"findings"`
	Qualified bool      `json:"qualified"`
}

// InspectionConfig supplies the reader-owned time and retained transition floor
// for one diagnostic inspection. It is not authority or an Endpoint floor.
type InspectionConfig struct {
	At                   time.Time
	AuditFloorGeneration uint64
	ExpectedPredecessor  string
}

type manifest struct {
	Schema        string         `json:"schema"`
	Candidate     string         `json:"candidate"`
	Transition    transition     `json:"transition"`
	Custody       custody        `json:"custody"`
	CandidateView candidateView  `json:"candidate_view"`
	Builders      []actor        `json:"builders"`
	Auditors      []actor        `json:"auditors"`
	Packages      []buildPackage `json:"packages"`
}

type transition struct {
	Generation  uint64 `json:"generation"`
	Predecessor string `json:"predecessor"`
	NotAfter    string `json:"not_after"`
	Revoked     bool   `json:"revoked"`
	Conflicting bool   `json:"conflicting"`
}

type custody struct {
	Threshold          int     `json:"threshold"`
	EmergencyThreshold int     `json:"emergency_threshold"`
	Members            []actor `json:"members"`
}

type actor struct {
	ID             string `json:"id"`
	PublicKey      string `json:"public_key"`
	Operator       string `json:"operator"`
	Organization   string `json:"organization"`
	Administration string `json:"administration"`
	Evidence       string `json:"evidence"`
}

type candidateView struct {
	Epoch                string          `json:"epoch"`
	InputLog             string          `json:"input_log"`
	MaterializationRules string          `json:"materialization_rules"`
	Audits               []auditEvidence `json:"audits"`
}

type auditEvidence struct {
	Auditor  string `json:"auditor"`
	InputLog string `json:"input_log"`
	Output   string `json:"output"`
}

type buildPackage struct {
	Artifact            string               `json:"artifact"`
	Source              string               `json:"source"`
	Dependencies        string               `json:"dependencies"`
	Recipe              string               `json:"recipe"`
	SBOM                string               `json:"sbom"`
	Qualification       string               `json:"qualification"`
	BuilderAttestations []builderAttestation `json:"builder_attestations"`
}

type builderAttestation struct {
	Builder  string `json:"builder"`
	Artifact string `json:"artifact"`
}

// Inspect parses and mechanically checks one declared public-control evidence manifest.
// It always returns external-evidence-required for a well-formed manifest: no
// local reader can establish whether people or organizations are independent.
func Inspect(raw []byte) (Report, error) {
	return InspectAt(raw, InspectionConfig{})
}

// InspectAt additionally evaluates one public transition against a supplied
// inspection time and independently retained generation floor.
func InspectAt(raw []byte, config InspectionConfig) (Report, error) {
	if len(raw) == 0 || len(raw) > MaximumEvidenceManifestSize {
		return Report{}, errors.New("public-control evidence manifest size is invalid")
	}
	var value manifest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return Report{}, fmt.Errorf("decode public-control evidence manifest: %w", err)
	}
	if decoder.More() {
		return Report{}, errors.New("public-control evidence manifest has trailing data")
	}
	if value.Schema != manifestSchema || !validDigest(value.Candidate) {
		return Report{}, errors.New("public-control evidence manifest identity is invalid")
	}

	report := Report{Schema: manifestSchema, Candidate: value.Candidate}
	checker := manifestChecker{report: &report}
	checker.transition(value.Transition, config)
	checker.custody(value.Custody)
	checker.actors("builder", value.Builders, 2)
	checker.actors("auditor", value.Auditors, 2)
	checker.boundaries(value.Custody.Members, value.Builders, value.Auditors)
	checker.candidateView(value.CandidateView, value.Auditors)
	checker.packages(value.Packages, value.Builders)
	checker.add(OutcomeExternalEvidenceRequired, "real independent custody, build, and audit operation needs external corroboration")
	return report, nil
}

type manifestChecker struct {
	report *Report
}

func (checker manifestChecker) transition(value transition, config InspectionConfig) {
	if value.Generation == 0 || !validDigest(value.Predecessor) {
		checker.add(OutcomeForged, "transition lacks a positive generation or exact predecessor identity")
		return
	}
	notAfter, err := time.Parse(time.RFC3339, value.NotAfter)
	if err != nil || !notAfter.Equal(notAfter.UTC()) {
		checker.add(OutcomeForged, "transition expiry is not canonical RFC3339 UTC")
		return
	}
	if !config.At.IsZero() && !config.At.UTC().Before(notAfter) {
		checker.add(OutcomeStale, "transition evidence is expired at the inspection time")
	}
	if config.AuditFloorGeneration > value.Generation {
		checker.add(OutcomeReplayed, "transition generation is below the supplied audit floor")
	}
	if config.ExpectedPredecessor != "" && config.ExpectedPredecessor != value.Predecessor {
		checker.add(OutcomeReplayed, "transition predecessor differs from the supplied audit predecessor")
	}
	if value.Revoked {
		checker.add(OutcomeRevoked, "transition evidence declares the candidate revoked")
	}
	if value.Conflicting {
		checker.add(OutcomeConflicting, "transition evidence declares a conflicting successor or auditor result")
	}
}

func (checker manifestChecker) custody(value custody) {
	if len(value.Members) != 5 {
		checker.add(OutcomeUnavailable, "custody needs exactly five declared members")
	}
	if value.Threshold != 3 || value.EmergencyThreshold != 4 {
		checker.add(OutcomeForged, "custody threshold must be 3-of-5 with a 4-of-5 emergency")
	}
	checker.actors("custodian", value.Members, 5)
}

func (checker manifestChecker) actors(role string, values []actor, count int) {
	if len(values) != count {
		checker.add(OutcomeUnavailable, fmt.Sprintf("%s evidence needs exactly %d declared actors", role, count))
	}
	for index, value := range values {
		if !validIdentifier(value.ID) || !validAuthorityKey(value.PublicKey) || !validBoundary(value.Operator) || !validBoundary(value.Organization) ||
			!validBoundary(value.Administration) || !validDigest(value.Evidence) {
			checker.add(OutcomeForged, fmt.Sprintf("%s %d has malformed identity, boundary, or evidence digest", role, index+1))
		}
	}
}

func (checker manifestChecker) boundaries(groups ...[]actor) {
	seenIDs := make(map[string]bool)
	seenKeys := make(map[string]bool)
	seenBoundaries := make(map[string]bool)
	for _, values := range groups {
		for _, value := range values {
			if seenIDs[value.ID] {
				checker.add(OutcomeIndependenceConflict, "one declared actor identifier occupies multiple control roles")
			}
			seenIDs[value.ID] = true
			if seenKeys[value.PublicKey] {
				checker.add(OutcomeIndependenceConflict, "one declared authority public key occupies multiple control roles")
			}
			seenKeys[value.PublicKey] = true
			for _, boundary := range []string{value.Operator, value.Organization, value.Administration} {
				if seenBoundaries[boundary] {
					checker.add(OutcomeIndependenceConflict, "a declared operator, organization, or administration boundary is shared")
				}
				seenBoundaries[boundary] = true
			}
		}
	}
}

func (checker manifestChecker) candidateView(value candidateView, auditors []actor) {
	if !validDigest(value.Epoch) || !validDigest(value.InputLog) || !validDigest(value.MaterializationRules) {
		checker.add(OutcomeUnavailable, "Candidate View lacks an Epoch, input-log, or materialization-rule artifact")
	}
	if len(value.Audits) != 2 {
		checker.add(OutcomeUnavailable, "Candidate View needs two full-auditor outputs")
	}
	known := make(map[string]bool, len(auditors))
	for _, auditor := range auditors {
		known[auditor.ID] = true
	}
	seen := make(map[string]bool)
	for _, audit := range value.Audits {
		if !known[audit.Auditor] || seen[audit.Auditor] || audit.InputLog != value.InputLog || !validDigest(audit.Output) {
			checker.add(OutcomeConflicting, "full-auditor evidence does not bind two declared auditors to the same complete input log")
		}
		seen[audit.Auditor] = true
	}
}

func (checker manifestChecker) packages(values []buildPackage, builders []actor) {
	if len(values) == 0 {
		checker.add(OutcomeUnavailable, "candidate has no reproducible package evidence")
	}
	known := make(map[string]bool, len(builders))
	for _, builder := range builders {
		known[builder.ID] = true
	}
	for _, value := range values {
		for _, digest := range []string{value.Artifact, value.Source, value.Dependencies, value.Recipe, value.SBOM, value.Qualification} {
			if !validDigest(digest) {
				checker.add(OutcomeUnavailable, "package lacks a retained reproducibility artifact")
				break
			}
		}
		if len(value.BuilderAttestations) != 2 {
			checker.add(OutcomeUnavailable, "package needs two matching builder attestations")
		}
		seen := make(map[string]bool)
		for _, attestation := range value.BuilderAttestations {
			if !known[attestation.Builder] || seen[attestation.Builder] || attestation.Artifact != value.Artifact {
				checker.add(OutcomeConflicting, "builder attestation does not bind two declared builders to the package artifact")
			}
			seen[attestation.Builder] = true
		}
	}
}

func (checker manifestChecker) add(outcome Outcome, detail string) {
	for _, finding := range checker.report.Findings {
		if finding.Outcome == outcome && finding.Detail == detail {
			return
		}
	}
	checker.report.Findings = append(checker.report.Findings, Finding{Outcome: outcome, Detail: detail})
	for _, present := range checker.report.Outcomes {
		if present == outcome {
			return
		}
	}
	checker.report.Outcomes = append(checker.report.Outcomes, outcome)
}

func validIdentifier(value string) bool {
	if len(value) == 0 || len(value) > 96 {
		return false
	}
	for _, character := range value {
		if !((character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '-') {
			return false
		}
	}
	return true
}

func validBoundary(value string) bool {
	return validIdentifier(value)
}

func validDigest(value string) bool {
	if !strings.HasPrefix(value, "sha256:") || len(value) != len("sha256:")+64 {
		return false
	}
	for _, character := range value[len("sha256:"):] {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
}

func validAuthorityKey(value string) bool {
	const prefix = "ed25519:"
	if !strings.HasPrefix(value, prefix) || len(value) != len(prefix)+64 {
		return false
	}
	for _, character := range value[len(prefix):] {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
}
