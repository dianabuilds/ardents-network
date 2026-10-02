//go:build linux

package node

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/node/hosting"
	"github.com/dianabuilds/ardents-network/internal/resource"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
	"github.com/dianabuilds/ardents-network/internal/route/replay"
	"github.com/dianabuilds/ardents-network/internal/route/terminal"
)

func TestClosedJoinNodeRefillRejectsReplayThroughBothCarriers(t *testing.T) {
	for _, carrier := range []routecarrier.CarrierProfile{routecarrier.ClosedCarrierTCP, routecarrier.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			fixture := newPrivateRecipientNetworkFixture(t, carrier, ardp.PurposeDataJoin, 2)
			first, firstNonce := sendJoinFixture(t, fixture, 0, 1)
			second, secondNonce := sendJoinFixture(t, fixture, 1, 2)
			nonces := [][32]byte{firstNonce, secondNonce}
			for index, connection := range []net.Conn{first, second} {
				result, err := ardp.ReadFrame(connection)
				if err != nil {
					t.Fatal(err)
				}
				if status, err := terminal.DecodeJoinResult(result.Body, nonces[index]); err != nil || status != 0 {
					t.Fatalf("JOIN side %d refused: %v", index, err)
				}
			}
			refill := ardp.Frame{Kind: ardp.KindAdmit, Lane: 0, Body: append([]byte{2}, fixture.tokens[2]...)}
			if err := ardp.WriteFrame(first, refill); err != nil {
				t.Fatal(err)
			}
			ack, err := ardp.ReadFrame(first)
			if err != nil {
				t.Fatal(err)
			}
			if status, credit, err := ardp.DecodeAcceptFrame(ack); err != nil || status != 0 || credit != 64<<10 {
				t.Fatalf("real refill ACK invalid: %v", err)
			}
			if err := ardp.WriteFrame(first, refill); err != nil {
				t.Fatal(err)
			}
			if frame, err := ardp.ReadFrame(first); err == nil {
				t.Fatalf("replayed fresh token emitted frame kind %d", frame.Kind)
			}
		})
	}
}

// Use real State-authorized token verification, provider-period ledger and
// receiving spend ledger. This isolates ordering from listener host-pressure
// retirement, which is a separate lifecycle responsibility.
func TestClosedJoinRefillActualHostingRefusalDoesNotSpendToken(t *testing.T) {
	var configured Config
	fixture := newPrivateRecipientNetworkFixtureWithStart(t, routecarrier.ClosedCarrierTCP, ardp.PurposeDataJoin, 2, func(config Config) (func() error, error) {
		configured = config
		events := make(chan Event, 32)
		config.Emit = func(_ context.Context, event Event) error { events <- event; return nil }
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { _, err := Run(ctx, config); done <- err }()
		waitForStateEvent(t, events, "READY")
		return func() error { cancel(); return <-done }, nil
	})
	resolved, err := resolveConfig(configured)
	if err != nil {
		t.Fatal(err)
	}
	root := hostingFixtureRoot(t)
	host, err := hosting.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := host.Close(); err != nil {
			t.Error(err)
		}
	}()
	spends, err := replay.Open(t.TempDir(), replay.Binding{NetworkID: fixture.receiver.NetworkID, ProfileDigest: fixture.receiver.ProfileDigest, ReceiverNodeID: fixture.receiver.NodeID, ReceiverDutyGeneration: fixture.receiver.DutyGeneration})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := spends.Close(); err != nil {
			t.Error(err)
		}
	}()
	end := time.Now().UTC().Add(time.Minute).Truncate(time.Second)
	input := route.ClosedAdmissionVerification{Class: 2, Token: fixture.tokens[0], Deadline: end, Hello: ardp.Hello{NetworkID: fixture.receiver.NetworkID, StateGeneration: fixture.receiver.StateGeneration, StateDigest: fixture.receiver.StateDigest, ProfileDigest: fixture.receiver.ProfileDigest, RecipientNodeID: fixture.receiver.NodeID, RecipientDutyGeneration: fixture.receiver.DutyGeneration, Purpose: ardp.PurposeDataJoin, ChannelNonce: [32]byte{1}, Deadline: end}, Exporter: [32]byte{2}}
	policy := hosting.NewJoinHandle(host, nodeAuthority(resolved), time.Now).Replenisher(fixture.receiver, spends)
	sample, err := host.Sample(t.Context(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	// Leave 64 MiB: enough not to hit the low watermark, insufficient for
	// JOIN's actual 130 MiB work+termination envelope in both directions.
	other, err := host.Reserve(t.Context(), resource.HostingTraffic{Tx: sample.Observation.RemainingBytes - (65 << 20)}, resource.HostingTraffic{Tx: 1 << 20}, end)
	if err != nil {
		t.Fatal(err)
	}
	if release, err := policy(input); err == nil {
		if release != nil {
			_ = release()
		}
		t.Fatal("missing actual provider allowance accepted refill")
	}
	if err := other.Release(t.Context()); err != nil {
		t.Fatal(err)
	}
	release, err := policy(input)
	if err != nil {
		t.Fatalf("refused Hosting reserve burned fresh token: %v", err)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
	before, err := host.Sample(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if release, err := policy(input); err == nil {
		if release != nil {
			_ = release()
		}
		t.Fatal("durably spent refill token accepted again")
	}
	after, err := host.Sample(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if after.Observation.ReservedBytes != before.Observation.ReservedBytes {
		t.Fatal("failed durable spend retained fresh host reservation")
	}
}
