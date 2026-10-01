//go:build linux

package node

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	admissiongrammar "github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/resource"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
	"github.com/dianabuilds/ardents-network/internal/route/terminal"
)

func TestClosedNodeRestrictionRefusesValidPrivateTokenWithoutSpendingIt(t *testing.T) {
	for _, transport := range []carrier.CarrierProfile{carrier.ClosedCarrierTCP, carrier.ClosedCarrierQUIC} {
		t.Run(string(transport), func(t *testing.T) {
			fixture := newClosedBootstrapFixture(t)
			token := closedRestrictionToken(t, fixture)
			certificate, key := nodeCertificate(t, 211, "restricted-interior")
			peerCertificate, peerKey := nodeCertificate(t, 212, "restricted-entry")
			fixture.snapshot.ProbeEndpoint, fixture.snapshot.CarrierProfile = reserveClosedBootstrapAddress(t, transport), string(transport)
			fixture.snapshot.NodePublicKey = key
			fixture.snapshot.Candidates[0].PublicKey = peerKey
			fixture.config.now = func() time.Time { return fixture.now }
			fixture.config.Current = func() (state.NodeDuty, error) { return fixture.snapshot, nil }
			fixture.config.CurrentClosedProfile = func() (state.ClosedProfileView, bool) { return fixture.view.Profile, true }
			fixture.config.ClosedForwarding = ClosedForwardingProfile{Root: filepath.Join(t.TempDir(), "spends"), Certificate: certificate, ConnectionLimit: 2, DrainTimeout: 2 * time.Second, AdmissionTraffic: resource.HostingTraffic{Tx: 1}, TerminationTraffic: resource.HostingTraffic{Tx: 1}, HostingRoot: hostingFixtureRoot(t)}
			if err := os.MkdirAll(fixture.config.ClosedForwarding.Root, 0o700); err != nil {
				t.Fatal(err)
			}
			server, err := startClosedForwarding(fixture.config.ClosedForwarding, projectRoleInputs(fixture.config), fixture.snapshot)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				server.Stop()
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				if err := server.Drain(ctx); err != nil {
					t.Error(err)
				}
			}()
			deadline := time.Now().Add(8 * time.Second)
			outer, err := carrier.OpenClosedNodeCarrier(t.Context(), carrier.ClosedNodeCarrierRequest{CarrierProfile: transport, Endpoint: fixture.snapshot.ProbeEndpoint,
				Certificate: peerCertificate, ExpectedPeerKey: key, Deadline: deadline})
			if err != nil {
				t.Fatal(err)
			}
			defer outer.Close()
			if err := outer.SetDeadline(deadline); err != nil {
				t.Fatal(err)
			}
			receiver := fixture.receiver
			hello := ardp.Hello{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest, ProfileDigest: receiver.ProfileDigest,
				RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration, Purpose: ardp.PurposeForwarding, ChannelNonce: [32]byte{110}, Deadline: fixture.now.Add(9 * time.Second)}
			body, err := ardp.EncodeHello(hello)
			if err != nil {
				t.Fatal(err)
			}
			if err := ardp.WriteFrame(outer, ardp.Frame{Kind: 1, Body: body}); err != nil {
				t.Fatal(err)
			}
			if accepted, err := ardp.ReadFrame(outer); err != nil || accepted.Kind != 5 {
				t.Fatalf("outer acceptance: %+v %v", accepted, err)
			}
			for index, restriction := range []route.ClosedChildRestriction{route.ClosedChildIssuerBootstrap, route.ClosedChildOrdinary} {
				lane := uint32(index*2 + 1)
				body, err := route.EncodeClosedNodeOpen(route.ClosedOpen{NextNodeID: receiver.NodeID, NextDutyGeneration: receiver.DutyGeneration,
					Purpose: ardp.PurposeForwarding, Deadline: fixture.now.Add(8 * time.Second)}, restriction)
				if err != nil {
					t.Fatal(err)
				}
				if err := ardp.WriteFrame(outer, ardp.Frame{Kind: 4, Lane: lane, Body: body}); err != nil {
					t.Fatal(err)
				}
				inner, err := carrier.OpenClosedRoleTLS(t.Context(), &outerTestInnerConn{outer: outer, lane: lane}, key, deadline)
				if err != nil {
					t.Fatal(err)
				}
				hello.ChannelNonce = [32]byte{byte(111 + index)}
				hello.Deadline = fixture.now.Add(8 * time.Second)
				body, err = ardp.EncodeHello(hello)
				if err != nil {
					t.Fatal(err)
				}
				if err := ardp.WriteFrame(inner, ardp.Frame{Kind: 1, Body: body}); err != nil {
					t.Fatal(err)
				}
				if err := ardp.WriteFrame(inner, ardp.Frame{Kind: 2, Body: append([]byte{2}, token...)}); err != nil {
					t.Fatal(err)
				}
				accepted, err := ardp.ReadFrame(inner)
				if restriction == route.ClosedChildIssuerBootstrap {
					if err == nil {
						t.Fatalf("restricted child accepted valid private token: %+v", accepted)
					}
				} else if err != nil || accepted.Kind != 5 || accepted.Body[0] != 0 {
					t.Fatalf("ordinary child could not spend same valid token: %+v %v", accepted, err)
				}
			}
		})
	}
}

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
	inventory, err := admissiongrammar.DecodeClosedIssuerProfile(receipt.Profile, public)
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
	permission := admissiongrammar.Permission{NetworkID: profile.NetworkID, IssuerNodeID: profile.IssuerNodeID, DutyGeneration: profile.IssuerDutyGeneration,
		PermissionID: [32]byte{101}, NotBefore: window, NotAfter: profile.NotAfter, Maxima: [3]uint32{0, 1, 0}, Signature: [64]byte{1}}
	copy(permission.HolderKey[:], holderPublic)
	// Public canonical encoding supplies the unsigned fields; replace the
	// nonzero placeholder with the actual fixture authority signature.
	raw, err := admissiongrammar.EncodePermission(permission)
	if err != nil {
		t.Fatal(err)
	}
	transcript := append([]byte("ardents-issuance-permission-v1\x00"), raw[:len(raw)-ed25519.SignatureSize]...)
	copy(permission.Signature[:], ed25519.Sign(authority, transcript))
	if err := admissiongrammar.VerifyPermission(permission, authorityPublic, profile.NetworkID, profile.IssuerNodeID, profile.IssuerDutyGeneration, fixture.now); err != nil {
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
	operation, err := terminal.EncodeIssuanceRequest(nonce, pending.Request())
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
