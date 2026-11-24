//go:build linux

package main

import (
	"bytes"
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
	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/admission/token"
	"github.com/dianabuilds/ardents-network/internal/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/network"
	"github.com/dianabuilds/ardents-network/internal/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	nodefixture "github.com/dianabuilds/ardents-network/tests/e2e/node"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// This fixture supplies signed bytes, never a trusted RuntimeView or claimed
// AuthorityFacts. Every observation traverses the opened new Network owner.
type networkAdmissionFixtureState struct {
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

// Return one actual checked view, then withdraw the fixture's trusted clock.
// A subsequent owner must observe that loss instead of accepting this draft.
func (f *networkAdmissionFixture) authorityLosingClockAfterDraft(t *testing.T) admissionAuthority {
	t.Helper()
	authority := f.authority
	authority.Current = func() (network.RuntimeView, error) {
		view, err := f.current()
		if err == nil {
			f.clockUnavailable.Store(true)
		}
		return view, err
	}
	t.Cleanup(func() { f.clockUnavailable.Store(false) })
	return authority
}

func TestNetworkIssuanceDraftRetainsExactBindingsAndHorizons(t *testing.T) {
	f := newNetworkAdmissionFixture(t)
	view, err := f.current()
	if err != nil {
		t.Fatal(err)
	}
	deadline := view.ObservedAt().Add(time.Second).UTC().Truncate(time.Second)
	intent := stock.IssuanceIntent{Challenges: []token.ClosedTokenContext{{NetworkID: f.profile.NetworkID,
		ProfileDigest: f.profile.Digest, IssuerNodeID: f.profile.IssuerNodeID, ReceiverNodeID: f.receiver.NodeID,
		ReceiverDutyGeneration: f.receiver.DutyGeneration, Class: 2, WindowStart: view.ObservedAt().UTC().Truncate(time.Hour)}},
		Selection: stock.ExchangeBinding{ID: [32]byte{1}, ProfileDigest: f.profile.Digest}, Bootstrap: true, Deadline: deadline}
	if err := funding.ValidateIntent(view, intent); err != nil {
		t.Fatal("valid same-observation draft refused", err)
	}
	for _, boundary := range []string{"profile", "network", "issuer", "receiver", "duty", "expired", "horizon"} {
		t.Run(boundary, func(t *testing.T) {
			changed := intent
			changed.Challenges = append([]token.ClosedTokenContext(nil), intent.Challenges...)
			switch boundary {
			case "profile":
				changed.Selection.ProfileDigest[0] ^= 1
			case "network":
				changed.Challenges[0].NetworkID[0] ^= 1
			case "issuer":
				changed.Challenges[0].IssuerNodeID[0] ^= 1
			case "receiver":
				changed.Challenges[0].ReceiverNodeID = [32]byte{255}
			case "duty":
				changed.Challenges[0].ReceiverDutyGeneration++
			case "expired":
				changed.Deadline = view.ObservedAt()
			case "horizon":
				changed.Deadline = f.profile.NotAfter.Add(time.Second)
			}
			if err := funding.ValidateIntent(view, changed); err == nil {
				t.Fatal("changed observed issuance binding accepted")
			}
		})
	}
}

func newNetworkAdmissionFixture(t *testing.T, prepare ...func(*networkAdmissionFixture)) *networkAdmissionFixture {
	t.Helper()
	dir := t.TempDir()
	spec, plan, inventory := processfixture.PrepareNetworkGenesis(t, dir)
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
	return f.tokensForClass(t, admission.ForwardClass, count)
}

func (f *networkAdmissionFixture) tokensForClass(t *testing.T, class admission.Class, count int) (*stock.Owner, [][]byte) {
	t.Helper()
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	role := admission.AllocationUser
	if class == admission.RegistrationClass {
		role = admission.AllocationPublisher
	}
	holder, err := stock.Open(root, role, f.authority.ObserveFacts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := holder.Close(); err != nil {
			t.Error(err)
		}
	})
	var maxima [3]uint32
	maxima[class-1] = uint32(count)
	raw, digest, err := holder.Request(maxima)
	if err != nil {
		t.Fatal(err)
	}
	encoded := processfixture.FixturePermission(t, raw, f.profile, f.spec.Authority, time.Now())
	if err := holder.Import(digest, encoded); err != nil {
		t.Fatal(err)
	}
	challenge := token.ClosedTokenContext{NetworkID: f.profile.NetworkID, ProfileDigest: f.profile.Digest, IssuerNodeID: f.profile.IssuerNodeID,
		ReceiverNodeID: f.receiver.NodeID, ReceiverDutyGeneration: f.receiver.DutyGeneration, Class: uint8(class), WindowStart: time.Now().UTC().Truncate(time.Hour)}
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
		return f.authority.Issuer(f.plan.KeyBinding.Signer)
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
		raw, err := holder.Take(t.Context(), presentation, uint8(class))
		if err != nil {
			t.Fatal(err)
		}
		tokens = append(tokens, raw)
	}
	return holder, tokens
}

type admissionAuthority = funding.AdmissionAuthority

var validAdmissionAuthority = funding.ValidAdmissionAuthority
var openAdmissionAuthority = funding.OpenAdmissionAuthority
var networkAdmissionAuthority = funding.NetworkAdmissionAuthority

func fundingAuthority(authority admissionAuthority) funding.Authority {
	return funding.Observe(authority.Current, authority.Witness, authority.Presentation)
}
func networkStateConfig(plan *networkAuthorityPlan) (state.Config, error) { return plan.Config() }

type localAdmissionReply = nodefixture.AdmissionReply

// admissionLocalConsole starts the same process console used by E2E fixtures.
func admissionLocalConsole(t *testing.T, operation string, config any) (func(any) localAdmissionReply, func()) {
	t.Helper()
	return nodefixture.OpenAdmissionConsole(t, compiledCommand(t), operation, hostingConfig(t, config))
}
