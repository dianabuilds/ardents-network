//go:build linux

package stock

import (
	"crypto/ed25519"
	"crypto/rand"
	admissionissuer "github.com/dianabuilds/ardents-network/internal/admission/issuer"
	admissiontoken "github.com/dianabuilds/ardents-network/internal/admission/token"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/attempts"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/client"
	"github.com/dianabuilds/ardents-network/internal/route/terminal"
)

type spendHostFixture struct {
	stockHostFixture
	root    string
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
func issuedStockFixture(t *testing.T) (*Owner, *spendHostFixture, ardp.Hello) {
	t.Helper()
	now := time.Now().UTC()
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
	profile := state.ClosedProfileView{NetworkID: fixtureID(1), StateGeneration: fixtureID(2), StateDigest: fixtureID(3),
		Digest: fixtureID(4), IssuerNodeID: fixtureID(5), IssuerDutyGeneration: 7, NotBefore: window, NotAfter: window.Add(time.Hour)}
	copy(profile.IssuanceAuthorityKey[:], authority.Public().(ed25519.PublicKey))
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	receipt, err := admissionissuer.InitializeClosedIssuerRoot(admissionissuer.ClosedIssuerRootConfig{
		Root: root, NetworkID: profile.NetworkID, NodeID: profile.IssuerNodeID, IdentityKey: identity,
		NotBefore: window, NotAfter: profile.NotAfter, Clock: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := admission.DecodeClosedIssuerProfile(receipt.Profile, public)
	if err != nil {
		t.Fatal(err)
	}
	profile.TokenKeyCount = uint8(len(inventory.Keys))
	for index, key := range inventory.Keys {
		profile.TokenKeys[index].WindowStart, profile.TokenKeys[index].Class = key.WindowStart, uint8(key.Class)
		copy(profile.TokenKeys[index].SPKI[:], key.SPKI)
	}
	host := &spendHostFixture{stockHostFixture: stockHostFixture{profile: profile, now: now}, root: t.TempDir()}
	owner := &Owner{}
	owner.Init(&sync.Mutex{}, host)
	t.Cleanup(func() {
		owner.StopLocked().Join()
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
	batch, err := owner.permission.reserveBatch(profile, now, []admissiontoken.ClosedTokenContext{challenge}, client.ClosedBootstrapSelection{}, false, nil, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	issuer, err := admissionissuer.OpenClosedTokenIssuer(admissionissuer.ClosedTokenIssuerConfig{Root: root, NetworkID: profile.NetworkID,
		CurrentProfile: func() (state.ClosedProfileView, bool) { return profile, true }, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := issuer.Close(); err != nil {
			t.Error(err)
		}
	}()
	nonce := fixtureID(9)
	operation, err := terminal.EncodeIssuanceRequest(nonce, batch.Pending.Request())
	if err != nil {
		t.Fatal(err)
	}
	response, err := issuer.IssueTerminalOperation(operation)
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.permission.acceptIssuedBatch(batch, nonce, response); err != nil {
		t.Fatal(err)
	}
	return owner, host, ardp.Hello{NetworkID: profile.NetworkID, StateGeneration: profile.StateGeneration, StateDigest: profile.StateDigest,
		ProfileDigest: profile.Digest, RecipientNodeID: challenge.ReceiverNodeID, RecipientDutyGeneration: challenge.ReceiverDutyGeneration,
		Purpose: ardp.PurposeForwarding, ChannelNonce: fixtureID(10), Deadline: profile.NotAfter}
}
