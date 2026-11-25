//go:build linux

package main

import (
	"bytes"
	"context"
	routereceiver "github.com/dianabuilds/ardents-network/internal/route/receiver"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/hosting"

	"github.com/dianabuilds/ardents-network/internal/route/role"
	"github.com/dianabuilds/ardents-network/internal/route/selection"
	"github.com/dianabuilds/ardents-network/internal/route/transport"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
)

func TestRoutePostSpendRefusalRetainsBurnAcrossReopen(t *testing.T) {
	for _, profile := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		t.Run(string(profile), func(t *testing.T) {
			f, reservations, certificates := newRouteFixture(t, profile)
			holder := routeStock(t, f)
			plan := routePrefixPlan{EntryRoot: filepath.Join(t.TempDir(), "entry"), InteriorRoot: filepath.Join(t.TempDir(), "interior"), HostingRoot: processfixture.ProcessBudget(t), Domain: 3, Deadline: time.Now().Add(20 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
			selected, err := selection.Open(selection.Config{EntryRoot: plan.EntryRoot, InteriorRoot: plan.InteriorRoot, Domain: 3, Current: f.current})
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
			binding := receiving.Receiver{NetworkID: f.profile.NetworkID, StateGeneration: f.profile.StateGeneration, StateDigest: f.profile.StateDigest, ProfileDigest: f.profile.Digest, NodeID: m.NodeID, DutyGeneration: m.DutyGeneration}
			spendRoot := t.TempDir()
			observe := func() (receiving.Observation, error) { return f.authority.Receiver(binding, m.NotAfter()) }
			owner, err := receiving.Open(spendRoot, binding, observe)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = owner.Close() }()
			budget := networkTestBudget(t)
			var releases, spends atomic.Int32
			burnt := make(chan []byte, 1)
			reservations[m.NodeID]()
			server, err := routereceiver.Listen(t.Context(), routereceiver.ReceiverConfig{Authority: role.Authority{Current: f.current, Duty: leg.Entry, Profile: leg.Profile}, Certificate: certificates[m.NodeID], Admit: func(ctx context.Context, c routereceiver.Channel, raw []byte) (receiving.Grant, error) {
				grant, err := owner.Accept(ctx, admission.ForwardClass, raw, c.Hello.Deadline, func() (func() error, error) {
					release, err := networkTestReservation(t, budget, c.Hello.Deadline)
					if err != nil {
						return nil, err
					}
					return c.HoldReservation(func() error { releases.Add(1); return release() })
				})
				if err == nil {
					spends.Add(1)
					burnt <- bytes.Clone(raw)
					f.clockUnavailable.Store(true)
				}
				return grant, err
			}})
			if err != nil {
				t.Fatal(err)
			}
			_, err = startRoutePrefix(t.Context(), plan, f.authority, holder)
			if err == nil || spends.Load() != 1 {
				t.Fatal("post-spend authority loss accepted or failed before burn", err, spends.Load())
			}
			f.clockUnavailable.Store(false)
			_ = server.Close()
			if releases.Load() != 1 {
				t.Fatal("refused Grant capacity did not return exactly once", releases.Load())
			}
			observation, err := budget.Observe(t.Context())
			if err != nil || observation.ReservedBytes != 0 {
				t.Fatal("refused physical work leaked reserve", err)
			}
			if err := owner.Close(); err != nil {
				t.Fatal(err)
			}
			owner, err = receiving.Open(spendRoot, binding, observe)
			if err != nil {
				t.Fatal(err)
			}
			var raw []byte
			select {
			case raw = <-burnt:
			default:
				t.Fatal("actual presented token not captured")
			}
			defer clear(raw)
			deadline := time.Now().Add(5 * time.Second)
			grant, err := owner.Accept(t.Context(), admission.ForwardClass, raw, deadline, func() (func() error, error) { return networkTestReservation(t, budget, deadline) })
			if err == nil {
				_ = grant.Release()
				t.Fatal("reopen restored post-spend refused token")
			}
			observation, err = budget.Observe(t.Context())
			if err != nil || observation.ReservedBytes != 0 {
				t.Fatal("replay refusal leaked rollback reserve", err)
			}
		})
	}
}
