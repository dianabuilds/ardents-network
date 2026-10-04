//go:build linux

package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/allocation"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuance"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/token"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// This fixture supplies signed bytes, never a trusted RuntimeView or claimed
// AuthorityFacts. Every observation traverses the opened new Network owner.
type networkAdmissionFixture struct {
	current          func() (network.RuntimeView, error)
	authority        admissionAuthority
	profile          admission.AuthorityFacts
	receiver         receiving.Receiver
	plan             issuer.Plan
	spec             networkfixture.ClosedSpec
	digest           [32]byte
	accept           func(networkfixture.Closed) error
	acceptProfile    func([]byte) error
	clockUnavailable atomic.Bool
	close            func() error
	reopen           func()
}

func newNetworkAdmissionFixture(t *testing.T, prepare ...func(*networkAdmissionFixture)) *networkAdmissionFixture {
	t.Helper()
	dir := t.TempDir()
	start := time.Now().UTC().Truncate(time.Hour)
	end := start.Add(time.Hour)
	if time.Until(end) < 2*time.Minute {
		end = end.Add(time.Hour)
	}
	_, authorityKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_, issuerKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_, receiverKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { clear(authorityKey); clear(issuerKey); clear(receiverKey) })
	f := &networkAdmissionFixture{}
	networkID := sha256.Sum256([]byte("new Network integration"))
	issuerID, receiverID := [32]byte{11}, [32]byte{12}
	var signer, permissionAuthority [32]byte
	copy(signer[:], issuerKey.Public().(ed25519.PublicKey))
	copy(permissionAuthority[:], authorityKey.Public().(ed25519.PublicKey))
	f.plan = issuer.Plan{AdmissionRoot: filepath.Join(dir, "quota"), KeyRoot: filepath.Join(dir, "keys"), ResultRoot: filepath.Join(dir, "results"),
		KeyBinding: issuance.Binding{Network: networkID, Issuer: issuerID, Signer: signer, Start: start, End: end}}
	if err := issuance.Initialize(t.Context(), f.plan.KeyRoot, f.plan.KeyBinding); err != nil {
		t.Fatal(err)
	}
	keys, err := issuance.Open(t.Context(), f.plan.KeyRoot, f.plan.KeyBinding)
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
	f.spec = networkfixture.ClosedSpec{NetworkID: networkID, Seed: [32]byte{13}, IssuanceAuthority: permissionAuthority, Number: 1,
		NotBefore: start, NotAfter: end, Authority: authorityKey, Nodes: []networkfixture.ClosedNode{
			{RecordSpec: networkfixture.RecordSpec{NodeID: issuerID, Generation: 7, ValidFrom: start, ValidUntil: end, Endpoint: "127.0.0.1:4401", Carrier: epoch.CarrierClosedTCP, Capability: 2, Capacity: 1, PrivateKey: issuerKey}, RoleDomain: 2, Subrole: 6},
			{RecordSpec: networkfixture.RecordSpec{NodeID: receiverID, Generation: 9, ValidFrom: start, ValidUntil: end, Endpoint: "127.0.0.1:4402", Carrier: epoch.CarrierClosedTCP, Capability: 2, Capacity: 1, PrivateKey: receiverKey}, RoleDomain: 3, Subrole: 1},
		}}
	for _, key := range inventory.Keys {
		f.spec.Keys = append(f.spec.Keys, networkfixture.ClosedTokenKey{WindowStart: time.Unix(int64(key.Window), 0).UTC(), Class: key.Class, SPKI: key.SPKI})
	}
	for _, option := range prepare {
		option(f)
	}
	config := state.Config{Root: filepath.Join(dir, "network"), NetworkID: networkID,
		Authorities: map[[32]byte]ed25519.PublicKey{sha256.Sum256(authorityKey.Public().(ed25519.PublicKey)): authorityKey.Public().(ed25519.PublicKey)}, Threshold: 1,
		ClosedProfileAuthority: authorityKey.Public().(ed25519.PublicKey), AcceptedProfile: epoch.ProfileClosedRoute, Clock: time.Now,
		ObserveClock: func() time.Time {
			if f.clockUnavailable.Load() {
				return time.Time{}
			}
			return time.Now()
		}}
	owner, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	f.close = func() error { return owner.Close() }
	f.current = func() (network.RuntimeView, error) { return owner.CurrentRuntime() }
	t.Cleanup(func() {
		if err := f.close(); err != nil {
			t.Error(err)
		}
	})
	f.accept = func(bundle networkfixture.Closed) error {
		if _, err := owner.Accept(t.Context(), bundle.Epoch.Raw, bundle.Epoch.Inputs, bundle.Epoch.Materials); err != nil {
			return err
		}
		_, err := owner.AcceptClosedProfile(bundle.Profile)
		return err
	}
	f.acceptProfile = func(raw []byte) error {
		_, err := owner.AcceptClosedProfile(raw)
		return err
	}
	bundle, err := networkfixture.BuildClosed(f.spec)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.accept(bundle); err != nil {
		t.Fatal(err)
	}
	f.digest = bundle.Epoch.Digest
	f.authority = networkAdmissionAuthority(owner.CurrentRuntime, f.close)
	f.reopen = func() {
		t.Helper()
		if err := f.close(); err != nil {
			t.Fatal(err)
		}
		owner, err = state.Open(config)
		if err != nil {
			t.Fatal(err)
		}
		f.authority = networkAdmissionAuthority(owner.CurrentRuntime, f.close)
	}
	f.profile, _, err = f.authority.observe()
	if err != nil {
		t.Fatal(err)
	}
	f.receiver = receiving.Receiver{NetworkID: networkID, StateGeneration: f.profile.StateGeneration, StateDigest: f.profile.StateDigest, ProfileDigest: f.profile.Digest, NodeID: receiverID, DutyGeneration: 9}
	f.plan.AdmissionBinding = quota.LedgerBinding{Network: networkID, Issuer: issuerID, Authority: permissionAuthority, Profile: f.profile.Digest, Duty: 7, Start: start, End: end, Keys: inventory.Keys}
	if err := quota.Initialize(f.plan.AdmissionRoot, f.plan.AdmissionBinding); err != nil {
		t.Fatal(err)
	}
	if result := issuer.Initialize(t.Context(), f.plan); result.Outcome != "initialized-results" {
		t.Fatal(result)
	}
	return f
}

func (f *networkAdmissionFixture) successor(t *testing.T) {
	t.Helper()
	next := f.spec
	next.Number++
	next.Previous = f.digest
	next.Seed[0]++
	bundle, err := networkfixture.BuildClosed(next)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.accept(bundle); err != nil {
		t.Fatal(err)
	}
	f.spec, f.digest = next, bundle.Epoch.Digest
}

func (f *networkAdmissionFixture) profileConflict(t *testing.T) error {
	t.Helper()
	changed := f.spec
	changed.IssuanceAuthority[0] ^= 1
	bundle, err := networkfixture.BuildClosed(changed)
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Epoch.Digest != f.digest {
		t.Fatal("profile conflict changed its Epoch")
	}
	return f.acceptProfile(bundle.Profile)
}

func (f *networkAdmissionFixture) tokens(t *testing.T, count int) (*stock.Owner, [][]byte) {
	t.Helper()
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	holder, err := stock.Open(root, admission.AllocationUser, f.authority.observe)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := holder.Close(); err != nil {
			t.Error(err)
		}
	})
	raw, digest, err := holder.Request([3]uint32{0, uint32(count), 0})
	if err != nil {
		t.Fatal(err)
	}
	request, err := allocation.Prepare(raw, f.profile.NetworkID, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	decision, err := request.Decide(nil, f.profile.IssuanceAuthorityKey, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	// The test owns external authority custody: commit/read back public allocation
	// before signing. It does not claim encrypted Custody integration.
	journal := filepath.Join(t.TempDir(), "allocation")
	if err := os.WriteFile(journal, decision.Journal(), 0600); err != nil {
		t.Fatal(err)
	}
	committed, err := os.ReadFile(journal)
	if err != nil || !bytes.Equal(committed, decision.Journal()) {
		t.Fatal("allocation readback", err)
	}
	permission := decision.Permission()
	copy(permission.Signature[:], ed25519.Sign(f.spec.Authority, admission.PermissionTranscript(permission)))
	encoded, err := admission.EncodePermission(permission)
	if err != nil {
		t.Fatal(err)
	}
	if err := holder.Import(digest, encoded); err != nil {
		t.Fatal(err)
	}
	challenge := token.ClosedTokenContext{NetworkID: f.profile.NetworkID, ProfileDigest: f.profile.Digest, IssuerNodeID: f.profile.IssuerNodeID,
		ReceiverNodeID: f.receiver.NodeID, ReceiverDutyGeneration: f.receiver.DutyGeneration, Class: 2, WindowStart: time.Now().UTC().Truncate(time.Hour)}
	challenges := make([]token.ClosedTokenContext, count)
	for index := range challenges {
		challenges[index] = challenge
	}
	attempt, err := holder.Begin(stock.IssuanceIntent{Challenges: challenges, Selection: stock.ExchangeBinding{ID: [32]byte{20}, ProfileDigest: f.profile.Digest}, Bootstrap: true, Deadline: f.profile.NotAfter})
	if err != nil {
		t.Fatal(err)
	}
	batch, _, err := attempt.Request()
	if err != nil {
		t.Fatal(err)
	}
	observe := func() (admission.AuthorityFacts, time.Time, error) {
		return f.authority.issuer(f.plan.KeyBinding.Signer)
	}
	issued := issuer.IssueCurrent(t.Context(), f.plan, batch, quota.Bootstrap, observe)
	if issued.Outcome != "issued-offline" {
		t.Fatal(issued)
	}
	repeated := issuer.IssueCurrent(t.Context(), f.plan, batch, quota.Bootstrap, observe)
	if repeated.Outcome != "already-issued" || !bytes.Equal(repeated.Response, issued.Response) {
		t.Fatal("issuance exact retry", repeated)
	}
	if err := attempt.Complete(issued.Response, nil); err != nil {
		t.Fatal(err)
	}
	var tokens [][]byte
	for index := range count {
		presentation := stock.Presentation{NetworkID: f.profile.NetworkID, StateGeneration: f.profile.StateGeneration, StateDigest: f.profile.StateDigest, ProfileDigest: f.profile.Digest,
			RecipientNodeID: f.receiver.NodeID, RecipientDutyGeneration: f.receiver.DutyGeneration, ChannelNonce: [32]byte{byte(index + 1)}, Deadline: f.profile.NotAfter}
		raw, err := holder.Take(t.Context(), presentation, uint8(admission.ForwardClass))
		if err != nil {
			t.Fatal(err)
		}
		tokens = append(tokens, raw)
	}
	return holder, tokens
}
