//go:build linux

package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/route"
	"github.com/dianabuilds/ardents-network/internal/successor/route/carrier"
	"github.com/dianabuilds/ardents-network/internal/successor/route/selection"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
)

func TestRouteGenuineRetainedPrefixBothCarriers(t *testing.T) {
	for _, profile := range []carrier.CarrierProfile{carrier.ClosedCarrierTCP, carrier.ClosedCarrierQUIC} {
		for _, mode := range []string{"close", "concurrent", "cancel", "clock-loss", "profile-conflict", "successor", "expiry"} {
			t.Run(string(profile)+"/"+mode, func(t *testing.T) {
				f, reservations, certificates := newRouteFixture(t, profile)
				holder := routeStock(t, f)
				var accepted atomic.Int32
				var receivers []*transport.Receiver
				var budgets []*hosting.Budget
				for i := byte(12); i < 16; i++ {
					id := [32]byte{i}
					view, err := f.current()
					if err != nil {
						t.Fatal(err)
					}
					m, err := view.Member(id, view.ObservedAt())
					if err != nil {
						t.Fatal(err)
					}
					duty, err := view.RetainDuty(id, view.ObservedAt())
					if err != nil {
						t.Fatal(err)
					}
					binding := receiving.Receiver{NetworkID: f.profile.NetworkID, StateGeneration: f.profile.StateGeneration, StateDigest: f.profile.StateDigest, ProfileDigest: f.profile.Digest, NodeID: id, DutyGeneration: 9}
					receivingOwner, err := receiving.Open(t.TempDir(), binding, func() (receiving.Observation, error) { return f.authority.receiver(binding, m.NotAfter()) })
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() {
						if err := receivingOwner.Close(); err != nil {
							t.Error(err)
						}
					})
					budget := networkTestBudget(t)
					budgets = append(budgets, budget)
					reservations[id]()
					server, err := transport.Listen(t.Context(), transport.ReceiverConfig{Authority: transport.Authority{Current: f.current, Duty: duty, Profile: view.Profile().ProfileBinding}, Certificate: certificates[id], Admit: func(ctx context.Context, c transport.Channel, raw []byte) (receiving.Grant, error) {
						grant, err := receivingOwner.Accept(ctx, admission.ForwardClass, raw, c.Hello.Deadline, func() (func() error, error) {
							release, err := networkTestReservation(t, budget, c.Hello.Deadline)
							if err != nil {
								return nil, err
							}
							return c.HoldReservation(release)
						})
						if err == nil {
							accepted.Add(1)
						}
						return grant, err
					}})
					if err != nil {
						t.Fatal(err)
					}
					receivers = append(receivers, server)
				}
				defer func() {
					for _, server := range receivers {
						if err := server.Close(); err != nil && mode != "clock-loss" && mode != "profile-conflict" && mode != "successor" && !(mode == "expiry" && routeExpiryCloseOnly(err)) {
							t.Error("receiving join", err)
						}
					}
				}()
				root := t.TempDir()
				budgetRoot := filepath.Join(root, "endpoint-budget")
				start := time.Now().UTC().Truncate(time.Hour)
				if err := hosting.Initialize(budgetRoot, hosting.Policy{Provider: "Route component", Start: start, End: start.Add(2 * time.Hour), Unit: "MiB", Quantity: 100, Directions: "tx+rx", Interfaces: []string{"lo"}, LowWatermarkBytes: 1000}); err != nil {
					t.Fatal(err)
				}
				plan := routePrefixPlan{EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "interior"), HostingRoot: budgetRoot, Domain: 3, Deadline: time.Now().Add(30 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
				// Reopen before transport proves that the actual command consumes the
				// same durable selection, rather than a convenient supplied pair.
				selected, err := selection.Open(selection.Config{EntryRoot: plan.EntryRoot, InteriorRoot: plan.InteriorRoot, Domain: plan.Domain, Current: f.current})
				if err != nil {
					t.Fatal(err)
				}
				before, err := selected.Select()
				if err != nil {
					t.Fatal(err)
				}
				if err := selected.Close(); err != nil {
					t.Fatal(err)
				}
				selected, err = selection.Open(selection.Config{EntryRoot: plan.EntryRoot, InteriorRoot: plan.InteriorRoot, Domain: plan.Domain, Current: f.current})
				if err != nil {
					t.Fatal(err)
				}
				after, err := selected.Select()
				if err != nil {
					t.Fatal(err)
				}
				if before.Entry != after.Entry || before.Interior != after.Interior || before.NotAfter != after.NotAfter {
					t.Fatal("reopen replaced retained leg or horizon")
				}
				if err := selected.Close(); err != nil {
					t.Fatal(err)
				}
				if mode == "expiry" {
					plan.Deadline = time.Now().Add(4 * time.Second).UTC().Truncate(time.Second)
				}
				// Genuine clock-confidence loss must refuse before physical budget
				// or Stock presentation. Recovery permits the same retained tokens.
				f.clockUnavailable.Store(true)
				if _, err := startRoutePrefix(t.Context(), plan, f.authority, holder); err == nil || accepted.Load() != 0 {
					t.Fatal("pre-admission loss reached receiving effects", err)
				}
				f.clockUnavailable.Store(false)
				for _, excluded := range []route.Member{{NodeID: before.Entry.NodeID}, {PublicKey: before.Entry.PublicKey}, {FamilyID: before.Entry.FamilyID}} {
					refused := plan
					refused.Exclusions = []route.Member{excluded}
					if _, err := startRoutePrefix(t.Context(), refused, f.authority, holder); err == nil || accepted.Load() != 0 {
						t.Fatal("known controlled identity/key/family reached transport effects", err)
					}
				}
				prefixContext, cancelPrefix := context.WithCancel(t.Context())
				defer cancelPrefix()
				prefix, err := startRoutePrefix(prefixContext, plan, f.authority, holder)
				if err != nil {
					t.Fatal("actual prefix opening", err)
				}
				if accepted.Load() != 2 {
					t.Fatal("prefix did not admit both real receivers", accepted.Load())
				}
				wantedAdmissions := int32(2)
				var companions []routeHandle
				if mode == "concurrent" {
					wantedAdmissions = 6
					legs := []selection.Leg{before}
					handles := []routeHandle{prefix, {}, {}}
					var openings []func() (routeHandle, error)
					for range 2 {
						other := plan
						other.EntryRoot = filepath.Join(t.TempDir(), "entry")
						other.InteriorRoot = filepath.Join(t.TempDir(), "interior")
						other.HostingRoot = routeProcessBudget(t)
						selected, err := selection.Open(selection.Config{EntryRoot: other.EntryRoot, InteriorRoot: other.InteriorRoot, Domain: 3, Current: f.current})
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
						holder := routeStock(t, f)
						openings = append(openings, func() (routeHandle, error) { return startRoutePrefix(t.Context(), other, f.authority, holder) })
						legs = append(legs, leg)
					}
					start := make(chan struct{})
					type openingResult struct {
						index  int
						handle routeHandle
						err    error
					}
					results := make(chan openingResult, len(openings))
					for index, open := range openings {
						go func() { <-start; handle, err := open(); results <- openingResult{index, handle, err} }()
					}
					close(start)
					var openingErr error
					for range openings {
						opened := <-results
						openingErr = errors.Join(openingErr, opened.err)
						if opened.err == nil {
							handles[opened.index+1] = opened.handle
							companions = append(companions, opened.handle)
							defer opened.handle.close()
						}
					}
					if openingErr != nil {
						t.Fatal("independent concurrent prefix", openingErr)
					}
					if accepted.Load() != wantedAdmissions {
						t.Fatal("concurrent prefixes did not independently admit", accepted.Load())
					}
					first, second := -1, -1
					for i := range legs {
						for j := 0; j < i; j++ {
							if legs[i].Entry.NodeID == legs[j].Entry.NodeID && legs[i].Interior.NodeID == legs[j].Interior.NodeID {
								first, second = j, i
							}
						}
					}
					if first < 0 {
						t.Fatal("fixture did not create shared directed pair")
					}
					if err := handles[first].close(); err != nil {
						t.Fatal(err)
					}
					select {
					case <-handles[second].done:
						t.Fatal("one borrower erased a healthy shared Carrier sibling")
					case <-time.After(150 * time.Millisecond):
					}
				}
				var held uint64
				for _, budget := range budgets {
					observation, err := budget.Observe(t.Context())
					if err != nil {
						t.Fatal(err)
					}
					held += observation.ReservedBytes
				}
				if held == 0 {
					t.Fatal("admitted prefix has no retained physical allowance")
				}
				for _, handle := range companions {
					if err := handle.close(); err != nil {
						t.Fatal("concurrent joined prefix", err)
					}
				}
				switch mode {
				case "cancel":
					cancelPrefix()
				case "clock-loss":
					f.clockUnavailable.Store(true)
				case "profile-conflict":
					if err := f.profileConflict(t); err == nil {
						t.Fatal("conflicting profile accepted")
					}
				case "successor":
					f.successor(t)
				}
				if mode != "close" && mode != "concurrent" {
					select {
					case <-prefix.done:
					case <-time.After(6 * time.Second):
						t.Fatal("retained readiness survived revocation/expiry")
					}
				}
				closeErr := prefix.close()
				if (mode == "close" || mode == "concurrent") && closeErr != nil {
					t.Fatal("prefix joined close", closeErr)
				}
				if mode != "close" && mode != "concurrent" && closeErr == nil {
					t.Fatal("retirement lost its causal refusal")
				}
				if again := prefix.close(); again != closeErr {
					t.Fatal("repeated joined close replaced retained result", again)
				}
				if mode == "clock-loss" {
					f.clockUnavailable.Store(false)
				}
				if mode == "close" || mode == "concurrent" || mode == "cancel" {
					_, err = startRoutePrefix(t.Context(), plan, f.authority, holder)
					if err == nil || accepted.Load() != wantedAdmissions {
						t.Fatal("second opening reused spent stock", err, accepted.Load())
					}
					if errors.Is(err, context.DeadlineExceeded) {
						t.Fatal("refusal exceeded original opening bound")
					}
				}
				// Keep the genuine listeners live: refusal must come from burnt
				// stock, rather than an unreachable socket masking token reuse.
				for _, server := range receivers {
					err := server.Close()
					// Expiry can interrupt a started physical terminal frame;
					// preserve that closed-connection result through owner join.
					if err != nil && mode != "clock-loss" && mode != "profile-conflict" && mode != "successor" && !(mode == "expiry" && routeExpiryCloseOnly(err)) {
						t.Fatal(err)
					}
					if again := server.Close(); again != err {
						t.Fatal("receiver replaced retained terminal result", again)
					}
				}
				for _, budget := range budgets {
					observation, err := budget.Observe(t.Context())
					if err != nil {
						t.Fatal(err)
					}
					if observation.ReservedBytes != 0 {
						t.Fatal("joined receiver leaked reservation", observation.ReservedBytes)
					}
				}
			})
		}
	}
}

func routeExpiryCloseOnly(err error) bool {
	if err == nil {
		return false
	}
	// Inspect wrappers before matching: errors.Is on an aggregate accepts
	// one matching branch while concealing an unrelated sibling failure.
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		causes := joined.Unwrap()
		if len(causes) == 0 {
			return false
		}
		for _, cause := range causes {
			if !routeExpiryCloseOnly(cause) {
				return false
			}
		}
		return true
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return routeExpiryCloseOnly(wrapped.Unwrap())
	}
	return errors.Is(err, net.ErrClosed)
}

func TestRouteExpiryRejectsAdditionalTerminalFailure(t *testing.T) {
	unexpected := errors.New("unexpected physical or release failure")
	for _, err := range []error{
		nil,
		unexpected,
		errors.Join(net.ErrClosed, unexpected),
		fmt.Errorf("close: %w; release: %w", net.ErrClosed, unexpected),
		fmt.Errorf("receiving join: %w", errors.Join(net.ErrClosed, unexpected)),
		errors.Join(fmt.Errorf("physical close: %w", net.ErrClosed), fmt.Errorf("release: %w", unexpected)),
	} {
		if routeExpiryCloseOnly(err) {
			t.Fatalf("expiry assertion concealed an additional failure: %v", err)
		}
	}
	for _, err := range []error{
		net.ErrClosed,
		fmt.Errorf("physical close: %w", net.ErrClosed),
		&net.OpError{Op: "write", Net: "tcp", Err: net.ErrClosed},
		errors.Join(net.ErrClosed, fmt.Errorf("child close: %w", net.ErrClosed)),
	} {
		if !routeExpiryCloseOnly(err) {
			t.Fatalf("expiry assertion rejected a closed-connection-only result: %v", err)
		}
	}
}
