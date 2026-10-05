package prefix_test

import (
	"context"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	"io"
	"net"
	"testing"
	"time"
)

func TestPhysicalCloseInterruptsAndJoinsReader(t *testing.T) {
	for _, profile := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		t.Run(string(profile), func(t *testing.T) {
			now := time.Now()
			serverCert, serverKey := transportCertificate(t, now.Add(-time.Hour), now.Add(time.Hour))
			clientCert, clientKey := transportCertificate(t, now.Add(-time.Hour), now.Add(time.Hour))
			endpoint := transportEndpoint(t, profile)
			listener, err := listenTestSharedCarrier(profile, endpoint, serverCert, func(key [32]byte) bool { return key == clientKey }, 1)
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			accepted := make(chan transport.ClosedSharedCarrier, 1)
			acceptErr := make(chan error, 1)
			go func() {
				value, err := listener.Accept(ctx, time.Second)
				if err != nil {
					acceptErr <- err
					return
				}
				accepted <- value
			}()
			physical, err := openTestNodeCarrier(ctx, transport.ClosedNodeCarrierRequest{CarrierProfile: profile, Endpoint: endpoint, Certificate: clientCert, ExpectedPeerKey: serverKey, Deadline: now.Add(time.Second)})
			if err != nil {
				t.Fatal(err)
			}
			defer physical.Close()
			connection, ok := physical.(net.Conn)
			if !ok {
				t.Fatal("physical Node Carrier lacks net.Conn deadlines")
			}
			if _, err := connection.Write([]byte{1}); err != nil {
				t.Fatal(err)
			}
			var peer transport.ClosedSharedCarrier
			select {
			case peer = <-accepted:
			case err := <-acceptErr:
				t.Fatal(err)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			defer peer.Connection.Close()
			if _, err := io.ReadFull(peer.Connection, make([]byte, 1)); err != nil {
				t.Fatal(err)
			}
			started := make(chan struct{})
			joined := make(chan error, 1)
			go func() { close(started); _, err := connection.Read(make([]byte, 1)); joined <- err }()
			<-started
			first := connection.Close()
			if second := connection.Close(); second != first {
				t.Fatalf("close result replaced: %v / %v", first, second)
			}
			select {
			case err := <-joined:
				if err == nil {
					t.Fatal("retired physical reader succeeded")
				}
			case <-ctx.Done():
				t.Fatal("physical reader did not join")
			}
		})
	}
}
