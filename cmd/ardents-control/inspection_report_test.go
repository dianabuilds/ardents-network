package main

import (
	"bytes"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/alphacontrol"
	"github.com/dianabuilds/ardents-network/internal/alphacontrol/inspection"
)

func TestBundleInspectionReportExposesExactVerifiedControlInputs(t *testing.T) {
	validFrom := time.Date(2030, time.January, 2, 3, 4, 5, 0, time.UTC)
	validUntil := validFrom.Add(time.Hour)
	report := inspection.Report{
		Inspection: alphacontrol.Inspection{CatalogDigest: [32]byte{0xab}, Catalog: alphacontrol.OutcomeAccepted,
			Components: [3]alphacontrol.ComponentInspection{
				{Class: alphacontrol.ComponentRelease, Outcome: alphacontrol.OutcomeAccepted},
				{Class: alphacontrol.ComponentNetwork, Outcome: alphacontrol.OutcomeAccepted},
				{Class: alphacontrol.ComponentCompatibility, Outcome: alphacontrol.OutcomeAccepted},
			}},
		CatalogCohort: "closed-cohort-1", CatalogGeneration: 7, CatalogNotBefore: validFrom, CatalogNotAfter: validUntil,
		ComponentDetails: [3]inspection.ComponentDetails{
			{RootID: [32]byte{0x11}, Generation: 8, Digest: [32]byte{0x21}, NotBefore: validFrom, NotAfter: validUntil},
			{RootID: [32]byte{0x12}, Generation: 9, Digest: [32]byte{0x22}, NotBefore: validFrom, NotAfter: validUntil},
			{RootID: [32]byte{0x13}, Generation: 10, Digest: [32]byte{0x23}, NotBefore: validFrom, NotAfter: validUntil},
		},
		Release: "release-accepted", ReleaseIdentity: "ardents-alpha-1", BuildIdentity: "build-7",
		ArtifactDigest: [32]byte{0xcd}, ProtocolPhase: "required",
		BuildSafetyNoNewWorkAfter: validFrom.Add(15 * time.Minute), BuildSafetyTerminateAfter: validFrom.Add(30 * time.Minute),
		ReleaseAuthorizationPresent: true,
		NetworkID:                   [32]byte{0xef}, NetworkEpoch: 11, NetworkDigest: [32]byte{0x31}, NetworkProfile: "ardents-interactive-route-v1",
		NetworkValidUntil: validFrom.Add(45 * time.Minute),
	}
	var output bytes.Buffer
	if err := writeBundleInspectionReport(&output, report); err != nil {
		t.Fatal(err)
	}
	want := "{\"schema\":\"ardents-alpha-control-report-v1\",\"catalog\":\"accepted\",\"catalog_identity\":\"ab00000000000000000000000000000000000000000000000000000000000000\",\"catalog_cohort\":\"closed-cohort-1\",\"catalog_generation\":7,\"catalog_not_before\":\"2030-01-02T03:04:05Z\",\"catalog_not_after\":\"2030-01-02T04:04:05Z\",\"components\":[{\"class\":1,\"outcome\":\"accepted\",\"root_id\":\"1100000000000000000000000000000000000000000000000000000000000000\",\"generation\":8,\"digest\":\"2100000000000000000000000000000000000000000000000000000000000000\",\"not_before\":\"2030-01-02T03:04:05Z\",\"not_after\":\"2030-01-02T04:04:05Z\"},{\"class\":2,\"outcome\":\"accepted\",\"root_id\":\"1200000000000000000000000000000000000000000000000000000000000000\",\"generation\":9,\"digest\":\"2200000000000000000000000000000000000000000000000000000000000000\",\"not_before\":\"2030-01-02T03:04:05Z\",\"not_after\":\"2030-01-02T04:04:05Z\"},{\"class\":3,\"outcome\":\"accepted\",\"root_id\":\"1300000000000000000000000000000000000000000000000000000000000000\",\"generation\":10,\"digest\":\"2300000000000000000000000000000000000000000000000000000000000000\",\"not_before\":\"2030-01-02T03:04:05Z\",\"not_after\":\"2030-01-02T04:04:05Z\"}],\"release\":\"release-accepted\",\"release_identity\":\"ardents-alpha-1\",\"build_identity\":\"build-7\",\"artifact_digest\":\"cd00000000000000000000000000000000000000000000000000000000000000\",\"protocol_phase\":\"required\",\"build_safety_no_new_work_after\":\"2030-01-02T03:19:05Z\",\"build_safety_terminate_after\":\"2030-01-02T03:34:05Z\",\"release_authorization_present\":true,\"network_id\":\"ef00000000000000000000000000000000000000000000000000000000000000\",\"network_epoch\":11,\"network_digest\":\"3100000000000000000000000000000000000000000000000000000000000000\",\"network_profile\":\"ardents-interactive-route-v1\",\"network_valid_until\":\"2030-01-02T03:49:05Z\"}\n"
	if output.String() != want {
		t.Fatalf("bundle inspection report:\n%s\nwant:\n%s", output.String(), want)
	}
}
