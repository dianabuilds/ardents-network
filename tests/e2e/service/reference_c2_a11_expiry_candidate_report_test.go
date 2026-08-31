//go:build referencec2 && h4_8_a11

package service_test

import (
	"encoding/hex"
	"time"

	"github.com/dianabuilds/ardents-network/internal/alphacontrol/inspection"
	"github.com/dianabuilds/ardents-network/internal/release"
)

type h48A11ExpiryCandidateV2Report struct {
	Schema         string                        `json:"schema"`
	EndpointSHA256 string                        `json:"endpoint_sha256"`
	ControlSHA256  string                        `json:"control_sha256"`
	Identity       h48A11CandidateIdentityReport `json:"identity"`
	Bounds         h48A11CandidateBoundsReport   `json:"bounds"`
	Points         h48A11CandidatePointsReport   `json:"points"`
	Status         string                        `json:"status"`
}

type h48A11CandidateIdentityReport struct {
	Catalog    h48A11CandidateCatalogReport     `json:"catalog"`
	Components []h48A11CandidateComponentReport `json:"components"`
	Release    h48A11CandidateReleaseReport     `json:"release"`
	Network    h48A11CandidateNetworkReport     `json:"network"`
}

type h48A11CandidateCatalogReport struct {
	Cohort     string `json:"cohort"`
	Generation uint64 `json:"generation"`
	Digest     string `json:"digest"`
	NotBefore  string `json:"not_before"`
	NotAfter   string `json:"not_after"`
}

type h48A11CandidateComponentReport struct {
	Class      string `json:"class"`
	RootID     string `json:"root_id"`
	Generation uint64 `json:"generation"`
	Digest     string `json:"digest"`
	NotBefore  string `json:"not_before"`
	NotAfter   string `json:"not_after"`
}

type h48A11CandidateReleaseReport struct {
	Identity       string `json:"identity"`
	BuildIdentity  string `json:"build_identity"`
	ArtifactDigest string `json:"artifact_digest"`
	Protocol       string `json:"protocol"`
}

type h48A11CandidateNetworkReport struct {
	ID      string `json:"id"`
	Epoch   uint64 `json:"epoch"`
	Digest  string `json:"digest"`
	Profile string `json:"profile"`
}

type h48A11CandidateBoundsReport struct {
	ReferenceAt       string                     `json:"reference_at"`
	NoNewWorkAfter    string                     `json:"no_new_work_after"`
	Terminal          string                     `json:"terminal"`
	NetworkValidUntil string                     `json:"network_valid_until"`
	TUFExpires        h48A11CandidateTUFExpiries `json:"tuf_expires"`
}

type h48A11CandidateTUFExpiries struct {
	Timestamp string `json:"timestamp"`
	Snapshot  string `json:"snapshot"`
	Targets   string `json:"targets"`
}

type h48A11CandidatePointsReport struct {
	Reference        h48A11CandidateAcceptedPoint      `json:"reference"`
	NoNewMinusOne    h48A11CandidateAcceptedPoint      `json:"no_new_minus_one"`
	NoNewExact       h48A11CandidateNoNewExactPoint    `json:"no_new_exact"`
	TerminalMinusOne h48A11CandidateAcceptedPoint      `json:"terminal_minus_one"`
	TerminalExact    h48A11CandidateTerminalExactPoint `json:"terminal_exact"`
	TerminalPlusOne  h48A11CandidateDirectPoint        `json:"terminal_plus_one"`
}

type h48A11CandidateAcceptedPoint struct {
	At            string                          `json:"at"`
	Inspection    h48A11CandidateInspectionReport `json:"inspection"`
	DirectRelease h48A11CandidateDirectRelease    `json:"direct_release"`
}

type h48A11CandidateNoNewExactPoint struct {
	At                  string                          `json:"at"`
	FreshInspection     h48A11CandidateInspectionReport `json:"fresh_inspection"`
	PersistedInspection h48A11CandidateInspectionReport `json:"persisted_inspection"`
	PersistedRetry      h48A11CandidateInspectionReport `json:"persisted_retry"`
	DirectRelease       h48A11CandidateDirectRelease    `json:"direct_release"`
	DirectRetry         h48A11CandidateDirectRelease    `json:"direct_retry"`
}

type h48A11CandidateTerminalExactPoint struct {
	At                      string                        `json:"at"`
	FreshCatalogOutcome     string                        `json:"fresh_catalog_outcome"`
	FreshFullRefused        bool                          `json:"fresh_full_refused"`
	PersistedCatalogOutcome string                        `json:"persisted_catalog_outcome"`
	PersistedFullRefused    bool                          `json:"persisted_full_refused"`
	DirectComponentOutcomes []h48A11CandidateClassOutcome `json:"direct_component_outcomes"`
	DirectRelease           h48A11CandidateDirectRelease  `json:"direct_release"`
}

type h48A11CandidateDirectPoint struct {
	At            string                       `json:"at"`
	DirectRelease h48A11CandidateDirectRelease `json:"direct_release"`
}

type h48A11CandidateInspectionReport struct {
	CatalogOutcome    string                        `json:"catalog_outcome"`
	ComponentOutcomes []h48A11CandidateClassOutcome `json:"component_outcomes"`
	ReleaseOutcome    string                        `json:"release_outcome"`
	ReleaseAuthorized bool                          `json:"release_authorized"`
}

type h48A11CandidateClassOutcome struct {
	Class   string `json:"class"`
	Outcome string `json:"outcome"`
}

type h48A11CandidateDirectRelease struct {
	Outcome        string `json:"outcome"`
	BuildSafety    string `json:"build_safety"`
	Authorized     bool   `json:"authorized"`
	NoNewWorkAfter string `json:"no_new_work_after"`
	TerminateAfter string `json:"terminate_after"`
}

func h48A11CandidateIdentity(reference inspection.Report) h48A11CandidateIdentityReport {
	classes := [3]string{"release", "network", "compatibility"}
	components := make([]h48A11CandidateComponentReport, len(classes))
	for index, class := range classes {
		details := reference.ComponentDetails[index]
		components[index] = h48A11CandidateComponentReport{Class: class, RootID: hex.EncodeToString(details.RootID[:]),
			Generation: details.Generation, Digest: hex.EncodeToString(details.Digest[:]), NotBefore: h48A11CandidateTime(details.NotBefore),
			NotAfter: h48A11CandidateTime(details.NotAfter)}
	}
	return h48A11CandidateIdentityReport{
		Catalog: h48A11CandidateCatalogReport{Cohort: reference.CatalogCohort, Generation: reference.CatalogGeneration,
			Digest: hex.EncodeToString(reference.Inspection.CatalogDigest[:]), NotBefore: h48A11CandidateTime(reference.CatalogNotBefore),
			NotAfter: h48A11CandidateTime(reference.CatalogNotAfter)},
		Components: components,
		Release: h48A11CandidateReleaseReport{Identity: reference.ReleaseIdentity, BuildIdentity: reference.BuildIdentity,
			ArtifactDigest: hex.EncodeToString(reference.ArtifactDigest[:]), Protocol: reference.ProtocolPhase},
		Network: h48A11CandidateNetworkReport{ID: hex.EncodeToString(reference.NetworkID[:]), Epoch: reference.NetworkEpoch,
			Digest: hex.EncodeToString(reference.NetworkDigest[:]), Profile: reference.NetworkProfile},
	}
}

func h48A11CandidateInspection(report inspection.Report) h48A11CandidateInspectionReport {
	components := make([]h48A11CandidateClassOutcome, len(report.Inspection.Components))
	for index, component := range report.Inspection.Components {
		components[index] = h48A11CandidateClassOutcome{Class: h48A11CandidateClass(index), Outcome: string(component.Outcome)}
	}
	return h48A11CandidateInspectionReport{CatalogOutcome: string(report.Inspection.Catalog), ComponentOutcomes: components,
		ReleaseOutcome: report.Release, ReleaseAuthorized: report.ReleaseAuthorizationPresent}
}

func h48A11CandidateDirect(decision release.Decision) h48A11CandidateDirectRelease {
	_, authorized := decision.Authorization()
	return h48A11CandidateDirectRelease{Outcome: string(decision.Outcome), BuildSafety: string(decision.BuildSafety), Authorized: authorized,
		NoNewWorkAfter: h48A11CandidateTime(decision.BuildSafetyNoNewWorkAfter), TerminateAfter: h48A11CandidateTime(decision.BuildSafetyTerminateAfter)}
}

func h48A11CandidateClass(index int) string {
	return [3]string{"release", "network", "compatibility"}[index]
}

func h48A11CandidateTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
