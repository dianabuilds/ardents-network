//go:build linux

package node

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

// Explicit State-projection and offline-authority fixture. The client, three
// production listeners, mutual Node TLS, both inner TLS channels, issuer RSA
// keys, durable debit/retry and unblinding are real. This is not installed
// Endpoint, enrollment/State acceptance, or multi-process qualification.
type closedBootstrapNetwork struct {
	view      state.ClosedRouteView
	snapshot  state.Snapshot
	authority ed25519.PrivateKey
	selection route.ClosedBootstrapSelection
}

func (fixture *closedBootstrapNetwork) Current() (state.Snapshot, error) {
	return fixture.snapshot, nil
}
func (fixture *closedBootstrapNetwork) CurrentClosedRoute() (state.ClosedRouteView, error) {
	return fixture.view, nil
}

func newClosedBootstrapNetwork(t *testing.T, carrier route.CarrierProfile) *closedBootstrapNetwork {
	t.Helper()
	now := time.Now().UTC()
	window := now.Truncate(time.Hour)
	fixture := &closedBootstrapNetwork{}
	authorityPublic, authority, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	fixture.authority = authority
	profile := state.ClosedProfileView{NetworkID: [32]byte{21}, StateGeneration: [32]byte{22}, StateDigest: [32]byte{23}, Digest: [32]byte{24},
		IssuerNodeID: [32]byte{33}, IssuerDutyGeneration: 3, Epoch: 1, NotBefore: window, NotAfter: window.Add(time.Hour)}
	copy(profile.IssuanceAuthorityKey[:], authorityPublic)
	certificates := [3]tls.Certificate{}
	keys := [3][32]byte{}
	for index := range certificates {
		certificates[index], keys[index] = rendezvousCertificate(t, int64(161+index), fmt.Sprintf("bootstrap-%d", index))
	}
	issuerRoot := filepath.Join(t.TempDir(), "issuer")
	receipt, err := credential.InitializeClosedIssuerRoot(credential.ClosedIssuerRootConfig{Root: issuerRoot, NetworkID: profile.NetworkID,
		NodeID: profile.IssuerNodeID, IdentityKey: certificates[2].PrivateKey.(ed25519.PrivateKey), NotBefore: profile.NotBefore, NotAfter: profile.NotAfter, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := credential.DecodeClosedIssuerProfile(receipt.Profile, ed25519.PublicKey(keys[2][:]))
	if err != nil {
		t.Fatal(err)
	}
	profile.TokenKeyCount = uint8(len(inventory.Keys))
	for index, key := range inventory.Keys {
		profile.TokenKeys[index].WindowStart, profile.TokenKeys[index].Class = key.WindowStart, uint8(key.Class)
		copy(profile.TokenKeys[index].SPKI[:], key.SPKI)
	}
	fixture.view = state.ClosedRouteView{Profile: profile, NodeCount: 3}
	fixture.snapshot = state.Snapshot{Generation: hex.EncodeToString(profile.StateGeneration[:]), NetworkID: profile.NetworkID,
		Epoch: profile.Epoch, Digest: profile.StateDigest, EpochValidFrom: profile.NotBefore, ValidUntil: profile.NotAfter,
		Profile: route.ClosedRouteProfile, Freshness: "fresh", CandidateCount: 3}
	snapshots := [3]dutyFacts{}
	for index := 0; index < 3; index++ {
		id, record := [32]byte{byte(31 + index)}, [32]byte{byte(41 + index)}
		family := fmt.Sprintf("bootstrap-family-%d", index)
		domain, subrole := uint8(1), uint8(index+1)
		if index == 2 {
			domain, subrole = 2, 6
		}
		fixture.view.Nodes[index] = state.ClosedRouteNodeView{NodeID: id, RecordDigest: record, DutyGeneration: uint64(index + 1), RoleDomain: domain, Subrole: subrole}
		candidate := &fixture.snapshot.Candidates[index]
		candidate.NodeID, candidate.PublicKey, candidate.RecordDigest = id, keys[index], record
		candidate.Family, candidate.FamilyID = family, sha256.Sum256([]byte(family))
		candidate.Endpoint, candidate.CarrierProfile, candidate.Capacity = reserveClosedBootstrapAddress(t, carrier), string(carrier), 16
		candidate.ValidFrom, candidate.ValidUntil, candidate.AssignmentNotAfter = window, profile.NotAfter, profile.NotAfter
		snapshots[index] = dutyFacts{Generation: fixture.snapshot.Generation, NetworkID: profile.NetworkID, Epoch: profile.Epoch, Digest: profile.StateDigest,
			EpochValidFrom: window, ValidUntil: profile.NotAfter, Profile: route.ClosedRouteProfile, Fresh: true, RecordPresent: true,
			NodeID: id, NodePublicKey: keys[index], RecordGeneration: uint64(index + 1), RecordValidFrom: window, RecordValidUntil: profile.NotAfter,
			DeclaredFamily: family, ProbeEndpoint: candidate.Endpoint, CarrierProfile: string(carrier), CandidateCount: 3}
	}
	fixture.selection = route.ClosedBootstrapSelection{ProfileDigest: profile.Digest, EntryNodeID: fixture.view.Nodes[0].NodeID, InteriorNodeID: fixture.view.Nodes[1].NodeID}
	for index := 2; index >= 0; index-- {
		snapshot := snapshots[index]
		for peerIndex, candidate := range fixture.snapshot.Candidates[:3] {
			snapshot.Candidates[peerIndex] = dutyCandidate{NodeID: candidate.NodeID, PublicKey: candidate.PublicKey, RecordDigest: candidate.RecordDigest,
				FamilyID: candidate.FamilyID, Endpoint: candidate.Endpoint, CarrierProfile: candidate.CarrierProfile, ValidFrom: candidate.ValidFrom,
				ValidUntil: candidate.ValidUntil, AssignmentNotAfter: candidate.AssignmentNotAfter}
		}
		config := runtimeConfig{Config: Config{NetworkID: profile.NetworkID, NodeID: snapshot.NodeID,
			Current:              func() (DutyView, error) { return snapshot, nil },
			CurrentClosedProfile: func() (state.ClosedProfileView, bool) { return fixture.view.Profile, true },
			CurrentClosedRoute:   func() (state.ClosedRouteView, bool) { return fixture.view, true }}, now: time.Now}
		var server *probeServer
		if index == 2 {
			config.ClosedIssuer = ClosedIssuerProfile{Root: issuerRoot, AdmissionRoot: t.TempDir(), Certificate: certificates[index], ConnectionLimit: 4, DrainTimeout: 2 * time.Second}
			server, err = startClosedIssuer(config, snapshot)
		} else {
			root := filepath.Join(t.TempDir(), "spends")
			if err := os.MkdirAll(root, 0o700); err != nil {
				t.Fatal(err)
			}
			config.ClosedForwarding = ClosedForwardingProfile{Root: root, Certificate: certificates[index], ConnectionLimit: 4, DrainTimeout: 2 * time.Second}
			server, err = startClosedForwarding(config, snapshot)
		}
		if err != nil {
			t.Fatalf("start role %d: %v", index, err)
		}
		t.Cleanup(func() {
			server.Stop()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := server.Drain(ctx); err != nil {
				t.Error(err)
			}
		})
	}
	return fixture
}

func (fixture *closedBootstrapNetwork) batch(t *testing.T, count uint16) (*credential.PendingClosedTokenBatch, credential.ClosedTokenContext, [346]byte) {
	return fixture.batchFor(t, count, fixture.selection.EntryNodeID, 1, 71)
}
func (fixture *closedBootstrapNetwork) batchFor(t *testing.T, count uint16, receiver [32]byte, duty uint64, permissionID byte) (*credential.PendingClosedTokenBatch, credential.ClosedTokenContext, [346]byte) {
	t.Helper()
	profile := fixture.view.Profile
	challenge := credential.ClosedTokenContext{NetworkID: profile.NetworkID, ProfileDigest: profile.Digest, IssuerNodeID: profile.IssuerNodeID,
		ReceiverNodeID: receiver, ReceiverDutyGeneration: duty, Class: 2, WindowStart: profile.NotBefore}
	contexts := make([]credential.ClosedTokenContext, count)
	for index := range contexts {
		contexts[index] = challenge
	}
	pending, key := fixture.batchForChallenges(t, contexts, permissionID)
	return pending, challenge, key
}

func (fixture *closedBootstrapNetwork) batchForChallenges(t *testing.T, contexts []credential.ClosedTokenContext, permissionID byte) (*credential.PendingClosedTokenBatch, [346]byte) {
	t.Helper()
	profile := fixture.view.Profile
	public, holder, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	permission := credential.Permission{NetworkID: profile.NetworkID, IssuerNodeID: profile.IssuerNodeID, DutyGeneration: profile.IssuerDutyGeneration,
		PermissionID: [32]byte{permissionID}, NotBefore: profile.NotBefore, NotAfter: profile.NotAfter, Maxima: [3]uint32{0, uint32(len(contexts)), 0}, Signature: [64]byte{1}}
	copy(permission.HolderKey[:], public)
	raw, err := credential.EncodePermission(permission)
	if err != nil {
		t.Fatal(err)
	}
	transcript := append([]byte("ardents-issuance-permission-v1\x00"), raw[:len(raw)-64]...)
	copy(permission.Signature[:], ed25519.Sign(fixture.authority, transcript))
	pending, err := credential.PrepareClosedTokenBatch(credential.ClosedTokenBatchConfig{Profile: profile, Contexts: contexts,
		Permission: permission, HolderKey: holder, Now: time.Now().UTC()})
	clear(holder)
	if err != nil {
		t.Fatal(err)
	}
	request, err := credential.DecodeClosedTokenBatch(pending.Request())
	if err != nil {
		t.Fatal(err)
	}
	return pending, request.SPKI
}
