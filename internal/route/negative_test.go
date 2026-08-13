package route_test

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestRouteNodeRejectsWrongUpstreamIdentity(t *testing.T) {
	identities := []testRouteIdentity{routeIdentity(t, 21), routeIdentity(t, 22), routeIdentity(t, 23)}
	address := unusedAddress(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ready, done := make(chan route.Evidence, 1), make(chan error, 1)
	go func() {
		_, err := route.Run(ctx, route.Actor{Role: "initiator", NetworkID: [32]byte{1}, EpochDigest: [32]byte{2},
			NodeID: [32]byte{3}, ListenAddress: address, Certificate: identities[0].certificate,
			UpstreamPin: identities[1].public, NextNodeID: [32]byte{4}, NextAddress: unusedAddress(t),
			NextPin: identities[1].public, Deadline: time.Second}, func(value route.Evidence) { ready <- value })
		done <- err
	}()
	<-ready
	connection, err := tls.Dial("tcp", address, &tls.Config{MinVersion: tls.VersionTLS13,
		MaxVersion: tls.VersionTLS13, InsecureSkipVerify: true, Certificates: []tls.Certificate{identities[2].certificate}})
	if err == nil {
		connection.Close()
	}
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "upstream") {
			t.Fatalf("wrong identity did not fail at the upstream boundary: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("wrong-identity Node did not terminate within its bound")
	}
}

func TestPublisherRejectsMalformedPartialOversizedAndSlowFrames(t *testing.T) {
	tests := []struct {
		name  string
		write func(*testing.T, net.Conn)
	}{
		{"malformed", func(t *testing.T, connection net.Conn) { _, _ = connection.Write([]byte("BAD-FRAME")) }},
		{"partial", func(t *testing.T, connection net.Conn) { _, _ = connection.Write([]byte("ARCN\x01")) }},
		{"oversized", func(t *testing.T, connection net.Conn) {
			header := append([]byte("ARCN\x01"), make([]byte, 4)...)
			binary.BigEndian.PutUint32(header[5:], 64<<10)
			_, _ = connection.Write(header)
		}},
		{"slow", func(t *testing.T, connection net.Conn) { time.Sleep(350 * time.Millisecond) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			publisher, responder := routeIdentity(t, 31), routeIdentity(t, 32)
			address := unusedAddress(t)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			ready, done := make(chan route.Evidence, 1), make(chan error, 1)
			go func() {
				_, err := route.Run(ctx, route.Actor{Role: "publisher", NetworkID: [32]byte{1}, EpochDigest: [32]byte{2},
					NodeID: [32]byte{3}, ListenAddress: address, Certificate: publisher.certificate,
					UpstreamPin: responder.public, ServiceCertificate: publisher.certificate,
					Deadline: 500 * time.Millisecond}, func(value route.Evidence) { ready <- value })
				done <- err
			}()
			<-ready
			outer, err := tls.Dial("tcp", address, &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
				InsecureSkipVerify: true, Certificates: []tls.Certificate{responder.certificate}})
			if err != nil {
				t.Fatal(err)
			}
			if err := testLegBinding(outer, [32]byte{1}, [32]byte{2}, [32]byte{3}); err != nil {
				t.Fatal(err)
			}
			inner := tls.Client(outer, &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, InsecureSkipVerify: true})
			if err := inner.HandshakeContext(ctx); err != nil {
				t.Fatal(err)
			}
			test.write(t, inner)
			inner.Close()
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("invalid canary frame was accepted")
				}
			case <-ctx.Done():
				t.Fatal("invalid canary frame did not terminate within its bound")
			}
		})
	}
}

func TestCancelledRouteListenerReleasesItsAddress(t *testing.T) {
	identity, upstream := routeIdentity(t, 41), routeIdentity(t, 42)
	address := unusedAddress(t)
	ctx, cancel := context.WithCancel(context.Background())
	ready, done := make(chan route.Evidence, 1), make(chan error, 1)
	go func() {
		_, err := route.Run(ctx, route.Actor{Role: "initiator", NetworkID: [32]byte{1}, EpochDigest: [32]byte{2},
			NodeID: [32]byte{3}, ListenAddress: address, Certificate: identity.certificate,
			UpstreamPin: upstream.public, NextNodeID: [32]byte{4}, NextAddress: unusedAddress(t),
			NextPin: upstream.public, Deadline: time.Second}, func(value route.Evidence) { ready <- value })
		done <- err
	}()
	<-ready
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled listener returned %v", err)
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatalf("cancelled listener retained its address: %v", err)
	}
	listener.Close()
}

func testLegBinding(connection io.ReadWriter, network, epoch, destination [32]byte) error {
	frame := append([]byte("ARLG\x01"), network[:]...)
	frame = append(frame, epoch[:]...)
	frame = append(frame, destination[:]...)
	if _, err := connection.Write(frame); err != nil {
		return err
	}
	ack := make([]byte, 4)
	_, err := io.ReadFull(connection, ack)
	if err == nil && string(ack) != "ARLA" {
		return errors.New("leg binding acknowledgement is invalid")
	}
	return err
}
