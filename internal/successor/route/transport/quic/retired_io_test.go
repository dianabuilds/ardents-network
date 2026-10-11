package quic

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	"github.com/quic-go/quic-go"
)

// A framing reader observes the original remote close and retires its local
// adapter while an already selected writer has yet to enter adapter Write.
// That writer must retain the real native cause, not replace it with a bare
// local closed sentinel. Genuine local retirement remains a failed local cause.
func TestRetiredQUICIOPreservesOriginalNativeCause(t *testing.T) {
	for _, node := range []bool{false, true} {
		for _, remote := range []bool{false, true} {
			name := map[bool]string{false: "role", true: "node"}[node] + map[bool]string{false: "/local", true: "/remote"}[remote]
			t.Run(name, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
				defer cancel()
				end, _ := ctx.Deadline()
				serverCert, serverKey := transportCertificate(t, end.Add(-time.Hour), end.Add(time.Hour))
				clientCert, clientKey := transportCertificate(t, end.Add(-time.Hour), end.Add(time.Hour))
				endpoint := transportEndpoint(t, transport.ClosedCarrierQUIC)
				listener, err := ListenShared(endpoint, serverCert, func(key [32]byte) bool { return key == clientKey }, 1)
				if err != nil {
					t.Fatal(err)
				}
				defer listener.Close()
				type acceptance struct {
					value transport.ClosedSharedCarrier
					err   error
				}
				incoming := make(chan acceptance, 1)
				go func() { value, err := listener.Accept(ctx, time.Second); incoming <- acceptance{value, err} }()
				var client net.Conn
				if node {
					value, openErr := OpenNode(ctx, transport.ClosedNodeCarrierRequest{CarrierProfile: transport.ClosedCarrierQUIC, Endpoint: endpoint, Certificate: clientCert, ExpectedPeerKey: serverKey, Deadline: end})
					err = openErr
					if err == nil {
						client = value.(net.Conn)
					}
				} else {
					client, err = OpenEndpoint(ctx, transport.ClosedRoleCarrierRequest{CarrierProfile: transport.ClosedCarrierQUIC, Endpoint: endpoint, ExpectedServer: serverKey, Deadline: end})
				}
				if err != nil {
					t.Fatal(err)
				}
				defer client.Close()
				if err := client.SetDeadline(end); err != nil {
					t.Fatal(err)
				}
				if _, err := client.Write([]byte{1}); err != nil {
					t.Fatal(err)
				}
				peer := <-incoming
				if peer.err != nil {
					t.Fatal(peer.err)
				}
				defer peer.value.Connection.Close()
				if err := peer.value.Connection.SetDeadline(end); err != nil {
					t.Fatal(err)
				}
				if _, err := io.ReadFull(peer.value.Connection, make([]byte, 1)); err != nil {
					t.Fatal(err)
				}
				var original *quic.ApplicationError
				if remote {
					if err := peer.value.Connection.Close(); err != nil {
						t.Fatal(err)
					}
					_, err := client.Read(make([]byte, 1))
					if !errors.As(err, &original) || !original.Remote || !transport.IsPeerRetirementCause(err) {
						t.Fatal("original remote close not observed", err)
					}
				}
				if err := client.Close(); err != nil {
					t.Fatal(err)
				}
				for _, operation := range []string{"read", "write"} {
					var n int
					var err error
					if operation == "read" {
						n, err = client.Read(make([]byte, 1))
					} else {
						n, err = client.Write([]byte{1})
					}
					var native *quic.ApplicationError
					if n != 0 || !errors.Is(err, net.ErrClosed) || !errors.As(err, &native) || native.Remote != remote || transport.IsPeerRetirementCause(err) != remote {
						t.Fatal("retired I/O replaced native cause", operation, n, err)
					}
					if remote && native != original {
						t.Fatal("original native cause identity replaced", operation)
					}
				}
			})
		}
	}
}
