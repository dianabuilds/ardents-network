package quic

import (
	"context"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	"github.com/quic-go/quic-go"
	"io"
	"net"
	"testing"
	"time"
)

// The real QUIC window, unlike TCP buffering, bounds how much this writer can
// finish before the peer consumes bytes. This is an adapter-specific control,
// portable across operating systems, not a substitute for Route joined cleanup.
func TestQUICPhysicalRetirementInterruptsStartedWrite(t *testing.T) {
	for _, scenario := range []struct{ node, remote bool }{{false, false}, {true, false}, {false, true}, {true, true}} {
		node := scenario.node
		t.Run(map[bool]string{false: "role", true: "node"}[node]+map[bool]string{false: "/local", true: "/remote"}[scenario.remote], func(t *testing.T) {
			now := time.Now()
			serverCert, serverKey := transportCertificate(t, now.Add(-time.Hour), now.Add(time.Hour))
			clientCert, clientKey := transportCertificate(t, now.Add(-time.Hour), now.Add(time.Hour))
			endpoint := transportEndpoint(t, transport.ClosedCarrierQUIC)
			listener, err := ListenShared(endpoint, serverCert, func(key [32]byte) bool { return key == clientKey }, 1)
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			type accepted struct {
				value transport.ClosedSharedCarrier
				err   error
			}
			incoming := make(chan accepted, 1)
			go func() { value, err := listener.Accept(ctx, time.Second); incoming <- accepted{value, err} }()
			var connection net.Conn
			if node {
				var value transport.Carrier
				value, err = OpenNode(ctx, transport.ClosedNodeCarrierRequest{CarrierProfile: transport.ClosedCarrierQUIC, Endpoint: endpoint, Certificate: clientCert, ExpectedPeerKey: serverKey, Deadline: now.Add(3 * time.Second)})
				if err == nil {
					connection = value.(net.Conn)
				}
			} else {
				connection, err = OpenEndpoint(ctx, transport.ClosedRoleCarrierRequest{CarrierProfile: transport.ClosedCarrierQUIC, Endpoint: endpoint, ExpectedServer: serverKey, Deadline: now.Add(3 * time.Second)})
			}
			if err != nil {
				t.Fatal(err)
			}
			defer connection.Close()
			type written struct {
				count int
				err   error
			}
			output := make(chan written, 1)
			body := make([]byte, 1<<20)
			go func() { n, err := connection.Write(body); output <- written{n, err} }()
			var peer transport.ClosedSharedCarrier
			select {
			case result := <-incoming:
				if result.err != nil {
					t.Fatal(result.err)
				}
				peer = result.value
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			defer peer.Connection.Close()
			if err := peer.Connection.SetReadDeadline(now.Add(3 * time.Second)); err != nil {
				t.Fatal(err)
			}
			if _, err := io.ReadFull(peer.Connection, make([]byte, 1)); err != nil {
				t.Fatal(err)
			}
			// One byte has physically arrived. The remaining MiB cannot fit in
			// the fixed 32-KiB stream window without further peer consumption.
			select {
			case result := <-output:
				t.Fatalf("write not retained at physical interruption boundary: %+v", result)
			default:
			}
			retiring := connection
			if scenario.remote {
				retiring = peer.Connection
			}
			first := retiring.Close()
			if second := retiring.Close(); second != first {
				t.Fatalf("retirement result replaced: %v / %v", first, second)
			}
			select {
			case result := <-output:
				if result.err == nil || result.count >= len(body) {
					t.Fatalf("started write succeeded after physical interruption: %+v", result)
				}
				if transport.IsPeerRetirementCause(result.err) != scenario.remote {
					t.Fatal("native close phase misclassified", result.err)
				}
				var native *quic.ApplicationError
				if !errors.As(result.err, &native) || native.Remote != scenario.remote {
					t.Fatal("native close cause lost", result.err)
				}
			case <-ctx.Done():
				t.Fatal("original writer did not join", ctx.Err())
			}
			connection.Close()
			if n, err := connection.Write([]byte{1}); n != 0 || !errors.Is(err, net.ErrClosed) {
				t.Fatal("new write after local retirement", n, err)
			}
		})
	}
}
