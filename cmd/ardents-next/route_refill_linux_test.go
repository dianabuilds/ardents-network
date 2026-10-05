//go:build linux

package main

import (
	"context"
	"crypto/rand"
	"errors"
	routereceiver "github.com/dianabuilds/ardents-network/internal/successor/route/receiver"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"

	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
)

// This exercises a genuine receiving forwarding parent, not a successful
// substitute recipient. The ACK-loss gate injects failure only after real
// verification, additional Hosting reservation and durable spend. Compiled
// prefix/child consumers are covered separately by the process scenario.
func TestRouteReceivingRefillFailureAndReopenBothCarriers(t *testing.T) {
	for _, profile := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		t.Run(string(profile), func(t *testing.T) {
			f, reservations, certificates := newRouteFixture(t, profile)
			view, err := f.current()
			if err != nil {
				t.Fatal(err)
			}
			member, err := view.Member(f.receiver.NodeID, view.ObservedAt())
			if err != nil {
				t.Fatal(err)
			}
			duty, err := view.RetainDuty(member.NodeID, view.ObservedAt())
			if err != nil {
				t.Fatal(err)
			}
			reservations[member.NodeID]()
			for _, mode := range []string{"capacity-before-spend", "authority-before-spend", "lost-ack-after-spend", "authority-after-spend"} {
				t.Run(mode, func(t *testing.T) {
					holder := routeRoleStock(t, f, false, true)
					budgetRoot, spendRoot := routeProcessBudget(t), t.TempDir()
					budget, err := hosting.Open(budgetRoot)
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() {
						if err := budget.Close(); err != nil {
							t.Error(err)
						}
					})
					owner, err := receiving.Open(spendRoot, f.receiver, func() (receiving.Observation, error) { return f.authority.receiver(f.receiver, member.NotAfter()) })
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() {
						if err := owner.Close(); err != nil {
							t.Error(err)
						}
					})
					end := time.Now().Add(20 * time.Second).UTC().Truncate(time.Second)
					request := hosting.ReservationRequest{Work: hosting.Traffic{Tx: 16 << 20, Rx: 16 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, WorkUntil: end, HoldUntil: end.Add(time.Second)}
					var original *hosting.Reservation
					var blocker *hosting.Reservation
					var conn net.Conn
					gate := make(chan struct{})
					var gateOnce sync.Once
					openGate := func() { gateOnce.Do(func() { close(gate) }) }
					type result struct {
						remaining uint64
						err       error
					}
					refilled := make(chan result, 1)
					var lossOnce sync.Once
					var lossErr error
					current := func() (network.RuntimeView, error) {
						observed, err := f.current()
						if err != nil && f.clockUnavailable.Load() {
							// Retain the genuine observed failure for exact terminal
							// provenance; successful views always come from Network.
							lossOnce.Do(func() { lossErr = err })
							return observed, lossErr
						}
						return observed, err
					}
					joinedFailure := func(err error) bool { return err == nil || registrationAuthorityRetirementOnly(err, lossErr) }
					server, err := routereceiver.Listen(t.Context(), routereceiver.ReceiverConfig{Authority: role.Authority{Current: current, Duty: duty, Profile: view.Profile().ProfileBinding}, Certificate: certificates[member.NodeID],
						Admit: func(ctx context.Context, channel routereceiver.Channel, raw []byte) (receiving.Grant, error) {
							return owner.Accept(ctx, admission.ForwardClass, raw, channel.Hello.Deadline, func() (func() error, error) {
								var err error
								original, err = budget.Reserve(ctx, request)
								if err != nil {
									return nil, err
								}
								return channel.HoldReservation(func() error { return releaseRouteReservation(original) })
							})
						},
						Refill: func(ctx context.Context, channel routereceiver.Channel, prior receiving.Grant, remaining uint64, raw []byte) (receiving.Grant, error) {
							next, err := owner.Refill(ctx, prior, remaining, raw, func() (func() error, error) {
								limit := admission.ForwardClass.ByteLimit()
								if err := budget.CoversJoint(ctx, original, hosting.JointTraffic{Tx: limit, Rx: limit, Total: limit}); err != nil {
									return nil, err
								}
								delta := limit - remaining
								addition, err := budget.ReserveAdditionalJoint(ctx, original, hosting.JointTraffic{Tx: delta, Rx: delta, Total: delta})
								if err != nil {
									return nil, err
								}
								release, err := channel.HoldReservation(func() error { return releaseRouteReservation(addition) })
								if err == nil && mode == "authority-before-spend" {
									f.clockUnavailable.Store(true)
								}
								return release, err
							})
							if err == nil && mode == "authority-after-spend" {
								f.clockUnavailable.Store(true)
							}
							refilled <- result{remaining, err}
							if mode == "lost-ack-after-spend" && err == nil {
								<-gate
								return next, errors.Join(ctx.Err(), errors.New("injected loss after actual durable refill"))
							}
							return next, err
						}})
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() {
						openGate()
						if conn != nil {
							_ = conn.Close()
						}
						if err := server.Close(); !joinedFailure(err) {
							t.Error(err)
						}
						f.clockUnavailable.Store(false)
						if blocker != nil {
							if err := releaseRouteReservation(blocker); err != nil {
								t.Error(err)
							}
						}
					})
					conn, err = routeTestOpenEndpoint(t.Context(), transport.ClosedRoleCarrierRequest{CarrierProfile: profile, Endpoint: member.Endpoint, ExpectedServer: member.PublicKey, Deadline: end})
					if err != nil {
						t.Fatal(err)
					}
					if err := conn.SetDeadline(end); err != nil {
						t.Fatal(err)
					}
					hello := ardp.Hello{NetworkID: f.profile.NetworkID, StateGeneration: f.profile.StateGeneration, StateDigest: f.profile.StateDigest, ProfileDigest: f.profile.Digest, RecipientNodeID: member.NodeID, RecipientDutyGeneration: member.DutyGeneration, Purpose: ardp.PurposeForwarding, Deadline: end}
					if _, err := rand.Read(hello.ChannelNonce[:]); err != nil {
						t.Fatal(err)
					}
					body, err := ardp.EncodeHello(hello)
					if err != nil {
						t.Fatal(err)
					}
					if err := ardp.WriteFrame(conn, ardp.Frame{Kind: ardp.KindHello, Body: body}); err != nil {
						t.Fatal(err)
					}
					presentation := stock.Presentation{NetworkID: hello.NetworkID, StateGeneration: hello.StateGeneration, StateDigest: hello.StateDigest, ProfileDigest: hello.ProfileDigest, RecipientNodeID: hello.RecipientNodeID, RecipientDutyGeneration: hello.RecipientDutyGeneration, ChannelNonce: hello.ChannelNonce, Deadline: hello.Deadline}
					take := func() []byte {
						if err := f.authority.presentation(presentation); err != nil {
							t.Fatal(err)
						}
						raw, err := holder.Take(t.Context(), presentation, 2)
						if err != nil {
							t.Fatal(err)
						}
						if err := f.authority.presentation(presentation); err != nil {
							t.Fatal(err)
						}
						return raw
					}
					first := take()
					defer clear(first)
					if err := ardp.WriteFrame(conn, ardp.Frame{Kind: ardp.KindAdmit, Body: append([]byte{2}, first...)}); err != nil {
						t.Fatal(err)
					}
					ack, err := ardp.ReadFrame(conn)
					if err != nil {
						t.Fatal(err)
					}
					status, credit, err := ardp.DecodeAcceptFrame(ack)
					if err != nil || status != 0 || credit != 65536 {
						t.Fatal("initial genuine admission failed", err)
					}
					before, err := budget.Observe(t.Context())
					if err != nil || before.ReservedBytes != (32<<20)+(128<<10) {
						t.Fatal("original reserve", before, err)
					}
					if mode == "capacity-before-spend" {
						// Competing real work leaves less than the exact refill delta
						// above the existing watermark. No supplied capacity verdict.
						blocker, err = budget.Reserve(t.Context(), hosting.ReservationRequest{Work: hosting.Traffic{Tx: before.RemainingBytes - 1501}, Termination: hosting.Traffic{Tx: 1}, WorkUntil: end, HoldUntil: end.Add(time.Second)})
						if err != nil {
							t.Fatal("competing real reserve", err)
						}
					}
					raw := take()
					defer clear(raw)
					if err := ardp.WriteFrame(conn, ardp.Frame{Kind: ardp.KindAdmit, Body: append([]byte{2}, raw...)}); err != nil {
						t.Fatal(err)
					}
					var observed result
					select {
					case observed = <-refilled:
					case <-time.After(5 * time.Second):
						t.Fatal("refill did not reach genuine owner")
					}
					// Initial HELLO/ADMIT/ACCEPT = 617; later ADMIT = 371.
					if observed.remaining != (32<<20)-617-371 {
						t.Fatal("old wire accounting", observed.remaining)
					}
					if (mode == "authority-before-spend" || mode == "authority-after-spend") && !f.clockUnavailable.Load() {
						t.Fatal("authority-loss probe did not reach its actual reservation/spend boundary")
					}
					if mode == "capacity-before-spend" || mode == "authority-before-spend" {
						if mode == "capacity-before-spend" && !errors.Is(observed.err, hosting.ErrCapacity) {
							t.Fatal("not an actual capacity refusal", observed.err)
						}
						if mode == "authority-before-spend" && observed.err == nil {
							t.Fatal("authority loss reached successful spend")
						}
						if _, err := ardp.ReadFrame(conn); err == nil {
							t.Fatal("refused refill emitted acceptance")
						}
					} else if mode == "lost-ack-after-spend" {
						if observed.err != nil {
							t.Fatal("real refill failed before injected ACK loss", observed.err)
						}
						held, err := budget.Observe(t.Context())
						if err != nil || held.ReservedBytes != before.ReservedBytes+988 {
							t.Fatal("addition absent before join", held, err)
						}
						if err := conn.Close(); err != nil {
							t.Fatal(err)
						}
						closed := make(chan error, 1)
						go func() { closed <- server.Close() }()
						stillHeld, err := budget.Observe(t.Context())
						if err != nil || stillHeld.ReservedBytes != held.ReservedBytes {
							t.Fatal("reservation returned before blocked worker joined", stillHeld, err)
						}
						select {
						case err := <-closed:
							t.Fatal("unjoined refill closed early", err)
						default:
						}
						openGate()
						if err := <-closed; err != nil {
							t.Fatal(err)
						}
					} else {
						if observed.err != nil {
							t.Fatal("authority loss occurred before real spend", observed.err)
						}
						if _, err := ardp.ReadFrame(conn); err == nil {
							t.Fatal("lost authority emitted late acceptance")
						}
					}
					_ = conn.Close()
					if err := server.Close(); !joinedFailure(err) {
						t.Fatal(err)
					}
					f.clockUnavailable.Store(false)
					if blocker != nil {
						if err := releaseRouteReservation(blocker); err != nil {
							t.Fatal(err)
						}
						blocker = nil
					}
					if err := owner.Close(); err != nil {
						t.Fatal(err)
					}
					if err := budget.Close(); err != nil {
						t.Fatal(err)
					}
					budget, err = hosting.Open(budgetRoot)
					if err != nil {
						t.Fatal(err)
					}
					owner, err = receiving.Open(spendRoot, f.receiver, func() (receiving.Observation, error) { return f.authority.receiver(f.receiver, member.NotAfter()) })
					if err != nil {
						t.Fatal(err)
					}
					grant, err := owner.Accept(t.Context(), admission.ForwardClass, raw, end, func() (func() error, error) {
						reservation, err := budget.Reserve(t.Context(), request)
						if err != nil {
							return nil, err
						}
						return func() error { return releaseRouteReservation(reservation) }, nil
					})
					if mode == "capacity-before-spend" || mode == "authority-before-spend" {
						if err != nil {
							t.Fatal("pre-spend refusal consumed token", err)
						}
						if err := grant.Release(); err != nil {
							t.Fatal(err)
						}
					} else if err == nil || !strings.Contains(err.Error(), "closed token is already spent") {
						t.Fatal("lost ACK refunded durable spend", err)
					}
					final, err := budget.Observe(t.Context())
					if err != nil || final.ReservedBytes != 0 || final.UsedBytes < before.UsedBytes {
						t.Fatal("joined/reopened accounting", final, err)
					}
				})
			}
		})
	}
}
