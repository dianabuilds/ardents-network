package route

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"io"
	"testing"
	"time"
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
	listener, err := ListenClosedSharedCarrier(profile, closedRoleCarrierTestEndpoint(t, profile), certificate,
		func(key [32]byte) bool { return key == expected }, 16)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	done, finished, release := make(chan error, 1), make(chan struct{}), make(chan struct{})
	go func() {
		defer close(finished)
		accepted, err := listener.Accept(ctx, 10*time.Second)
		if err != nil {
			done <- err
			return
		}
		connection := accepted.Connection
		interrupted := make(chan struct{})
		stop := context.AfterFunc(ctx, func() { defer close(interrupted); _ = connection.SetDeadline(time.Now()) })
		defer func() {
			// Keep the actual Carrier alive until the client consumed the echo;
			// a QUIC connection close may discard an unacknowledged stream tail.
			<-release
			if !stop() {
				<-interrupted
			}
			if err := connection.Close(); err != nil {
				t.Error(err)
			}
		}()
		if accepted.Kind != ClosedSharedNode || accepted.NodeKey != expected {
			done <- errors.New("accepted Carrier lost Node authentication")
			return
		}
		if end, ok := ctx.Deadline(); ok {
			if err := connection.SetDeadline(end); err != nil {
				done <- err
				return
			}
		}
		var body [1]byte
		_, err = io.ReadFull(connection, body[:])
		if err == nil {
			_, err = connection.Write(body[:])
		}
		done <- err
	}()
	return closedSharedCarrierEndpoint(t, listener), func() {
		cancel()
		close(release)
		if err := listener.Close(); err != nil {
			t.Error(err)
		}
		<-finished
	}, done
}
