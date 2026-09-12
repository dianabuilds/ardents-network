package route

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestClosedSharedCarrierBoundsSilentTCP(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		name := "timeout"
		if canceled {
			name = "cancellation"
		}
		t.Run(name, func(t *testing.T) {
			certificate := entryBindingCertificate(t, 192)
			listener, err := ListenClosedSharedCarrier(ClosedCarrierTCP, closedRoleCarrierTestEndpoint(t, ClosedCarrierTCP), certificate, func([32]byte) bool { return false }, 1)
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			finished := make(chan error, 1)
			go func() {
				carrier, err := listener.Accept(ctx, 200*time.Millisecond)
				if carrier.Connection != nil {
					_ = carrier.Connection.Close()
				}
				finished <- err
			}()
			raw, err := net.DialTimeout("tcp", closedSharedCarrierEndpoint(t, listener), time.Second)
			if err != nil {
				t.Fatal(err)
			}
			defer raw.Close()
			if canceled {
				cancel()
			}
			// The real peer deliberately sends no TLS bytes. Neither the
			// finite handshake reservation nor cancellation may wait for it.
			select {
			case err := <-finished:
				if canceled {
					if !errors.Is(err, context.Canceled) {
						t.Fatalf("canceled handshake = %v", err)
					}
				} else {
					var timeout net.Error
					if !errors.As(err, &timeout) || !timeout.Timeout() {
						t.Fatalf("silent handshake = %v", err)
					}
				}
			case <-time.After(2 * time.Second):
				t.Fatal("silent peer retained handshake work")
			}
			if len(listener.(*closedSharedTCPListener).handshakes) != 0 {
				t.Fatal("failed handshake retained its reservation")
			}
		})
	}
}
