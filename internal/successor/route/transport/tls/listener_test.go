package tls_test

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	roletls "github.com/dianabuilds/ardents-network/internal/successor/route/transport/tls"
)

func tcpEndpoint(t *testing.T) string {
	t.Helper()
	reservation, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	endpoint := reservation.Addr().String()
	if err := reservation.Close(); err != nil {
		t.Fatal(err)
	}
	// The ordinary bind may fail if another process claims this address; there
	// is no retry, mock listener or passing skip for that external interference.
	return endpoint
}

func TestSharedTCPListenerClassifiesDirectAndNodeBeforeBytes(t *testing.T) {
	for _, node := range []bool{false, true} {
		name := "direct"
		if node {
			name = "node"
		}
		t.Run(name, func(t *testing.T) {
			now := time.Now()
			serverCertificate, serverKey := roleCertificate(t, now.Add(-time.Minute), now.Add(time.Minute))
			clientCertificate, clientKey := roleCertificate(t, now.Add(-time.Minute), now.Add(time.Minute))
			endpoint := tcpEndpoint(t)
			listener, err := roletls.ListenShared(endpoint, serverCertificate, func(key [32]byte) bool { return key == clientKey }, 1)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			end, _ := ctx.Deadline()
			joined := make(chan struct{})
			result := make(chan error, 1)
			go func() {
				defer close(joined)
				accepted, err := listener.Accept(ctx, time.Second)
				if err != nil {
					result <- err
					return
				}
				defer accepted.Connection.Close()
				if err := accepted.Connection.SetDeadline(end); err != nil {
					result <- err
					return
				}
				want := transport.ClosedSharedDirect
				if node {
					want = transport.ClosedSharedNode
				}
				if accepted.Kind != want || node && accepted.NodeKey != clientKey || !node && accepted.NodeKey != [32]byte{} {
					result <- errors.New("classified transport principal differs")
					return
				}
				value := make([]byte, 3)
				if _, err := io.ReadFull(accepted.Connection, value); err != nil {
					result <- err
					return
				}
				if string(value) != "one" {
					result <- errors.New("ordered input differs")
					return
				}
				if _, err := accepted.Connection.Write([]byte("two")); err != nil {
					result <- err
					return
				}
				_, err = io.ReadFull(accepted.Connection, value)
				if err == nil && string(value) != "ack" {
					err = errors.New("consumption acknowledgement differs")
				}
				result <- err
			}()
			t.Cleanup(func() { _ = listener.Close(); <-joined })
			var client transport.Carrier
			if node {
				client, err = roletls.OpenNode(ctx, transport.ClosedNodeCarrierRequest{CarrierProfile: transport.ClosedCarrierTCP,
					Endpoint: endpoint, Certificate: clientCertificate, ExpectedPeerKey: serverKey, Deadline: end})
			} else {
				client, err = roletls.OpenEndpoint(ctx, transport.ClosedRoleCarrierRequest{
					CarrierProfile: transport.ClosedCarrierTCP, Endpoint: endpoint, ExpectedServer: serverKey, Deadline: end,
				})
			}
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = client.Close() })
			if err := client.SetDeadline(end); err != nil {
				t.Fatal(err)
			}
			if _, err := client.Write([]byte("one")); err != nil {
				t.Fatal(err)
			}
			value := make([]byte, 3)
			if _, err := io.ReadFull(client, value); err != nil || string(value) != "two" {
				t.Fatal("actual response bytes differ", err)
			}
			if _, err := client.Write([]byte("ack")); err != nil {
				t.Fatal(err)
			}
			if err := <-result; err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestTCPAdapterRefusesOtherProfileBeforeDial(t *testing.T) {
	now := time.Now()
	certificate, key := roleCertificate(t, now.Add(-time.Minute), now.Add(time.Minute))
	connection, err := roletls.OpenNode(t.Context(), transport.ClosedNodeCarrierRequest{
		CarrierProfile: transport.ClosedCarrierQUIC, Endpoint: "127.0.0.1:1", Certificate: certificate, ExpectedPeerKey: key, Deadline: now.Add(time.Minute),
	})
	if connection != nil || err == nil || err.Error() != "closed Node Carrier profile is unsupported" {
		t.Fatal("wrong selected profile reached a physical dial", err)
	}
}
