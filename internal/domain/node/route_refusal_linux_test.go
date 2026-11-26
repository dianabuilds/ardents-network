//go:build linux

package node_test

import (
	"context"
	routereceiver "github.com/dianabuilds/ardents-network/internal/route/receiver"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/domain/hosting"

	"github.com/dianabuilds/ardents-network/internal/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/endpoint/session"
	"github.com/dianabuilds/ardents-network/internal/route/role"
	"github.com/dianabuilds/ardents-network/internal/route/selection"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
)

func TestRoutePostSpendRefusalRetainsBurnAcrossReopen(t *testing.T) {
	fixture, authority, clockUnavailable, reservations, certificates := openRouteReceivingFixture(t, 3, false, false)
	stateOwner := fixture.State
	profile, now, err := authority.ObserveFacts()
	if err != nil {
		t.Fatal(err)
	}
	holder := receivingFixtureStock(t, fixture, authority, 1, false)
	work := hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}
	termination := hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}
	plan := funding.Plan{EntryRoot: filepath.Join(t.TempDir(), "entry"), InteriorRoot: filepath.Join(t.TempDir(), "interior"),
		HostingRoot: processfixture.ProcessBudget(t), Domain: 3, Deadline: now.Add(20 * time.Second).UTC().Truncate(time.Second), Work: work, Termination: termination}

	selected, err := selection.Open(selection.Config{EntryRoot: plan.EntryRoot, InteriorRoot: plan.InteriorRoot, Domain: 3, Current: stateOwner.CurrentRuntime})
	if err != nil {
		t.Fatal(err)
	}
	leg, err := selected.Select()
	if err != nil {
		t.Fatal(err)
	}
	if err := selected.Close(); err != nil {
		t.Fatal(err)
	}
	m := leg.EntryMember
	binding := receiving.Receiver{NetworkID: profile.NetworkID, StateGeneration: profile.StateGeneration, StateDigest: profile.StateDigest, ProfileDigest: profile.Digest, NodeID: m.NodeID, DutyGeneration: m.DutyGeneration}
	spendRoot := t.TempDir()
	observe := func() (receiving.Observation, error) { return authority.Receiver(binding, m.NotAfter()) }
	owner, err := receiving.Open(spendRoot, binding, observe)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := owner.Close(); err != nil {
			t.Error("close original receiving owner:", err)
		}
	})
	budget, err := hosting.Open(processfixture.ProcessBudget(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := budget.Close(); err != nil {
			t.Error(err)
		}
	})
	reserve := func(deadline time.Time) (func() error, error) {
		held, err := budget.Reserve(t.Context(), hosting.ReservationRequest{
			Work: work, Termination: termination, WorkUntil: deadline, HoldUntil: deadline.Add(time.Second),
		})
		if err != nil {
			return nil, err
		}
		return func() error {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			return held.Release(ctx)
		}, nil
	}
	var releases, spends atomic.Int32
	spendEvidence := processfixture.ObserveReceivingSpends(t, owner, spendRoot, binding, observe)
	reservations[m.NodeID]()
	server, err := routereceiver.Listen(t.Context(), routereceiver.ReceiverConfig{Authority: role.Authority{Current: stateOwner.CurrentRuntime, Duty: leg.Entry, Profile: leg.Profile}, Certificate: certificates[m.NodeID], Admit: func(ctx context.Context, c routereceiver.Channel, raw []byte) (receiving.Grant, error) {
		grant, err := owner.Accept(ctx, admission.ForwardClass, raw, c.Hello.Deadline, func() (func() error, error) {
			release, err := reserve(c.Hello.Deadline)
			if err != nil {
				return nil, err
			}
			return c.HoldReservation(func() error { releases.Add(1); return release() })
		})
		if err == nil {
			spends.Add(1)
			spendEvidence.Record(admission.ForwardClass, raw)
			clockUnavailable.Store(true)
		}
		return grant, err
	}})
	if err != nil {
		t.Fatal(err)
	}
	var serverJoined bool
	t.Cleanup(func() {
		if !serverJoined {
			if err := server.Close(); err != nil {
				t.Error("join receiving server:", err)
			}
		}
	})
	prefix, err := session.OpenPrefix(t.Context(), plan, funding.Observe(authority.Current, authority.Witness, authority.Presentation), holder)
	if err == nil {
		if closeErr := prefix.Close(); closeErr != nil {
			t.Error("close unexpectedly admitted Prefix:", closeErr)
		}
	}
	if err == nil || spends.Load() != 1 {
		t.Fatal("post-spend authority loss accepted or failed before burn", err, spends.Load())
	}
	clockUnavailable.Store(false)
	joinErr := server.Close()
	serverJoined = true
	if joinErr != nil {
		t.Fatal("join receiver after authority loss:", joinErr)
	}
	if releases.Load() != 1 {
		t.Fatal("refused Grant capacity did not return exactly once", releases.Load())
	}
	observation, err := budget.Observe(t.Context())
	if err != nil || observation.ReservedBytes != 0 {
		t.Fatal("refused physical work leaked reserve", err)
	}
	if replayed := spendEvidence.RequireRetained(t, t.Context(), reserve); replayed != 1 {
		t.Fatalf("reopened %d original spends, want one", replayed)
	}
	observation, err = budget.Observe(t.Context())
	if err != nil || observation.ReservedBytes != 0 {
		t.Fatal("replay refusal leaked rollback reserve", err)
	}
}
