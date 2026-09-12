//go:build linux

package node

import (
	"crypto/ed25519"
	"crypto/rand"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

// Actual issuance/verification, with explicitly fixture-owned State and offline
// signing keys. This is a receiver-admission test, not the Endpoint client.
func closedRestrictionToken(t *testing.T, fixture *closedBootstrapFixture) []byte {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	window := fixture.now.Truncate(time.Hour)
	profile := &fixture.view.Profile
	profile.NotBefore, profile.NotAfter = window, window.Add(time.Hour)
	fixture.snapshot.EpochValidFrom = window
	root := filepath.Join(t.TempDir(), "issuer")
	receipt, err := credential.InitializeClosedIssuerRoot(credential.ClosedIssuerRootConfig{Root: root, NetworkID: profile.NetworkID,
		NodeID: profile.IssuerNodeID, IdentityKey: private, NotBefore: window, NotAfter: profile.NotAfter, Clock: func() time.Time { return fixture.now }})
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := credential.DecodeClosedIssuerProfile(receipt.Profile, public)
	if err != nil {
		t.Fatal(err)
	}
	authorityPublic, authority, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	holderPublic, holder, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	copy(profile.IssuanceAuthorityKey[:], authorityPublic)
	profile.TokenKeyCount = uint8(len(inventory.Keys))
	for index, key := range inventory.Keys {
		profile.TokenKeys[index].WindowStart, profile.TokenKeys[index].Class = key.WindowStart, uint8(key.Class)
		copy(profile.TokenKeys[index].SPKI[:], key.SPKI)
	}
	permission := credential.Permission{NetworkID: profile.NetworkID, IssuerNodeID: profile.IssuerNodeID, DutyGeneration: profile.IssuerDutyGeneration,
		PermissionID: [32]byte{101}, NotBefore: window, NotAfter: profile.NotAfter, Maxima: [3]uint32{0, 1, 0}, Signature: [64]byte{1}}
	copy(permission.HolderKey[:], holderPublic)
	// Public canonical encoding supplies the unsigned fields; replace the
	// nonzero placeholder with the actual fixture authority signature.
	raw, err := credential.EncodePermission(permission)
	if err != nil {
		t.Fatal(err)
	}
	transcript := append([]byte("ardents-issuance-permission-v1\x00"), raw[:len(raw)-ed25519.SignatureSize]...)
	copy(permission.Signature[:], ed25519.Sign(authority, transcript))
	if err := credential.VerifyPermission(permission, authorityPublic, profile.NetworkID, profile.IssuerNodeID, profile.IssuerDutyGeneration, fixture.now); err != nil {
		t.Fatal(err)
	}
	tokenContext := credential.ClosedTokenContext{NetworkID: profile.NetworkID, ProfileDigest: profile.Digest, ReceiverNodeID: fixture.receiver.NodeID,
		IssuerNodeID: profile.IssuerNodeID, ReceiverDutyGeneration: fixture.receiver.DutyGeneration, Class: 2, WindowStart: window}
	pending, err := credential.PrepareClosedTokenBatch(credential.ClosedTokenBatchConfig{Profile: *profile, Contexts: []credential.ClosedTokenContext{tokenContext}, Permission: permission,
		HolderKey: holder, Now: fixture.now})
	if err != nil {
		t.Fatal(err)
	}
	issuer, err := credential.OpenClosedTokenIssuer(credential.ClosedTokenIssuerConfig{Root: root, NetworkID: profile.NetworkID,
		CurrentProfile: func() (state.ClosedProfileView, bool) { return *profile, true }, Clock: func() time.Time { return fixture.now }})
	if err != nil {
		t.Fatal(err)
	}
	defer issuer.Close()
	nonce := [32]byte{102}
	operation, err := route.EncodeClosedIssuanceRequest(nonce, pending.Request())
	if err != nil {
		t.Fatal(err)
	}
	request, err := credential.DecodeClosedTokenBatch(pending.Request())
	if err != nil {
		t.Fatal(err)
	}
	result, err := issuer.IssueTerminalOperation(operation)
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := pending.FinalizeTerminalOperation(nonce, result)
	if err != nil || len(tokens) != 1 {
		t.Fatalf("real issuance: %d %v", len(tokens), err)
	}
	if err := credential.VerifyClosedToken(tokenContext, request.SPKI[:], tokens[0]); err != nil {
		t.Fatal(err)
	}
	return tokens[0]
}
