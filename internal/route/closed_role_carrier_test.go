package route

import (
	"context"
	"crypto/ed25519"
	"io"
	"net"
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

func closedRoleCarrierTestEndpoint(t *testing.T, profile CarrierProfile) string {
	t.Helper()
	switch profile {
	case ClosedCarrierTCP:
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		endpoint := listener.Addr().String()
		if err := listener.Close(); err != nil {
			t.Fatal(err)
		}
		return endpoint
	case ClosedCarrierQUIC:
		listener, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
		if err != nil {
			t.Fatal(err)
		}
		endpoint := listener.LocalAddr().String()
		if err := listener.Close(); err != nil {
			t.Fatal(err)
		}
		return endpoint
	default:
		t.Fatal("unknown role carrier profile")
		return ""
	}
}
