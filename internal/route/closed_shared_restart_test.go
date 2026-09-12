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

func TestClosedSharedQUICCloseReleasesPortWithAcceptedCarrier(t *testing.T) {
	certificate := entryBindingCertificate(t, 181)
	key := identifierFromKey(certificate.Leaf.PublicKey.(ed25519.PublicKey))
	listener, err := ListenClosedSharedCarrier(ClosedCarrierQUIC, closedRoleCarrierTestEndpoint(t, ClosedCarrierQUIC), certificate, func([32]byte) bool { return false }, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	endpoint := closedSharedCarrierEndpoint(t, listener)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	ready := make(chan error, 1)
	go func() {
		carrier, err := listener.Accept(ctx, time.Second)
		if err == nil {
			var first [1]byte
			_, err = io.ReadFull(carrier.Connection, first[:])
			err = errors.Join(err, carrier.Connection.Close())
		}
		ready <- err
	}()
	client, err := OpenClosedRoleCarrier(ctx, ClosedRoleCarrierRequest{CarrierProfile: ClosedCarrierQUIC, Endpoint: endpoint, ExpectedServer: key, Deadline: time.Now().Add(5 * time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if _, err := client.Write([]byte{1}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-ready:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	rebound, err := net.ListenPacket("udp", endpoint)
	if err != nil {
		t.Fatalf("closed shared listener retained its UDP socket: %v", err)
	}
	if err := rebound.Close(); err != nil {
		t.Fatal(err)
	}
}
