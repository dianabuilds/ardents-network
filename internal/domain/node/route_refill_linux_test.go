//go:build linux

package node_test

import (
	"context"
	"crypto/rand"
	framing "github.com/dianabuilds/ardents-network/internal/route/channel"
	routereceiver "github.com/dianabuilds/ardents-network/internal/route/receiver"
	roletls "github.com/dianabuilds/ardents-network/internal/route/transport/tls"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	"github.com/dianabuilds/ardents-network/internal/domain/network"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"

	"github.com/dianabuilds/ardents-network/internal/route/role"
	"github.com/dianabuilds/ardents-network/internal/route/transport"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
)

// This exercises authority loss at a genuine receiving forwarding parent
// before and after the durable refill spend.
func TestRouteReceivingRefillFailureAndReopenTCP(t *testing.T) {
	fixture, authority, clockUnavailable, reservations, certificates := openRouteReceivingFixture(t, 3, false, false)
	currentState := fixture.State.CurrentRuntime
	profile, _, err := authority.ObserveFacts()
	if err != nil {
		t.Fatal(err)
	}
	view, err := currentState()
	if err != nil {
		t.Fatal(err)
	}
	member, err := view.Member(fixture.Spec.Nodes[1].NodeID, view.ObservedAt())
	if err != nil {
		t.Fatal(err)
	}
	duty, err := view.RetainDuty(member.NodeID, view.ObservedAt())
	if err != nil {
		t.Fatal(err)
	}
	binding := receiving.Receiver{NetworkID: profile.NetworkID, StateGeneration: profile.StateGeneration, StateDigest: profile.StateDigest,
		ProfileDigest: profile.Digest, NodeID: member.NodeID, DutyGeneration: member.DutyGeneration}

	reservations[member.NodeID]()
	for _, mode := range []string{"authority-before-spend", "authority-after-spend"} {
		t.Run(mode, func(t *testing.T) {
			holder := receivingFixtureStock(t, fixture, authority, 2, false)
			budgetRoot, spendRoot := processfixture.ProcessBudget(t), t.TempDir()
			budget, err := hosting.Open(budgetRoot)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := budget.Close(); err != nil {
					t.Error(err)
				}
			})
			owner, err := receiving.Open(spendRoot, binding, func() (receiving.Observation, error) { return authority.Receiver(binding, member.NotAfter()) })
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
			var conn net.Conn
			type result struct {
				remaining uint64
				err       error
			}
			refilled := make(chan result, 1)
			var lossOnce sync.Once
			var lossErr error
			current := func() (network.RuntimeView, error) {
				observed, err := currentState()
				if err != nil && clockUnavailable.Load() {
					// Retain the genuine observed failure for exact terminal
					// provenance; successful views always come from Network.
					lossOnce.Do(func() { lossErr = err })
					return observed, lossErr
				}
				return observed, err
			}
			joinedFailure := func(err error) bool { return err == nil || refillAuthorityRetirementOnly(err, lossErr) }
			server, err := routereceiver.Listen(t.Context(), routereceiver.ReceiverConfig{Authority: role.Authority{Current: current, Duty: duty, Profile: view.Profile().ProfileBinding}, Certificate: certificates[member.NodeID],
				Admit: func(ctx context.Context, channel routereceiver.Channel, raw []byte) (receiving.Grant, error) {
					return owner.Accept(ctx, admission.ForwardClass, raw, channel.Hello.Deadline, func() (func() error, error) {
						var err error
						original, err = budget.Reserve(ctx, request)
						if err != nil {
							return nil, err
						}
						return channel.HoldReservation(original.ReleaseJoined)
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
						release, err := channel.HoldReservation(addition.ReleaseJoined)
						if err == nil && mode == "authority-before-spend" {
							clockUnavailable.Store(true)
						}
						return release, err
					})
					if err == nil && mode == "authority-after-spend" {
						clockUnavailable.Store(true)
					}
					refilled <- result{remaining, err}
					return next, err
				}})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if conn != nil {
					_ = conn.Close()
				}
				if err := server.Close(); !joinedFailure(err) {
					t.Error(err)
				}
				clockUnavailable.Store(false)
			})
			conn, err = roletls.OpenEndpoint(t.Context(), transport.ClosedRoleCarrierRequest{CarrierProfile: transport.ClosedCarrierTCP, Endpoint: member.Endpoint, ExpectedServer: member.PublicKey, Deadline: end})
			if err != nil {
				t.Fatal(err)
			}
			if err := conn.SetDeadline(end); err != nil {
				t.Fatal(err)
			}
			hello := ardp.Hello{NetworkID: profile.NetworkID, StateGeneration: profile.StateGeneration, StateDigest: profile.StateDigest, ProfileDigest: profile.Digest, RecipientNodeID: member.NodeID, RecipientDutyGeneration: member.DutyGeneration, Purpose: ardp.PurposeForwarding, Deadline: end}
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
				if err := authority.Presentation(presentation); err != nil {
					t.Fatal(err)
				}
				raw, err := holder.Take(t.Context(), presentation, 2)
				if err != nil {
					t.Fatal(err)
				}
				if err := authority.Presentation(presentation); err != nil {
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
			if !clockUnavailable.Load() {
				t.Fatal("authority-loss probe did not reach its actual reservation/spend boundary")
			}
			if mode == "authority-before-spend" {
				if observed.err == nil {
					t.Fatal("authority loss reached successful spend")
				}
				if _, err := ardp.ReadFrame(conn); err == nil {
					t.Fatal("refused refill emitted acceptance")
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
			clockUnavailable.Store(false)
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
			owner, err = receiving.Open(spendRoot, binding, func() (receiving.Observation, error) { return authority.Receiver(binding, member.NotAfter()) })
			if err != nil {
				t.Fatal(err)
			}
			grant, err := owner.Accept(t.Context(), admission.ForwardClass, raw, end, func() (func() error, error) {
				reservation, err := budget.Reserve(t.Context(), request)
				if err != nil {
					return nil, err
				}
				return reservation.ReleaseJoined, nil
			})
			if mode == "authority-before-spend" {
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
}

func refillAuthorityRetirementOnly(err, authorityFailure error) bool {
	if authorityFailure == nil {
		return false
	}
	return processfixture.OnlyFailureBranches(err, func(branch error) bool {
		return branch == authorityFailure || framing.TerminalFailureStage(branch) == "peer-retired-write"
	})
}
