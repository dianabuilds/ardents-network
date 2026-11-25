//go:build linux || darwin

package networkfixture

import (
	"crypto/ed25519"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/network"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	epochfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// SignedAdmission groups test-owned State, key inventory bindings and signed inputs.
type SignedAdmission struct {
	Spec   epochfixture.ClosedSpec
	Plan   issuer.Plan
	Config state.Config
	State  *state.State
	// StopClock joins the independent clock updater and can be called again during cleanup.
	StopClock func()
}

// OpenSignedAdmission opens a real State over independently signed fixture bytes.
func OpenSignedAdmission(t testing.TB, configure ...func(*epochfixture.ClosedSpec)) *SignedAdmission {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	spec, issuerPlan, inventory := PrepareNetworkGenesis(t, root)
	for _, apply := range configure {
		apply(&spec)
	}
	bundle, err := epochfixture.BuildClosed(spec)
	if err != nil {
		t.Fatal(err)
	}
	clock, stop := StartObservedClock(t)
	stopClock := sync.OnceFunc(stop)
	t.Cleanup(stopClock)
	var authority [32]byte
	copy(authority[:], spec.Authority.Public().(ed25519.PublicKey))
	declaration := SeedNetworkState(t, spec.NetworkID, authority, bundle, clock)
	config, err := declaration.Config()
	if err != nil {
		t.Fatal(err)
	}
	networkOwner, err := state.OpenRetained(config)
	if err != nil {
		t.Fatal(err)
	}
	fixture := &SignedAdmission{Spec: spec, Plan: issuerPlan, Config: config, State: networkOwner, StopClock: stopClock}
	t.Cleanup(func() {
		if err := fixture.State.Close(); err != nil {
			t.Error(err)
		}
	})
	p, _, err := fixture.ObserveIssuer()
	if err != nil {
		t.Fatal(err)
	}
	PrepareIssuer(t, &fixture.Plan, inventory, spec, spec.Nodes[0], p.Digest)
	return fixture
}

// ObserveIssuer freshly binds the fixture signer to the current Network duty.
func (fixture *SignedAdmission) ObserveIssuer() (admission.AuthorityFacts, time.Time, error) {
	view, err := fixture.State.CurrentRuntime()
	if err != nil {
		return admission.AuthorityFacts{}, time.Time{}, err
	}
	facts, now, err := AdmissionFacts(view)
	if err != nil {
		return admission.AuthorityFacts{}, time.Time{}, err
	}
	return facts, now, issuer.ValidateCurrentAuthority(view, facts, now, fixture.Plan.KeyBinding.Signer)
}

// AdmissionFacts independently projects the fixture State observation.
func AdmissionFacts(view network.RuntimeView) (admission.AuthorityFacts, time.Time, error) {
	profile := view.Profile()
	facts := admission.AuthorityFacts{
		NetworkID: profile.Network, StateGeneration: profile.Generation, StateDigest: profile.EpochDigest,
		Digest: profile.Digest, IssuanceAuthorityKey: profile.IssuanceAuthorityKey,
		IssuerNodeID: profile.IssuerNodeID, IssuerDutyGeneration: profile.IssuerDutyGeneration,
		Epoch: profile.Epoch, NotBefore: profile.NotBefore, NotAfter: profile.NotAfter, TokenKeyCount: uint8(len(profile.TokenKeys)),
	}
	for index, key := range profile.TokenKeys {
		facts.TokenKeys[index] = admission.TokenKey{WindowStart: key.WindowStart, Class: key.Class, SPKI: key.SPKI}
	}
	now := view.ObservedAt()
	if err := facts.ValidateAt(now); err != nil {
		return admission.AuthorityFacts{}, time.Time{}, err
	}
	return facts, now, nil
}
