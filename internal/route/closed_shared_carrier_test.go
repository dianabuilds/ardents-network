//go:build linux

package route

import (
	"context"
	"crypto/ed25519"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

func TestClosedSharedCarrierClassifiesDirectAndCurrentNode(t *testing.T) {
	for _, profile := range []CarrierProfile{ClosedCarrierTCP, ClosedCarrierQUIC} {
		t.Run(string(profile), func(t *testing.T) {
			serverCertificate := entryBindingCertificate(t, 181)
			clientCertificate := entryBindingCertificate(t, 182)
			serverKey := identifierFromKey(serverCertificate.Leaf.PublicKey.(ed25519.PublicKey))
			clientKey := identifierFromKey(clientCertificate.Leaf.PublicKey.(ed25519.PublicKey))
			listener, err := ListenClosedSharedCarrier(profile, closedRoleCarrierTestEndpoint(t, profile), serverCertificate, func(key [32]byte) bool { return key == clientKey }, 16)
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			endpoint := closedSharedCarrierEndpoint(t, listener)
			deadline := time.Now().Add(10 * time.Second)
			accepted := make(chan ClosedSharedCarrier, 2)
			done := make(chan error, 1)
			go func() {
				for range 2 {
					carrier, acceptErr := listener.Accept(context.Background(), 10*time.Second)
					if acceptErr != nil {
						done <- acceptErr
						return
					}
					accepted <- carrier
				}
				done <- nil
			}()
			direct, err := OpenClosedRoleCarrier(t.Context(), ClosedRoleCarrierRequest{CarrierProfile: profile, Endpoint: endpoint, ExpectedServer: serverKey, Deadline: deadline})
			if err != nil {
				t.Fatal(err)
			}
			node, err := OpenClosedNodeCarrier(t.Context(), ClosedNodeCarrierRequest{CarrierProfile: profile, Endpoint: endpoint, Certificate: clientCertificate, ExpectedPeerKey: serverKey, Deadline: deadline})
			if err != nil {
				_ = direct.Close()
				t.Fatal(err)
			}
			// QUIC announces a stream only with its first bytes. Classification
			// must preserve those bytes for the receiving ARDP owner.
			if _, err := direct.Write([]byte{7}); err != nil {
				t.Fatal(err)
			}
			if _, err := node.Write([]byte{8}); err != nil {
				t.Fatal(err)
			}
			first := <-accepted
			second := <-accepted
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			if first.Kind == second.Kind || (first.Kind != ClosedSharedDirect && second.Kind != ClosedSharedDirect) || (first.Kind != ClosedSharedNode && second.Kind != ClosedSharedNode) {
				t.Fatalf("shared carrier kinds = %v, %v", first.Kind, second.Kind)
			}
			for _, acceptedCarrier := range []ClosedSharedCarrier{first, second} {
				if acceptedCarrier.Kind == ClosedSharedNode && acceptedCarrier.NodeKey != clientKey {
					t.Fatal("shared Node carrier lost State-authorized key")
				}
				want := byte(7)
				if acceptedCarrier.Kind == ClosedSharedNode {
					want = 8
				}
				var firstByte [1]byte
				if _, err := io.ReadFull(acceptedCarrier.Connection, firstByte[:]); err != nil || firstByte[0] != want {
					t.Fatalf("classified carrier first byte = %v, %v", firstByte, err)
				}
				if err := acceptedCarrier.Connection.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if err := direct.Close(); err != nil {
				t.Fatal(err)
			}
			if err := node.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestClosedSharedQUICHandshakeReservationPrecedesTLS(t *testing.T) {
	slots := make(chan struct{}, 1)
	reserve := closedSharedQUICHandshakeContext(slots)
	first, cancel := context.WithCancel(t.Context())
	defer cancel()
	reserved, err := reserve(first, nil)
	if err != nil || reserved.Value(closedSharedQUICHandshakeContextKey{}) == nil {
		t.Fatalf("first pre-TLS reservation = %v / %v", reserved, err)
	}
	reservation := reserved.Value(closedSharedQUICHandshakeContextKey{}).(*closedSharedQUICHandshakeReservation)
	if _, err := reserve(t.Context(), nil); err == nil {
		t.Fatal("second pre-TLS handshake bypassed finite reservation")
	}
	cancel()
	select {
	case <-reservation.released:
	case <-time.After(time.Second):
		t.Fatal("closed pre-TLS reservation was not released after connection cancellation")
	}
	if _, err := reserve(t.Context(), nil); err != nil {
		t.Fatalf("released pre-TLS reservation remained unavailable: %v", err)
	}
}

func TestClosedSharedCarrierRejectsUnknownNodeBeforeARPDPayload(t *testing.T) {
	for _, profile := range []CarrierProfile{ClosedCarrierTCP, ClosedCarrierQUIC} {
		t.Run(string(profile), func(t *testing.T) {
			serverCertificate := entryBindingCertificate(t, 183)
			clientCertificate := entryBindingCertificate(t, 184)
			serverKey := identifierFromKey(serverCertificate.Leaf.PublicKey.(ed25519.PublicKey))
			clientKey := identifierFromKey(clientCertificate.Leaf.PublicKey.(ed25519.PublicKey))
			var rejectedKey [32]byte
			listener, err := ListenClosedSharedCarrier(profile, closedRoleCarrierTestEndpoint(t, profile), serverCertificate, func(key [32]byte) bool {
				rejectedKey = key
				return false
			}, 16)
			if err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(10 * time.Second)
			attempt, cancel := context.WithDeadline(t.Context(), deadline)
			accepted := make(chan error, 1)
			finished := make(chan struct{})
			go func() {
				defer close(finished)
				result, acceptErr := listener.Accept(attempt, 10*time.Second)
				if result.Connection != nil {
					_ = result.Connection.Close()
				}
				accepted <- acceptErr
			}()
			defer func() {
				cancel()
				_ = listener.Close()
				<-finished
			}()
			carrier, openErr := OpenClosedNodeCarrier(attempt, ClosedNodeCarrierRequest{CarrierProfile: profile, Endpoint: closedSharedCarrierEndpoint(t, listener), Certificate: clientCertificate, ExpectedPeerKey: serverKey, Deadline: deadline})
			if carrier != nil {
				// Keep the peer alive until the server classifies its certificate.
				// An early QUIC close can discard the connection before Accept.
				defer carrier.Close()
				_, _ = carrier.Write([]byte{1})
			}
			select {
			case acceptErr := <-accepted:
				var timeout net.Error
				if attempt.Err() != nil || errors.Is(acceptErr, context.Canceled) ||
					errors.Is(acceptErr, context.DeadlineExceeded) ||
					(errors.As(acceptErr, &timeout) && timeout.Timeout()) {
					t.Fatalf("deadline or cancellation is not a State-key refusal: %v", acceptErr)
				}
				if acceptErr == nil {
					t.Fatal("unknown Node certificate reached ARDP state")
				}
				// The receive publishes the verifier's write. A handshake failure
				// or a deadline alone is not evidence of State-key rejection.
				if rejectedKey != clientKey {
					t.Fatalf("unknown Node did not reach State-key rejection: %v", acceptErr)
				}
			case <-attempt.Done():
				t.Fatal("unknown Node refusal did not complete before the deadline")
			}
			if openErr == nil && carrier == nil {
				t.Fatal("Node carrier vanished without an authenticated refusal")
			}
		})
	}
}
