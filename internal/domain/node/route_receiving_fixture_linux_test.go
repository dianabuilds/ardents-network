//go:build linux

package node_test

import (
	"context"
	"crypto/tls"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/token"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
	"github.com/dianabuilds/ardents-network/internal/domain/route/transport"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
	networkinputs "github.com/dianabuilds/ardents-network/tests/networkfixture"
)

func openRouteReceivingFixture(t *testing.T, domain byte, introduction, publication bool, configure ...func(*networkfixture.ClosedSpec, map[[32]byte]func(), map[[32]byte]tls.Certificate)) (*networkinputs.SignedAdmission, funding.AdmissionAuthority, *atomic.Bool, map[[32]byte]func(), map[[32]byte]tls.Certificate) {
	t.Helper()
	reservations := make(map[[32]byte]func())
	certificates := make(map[[32]byte]tls.Certificate)
	fixture, authority, clockUnavailable := openReceivingFixtureState(t, func(spec *networkfixture.ClosedSpec) {
		processfixture.AddRouteFixtureNodes(t, spec, reservations, certificates, "127.0.0.1", string(transport.ClosedCarrierTCP), true, domain, introduction, false)
		if publication {
			processfixture.AddPublicationFixtureNodes(t, spec, reservations, certificates, "127.0.0.1", string(transport.ClosedCarrierTCP), true, false)
		}
		for _, extend := range configure {
			extend(spec, reservations, certificates)
		}
	})
	return fixture, authority, clockUnavailable, reservations, certificates
}

func openReceivingFixtureState(t *testing.T, configure func(*networkfixture.ClosedSpec)) (*networkinputs.SignedAdmission, funding.AdmissionAuthority, *atomic.Bool) {
	t.Helper()
	processfixture.WaitAdmissionHour(t)
	clockUnavailable := new(atomic.Bool)
	fixture := networkinputs.OpenSignedAdmission(t, configure)
	if err := fixture.State.Close(); err != nil {
		t.Fatal(err)
	}
	fixture.StopClock()
	config := fixture.Config
	config.ClockObservationFile = ""
	config.ObserveClock = func() time.Time {
		if clockUnavailable.Load() {
			return time.Time{}
		}
		return time.Now()
	}
	stateOwner, err := state.OpenRetained(config)
	if err != nil {
		t.Fatal(err)
	}
	fixture.State = stateOwner
	authority := funding.NetworkAdmissionAuthority(stateOwner.CurrentRuntime, stateOwner.Close, stateOwner.CurrentRuntimeWitness)
	return fixture, authority, clockUnavailable
}

func receivingFixtureStock(t *testing.T, fixture *networkinputs.SignedAdmission, authority funding.AdmissionAuthority, perMember uint32, introduction bool) *stock.Owner {
	t.Helper()
	facts, now, err := authority.ObserveFacts()
	if err != nil {
		t.Fatal(err)
	}
	role, maxima := admission.AllocationUser, [3]uint32{0, 4 * perMember, 0}
	if introduction {
		role, maxima[2] = admission.AllocationPublisher, 1
	}
	holder := processfixture.OpenPermittedStock(t, t.TempDir(), role, maxima,
		authority.ObserveFacts, facts, fixture.Spec.Authority, now)
	view, err := fixture.State.CurrentRuntime()
	if err != nil {
		t.Fatal(err)
	}
	var challenges []token.ClosedTokenContext
	for i := byte(12); i < 16; i++ {
		id := [32]byte{i}
		member, err := view.Member(id, view.ObservedAt())
		if err != nil {
			t.Fatalf("fixture Forwarder %d unavailable: %v", i, err)
		}
		challenge := token.ClosedTokenContext{
			NetworkID: facts.NetworkID, ProfileDigest: facts.Digest, IssuerNodeID: facts.IssuerNodeID, ReceiverNodeID: id,
			ReceiverDutyGeneration: member.DutyGeneration, Class: uint8(admission.ForwardClass), WindowStart: now.UTC().Truncate(time.Hour),
		}
		for range perMember {
			challenges = append(challenges, challenge)
		}
	}
	batches := [][]token.ClosedTokenContext{challenges}
	if introduction {
		member, err := view.Member([32]byte{16}, view.ObservedAt())
		if err != nil {
			t.Fatal(err)
		}
		batches = append(batches, []token.ClosedTokenContext{{
			NetworkID: facts.NetworkID, ProfileDigest: facts.Digest, IssuerNodeID: facts.IssuerNodeID, ReceiverNodeID: member.NodeID,
			ReceiverDutyGeneration: member.DutyGeneration, Class: uint8(admission.RegistrationClass), WindowStart: now.UTC().Truncate(time.Hour),
		}})
	}
	for index, contexts := range batches {
		attempt, err := holder.Begin(stock.IssuanceIntent{Challenges: contexts,
			Selection: stock.ExchangeBinding{ID: [32]byte{byte(31 + index)}, ProfileDigest: facts.Digest}, Bootstrap: true, Deadline: facts.NotAfter})
		if err != nil {
			t.Fatal(err)
		}
		batch, _, err := attempt.Request()
		if err != nil {
			t.Fatal(err)
		}
		issued := issuer.IssueCurrent(t.Context(), fixture.Plan, batch, quota.Bootstrap, fixture.ObserveIssuer)
		if issued.Outcome != "issued-offline" {
			t.Fatal(issued.Outcome)
		}
		if err := attempt.Complete(issued.Response, nil); err != nil {
			t.Fatal(err)
		}
	}
	return holder
}

func openReceivingFixtureBudget(t *testing.T) *hosting.Budget {
	t.Helper()
	budget, err := hosting.Open(processfixture.ProcessBudget(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := budget.Close(); err != nil {
			t.Error(err)
		}
	})
	return budget
}

func reserveReceivingFixtureWork(ctx context.Context, budget *hosting.Budget, deadline time.Time) (func() error, error) {
	hold, err := budget.Reserve(ctx, hosting.ReservationRequest{
		Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10},
		WorkUntil: deadline, HoldUntil: deadline.Add(time.Second),
	})
	if err != nil {
		return nil, err
	}
	return hold.ReleaseJoined, nil
}
