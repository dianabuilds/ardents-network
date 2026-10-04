//go:build linux

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/carrier"
	"github.com/dianabuilds/ardents-network/internal/successor/route/selection"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
)

func TestRoutePostSpendRefusalRetainsBurnAcrossReopen(t *testing.T) {
	for _, profile := range []carrier.CarrierProfile{carrier.ClosedCarrierTCP, carrier.ClosedCarrierQUIC} {
		t.Run(string(profile), func(t *testing.T) {
			f, reservations, certificates := newRouteFixture(t, profile)
			holder := routeStock(t, f)
			plan := routePrefixPlan{EntryRoot: filepath.Join(t.TempDir(), "entry"), InteriorRoot: filepath.Join(t.TempDir(), "interior"), HostingRoot: routeProcessBudget(t), Domain: 3, Deadline: time.Now().Add(20 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
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
			observe := func() (receiving.Observation, error) { return f.authority.receiver(binding, m.NotAfter()) }
			owner, err := receiving.Open(spendRoot, binding, observe)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = owner.Close() }()
			budget := networkTestBudget(t)
			var releases, spends atomic.Int32
			burnt := make(chan []byte, 1)
			reservations[m.NodeID]()
			server, err := transport.Listen(t.Context(), transport.ReceiverConfig{Authority: transport.Authority{Current: f.current, Duty: leg.Entry, Profile: leg.Profile}, Certificate: certificates[m.NodeID], Admit: func(ctx context.Context, c transport.Channel, raw []byte) (receiving.Grant, error) {
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

func TestRouteInvalidHELLORefusesBeforeAdmission(t *testing.T) {
	for _, profile := range []carrier.CarrierProfile{carrier.ClosedCarrierTCP, carrier.ClosedCarrierQUIC} {
		t.Run(string(profile), func(t *testing.T) {
			f, reservations, certificates := newRouteFixture(t, profile)
			view, err := f.current()
			if err != nil {
				t.Fatal(err)
			}
			m, err := view.Member([32]byte{12}, view.ObservedAt())
			if err != nil {
				t.Fatal(err)
			}
			duty, err := view.RetainDuty(m.NodeID, view.ObservedAt())
			if err != nil {
				t.Fatal(err)
			}
			binding := receiving.Receiver{NetworkID: f.profile.NetworkID, StateGeneration: f.profile.StateGeneration, StateDigest: f.profile.StateDigest, ProfileDigest: f.profile.Digest, NodeID: m.NodeID, DutyGeneration: m.DutyGeneration}
			owner, err := receiving.Open(t.TempDir(), binding, func() (receiving.Observation, error) { return f.authority.receiver(binding, m.NotAfter()) })
			if err != nil {
				t.Fatal(err)
			}
			defer owner.Close()
			budget := networkTestBudget(t)
			var admissionCalls atomic.Int32
			reservations[m.NodeID]()
			server, err := transport.Listen(t.Context(), transport.ReceiverConfig{Authority: transport.Authority{Current: f.current, Duty: duty, Profile: view.Profile().ProfileBinding}, Certificate: certificates[m.NodeID], Admit: func(ctx context.Context, c transport.Channel, raw []byte) (receiving.Grant, error) {
				admissionCalls.Add(1)
				return owner.Accept(ctx, admission.ForwardClass, raw, c.Hello.Deadline, func() (func() error, error) {
					release, err := networkTestReservation(t, budget, c.Hello.Deadline)
					if err != nil {
						return nil, err
					}
					return c.HoldReservation(release)
				})
			}})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := server.Close(); err != nil {
					t.Error(err)
				}
			}()
			for _, fault := range []string{"recipient", "duty", "purpose", "state", "profile", "deadline"} {
				t.Run(fault, func(t *testing.T) {
					conn, err := carrier.OpenClosedRoleCarrier(t.Context(), carrier.ClosedRoleCarrierRequest{CarrierProfile: profile, Endpoint: m.Endpoint, ExpectedServer: m.PublicKey, Deadline: time.Now().Add(3 * time.Second)})
					if err != nil {
						t.Fatal(err)
					}
					defer conn.Close()
					h := ardp.Hello{NetworkID: f.profile.NetworkID, StateGeneration: f.profile.StateGeneration, StateDigest: f.profile.StateDigest, ProfileDigest: f.profile.Digest, RecipientNodeID: m.NodeID, RecipientDutyGeneration: m.DutyGeneration, Purpose: ardp.PurposeForwarding, ChannelNonce: [32]byte{1}, Deadline: time.Now().Add(20 * time.Second).UTC().Truncate(time.Second)}
					switch fault {
					case "recipient":
						h.RecipientNodeID = [32]byte{99}
					case "duty":
						h.RecipientDutyGeneration++
					case "purpose":
						h.Purpose = ardp.PurposeDataJoin
					case "state":
						h.StateGeneration[0] ^= 1
					case "profile":
						h.ProfileDigest[0] ^= 1
					case "deadline":
						h.Deadline = m.NotAfter().Add(time.Second)
					}
					body, err := ardp.EncodeHello(h)
					if err != nil {
						t.Fatal(err)
					}
					if err := ardp.WriteFrame(conn, ardp.Frame{Kind: ardp.KindHello, Body: body}); err != nil {
						t.Fatal(err)
					}
					if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
						t.Fatal(err)
					}
					if _, err := ardp.ReadFrame(conn); err == nil || errors.Is(err, os.ErrDeadlineExceeded) {
						t.Fatal("invalid HELLO reached acceptance or waited for ADMIT", err)
					}
					if admissionCalls.Load() != 0 {
						t.Fatal("invalid HELLO reached receiving Admission")
					}
				})
			}
			observation, err := budget.Observe(t.Context())
			if err != nil || observation.ReservedBytes != 0 {
				t.Fatal("refusal reserved physical capacity", observation, err)
			}
		})
	}
}
