//go:build linux

package endpoint_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/domain/network"
	"github.com/dianabuilds/ardents-network/internal/domain/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
	networkinputs "github.com/dianabuilds/ardents-network/tests/networkfixture"
)

var commandBuild = processfixture.CommandBuild{Repository: filepath.Join("..", ".."), Name: "ardents-next"}

func TestMain(m *testing.M) {
	os.Exit(commandBuild.Run(m))
}

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
	spec, plan, inventory := networkinputs.PrepareNetworkGenesis(t, dir)
	f := &networkAdmissionFixture{spec: spec, plan: plan}
	networkID, authorityKey, permissionAuthority := spec.NetworkID, spec.Authority, spec.IssuanceAuthority
	issuerID, receiverID := spec.Nodes[0].NodeID, spec.Nodes[1].NodeID
	start, end := spec.NotBefore, spec.NotAfter
	for _, option := range prepare {
		option(f)
	}
	var lastClock atomic.Value
	var largestObservationGap atomic.Int64
	clock := func() time.Time {
		now := time.Now()
		lastClock.Store(now)
		return now
	}
	t.Cleanup(func() {
		if t.Failed() {
			t.Log("maximum fixture wall-to-independent observation elapsed gap", time.Duration(largestObservationGap.Load()))
		}
	})
	config := state.Config{Root: filepath.Join(dir, "network"), NetworkID: networkID,
		Authorities: map[[32]byte]ed25519.PublicKey{sha256.Sum256(authorityKey.Public().(ed25519.PublicKey)): authorityKey.Public().(ed25519.PublicKey)}, Threshold: 1,
		ClosedProfileAuthority: authorityKey.Public().(ed25519.PublicKey), AcceptedProfile: epoch.ProfileClosedRoute, Clock: clock,
		ObserveClock: func() time.Time {
			if f.clockUnavailable.Load() {
				return time.Time{}
			}
			now := time.Now()
			if prior, ok := lastClock.Load().(time.Time); ok {
				gap := now.Sub(prior).Nanoseconds()
				for before := largestObservationGap.Load(); gap > before; before = largestObservationGap.Load() {
					if largestObservationGap.CompareAndSwap(before, gap) {
						break
					}
				}
			}
			return now
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
		if _, err := owner.AcceptEpoch(t.Context(), bundle.Epoch.Raw, bundle.Epoch.Inputs, bundle.Epoch.Materials); err != nil {
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
	f.authority = networkAdmissionAuthority(owner.CurrentRuntime, f.close, owner.CurrentRuntimeWitness)
	f.reopen = func() {
		t.Helper()
		if err := f.close(); err != nil {
			t.Fatal(err)
		}
		owner, err = state.Open(config)
		if err != nil {
			t.Fatal(err)
		}
		f.authority = networkAdmissionAuthority(owner.CurrentRuntime, f.close, owner.CurrentRuntimeWitness)
	}
	f.profile, _, err = f.authority.ObserveFacts()
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

type admissionAuthority = funding.AdmissionAuthority

var networkAdmissionAuthority = funding.NetworkAdmissionAuthority

// The receiving side retains genuine Network history independently of the
// Publisher. Only signed public inputs and detached initial facts are shared;
// Publisher conflict cannot mutate this root, its lease or its currentness.
// Signing and clock infrastructure still have one fixture controller.
func publicationReceivingNetwork(t *testing.T, publisher *networkAdmissionFixture) *networkAdmissionFixture {
	t.Helper()
	original, err := publisher.current()
	if err != nil {
		t.Fatal(err)
	}
	owner := networkinputs.OpenIndependentState(t, publisher.spec, publisher.digest)
	current, err := owner.CurrentRuntime()
	if err != nil || current.Profile().ProfileBinding != original.Profile().ProfileBinding {
		t.Fatal("receiving authority differs from original Publisher", err)
	}
	f := &networkAdmissionFixture{}
	f.current, f.close = owner.CurrentRuntime, owner.Close
	f.authority = networkAdmissionAuthority(owner.CurrentRuntime, owner.Close, owner.CurrentRuntimeWitness)
	f.profile = publisher.profile
	return f
}
