package route

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/quic-go/quic-go"
)

func TestClosedNodeCarrierUsesV3MutualTLSOverBothCarriers(t *testing.T) {
	for _, profile := range []CarrierProfile{ClosedCarrierTCP, ClosedCarrierQUIC} {
		t.Run(string(profile), func(t *testing.T) {
			serverCertificate := entryBindingCertificate(t, 121)
			clientCertificate := entryBindingCertificate(t, 122)
			serverID := identifierFromKey(serverCertificate.Leaf.PublicKey.(ed25519.PublicKey))
			clientID := identifierFromKey(clientCertificate.Leaf.PublicKey.(ed25519.PublicKey))
			endpoint, closeListener, accept := closedNodeCarrierTestServer(t, profile, serverCertificate, clientID)
			defer closeListener()
			deadline := time.Now().Add(10 * time.Second)
			carrier, err := OpenClosedNodeCarrier(t.Context(), ClosedNodeCarrierRequest{CarrierProfile: profile, Endpoint: endpoint,
				Certificate: clientCertificate, ExpectedPeerKey: serverID, Deadline: deadline})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := carrier.Write([]byte{0x42}); err != nil {
				t.Fatal(err)
			}
			response := make([]byte, 1)
			if _, err := io.ReadFull(carrier, response); err != nil || response[0] != 0x42 {
				t.Fatalf("v3 carrier response = %x / %v", response, err)
			}
			if err := carrier.Close(); err != nil {
				t.Fatal(err)
			}
			if err := <-accept; err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestClosedNodeCarrierRejectsOldProfileBeforeDial(t *testing.T) {
	certificate := entryBindingCertificate(t, 123)
	carrier, err := OpenClosedNodeCarrier(t.Context(), ClosedNodeCarrierRequest{CarrierProfile: CarrierTCP, Endpoint: "127.0.0.1:9",
		Certificate: certificate, ExpectedPeerKey: identifier(1), Deadline: time.Now().Add(time.Second)})
	if err == nil || carrier != nil {
		t.Fatal("generation-2 carrier reached successor transport")
	}
	config := closedNodeQUICConfig()
	if config.Allow0RTT || config.EnableDatagrams || config.InitialPacketSize != 1200 || config.MaxIncomingStreams != -1 || config.MaxIncomingUniStreams != -1 {
		t.Fatal("successor QUIC configuration enables an unselected feature")
	}
}

func closedNodeCarrierTestServer(t *testing.T, profile CarrierProfile, certificate tls.Certificate, expected [32]byte) (string, func(), <-chan error) {
	t.Helper()
	done := make(chan error, 1)
	if profile == ClosedCarrierTCP {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		go func() {
			raw, acceptErr := listener.Accept()
			if acceptErr != nil {
				done <- acceptErr
				return
			}
			secured := tls.Server(raw, closedNodeServerTLS(certificate, expected))
			if acceptErr = secured.HandshakeContext(context.Background()); acceptErr == nil {
				buffer := make([]byte, 1)
				if _, acceptErr = io.ReadFull(secured, buffer); acceptErr == nil {
					_, acceptErr = secured.Write(buffer)
				}
			}
			closeErr := secured.Close()
			done <- errors.Join(acceptErr, closeErr)
		}()
		return listener.Addr().String(), func() { _ = listener.Close() }, done
	}
	listener, err := quic.ListenAddr("127.0.0.1:0", closedNodeServerTLS(certificate, expected), closedNodeQUICServerConfig())
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		connection, acceptErr := listener.Accept(context.Background())
		if acceptErr == nil {
			stream, streamErr := connection.AcceptStream(context.Background())
			if streamErr != nil {
				acceptErr = streamErr
			} else {
				buffer := make([]byte, 1)
				if _, acceptErr = io.ReadFull(stream, buffer); acceptErr == nil {
					_, acceptErr = stream.Write(buffer)
				}
				if closeErr := stream.Close(); closeErr != nil {
					acceptErr = errors.Join(acceptErr, closeErr)
				}
			}
		}
		done <- acceptErr
	}()
	return listener.Addr().String(), func() { _ = listener.Close() }, done
}
