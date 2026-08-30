package main

import (
	"encoding/hex"
	"encoding/json"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/alphacontrol"
	"github.com/dianabuilds/ardents-network/internal/alphacontrol/inspection"
)

type bundleInspectionOutput struct {
	Schema                      string                             `json:"schema"`
	Catalog                     alphacontrol.Outcome               `json:"catalog"`
	CatalogIdentity             string                             `json:"catalog_identity"`
	CatalogCohort               string                             `json:"catalog_cohort"`
	CatalogGeneration           uint64                             `json:"catalog_generation"`
	CatalogNotBefore            string                             `json:"catalog_not_before"`
	CatalogNotAfter             string                             `json:"catalog_not_after"`
	Components                  [3]bundleComponentInspectionOutput `json:"components"`
	Release                     string                             `json:"release"`
	ReleaseIdentity             string                             `json:"release_identity"`
	BuildIdentity               string                             `json:"build_identity"`
	ArtifactDigest              string                             `json:"artifact_digest"`
	ProtocolPhase               string                             `json:"protocol_phase"`
	BuildSafetyNoNewWorkAfter   string                             `json:"build_safety_no_new_work_after"`
	BuildSafetyTerminateAfter   string                             `json:"build_safety_terminate_after"`
	ReleaseAuthorizationPresent bool                               `json:"release_authorization_present"`
	NetworkID                   string                             `json:"network_id"`
	NetworkEpoch                uint64                             `json:"network_epoch"`
	NetworkDigest               string                             `json:"network_digest"`
	NetworkProfile              string                             `json:"network_profile"`
	NetworkValidUntil           string                             `json:"network_valid_until"`
}

type bundleComponentInspectionOutput struct {
	Class      alphacontrol.ComponentClass `json:"class"`
	Outcome    alphacontrol.Outcome        `json:"outcome"`
	RootID     string                      `json:"root_id"`
	Generation uint64                      `json:"generation"`
	Digest     string                      `json:"digest"`
	NotBefore  string                      `json:"not_before"`
	NotAfter   string                      `json:"not_after"`
}

// transitionInspectionOutput makes the four alpha-control transition
// contracts visible next to the non-authorizing control inspection. It is a
// diagnostic report, never an input to Endpoint readiness or control authority.
type transitionInspectionOutput struct {
	Schema      string                      `json:"schema"`
	Control     bundleInspectionOutput      `json:"control"`
	Transitions []transitionInspectionEntry `json:"transitions"`
}

type transitionInspectionEntry struct {
	Domain      alphacontrol.TransitionDomain `json:"domain"`
	Selected    bool                          `json:"selected"`
	Outcome     string                        `json:"outcome"`
	UserFailure string                        `json:"user_failure"`
	Evidence    string                        `json:"evidence"`
}

func writeBundleInspectionReport(output io.Writer, report inspection.Report) error {
	return json.NewEncoder(output).Encode(bundleInspectionReport(report))
}

func bundleInspectionReport(report inspection.Report) bundleInspectionOutput {
	encoded := bundleInspectionOutput{
		Schema: "ardents-alpha-control-report-v1", Catalog: report.Inspection.Catalog,
		CatalogIdentity: hex.EncodeToString(report.Inspection.CatalogDigest[:]), CatalogCohort: report.CatalogCohort,
		CatalogGeneration: report.CatalogGeneration, CatalogNotBefore: reportTime(report.CatalogNotBefore),
		CatalogNotAfter: reportTime(report.CatalogNotAfter), Release: report.Release, ReleaseIdentity: report.ReleaseIdentity,
		BuildIdentity: report.BuildIdentity, ArtifactDigest: hex.EncodeToString(report.ArtifactDigest[:]), ProtocolPhase: report.ProtocolPhase,
		BuildSafetyNoNewWorkAfter: reportTime(report.BuildSafetyNoNewWorkAfter),
		BuildSafetyTerminateAfter: reportTime(report.BuildSafetyTerminateAfter), ReleaseAuthorizationPresent: report.ReleaseAuthorizationPresent,
		NetworkID: hex.EncodeToString(report.NetworkID[:]), NetworkEpoch: report.NetworkEpoch,
		NetworkDigest: hex.EncodeToString(report.NetworkDigest[:]), NetworkProfile: report.NetworkProfile,
		NetworkValidUntil: reportTime(report.NetworkValidUntil),
	}
	for index, component := range report.ComponentDetails {
		encoded.Components[index] = bundleComponentInspectionOutput{
			Class: report.Inspection.Components[index].Class, Outcome: report.Inspection.Components[index].Outcome,
			RootID: hex.EncodeToString(component.RootID[:]), Generation: component.Generation,
			Digest: hex.EncodeToString(component.Digest[:]), NotBefore: reportTime(component.NotBefore), NotAfter: reportTime(component.NotAfter),
		}
	}
	return encoded
}

func transitionInspectionReport(report inspection.Report) transitionInspectionOutput {
	contracts := alphacontrol.TransitionContracts()
	result := transitionInspectionOutput{
		Schema: "ardents-alpha-transition-report-v1", Control: bundleInspectionReport(report),
		Transitions: make([]transitionInspectionEntry, len(contracts)),
	}
	for index, contract := range contracts {
		outcome := "not-selected"
		if contract.Selected {
			outcome = transitionOutcome(report, contract.Domain)
		}
		result.Transitions[index] = transitionInspectionEntry{Domain: contract.Domain, Selected: contract.Selected,
			Outcome: outcome, UserFailure: contract.UserFailure, Evidence: contract.Evidence}
	}
	return result
}

func transitionOutcome(report inspection.Report, domain alphacontrol.TransitionDomain) string {
	if report.Inspection.Catalog != alphacontrol.OutcomeAccepted {
		return classifyTransitionOutcome(report.Inspection.Catalog)
	}
	if domain == alphacontrol.DomainCompatibility && report.Release == "release-revoked" {
		return "revoked"
	}
	component, selected := alphacontrol.ComponentForTransitionDomain(domain)
	if !selected {
		return "forged"
	}
	index := int(component) - int(alphacontrol.ComponentRelease)
	if index < 0 || index >= len(report.Inspection.Components) {
		return "forged"
	}
	if domain == alphacontrol.DomainReleaseSafety && report.Release == "release-revoked" {
		return "revoked"
	}
	return classifyTransitionOutcome(report.Inspection.Components[index].Outcome)
}

func classifyTransitionOutcome(outcome alphacontrol.Outcome) string {
	switch outcome {
	case alphacontrol.OutcomeAccepted:
		return "accepted"
	case alphacontrol.OutcomeExpired:
		return "stale"
	case alphacontrol.OutcomeLowerFloor:
		return "replayed"
	case alphacontrol.OutcomeConflict:
		return "conflicting"
	case alphacontrol.OutcomeUnavailable:
		return "unavailable"
	default:
		return "forged"
	}
}

func reportTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
