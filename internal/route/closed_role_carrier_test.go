//go:build linux

package route

import (
	"context"
	"crypto/ed25519"
	"io"
	"testing"
	"time"
)

func TestClosedRoleCarrierAuthenticatesDirectTCPAndQUIC(t *testing.T) {
	for _, profile := range []CarrierProfile{ClosedCarrierTCP, ClosedCarrierQUIC} {
		t.Run(string(profile), func(t *testing.T) {
			certificate := entryBindingCertificate(t, 151)
			server := identifierFromKey(certificate.Leaf.PublicKey.(ed25519.PublicKey))
			listener, err := ListenClosedRoleCarrier(profile, closedRoleCarrierTestEndpoint(t, profile), certificate)
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			endpoint := ""
			switch value := listener.(type) {
			case *closedRoleTCPListener:
				endpoint = value.listener.Addr().String()
			case *closedRoleQUICListener:
				endpoint = value.listener.Addr().String()
			default:
				t.Fatal("unknown closed role listener")
			}
			deadline := time.Now().Add(10 * time.Second)
			done := make(chan error, 1)
			release := make(chan struct{})
			go func() {
				connection, err := listener.Accept(context.Background(), deadline)
				if err == nil {
					defer connection.Close()
					value := []byte{0}
					if _, err = io.ReadFull(connection, value); err == nil && value[0] == 7 {
						_, err = connection.Write([]byte{8})
					}
					if err == nil {
						<-release
					}
				}
				done <- err
			}()
			connection, err := OpenClosedRoleCarrier(t.Context(), ClosedRoleCarrierRequest{CarrierProfile: profile, Endpoint: endpoint, ExpectedServer: server, Deadline: deadline})
			if err != nil {
				t.Fatal(err)
			}
			defer connection.Close()
			if _, err := connection.Write([]byte{7}); err != nil {
				t.Fatal(err)
			}
			response := []byte{0}
			if _, err := io.ReadFull(connection, response); err != nil || response[0] != 8 {
				t.Fatalf("role carrier response = %x / %v", response, err)
			}
			close(release)
			if err := <-done; err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestClosedRoleCarrierReportsQUICPeerMismatch(t *testing.T) {
	certificate := entryBindingCertificate(t, 152)
	listener, err := ListenClosedRoleCarrier(ClosedCarrierQUIC, closedRoleCarrierTestEndpoint(t, ClosedCarrierQUIC), certificate)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	endpoint := listener.(*closedRoleQUICListener).listener.Addr().String()
	wrong := identifierFromKey(entryBindingCertificate(t, 153).Leaf.PublicKey.(ed25519.PublicKey))
	_, err = OpenClosedRoleCarrier(t.Context(), ClosedRoleCarrierRequest{CarrierProfile: ClosedCarrierQUIC, Endpoint: endpoint, ExpectedServer: wrong, Deadline: time.Now().Add(5 * time.Second)})
	if got := closedRoleOpenFailureDetail(err); got != "quic-dial-peer-mismatch" {
		t.Fatalf("QUIC peer mismatch detail = %q / %v", got, err)
	}
}
