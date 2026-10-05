//go:build linux

package main

import (
	"context"
	"errors"
	"fmt"
	routeprefix "github.com/dianabuilds/ardents-network/internal/successor/route/prefix"
	routereceiver "github.com/dianabuilds/ardents-network/internal/successor/route/receiver"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"

	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
	"github.com/dianabuilds/ardents-network/internal/successor/route/selection"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
)

// Each observation returns genuine signed Network facts. Only scheduling of
// caller cancellation propagation is delayed, using the registration harness.
// No successful TLS, receiver admission, ACK, or Hosting result is substituted.
func TestRoutePrefixOriginalCallerHandoffBothCarriers(t *testing.T) {
	for _, profile := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		for checkpoint := int32(0); checkpoint <= 6; checkpoint++ {
			t.Run(fmt.Sprintf("%s/observation-%d", profile, checkpoint), func(t *testing.T) {
				f, reservations, certificates := newRouteFixture(t, profile)
				holder := routeStock(t, f)
				var accepted atomic.Int32
				var servers []*routereceiver.Receiver
				var budgets []*hosting.Budget
				for i := byte(12); i < 16; i++ {
					id := [32]byte{i}
					view, err := f.current()
					if err != nil {
						t.Fatal(err)
					}
					member, err := view.Member(id, view.ObservedAt())
					if err != nil {
						t.Fatal(err)
					}
					duty, err := view.RetainDuty(id, view.ObservedAt())
					if err != nil {
						t.Fatal(err)
					}
					binding := receiving.Receiver{NetworkID: f.profile.NetworkID, StateGeneration: f.profile.StateGeneration, StateDigest: f.profile.StateDigest, ProfileDigest: f.profile.Digest, NodeID: id, DutyGeneration: 9}
					owner, err := receiving.Open(t.TempDir(), binding, func() (receiving.Observation, error) { return f.authority.receiver(binding, member.NotAfter()) })
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() {
						if err := owner.Close(); err != nil {
							t.Error(err)
						}
					})
					budget := networkTestBudget(t)
					budgets = append(budgets, budget)
					reservations[id]()
					server, err := routereceiver.Listen(t.Context(), routereceiver.ReceiverConfig{Authority: role.Authority{Current: f.current, Duty: duty, Profile: view.Profile().ProfileBinding}, Certificate: certificates[id], Admit: func(ctx context.Context, channel routereceiver.Channel, raw []byte) (receiving.Grant, error) {
						grant, err := owner.Accept(ctx, admission.ForwardClass, raw, channel.Hello.Deadline, func() (func() error, error) {
							release, err := networkTestReservation(t, budget, channel.Hello.Deadline)
							if err != nil {
								return nil, err
							}
							return channel.HoldReservation(release)
						})
						if err == nil {
							accepted.Add(1)
						}
						return grant, err
					}})
					if err != nil {
						t.Fatal(err)
					}
					servers = append(servers, server)
				}
				defer func() {
					for _, server := range servers {
						_ = server.Close()
					}
				}()
				root := t.TempDir()
				end := time.Now().Add(30 * time.Second).UTC().Truncate(time.Second)
				selected, err := selection.Open(selection.Config{EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "interior"), Domain: 3, Current: f.current})
				if err != nil {
					t.Fatal(err)
				}
				leg, err := selected.Select()
				if err != nil {
					t.Fatal(err)
				}
				budget, err := hosting.Open(routeProcessBudget(t))
				if err != nil {
					t.Fatal(err)
				}
				defer budget.Close()
				held, err := budget.Reserve(t.Context(), hosting.ReservationRequest{Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, WorkUntil: end, HoldUntil: end.Add(5 * time.Second)})
				if err != nil {
					t.Fatal(err)
				}
				var releaseOnce sync.Once
				var releases atomic.Int32
				var releaseErr error
				release := func() error {
					releaseOnce.Do(func() { releases.Add(1); releaseErr = errors.Join(releaseRouteReservation(held), selected.Close()) })
					return releaseErr
				}
				defer release()
				original, cancelOriginal := context.WithCancel(t.Context())
				defer cancelOriginal()
				caller := &registrationDeferredCaller{Context: original}
				var presented, observations atomic.Int32
				var triggered, returned atomic.Bool
				var presentationMu sync.Mutex
				var presentations []stock.Presentation
				current := func() (network.RuntimeView, error) {
					view, err := f.current()
					if err == nil && presented.Load() == 2 && accepted.Load() == 2 && !returned.Load() {
						observation := observations.Add(1)
						if checkpoint != 0 && observation == checkpoint {
							cancelOriginal()
							triggered.Store(true)
						}
					}
					return view, err
				}
				prefix, openErr := routeprefix.Open(caller, routeprefix.Config{Leg: leg, Current: current, Deadline: end, Release: release, Present: func(ctx context.Context, hello ardp.Hello) ([]byte, error) {
					presentation := stock.Presentation{NetworkID: hello.NetworkID, StateGeneration: hello.StateGeneration, StateDigest: hello.StateDigest, ProfileDigest: hello.ProfileDigest, RecipientNodeID: hello.RecipientNodeID, RecipientDutyGeneration: hello.RecipientDutyGeneration, ChannelNonce: hello.ChannelNonce, Deadline: hello.Deadline}
					if err := f.authority.presentation(presentation); err != nil {
						return nil, err
					}
					raw, err := holder.Take(ctx, presentation, 2)
					if err == nil {
						presentationMu.Lock()
						presentations = append(presentations, presentation)
						presentationMu.Unlock()
						presented.Add(1)
					}
					return raw, err
				}})
				returned.Store(true)
				cancelledAtHandoff := original.Err() != nil
				if prefix != nil {
					closeErr := prefix.Close()
					if closeErr != nil && framing.TerminalFailureStage(closeErr) != "peer-retired-write" {
						t.Error("prefix physical join", closeErr)
					}
					if again := prefix.Close(); again != closeErr {
						t.Error("prefix replaced retained join result", again)
					}
				}
				if releaseErr != nil || releases.Load() != 1 {
					t.Fatal("local joined reservation release", releaseErr, releases.Load())
				}
				if accepted.Load() != 2 || presented.Load() != 2 {
					t.Fatal("scenario lacked two genuine presentations/spends", presented.Load(), accepted.Load(), openErr)
				}
				presentationMu.Lock()
				burnt := append([]stock.Presentation(nil), presentations...)
				presentationMu.Unlock()
				for _, presentation := range burnt {
					raw, err := holder.Take(t.Context(), presentation, 2)
					clear(raw)
					if err == nil {
						t.Error("cancelled handoff refunded genuine presentation")
					}
				}
				for _, server := range servers {
					err := server.Close()
					if err != nil && framing.TerminalFailureStage(err) != "peer-retired-write" {
						t.Error("receiver physical join", err)
					}
					if server.Close() != err {
						t.Error("receiver replaced retained join result")
					}
				}
				for _, physicalBudget := range append(budgets, budget) {
					observation, err := physicalBudget.Observe(t.Context())
					if err != nil || observation.ReservedBytes != 0 {
						t.Error("joined physical reservation retained", observation.ReservedBytes, err)
					}
				}
				if checkpoint == 0 {
					if openErr != nil || prefix == nil || cancelledAtHandoff {
						t.Fatal("positive actual prefix control failed", openErr)
					}
					return
				}
				if !triggered.Load() {
					t.Log("checkpoint beyond opening observations", observations.Load())
					return
				}
				if !cancelledAtHandoff {
					t.Fatal("test cancellation missed original handoff")
				}
				if openErr == nil || prefix != nil {
					t.Errorf("original caller cancelled at genuine post-presentation observation %d, but OpenPrefix returned an accepting prefix", checkpoint)
				} else if !errors.Is(openErr, context.Canceled) {
					t.Errorf("original caller cancellation cause lost: %v", openErr)
				}
			})
		}
	}
}

// Each observation returns genuine signed Network facts. Only scheduling of
// caller cancellation propagation is delayed, using the registration harness.
// No successful TLS, receiver admission, ACK, or Hosting result is substituted.
// Observations 1..5 cover public entry, pre/post-presentation, scheduling and
// selected writer currentness. 6 cancels after durable presentation; 7 after
// genuine additional capacity; 8 after receiving durable spend; 9 during
// Interior presentation after genuine Entry refill has completed.
func TestRouteRefillPublicOriginalCallerBothCarriers(t *testing.T) {
	for _, profile := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		for checkpoint := int32(1); checkpoint <= 9; checkpoint++ {
			t.Run(fmt.Sprintf("%s/observation-%d", profile, checkpoint), func(t *testing.T) {
				f, reservations, certificates := newRouteFixture(t, profile)
				holder := routeRoleStock(t, f, false, true)
				end := time.Now().Add(30 * time.Second).UTC().Truncate(time.Second)
				refillOriginal, cancelRefill := context.WithCancel(t.Context())
				defer cancelRefill()
				refillCaller := &registrationDeferredCaller{Context: refillOriginal}
				type spentToken struct {
					owner  *receiving.Owner
					budget *hosting.Budget
					raw    []byte
				}
				spent := make(chan spentToken, 2)
				var accepted, refillReceived atomic.Int32
				var servers []*routereceiver.Receiver
				var budgets []*hosting.Budget
				for i := byte(12); i < 16; i++ {
					id := [32]byte{i}
					view, err := f.current()
					if err != nil {
						t.Fatal(err)
					}
					member, err := view.Member(id, view.ObservedAt())
					if err != nil {
						t.Fatal(err)
					}
					duty, err := view.RetainDuty(id, view.ObservedAt())
					if err != nil {
						t.Fatal(err)
					}
					binding := receiving.Receiver{NetworkID: f.profile.NetworkID, StateGeneration: f.profile.StateGeneration, StateDigest: f.profile.StateDigest, ProfileDigest: f.profile.Digest, NodeID: id, DutyGeneration: 9}
					owner, err := receiving.Open(t.TempDir(), binding, func() (receiving.Observation, error) { return f.authority.receiver(binding, member.NotAfter()) })
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() {
						if err := owner.Close(); err != nil {
							t.Error(err)
						}
					})
					budget := networkTestBudget(t)
					var originalReservation *hosting.Reservation
					budgets = append(budgets, budget)
					reservations[id]()
					server, err := routereceiver.Listen(t.Context(), routereceiver.ReceiverConfig{Authority: role.Authority{Current: f.current, Duty: duty, Profile: view.Profile().ProfileBinding}, Certificate: certificates[id], Admit: func(ctx context.Context, channel routereceiver.Channel, raw []byte) (receiving.Grant, error) {
						grant, err := owner.Accept(ctx, admission.ForwardClass, raw, channel.Hello.Deadline, func() (func() error, error) {
							var err error
							originalReservation, err = budget.Reserve(ctx, hosting.ReservationRequest{Work: hosting.Traffic{Tx: 16 << 20, Rx: 16 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, WorkUntil: end, HoldUntil: end.Add(time.Second)})
							if err != nil {
								return nil, err
							}
							return channel.HoldReservation(func() error { return releaseRouteReservation(originalReservation) })
						})
						if err == nil {
							accepted.Add(1)
						}
						return grant, err
					}, Refill: func(ctx context.Context, channel routereceiver.Channel, prior receiving.Grant, remaining uint64, raw []byte) (receiving.Grant, error) {
						refillReceived.Add(1)
						if checkpoint >= 8 {
							grant, err := owner.Refill(ctx, prior, remaining, raw, func() (func() error, error) {
								delta := admission.ForwardClass.ByteLimit() - remaining
								addition, err := budget.ReserveAdditionalJoint(ctx, originalReservation, hosting.JointTraffic{Tx: delta, Rx: delta, Total: delta})
								if err != nil {
									return nil, err
								}
								return channel.HoldReservation(func() error { return releaseRouteReservation(addition) })
							})
							if err == nil {
								spent <- spentToken{owner: owner, budget: budget, raw: append([]byte(nil), raw...)}
								if checkpoint == 8 {
									cancelRefill()
								}
							}
							return grant, err
						}
						return receiving.Grant{}, errors.New("canceled public request must not reach receiving refill")
					}})
					if err != nil {
						t.Fatal(err)
					}
					servers = append(servers, server)
				}
				defer func() {
					for _, server := range servers {
						_ = server.Close()
					}
				}()
				root := t.TempDir()
				selected, err := selection.Open(selection.Config{EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "interior"), Domain: 3, Current: f.current})
				if err != nil {
					t.Fatal(err)
				}
				leg, err := selected.Select()
				if err != nil {
					t.Fatal(err)
				}
				budget, err := hosting.Open(routeProcessBudget(t))
				if err != nil {
					t.Fatal(err)
				}
				defer budget.Close()
				held, err := budget.Reserve(t.Context(), hosting.ReservationRequest{Work: hosting.Traffic{Tx: 16 << 20, Rx: 16 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, WorkUntil: end, HoldUntil: end.Add(5 * time.Second)})
				if err != nil {
					t.Fatal(err)
				}
				var releaseOnce sync.Once
				var releases atomic.Int32
				var releaseErr error
				release := func() error {
					releaseOnce.Do(func() { releases.Add(1); releaseErr = errors.Join(releaseRouteReservation(held), selected.Close()) })
					return releaseErr
				}
				defer release()
				original, cancelOriginal := context.WithCancel(t.Context())
				defer cancelOriginal()
				caller := &registrationDeferredCaller{Context: original}
				var presented, holdCalls atomic.Int32
				var refillActive atomic.Bool
				var observations atomic.Int32
				var presentationMu sync.Mutex
				var presentations []stock.Presentation
				current := func() (network.RuntimeView, error) {
					view, err := f.current()
					if err == nil && refillActive.Load() && observations.Add(1) == checkpoint && checkpoint <= 5 {
						cancelRefill()
					}
					return view, err
				}
				prefix, openErr := routeprefix.Open(caller, routeprefix.Config{Leg: leg, Current: current, Deadline: end, Release: release, HoldRefill: func(ctx context.Context, delta uint64) (func() error, error) {
					holdCalls.Add(1)
					if checkpoint >= 7 {
						addition, err := budget.ReserveAdditionalJoint(ctx, held, hosting.JointTraffic{Tx: delta, Rx: delta, Total: delta})
						if err != nil {
							return nil, err
						}
						if checkpoint == 7 {
							cancelRefill()
						}
						return func() error { return releaseRouteReservation(addition) }, nil
					}
					return nil, errors.New("test stops at forbidden capacity effect; no successful reserve substituted")
				}, Present: func(ctx context.Context, hello ardp.Hello) ([]byte, error) {
					presentation := stock.Presentation{NetworkID: hello.NetworkID, StateGeneration: hello.StateGeneration, StateDigest: hello.StateDigest, ProfileDigest: hello.ProfileDigest, RecipientNodeID: hello.RecipientNodeID, RecipientDutyGeneration: hello.RecipientDutyGeneration, ChannelNonce: hello.ChannelNonce, Deadline: hello.Deadline}
					if err := f.authority.presentation(presentation); err != nil {
						return nil, err
					}
					raw, err := holder.Take(ctx, presentation, 2)
					if err == nil {
						presentationMu.Lock()
						presentations = append(presentations, presentation)
						presentationMu.Unlock()
						presented.Add(1)
						if refillActive.Load() && (checkpoint == 6 || checkpoint == 9 && presented.Load() == 4) {
							cancelRefill()
						}
					}
					return raw, err
				}})
				if openErr != nil {
					t.Fatal(openErr)
				}
				refillActive.Store(true)
				refillErr := prefix.Replenish(refillCaller)
				refillActive.Store(false)
				if refillOriginal.Err() == nil {
					t.Fatal("original cancellation did not occur")
				}
				t.Logf("refill error=%v original=%v presented=%d initialAccepts=%d holdCalls=%d", refillErr, refillOriginal.Err(), presented.Load(), accepted.Load(), holdCalls.Load())
				expectedPresentations := int32(3)
				if checkpoint <= 2 {
					expectedPresentations = 2
				}
				if checkpoint == 9 {
					expectedPresentations = 4
				}
				expectedHolds := int32(0)
				if checkpoint >= 7 {
					expectedHolds = 1
				}
				if presented.Load() != expectedPresentations || holdCalls.Load() != expectedHolds {
					t.Errorf("public Replenish performed effects after original caller cancellation: presentations=%d holds=%d", presented.Load(), holdCalls.Load())
				}
				if !errors.Is(refillErr, context.Canceled) {
					t.Error("original refill cancellation cause lost")
				}
				if expectedPresentations >= 3 {
					presentationMu.Lock()
					last := presentations[len(presentations)-1]
					presentationMu.Unlock()
					// This recipient had exactly two genuine issued tokens: initial
					// admission and this refill presentation. Cancellation restores neither.
					raw, err := holder.Take(t.Context(), last, 2)
					clear(raw)
					if err == nil {
						t.Error("canceled public refill refunded durable presentation")
					}
				}
				_ = prefix.Close()
				for _, server := range servers {
					_ = server.Close()
				}
				expectedRefills := int32(0)
				if checkpoint >= 8 {
					expectedRefills = 1
				}
				if refillReceived.Load() != expectedRefills || accepted.Load() != 2 || releases.Load() != 1 || releaseErr != nil {
					t.Error("canceled request emitted refill or failed joined release", refillReceived.Load(), accepted.Load(), releases.Load(), releaseErr)
				}
				if checkpoint >= 8 {
					select {
					case token := <-spent:
						defer clear(token.raw)
						grant, err := token.owner.Accept(t.Context(), admission.ForwardClass, token.raw, end, func() (func() error, error) { return networkTestReservation(t, token.budget, end) })
						if err == nil {
							_ = grant.Release()
						}
						if err == nil || !strings.Contains(err.Error(), "closed token is already spent") {
							t.Error("canceled public refill refunded genuine spend", err)
						}
					default:
						t.Fatal("cancellation did not follow genuine receiving spend")
					}
				}
				for _, physicalBudget := range append(budgets, budget) {
					observation, err := physicalBudget.Observe(t.Context())
					if err != nil || observation.ReservedBytes != 0 {
						t.Error("joined capacity", err, observation.ReservedBytes)
					}
				}
			})
		}
	}
}
