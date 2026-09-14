//go:build linux

package endpoint

import (
	"crypto/sha256"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/custody"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func TestTextNetworkFixtureWindowAvoidsExpiredPermissionImport(t *testing.T) {
	boundary := time.Date(2026, time.September, 14, 23, 59, 45, 0, time.UTC)

	// This was the old fixture behavior: the request is valid when Custody signs
	// it, then correctly rejected when the Endpoint imports it after midnight.
	current := boundary
	owner, response, closeOwner := textNetworkFixturePermission(t, &current)
	defer closeOwner()
	current = boundary.Add(20 * time.Second)
	if err := owner.importTextPermission(response.digest, response.raw); err == nil {
		t.Fatal("expired fixture permission imported across its hour boundary")
	}

	start, end := textNetworkFixtureWindow(boundary)
	if !start.Equal(boundary.Truncate(time.Hour).Add(time.Hour)) || end.Sub(start) != time.Hour {
		t.Fatalf("fixture window = %v to %v", start, end)
	}
	current = start
	owner, response, closeOwner = textNetworkFixturePermission(t, &current)
	defer closeOwner()
	current = start.Add(textNetworkFixtureMinimumWindow - time.Second)
	if err := owner.importTextPermission(response.digest, response.raw); err != nil {
		t.Fatalf("fixture permission did not cover its bounded carrier episode: %v", err)
	}
}

type textNetworkFixturePermissionResponse struct {
	digest [32]byte
	raw    []byte
}

func textNetworkFixturePermission(t *testing.T, current *time.Time) (*textContext, textNetworkFixturePermissionResponse, func()) {
	t.Helper()
	vault, err := custody.Open(custody.VaultConfig{Root: t.TempDir(), Now: func() time.Time { return *current }})
	if err != nil {
		t.Fatal(err)
	}
	created, err := vault.Execute(t.Context(), custody.Operation{Kind: custody.OperationCreateAdmissionAuthority,
		Authority: custody.AuthorityState{Binding: custody.AuthorityBinding{Environment: fixtureID(241), Network: fixtureID(242), Root: fixtureID(243), Kind: custody.AuthorityAdmission}}}, textPermissionSecretFixture{})
	if err != nil {
		_ = vault.Close()
		t.Fatal(err)
	}
	endpoint, principal := textContextEndpoint(t)
	endpoint.network, endpoint.clock = fixtureID(242), func() time.Time { return *current }
	endpoint.closedState = &textPermissionStateFixture{profile: state.ClosedProfileView{NetworkID: endpoint.network,
		StateGeneration: fixtureID(244), StateDigest: fixtureID(245), Digest: fixtureID(246), IssuanceAuthorityKey: created.AdmissionAuthority.Public,
		IssuerNodeID: fixtureID(247), IssuerDutyGeneration: 1, NotBefore: current.Truncate(time.Hour), NotAfter: current.Truncate(time.Hour).Add(3 * time.Hour)}}
	owner := textPermissionContextFixture(t, endpoint, principal, broker.Connection)
	request, digest, err := owner.requestTextPermission([3]uint32{32, 32, 0})
	if err != nil {
		_ = endpoint.Close()
		_ = vault.Close()
		t.Fatal(err)
	}
	issued, err := vault.Execute(t.Context(), custody.Operation{Kind: custody.OperationIssueAdmissionPermission, RecordID: created.RecordID,
		Expected: created.Authority.Binding, AdmissionRequest: request, AdmissionRequestCommitment: sha256.Sum256(request)}, textPermissionSecretFixture{})
	if err != nil {
		_ = endpoint.Close()
		_ = vault.Close()
		t.Fatal(err)
	}
	return owner, textNetworkFixturePermissionResponse{digest: digest, raw: issued.AdmissionPermission}, func() {
		if err := endpoint.Close(); err != nil {
			t.Error(err)
		}
		if err := vault.Close(); err != nil {
			t.Error(err)
		}
	}
}
