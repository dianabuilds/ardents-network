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

func writeBundleInspectionReport(output io.Writer, report inspection.Report) error {
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
	return json.NewEncoder(output).Encode(encoded)
}

func reportTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
