//go:build linux

package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
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
	"github.com/dianabuilds/ardents-network/internal/successor/route/carrier"
	"github.com/dianabuilds/ardents-network/internal/successor/route/selection"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
)

// Each observation returns genuine signed Network facts. Only scheduling of
// caller cancellation propagation is delayed, using the registration harness.
// No successful TLS, receiver admission, ACK, or Hosting result is substituted.
func TestRoutePrefixOriginalCallerHandoffBothCarriers(t *testing.T) {
	for _, profile := range []carrier.CarrierProfile{carrier.ClosedCarrierTCP, carrier.ClosedCarrierQUIC} {
		for checkpoint := int32(0); checkpoint <= 6; checkpoint++ {
			t.Run(fmt.Sprintf("%s/observation-%d", profile, checkpoint), func(t *testing.T) {
				f, reservations, certificates := newRouteFixture(t, profile)
				holder := routeStock(t, f)
				var accepted atomic.Int32
				var servers []*transport.Receiver
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
					server, err := transport.Listen(t.Context(), transport.ReceiverConfig{Authority: transport.Authority{Current: f.current, Duty: duty, Profile: view.Profile().ProfileBinding}, Certificate: certificates[id], Admit: func(ctx context.Context, channel transport.Channel, raw []byte) (receiving.Grant, error) {
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
				prefix, openErr := transport.OpenPrefix(caller, transport.PrefixConfig{Leg: leg, Current: current, Deadline: end, Release: release, Present: func(ctx context.Context, hello ardp.Hello) ([]byte, error) {
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
					if closeErr != nil && transport.TerminalFailureStage(closeErr) != "peer-retired-write" {
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
					if err != nil && transport.TerminalFailureStage(err) != "peer-retired-write" {
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
