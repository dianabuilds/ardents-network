//go:build linux

package route

import (
	"context"
	"crypto/ed25519"
	"errors"
	"testing"
	"time"
)

func TestClosedQUICAcceptanceBoundsSilentAuthenticatedPeer(t *testing.T) {
	for _, kind := range []string{"role", "shared"} {
		t.Run(kind, func(t *testing.T) {
			certificate := entryBindingCertificate(t, 187)
			server := identifierFromKey(certificate.Leaf.PublicKey.(ed25519.PublicKey))
			address := closedRoleCarrierTestEndpoint(t, ClosedCarrierQUIC)
			deadline := time.Now().Add(time.Second)
			completed := make(chan error, 1)
			if kind == "role" {
				listener, err := ListenClosedRoleCarrier(ClosedCarrierQUIC, address, certificate)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = listener.Close() })
				go func() {
					connection, err := listener.Accept(context.Background(), deadline)
					if connection != nil {
						_ = connection.Close()
					}
					completed <- err
				}()
			} else {
				listener, err := ListenClosedSharedCarrier(ClosedCarrierQUIC, address, certificate, func([32]byte) bool { return false }, 1)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = listener.Close() })
				go func() {
					accepted, err := listener.Accept(context.Background(), time.Second)
					if accepted.Connection != nil {
						_ = accepted.Connection.Close()
					}
					completed <- err
				}()
			}
			connection, err := OpenClosedRoleCarrier(t.Context(), ClosedRoleCarrierRequest{
				CarrierProfile: ClosedCarrierQUIC, Endpoint: address, ExpectedServer: server, Deadline: deadline,
			})
			if err != nil {
				t.Fatal(err)
			}
			defer connection.Close()
			// Complete TLS, then send no stream bytes. The accepted connection
			// must release its handshake reservation at the supplied bound.
			select {
			case err := <-completed:
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("silent peer outcome = %v", err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("silent authenticated peer retained unbounded acceptance work")
			}
		})
	}
}
