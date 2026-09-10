//go:build linux

package endpoint

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/custody"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

// These tests exercise the local permission owner and actual Custody ledger.
// State acceptance and installed worker qualification are explicit fixtures;
// this is not evidence of a network issuance or installed confinement journey.
type textPermissionStateFixture struct {
	profile state.ClosedProfileView
	err     error
}

func (fixture *textPermissionStateFixture) CurrentClosedProfile() (state.ClosedProfileView, error) {
	return fixture.profile, fixture.err
}

type textPermissionSecretFixture struct{}

func (textPermissionSecretFixture) ReadSecret(context.Context, custody.SecretPrompt) ([]byte, error) {
	return []byte("text permission isolated test vault password"), nil
}

func (textPermissionSecretFixture) Confirm(context.Context, custody.ConfirmationPrompt) (bool, error) {
	return false, errors.New("unexpected custody confirmation")
}

func textPermissionContextFixture(t *testing.T, endpoint *endpoint, principal [32]byte, surface broker.Surface) *textContext {
	t.Helper()
	owner := admittedTextContext(t, endpoint, principal, surface)
	job, err := beginTextTestJob(t, owner, endpoint, surface)
	if err != nil {
		t.Fatal(err)
	}
	grant, err := broker.New(broker.Config{ID: fixtureID(217), Grants: []broker.Grant{{Principal: fixtureID(218), Surface: broker.Connection}}})
	if err != nil {
		t.Fatal(err)
	}
	owner.mu.Lock()
	job.workerGrant = grant
	owner.verifiedJob = job
	owner.mu.Unlock()
	owner.retireJob(job)
	if err := owner.finishJobCleanup(job, nil); err != nil {
		t.Fatal(err)
	}
	return owner
}

func TestTextPermissionCustodyRoundTripAndContextOwnership(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	vault, err := custody.Open(custody.VaultConfig{Root: t.TempDir(), Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	created, err := vault.Execute(t.Context(), custody.Operation{Kind: custody.OperationCreateAdmissionAuthority,
		Authority: custody.AuthorityState{Binding: custody.AuthorityBinding{Environment: fixtureID(221), Network: fixtureID(222),
			Root: fixtureID(223), Kind: custody.AuthorityAdmission}}}, textPermissionSecretFixture{})
	if err != nil {
		t.Fatal(err)
	}
	endpoint, principal := textContextEndpoint(t)
	endpoint.network, endpoint.clock = fixtureID(222), func() time.Time { return now }
	projection := &textPermissionStateFixture{profile: state.ClosedProfileView{NetworkID: endpoint.network,
		StateGeneration: fixtureID(224), StateDigest: fixtureID(225), Digest: fixtureID(226),
		IssuanceAuthorityKey: created.AdmissionAuthority.Public, IssuerNodeID: fixtureID(227), IssuerDutyGeneration: 1,
		NotBefore: now.Truncate(time.Hour), NotAfter: now.Truncate(time.Hour).Add(6 * time.Hour)}}
	endpoint.closedState = projection
	reader := textPermissionContextFixture(t, endpoint, principal, broker.Connection)
	public, digest, err := reader.requestTextPermission([3]uint32{32, 32, 0})
	if err != nil || sha256.Sum256(public) != digest {
		t.Fatalf("prepare: %v", err)
	}
	request, err := credential.DecodePermissionRequest(public)
	if err != nil || request.Role != credential.AllocationUser {
		t.Fatalf("reader role: %v", err)
	}
	public[0] ^= 1
	public, repeated, err := reader.requestTextPermission(request.Permission.Maxima)
	if err != nil || repeated != digest || sha256.Sum256(public) != digest {
		t.Fatal("public response aliases retained request")
	}
	if _, _, err := reader.requestTextPermission([3]uint32{33, 32, 0}); err == nil {
		t.Fatal("replaced allocation in same hour")
	}
	issue := custody.Operation{Kind: custody.OperationIssueAdmissionPermission, RecordID: created.RecordID,
		Expected: created.Authority.Binding, AdmissionRequest: public, AdmissionRequestCommitment: digest}
	wrongApproval := issue
	wrongApproval.AdmissionRequestCommitment[0] ^= 1
	if _, err := vault.Execute(t.Context(), wrongApproval, textPermissionSecretFixture{}); err == nil {
		t.Fatal("Custody accepted wrong approval digest")
	}
	approved, err := vault.Execute(t.Context(), issue, textPermissionSecretFixture{})
	if err != nil || approved.Authority.Generation != 2 {
		t.Fatalf("durable allocation: %v", err)
	}
	for range 2 {
		if err := reader.importTextPermission(digest, approved.AdmissionPermission); err != nil {
			t.Fatalf("import: %v", err)
		}
	}
	foreignDigest := digest
	foreignDigest[0] ^= 1
	if err := reader.importTextPermission(foreignDigest, approved.AdmissionPermission); err == nil {
		t.Fatal("foreign digest accepted")
	}
	corrupt := bytes.Clone(approved.AdmissionPermission)
	corrupt[len(corrupt)-1] ^= 1
	if err := reader.importTextPermission(digest, corrupt); err == nil {
		t.Fatal("bad signature accepted")
	}

	// Same Principal, role, hour and maxima still create distinct contexts.
	// A per-role or per-installation holder would pass reader/Publisher checks
	// but would expose a cross-context identifier to the issuer.
	otherReader := textPermissionContextFixture(t, endpoint, principal, broker.Connection)
	otherRaw, otherDigest, err := otherReader.requestTextPermission(request.Permission.Maxima)
	if err != nil {
		t.Fatal(err)
	}
	otherRequest, err := credential.DecodePermissionRequest(otherRaw)
	if err != nil || otherRequest.Role != request.Role || otherRequest.Permission.HolderKey == request.Permission.HolderKey ||
		otherRequest.Permission.PermissionID == request.Permission.PermissionID || otherDigest == digest {
		t.Fatalf("same-role contexts shared issuer identity: %v", err)
	}
	if err := otherReader.importTextPermission(otherDigest, approved.AdmissionPermission); err == nil {
		t.Fatal("same-role context accepted another context's approval")
	}
	otherIssue := custody.Operation{Kind: custody.OperationIssueAdmissionPermission, RecordID: created.RecordID,
		Expected: created.Authority.Binding, AdmissionRequest: otherRaw, AdmissionRequestCommitment: otherDigest}
	otherApproval, err := vault.Execute(t.Context(), otherIssue, textPermissionSecretFixture{})
	if err != nil {
		t.Fatal(err)
	}
	if err := otherReader.importTextPermission(otherDigest, otherApproval.AdmissionPermission); err != nil {
		t.Fatal(err)
	}
	if err := reader.importTextPermission(digest, otherApproval.AdmissionPermission); err == nil {
		t.Fatal("original context accepted the second context's approval")
	}
	if err := otherReader.Close(); err != nil {
		t.Fatal(err)
	}
	if retained, current, err := reader.requestTextPermission(request.Permission.Maxima); err != nil || current != digest || !bytes.Equal(retained, public) {
		t.Fatal("closing another reader changed the surviving context's allocation")
	}

	publisher := textPermissionContextFixture(t, endpoint, principal, broker.Administration)
	publisherRaw, publisherDigest, err := publisher.requestTextPermission([3]uint32{0, 64, 0})
	if err != nil {
		t.Fatal(err)
	}
	publisherRequest, err := credential.DecodePermissionRequest(publisherRaw)
	if err != nil || publisherRequest.Role != credential.AllocationPublisher || publisherRequest.Permission.HolderKey == request.Permission.HolderKey {
		t.Fatal("Publisher inherited reader role or holder")
	}
	if err := publisher.importTextPermission(publisherDigest, approved.AdmissionPermission); err == nil {
		t.Fatal("signed foreign holder accepted")
	}

	// A successfully joined replacement worker cannot rotate the context holder.
	job, err := beginTextTestJob(t, reader, endpoint, broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	reader.retireJob(job)
	if err := reader.finishJobCleanup(job, nil); err != nil {
		t.Fatal(err)
	}
	if _, current, err := reader.requestTextPermission(request.Permission.Maxima); err != nil || current != digest {
		t.Fatal("worker loss rotated permission")
	}

	projection.profile.Digest[0] ^= 1
	if err := reader.importTextPermission(digest, approved.AdmissionPermission); err == nil {
		t.Fatal("State successor accepted old permission")
	}
	if _, _, err := reader.requestTextPermission(request.Permission.Maxima); err == nil {
		t.Fatal("State successor replaced same-hour holder")
	}
	projection.profile.Digest[0] ^= 1
	projection.err = errors.New("State authority unavailable")
	if _, _, err := reader.requestTextPermission(request.Permission.Maxima); err == nil {
		t.Fatal("unavailable State accepted")
	}
	projection.err = nil

	oldHolder, oldPublic := reader.permission.holder, reader.permission.public
	now = now.Add(time.Hour)
	if err := reader.importTextPermission(digest, approved.AdmissionPermission); err == nil {
		t.Fatal("expired permission accepted")
	}
	newRaw, newDigest, err := reader.requestTextPermission(request.Permission.Maxima)
	if err != nil || newDigest == digest {
		t.Fatalf("next-hour holder: %v", err)
	}
	newRequest, err := credential.DecodePermissionRequest(newRaw)
	if err != nil || newRequest.Permission.HolderKey == request.Permission.HolderKey {
		t.Fatal("holder reused across hours")
	}
	if !bytes.Equal(oldHolder, make([]byte, len(oldHolder))) || !bytes.Equal(oldPublic, make([]byte, len(oldPublic))) {
		t.Fatal("old allocation was not erased")
	}
	retainedHolder := reader.permission.holder
	if err := endpoint.admission.Revoke(principal, broker.Connection); err != nil {
		t.Fatal(err)
	}
	if _, _, err := reader.requestTextPermission(request.Permission.Maxima); err == nil {
		t.Fatal("revoked context prepared permission")
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if reader.permission != nil || !bytes.Equal(retainedHolder, make([]byte, len(retainedHolder))) {
		t.Fatal("revoked context retained private holder")
	}
	if _, _, err := publisher.requestTextPermission([3]uint32{0, 64, 0}); err != nil {
		t.Fatalf("reader revoke destroyed Publisher role: %v", err)
	}
	if err := endpoint.closeTextContexts(); err != nil {
		t.Fatal(err)
	}
	if publisher.permission != nil {
		t.Fatal("Endpoint loss retained Publisher allocation")
	}
}

func TestTextPermissionRequiresVerifiedContext(t *testing.T) {
	endpoint, principal := textContextEndpoint(t)
	owner := admittedTextContext(t, endpoint, principal, broker.Connection)
	if _, _, err := owner.requestTextPermission([3]uint32{1, 0, 0}); err == nil {
		t.Fatal("unqualified context created holder")
	}
	if owner.permission != nil {
		t.Fatal("failed request retained key")
	}
}
