//go:build linux

package node_test

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	framing "github.com/dianabuilds/ardents-network/internal/route/channel"
	routereceiver "github.com/dianabuilds/ardents-network/internal/route/receiver"
	"github.com/dianabuilds/ardents-network/internal/route/role"
	"github.com/dianabuilds/ardents-network/internal/route/transport"
	roletls "github.com/dianabuilds/ardents-network/internal/route/transport/tls"
)

func TestRoutePendingBootstrapLimitLeavesOrdinaryChildUsableOnSameCarrier(t *testing.T) {
	for _, profile := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		t.Run(string(profile), func(t *testing.T) {
			fixture, authority, _, sockets, certificates := openRouteReceivingFixture(t, 1, false, false)
			current := fixture.State.CurrentRuntime
			facts, _, err := authority.ObserveFacts()
			if err != nil {
				t.Fatal(err)
			}
			// Two distinct genuine tokens: refusal must not refund/reuse the
			// durably presented restricted token to make the sibling succeed.
			holder := receivingFixtureStock(t, fixture, authority, 2, false)
			view, err := current()
			if err != nil {
				t.Fatal(err)
			}
			member, err := view.Member([32]byte{14}, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			duty, err := view.RetainDuty(member.NodeID, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			a := role.Authority{Current: current, Duty: duty, Profile: view.Profile().ProfileBinding}
			binding := receiving.Receiver{NetworkID: facts.NetworkID, StateGeneration: facts.StateGeneration, StateDigest: facts.StateDigest, ProfileDigest: facts.Digest, NodeID: member.NodeID, DutyGeneration: member.DutyGeneration}
			owner, err := receiving.Open(t.TempDir(), binding, func() (receiving.Observation, error) { return authority.Receiver(binding, member.NotAfter()) })
			if err != nil {
				t.Fatal(err)
			}
			defer owner.Close()
			budget := openReceivingFixtureBudget(t)
			var admits, releases, bootstrapHolds, bootstrapReturns atomic.Int32
			sockets[member.NodeID]()
			server, err := routereceiver.Listen(t.Context(), routereceiver.ReceiverConfig{Authority: a, Certificate: certificates[member.NodeID], ReserveBootstrap: func(ctx context.Context, end time.Time) (func() error, error) {
				held, err := budget.Reserve(ctx, hosting.ReservationRequest{Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, WorkUntil: end, HoldUntil: end.Add(time.Second)})
				if err != nil {
					return nil, err
				}
				bootstrapHolds.Add(1)
				return func() error { bootstrapReturns.Add(1); return held.ReleaseJoined() }, nil
			}, Admit: func(ctx context.Context, c routereceiver.Channel, raw []byte) (receiving.Grant, error) {
				admits.Add(1)
				return owner.Accept(ctx, admission.ForwardClass, raw, c.Hello.Deadline, func() (func() error, error) {
					release, err := reserveReceivingFixtureWork(ctx, budget, c.Hello.Deadline)
					if err != nil {
						return nil, err
					}
					return c.HoldReservation(func() error { releases.Add(1); return release() })
				})
			}})
			if err != nil {
				t.Fatal(err)
			}
			defer server.Close()
			end := time.Now().Add(8 * time.Second).UTC().Truncate(time.Second)
			request := transport.ClosedNodeCarrierRequest{CarrierProfile: profile, Endpoint: member.Endpoint, Certificate: certificates[[32]byte{12}], ExpectedPeerKey: member.PublicKey, Deadline: end}
			carrier, err := roletls.OpenNode(t.Context(), request)
			if err != nil {
				t.Fatal(err)
			}
			defer carrier.Close()
			conn, ok := carrier.(net.Conn)
			if !ok {
				t.Fatal("actual Node Carrier lacks connection contract")
			}
			if err := conn.SetDeadline(end); err != nil {
				t.Fatal(err)
			}
			outer, err := a.FreshHello(end, ardp.PurposeForwarding, true)
			if err != nil {
				t.Fatal(err)
			}
			if err := framing.SendHello(conn, outer); err != nil {
				t.Fatal(err)
			}
			if err := framing.Accepted(conn); err != nil {
				t.Fatal(err)
			}
			opened := ardp.Open{RecipientNodeID: member.NodeID, RecipientDutyGeneration: member.DutyGeneration, Purpose: uint8(ardp.PurposeForwarding), Deadline: end}
			restricted, err := ardp.EncodeNodeOpen(ardp.NodeOpen{Recipient: opened, Restriction: ardp.IssuerBootstrapChild})
			if err != nil {
				t.Fatal(err)
			}
			s := framing.New(t.Context(), conn, end, 32<<20, func() error { _, err := a.Hello(outer, true); return err }, false, framing.NewBudget(1<<20), nil)
			defer s.Close()
			var restrictedLanes []*framing.Lane
			for range 4 {
				lane, err := s.Open(t.Context(), t.Context(), restricted)
				if err != nil {
					t.Fatal(err)
				}
				defer lane.Finish()
				restrictedLanes = append(restrictedLanes, lane)
			}
			if !s.Live() || admits.Load() != 0 {
				t.Fatal("bootstrap capacity reached private spend or retired shared Carrier", admits.Load())
			}
			waitUntil := time.Now().Add(2 * time.Second)
			for bootstrapHolds.Load() != 4 && time.Now().Before(waitUntil) {
				time.Sleep(time.Millisecond)
			}
			heldObservation, err := budget.Observe(t.Context())
			if err != nil || bootstrapHolds.Load() != 4 || bootstrapReturns.Load() != 0 || heldObservation.ReservedBytes == 0 {
				t.Fatal("pending TLS did not retain genuine Hosting work", err, bootstrapHolds.Load(), bootstrapReturns.Load())
			}
			// Complete TLS on one restricted child, then attempt actual private
			// admission with a genuinely issued and durably presented token.
			// Route must reject before invoking receiving Admission at all.
			restrictedTLS, err := roletls.OpenRole(t.Context(), restrictedLanes[0], member.PublicKey, end)
			if err != nil {
				t.Fatal("restricted TLS setup", err)
			}
			restrictedHello, err := a.FreshHello(end, ardp.PurposeForwarding, false)
			if err != nil {
				t.Fatal(err)
			}
			presented := false
			err = role.Present(t.Context(), t.Context(), restrictedTLS, a, restrictedHello, func(ctx context.Context, hello ardp.Hello) ([]byte, error) {
				p := stock.Presentation{NetworkID: hello.NetworkID, StateGeneration: hello.StateGeneration, StateDigest: hello.StateDigest, ProfileDigest: hello.ProfileDigest, RecipientNodeID: hello.RecipientNodeID, RecipientDutyGeneration: hello.RecipientDutyGeneration, ChannelNonce: hello.ChannelNonce, Deadline: hello.Deadline}
				if err := authority.Presentation(p); err != nil {
					return nil, err
				}
				raw, err := holder.Take(ctx, p, 2)
				if err != nil {
					return nil, err
				}
				presented = true // Take has independently verified the signature.
				return raw, nil
			})
			if err == nil || errors.Is(err, os.ErrDeadlineExceeded) || !presented || admits.Load() != 0 || !s.Live() || !time.Now().Before(end) {
				t.Fatal("restricted valid-token admission crossed receiving boundary", err, presented, admits.Load(), s.Live())
			}
			if n, err := restrictedLanes[0].Read(make([]byte, 1)); n != 0 || !errors.Is(err, io.EOF) {
				t.Fatal("restricted refusal omitted actual peer lane termination", n, err)
			}
			// This intentional protocol violation ends at the peer's CLOSE.
			// Retain the resulting local TLS shutdown failure rather than
			// reclassifying the forbidden operation as a successful exchange.
			if err := restrictedTLS.Close(); !errors.Is(err, io.EOF) {
				t.Fatal("restricted TLS refusal lost peer-retirement failure", err)
			}
			ordinary, err := ardp.EncodeNodeOpen(ardp.NodeOpen{Recipient: opened, Restriction: ardp.OrdinaryChild})
			if err != nil {
				t.Fatal(err)
			}
			lane, err := s.Open(t.Context(), t.Context(), ordinary)
			if err != nil {
				t.Fatal(err)
			}
			defer lane.Finish()
			secured, err := roletls.OpenRole(t.Context(), lane, member.PublicKey, end)
			if err != nil {
				t.Fatal("ordinary TLS failed alongside pending restricted children", err)
			}
			defer secured.Close()
			h, err := a.FreshHello(end, ardp.PurposeForwarding, false)
			if err != nil {
				t.Fatal(err)
			}
			if err := role.Present(t.Context(), t.Context(), secured, a, h, func(ctx context.Context, hello ardp.Hello) ([]byte, error) {
				p := stock.Presentation{NetworkID: hello.NetworkID, StateGeneration: hello.StateGeneration, StateDigest: hello.StateDigest, ProfileDigest: hello.ProfileDigest, RecipientNodeID: hello.RecipientNodeID, RecipientDutyGeneration: hello.RecipientDutyGeneration, ChannelNonce: hello.ChannelNonce, Deadline: hello.Deadline}
				if err := authority.Presentation(p); err != nil {
					return nil, err
				}
				return holder.Take(ctx, p, 2)
			}); err != nil {
				t.Fatal("ordinary genuine admission unavailable on same Carrier", err)
			}
			if admits.Load() != 1 {
				t.Fatal("restricted allocations entered receiving Admission", admits.Load())
			}
			for _, l := range restrictedLanes {
				if err := l.Close(); err != nil {
					t.Fatal("restricted child retirement failed", err)
				}
			}
			// Keep the shared Carrier alive until the peer has completed its
			// TLS shutdown and emitted this child's actual terminal frame.
			if err := secured.CloseWrite(); err != nil {
				t.Fatal("ordinary TLS half-close failed", err)
			}
			if _, err := io.Copy(io.Discard, secured); err != nil {
				t.Fatal("ordinary TLS peer shutdown failed", err)
			}
			if n, err := lane.Read(make([]byte, 1)); n != 0 || err == nil || errors.Is(err, os.ErrDeadlineExceeded) || !s.Live() {
				t.Fatal("ordinary child did not reach its peer terminal", n, err)
			}
			if err := secured.Close(); err != nil {
				t.Fatal("ordinary TLS retirement failed", err)
			}
			if err := s.Close(); err != nil {
				t.Fatal("shared Carrier retirement failed", err)
			}
			if err := server.Close(); err != nil {
				t.Fatal("receiving mixed Carrier did not join cleanly", err)
			}
			observation, err := budget.Observe(t.Context())
			if err != nil || observation.ReservedBytes != 0 || releases.Load() != 1 || bootstrapHolds.Load() != 4 || bootstrapReturns.Load() != 4 {
				t.Fatal("joined mixed Carrier lost physical reservation", err, releases.Load(), bootstrapHolds.Load(), bootstrapReturns.Load())
			}
		})
	}
}
