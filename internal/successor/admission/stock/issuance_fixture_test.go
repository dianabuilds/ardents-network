//go:build linux

package stock

import "github.com/dianabuilds/ardents-network/internal/successor/admission/quota"

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"

	admissionissuer "github.com/dianabuilds/ardents-network/internal/successor/admission/issuer"
	admissiontoken "github.com/dianabuilds/ardents-network/internal/successor/admission/token"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuance"
	"path/filepath"

	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/attempts"
)

type spendHostFixture struct {
	stockHostFixture
	root    string
	plan    admissionissuer.Plan
	journal *attempts.Journal
}

func (host *spendHostFixture) Journal() (*attempts.Journal, error) {
	if host.journal == nil {
		journal, err := attempts.Open(host.root, host.profile.NetworkID, func() time.Time { return host.now })
		if err != nil {
			return nil, err
		}
		host.journal = journal
	}
	return host.journal, nil
}

// Real request signing, blind preparation, issuer debit, finalization and journal
// are used. accepted State and the offline signing authority are explicit local
// fixtures; Endpoint tests separately exercise Custody and network transport.
func issuedStockFixture(t *testing.T) (*Owner, *spendHostFixture, Presentation) {
	return issuedStockFixtureHours(t, 1)
}

func issuedStockFixtureHours(t *testing.T, hours int) (*Owner, *spendHostFixture, Presentation) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	window := now.Truncate(time.Hour)
	public, identity, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(identity)
	_, authority, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(authority)
	profile := admission.AuthorityFacts{NetworkID: fixtureID(1), StateGeneration: fixtureID(2), StateDigest: fixtureID(3),
		Digest: fixtureID(4), IssuerNodeID: fixtureID(5), IssuerDutyGeneration: 7, NotBefore: window, NotAfter: window.Add(time.Duration(hours) * time.Hour)}
	copy(profile.IssuanceAuthorityKey[:], authority.Public().(ed25519.PublicKey))
	root := t.TempDir()
	plan := admissionissuer.Plan{
		AdmissionRoot: filepath.Join(root, "quota"), KeyRoot: filepath.Join(root, "keys"), ResultRoot: filepath.Join(root, "results"),
		KeyBinding: issuance.Binding{Network: profile.NetworkID, Issuer: profile.IssuerNodeID, Signer: [32]byte(public), Start: window, End: profile.NotAfter},
	}
	if err := issuance.Initialize(context.Background(), plan.KeyRoot, plan.KeyBinding); err != nil {
		t.Fatal(err)
	}
	keys, err := issuance.Open(context.Background(), plan.KeyRoot, plan.KeyBinding)
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := keys.Inventory()
	if err != nil {
		t.Fatal(err)
	}
	if err := keys.Close(); err != nil {
		t.Fatal(err)
	}
	plan.AdmissionBinding = quota.LedgerBinding{Network: profile.NetworkID, Issuer: profile.IssuerNodeID, Authority: profile.IssuanceAuthorityKey, Profile: profile.Digest, Duty: profile.IssuerDutyGeneration, Start: window, End: profile.NotAfter, Keys: inventory.Keys}
	if err := quota.Initialize(plan.AdmissionRoot, plan.AdmissionBinding); err != nil {
		t.Fatal(err)
	}
	if r := admissionissuer.Initialize(context.Background(), plan); r.Outcome != "initialized-results" {
		t.Fatalf("initialize: %+v", r)
	}
	profile.TokenKeyCount = uint8(len(inventory.Keys))
	for index, key := range inventory.Keys {
		profile.TokenKeys[index].WindowStart = time.Unix(int64(key.Window), 0).UTC()
		profile.TokenKeys[index].Class = key.Class
		copy(profile.TokenKeys[index].SPKI[:], key.SPKI)
	}
	host := &spendHostFixture{stockHostFixture: stockHostFixture{profile: profile, now: now}, root: t.TempDir(), plan: plan}
	journal, err := host.Journal()
	if err != nil {
		t.Fatal(err)
	}
	owner, err := New(admission.AllocationUser, host.ProfileLocked, journal)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		owner.Close()
		if host.journal != nil {
			if err := host.journal.Close(); err != nil {
				t.Error(err)
			}
		}
	})
	raw, digest, err := owner.Request([3]uint32{0, 4, 0})
	if err != nil {
		t.Fatal(err)
	}
	request, err := admission.DecodePermissionRequest(raw)
	if err != nil {
		t.Fatal(err)
	}
	permission := request.Permission
	copy(permission.Signature[:], ed25519.Sign(authority, admission.PermissionTranscript(permission)))
	signed, err := admission.EncodePermission(permission)
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.Import(digest, signed); err != nil {
		t.Fatal(err)
	}
	challenge := admissiontoken.ClosedTokenContext{NetworkID: profile.NetworkID, ProfileDigest: profile.Digest, IssuerNodeID: profile.IssuerNodeID,
		ReceiverNodeID: fixtureID(6), ReceiverDutyGeneration: 8, Class: 2, WindowStart: window}
	selection := ExchangeBinding{ID: fixtureID(11), ProfileDigest: profile.Digest}
	operation, err := owner.Begin(IssuanceIntent{Challenges: []admissiontoken.ClosedTokenContext{challenge}, Selection: selection, Bootstrap: true, Deadline: profile.NotAfter})
	if err != nil {
		t.Fatal(err)
	}
	requestBytes, _, err := operation.Request()
	if err != nil {
		t.Fatal(err)
	}
	result := admissionissuer.IssueCurrent(context.Background(), plan, requestBytes, quota.Bootstrap, host.ProfileLocked)
	if result.Outcome != "issued-offline" {
		t.Fatalf("issue: phase=%s outcome=%s cleanup=%v", result.Phase, result.Outcome, result.Cleanup)
	}
	repeated := admissionissuer.IssueCurrent(context.Background(), plan, requestBytes, quota.Bootstrap, host.ProfileLocked)
	if repeated.Outcome != "already-issued" || string(result.Response) != string(repeated.Response) {
		t.Fatal("reopened exact retry changed result")
	}
	if err := operation.Complete(result.Response, nil); err != nil {
		t.Fatal(err)
	}
	return owner, host, Presentation{NetworkID: profile.NetworkID, StateGeneration: profile.StateGeneration, StateDigest: profile.StateDigest,
		ProfileDigest: profile.Digest, RecipientNodeID: challenge.ReceiverNodeID, RecipientDutyGeneration: challenge.ReceiverDutyGeneration,
		ChannelNonce: fixtureID(10), Deadline: profile.NotAfter}
}
