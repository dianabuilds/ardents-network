package route

import (
	"context"
	"crypto/ed25519"
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
					carrier, acceptErr := listener.Accept(context.Background(), deadline)
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

func TestClosedSharedCarrierRejectsUnknownNodeBeforeARPDPayload(t *testing.T) {
	for _, profile := range []CarrierProfile{ClosedCarrierTCP, ClosedCarrierQUIC} {
		t.Run(string(profile), func(t *testing.T) {
			serverCertificate := entryBindingCertificate(t, 183)
			clientCertificate := entryBindingCertificate(t, 184)
			serverKey := identifierFromKey(serverCertificate.Leaf.PublicKey.(ed25519.PublicKey))
			listener, err := ListenClosedSharedCarrier(profile, closedRoleCarrierTestEndpoint(t, profile), serverCertificate, func([32]byte) bool { return false }, 16)
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			deadline := time.Now().Add(10 * time.Second)
			accepted := make(chan error, 1)
			go func() {
				_, acceptErr := listener.Accept(context.Background(), deadline)
				accepted <- acceptErr
			}()
			carrier, openErr := OpenClosedNodeCarrier(t.Context(), ClosedNodeCarrierRequest{CarrierProfile: profile, Endpoint: closedSharedCarrierEndpoint(t, listener), Certificate: clientCertificate, ExpectedPeerKey: serverKey, Deadline: deadline})
			if carrier != nil {
				_, _ = carrier.Write([]byte{1})
				_ = carrier.Close()
			}
			if acceptErr := <-accepted; acceptErr == nil {
				t.Fatal("unknown Node certificate reached ARDP state")
			}
			if openErr == nil && carrier == nil {
				t.Fatal("Node carrier vanished without an authenticated refusal")
			}
		})
	}
}

func closedSharedCarrierEndpoint(t *testing.T, listener ClosedSharedCarrierListener) string {
	t.Helper()
	switch value := listener.(type) {
	case *closedSharedTCPListener:
		return value.listener.Addr().String()
	case *closedSharedQUICListener:
		return value.listener.Addr().String()
	default:
		t.Fatal("unknown closed shared listener")
		return ""
	}
}
