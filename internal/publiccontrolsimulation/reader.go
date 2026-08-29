package publiccontrolsimulation

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/dianabuilds/ardents-network/internal/publiccontrol"
)

type readerManifest struct {
	Schema        string              `json:"schema"`
	Candidate     string              `json:"candidate"`
	Transition    readerTransition    `json:"transition"`
	Custody       readerCustody       `json:"custody"`
	CandidateView readerCandidateView `json:"candidate_view"`
	Builders      []readerActor       `json:"builders"`
	Auditors      []readerActor       `json:"auditors"`
	Packages      []readerPackage     `json:"packages"`
}

type readerTransition struct {
	Generation  uint64 `json:"generation"`
	Predecessor string `json:"predecessor"`
	NotAfter    string `json:"not_after"`
	Revoked     bool   `json:"revoked"`
	Conflicting bool   `json:"conflicting"`
}

type readerCustody struct {
	Threshold          int           `json:"threshold"`
	EmergencyThreshold int           `json:"emergency_threshold"`
	Members            []readerActor `json:"members"`
}

type readerActor struct {
	ID             string `json:"id"`
	PublicKey      string `json:"public_key"`
	Operator       string `json:"operator"`
	Organization   string `json:"organization"`
	Administration string `json:"administration"`
	Evidence       string `json:"evidence"`
}

type readerCandidateView struct {
	Epoch                string        `json:"epoch"`
	InputLog             string        `json:"input_log"`
	MaterializationRules string        `json:"materialization_rules"`
	Audits               []readerAudit `json:"audits"`
}

type readerAudit struct {
	Auditor  string `json:"auditor"`
	InputLog string `json:"input_log"`
	Output   string `json:"output"`
}

type readerPackage struct {
	Artifact            string                     `json:"artifact"`
	Source              string                     `json:"source"`
	Dependencies        string                     `json:"dependencies"`
	Recipe              string                     `json:"recipe"`
	SBOM                string                     `json:"sbom"`
	Qualification       string                     `json:"qualification"`
	BuilderAttestations []readerBuilderAttestation `json:"builder_attestations"`
}

type readerBuilderAttestation struct {
	Builder  string `json:"builder"`
	Artifact string `json:"artifact"`
}

func exerciseReaderMatrix() ([]string, []string, error) {
	base := newReaderManifest()
	if err := expectReaderOutcome(base, publiccontrol.InspectionConfig{At: time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC)}, publiccontrol.OutcomeExternalEvidenceRequired); err != nil {
		return nil, nil, err
	}
	checks := []struct {
		name    string
		outcome publiccontrol.Outcome
		mutate  func(*readerManifest)
		config  publiccontrol.InspectionConfig
	}{
		{"reader-forged", publiccontrol.OutcomeForged, func(value *readerManifest) { value.Custody.Members[0].PublicKey = "ed25519:bad" }, publiccontrol.InspectionConfig{}},
		{"reader-stale", publiccontrol.OutcomeStale, func(value *readerManifest) { value.Transition.NotAfter = "2029-01-01T00:00:00Z" }, publiccontrol.InspectionConfig{At: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)}},
		{"reader-replayed", publiccontrol.OutcomeReplayed, func(value *readerManifest) {}, publiccontrol.InspectionConfig{AuditFloorGeneration: 2}},
		{"reader-revoked", publiccontrol.OutcomeRevoked, func(value *readerManifest) { value.Transition.Revoked = true }, publiccontrol.InspectionConfig{}},
		{"reader-conflicting", publiccontrol.OutcomeConflicting, func(value *readerManifest) { value.Transition.Conflicting = true }, publiccontrol.InspectionConfig{}},
		{"reader-unavailable", publiccontrol.OutcomeUnavailable, func(value *readerManifest) { value.Packages = nil }, publiccontrol.InspectionConfig{}},
		{"reader-independence-conflict", publiccontrol.OutcomeIndependenceConflict, func(value *readerManifest) {
			value.Builders[0].Administration = value.Custody.Members[0].Administration
		}, publiccontrol.InspectionConfig{}},
	}
	rejected := make([]string, 0, len(checks)+1)
	for _, check := range checks {
		value := newReaderManifest()
		check.mutate(&value)
		if err := expectReaderOutcome(value, check.config, check.outcome); err != nil {
			return nil, nil, fmt.Errorf("simulation %s: %w", check.name, err)
		}
		rejected = append(rejected, check.name)
	}
	if _, err := publiccontrol.Inspect([]byte("{")); err == nil {
		return nil, nil, fmt.Errorf("simulation reader-malformed was accepted")
	}
	rejected = append(rejected, "reader-malformed")
	return []string{"reader-diagnostic-matrix"}, rejected, nil
}

func expectReaderOutcome(value readerManifest, config publiccontrol.InspectionConfig, expected publiccontrol.Outcome) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	report, err := publiccontrol.InspectAt(raw, config)
	if err != nil {
		return err
	}
	for _, outcome := range report.Outcomes {
		if outcome == expected {
			return nil
		}
	}
	return fmt.Errorf("missing %s outcome: %+v", expected, report.Outcomes)
}

func newReaderManifest() readerManifest {
	digest := digest([]byte("reader-evidence"))
	actors := func(role string, count int) []readerActor {
		result := make([]readerActor, count)
		for index := range result {
			fill := fmt.Sprintf("%x", index+1)
			result[index] = readerActor{ID: fmt.Sprintf("%s-%d", role, index+1), PublicKey: "ed25519:" + repeatHex(fill), Operator: fmt.Sprintf("%s-operator-%d", role, index+1), Organization: fmt.Sprintf("%s-organization-%d", role, index+1), Administration: fmt.Sprintf("%s-administration-%d", role, index+1), Evidence: digest}
		}
		return result
	}
	custodians, builders, auditors := actors("custodian", 5), actors("builder", 2), actors("auditor", 2)
	return readerManifest{Schema: "ardents-public-control-evidence-v1", Candidate: digest,
		Transition: readerTransition{Generation: 1, Predecessor: digest, NotAfter: "2031-01-01T00:00:00Z"}, Custody: readerCustody{Threshold: 3, EmergencyThreshold: 4, Members: custodians},
		CandidateView: readerCandidateView{Epoch: digest, InputLog: digest, MaterializationRules: digest, Audits: []readerAudit{{Auditor: auditors[0].ID, InputLog: digest, Output: digest}, {Auditor: auditors[1].ID, InputLog: digest, Output: digest}}},
		Builders:      builders, Auditors: auditors, Packages: []readerPackage{{Artifact: digest, Source: digest, Dependencies: digest, Recipe: digest, SBOM: digest, Qualification: digest, BuilderAttestations: []readerBuilderAttestation{{Builder: builders[0].ID, Artifact: digest}, {Builder: builders[1].ID, Artifact: digest}}}}}
}

func repeatHex(value string) string {
	result := ""
	for len(result) < 64 {
		result += value
	}
	return result[:64]
}
