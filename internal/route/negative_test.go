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
	ready := make(chan route.Evidence, 1)
	done := make(chan struct {
		value route.Evidence
		err   error
	}, 1)
	go func() {
		value, err := route.Run(ctx, route.Actor{Role: "initiator", ManifestDigest: [32]byte{99}, NetworkID: [32]byte{1}, EpochDigest: [32]byte{2},
			NodeID: [32]byte{3}, ListenAddress: address, Certificate: identities[0].certificate,
			UpstreamPin: identities[1].public, NextNodeID: [32]byte{4}, NextAddress: unusedAddress(t),
			NextPin: identities[1].public, Deadline: time.Second}, func(value route.Evidence) { ready <- value })
		done <- struct {
			value route.Evidence
			err   error
		}{value, err}
	}()
	readyValue := <-ready
	if readyValue.DeadlineMillis != 1_000 || readyValue.LifetimeMillis != 1_000 {
		t.Fatalf("defaulted ready timing=%d/%d", readyValue.DeadlineMillis, readyValue.LifetimeMillis)
	}
	connection, err := tls.Dial("tcp", address, &tls.Config{MinVersion: tls.VersionTLS13,
		MaxVersion: tls.VersionTLS13, InsecureSkipVerify: true, Certificates: []tls.Certificate{identities[2].certificate}})
	if err == nil {
		connection.Close()
	}
	select {
	case outcome := <-done:
		if outcome.err == nil || !strings.Contains(outcome.err.Error(), "upstream") {
			t.Fatalf("wrong identity did not fail at the upstream boundary: %v", outcome.err)
		}
		if outcome.value.DeadlineMillis != readyValue.DeadlineMillis ||
			outcome.value.LifetimeMillis != readyValue.LifetimeMillis {
			t.Fatalf("ready/complete timing mismatch: ready=%+v complete=%+v", readyValue, outcome.value)
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
				_, err := route.Run(ctx, route.Actor{Role: "publisher", ManifestDigest: [32]byte{99}, NetworkID: [32]byte{1}, EpochDigest: [32]byte{2},
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
		_, err := route.Run(ctx, route.Actor{Role: "initiator", ManifestDigest: [32]byte{99}, NetworkID: [32]byte{1}, EpochDigest: [32]byte{2},
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

func TestCancellationDuringActiveRelayReleasesConnectionsAndGoroutines(t *testing.T) {
	node, upstream, downstream := routeIdentity(t, 61), routeIdentity(t, 62), routeIdentity(t, 63)
	nodeAddress := unusedAddress(t)
	nextListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	downstreamDone := make(chan struct{})
	go func() {
		defer close(downstreamDone)
		raw, acceptErr := nextListener.Accept()
		_ = nextListener.Close()
		if acceptErr != nil {
			return
		}
		defer raw.Close()
		secured := tls.Server(raw, &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
			Certificates: []tls.Certificate{downstream.certificate}, ClientAuth: tls.RequireAnyClientCert})
		if secured.Handshake() != nil {
			return
		}
		frame := make([]byte, 101)
		if _, readErr := io.ReadFull(secured, frame); readErr != nil {
			return
		}
		_, _ = secured.Write([]byte("ARLA"))
		_, _ = io.Copy(io.Discard, secured)
	}()
	ctx, cancel := context.WithCancel(context.Background())
	ready := make(chan route.Evidence, 1)
	done := make(chan route.Evidence, 1)
	go func() {
		result, runErr := route.Run(ctx, route.Actor{Role: "initiator", ManifestDigest: [32]byte{99},
			NetworkID: [32]byte{1}, EpochDigest: [32]byte{2}, NodeID: [32]byte{3}, ListenAddress: nodeAddress,
			Certificate: node.certificate, UpstreamPin: upstream.public, NextNodeID: [32]byte{4},
			NextAddress: nextListener.Addr().String(), NextPin: downstream.public, Deadline: 3 * time.Second},
			func(value route.Evidence) { ready <- value })
		if runErr != nil {
			result.Error = runErr.Error()
		}
		done <- result
	}()
	<-ready
	connection, err := tls.Dial("tcp", nodeAddress, &tls.Config{MinVersion: tls.VersionTLS13,
		MaxVersion: tls.VersionTLS13, InsecureSkipVerify: true, Certificates: []tls.Certificate{upstream.certificate}})
	if err != nil {
		t.Fatal(err)
	}
	if err := testLegBinding(connection, [32]byte{1}, [32]byte{2}, [32]byte{3}); err != nil {
		t.Fatal(err)
	}
	_, _ = connection.Write(make([]byte, 1024))
	time.Sleep(25 * time.Millisecond)
	cancel()
	select {
	case result := <-done:
		if !result.Cancelled || !result.Cleanup {
			t.Fatalf("active cancellation evidence incomplete: %+v", result)
		}
	case <-time.After(time.Second):
		t.Fatal("active relay did not terminate after cancellation")
	}
	connection.Close()
	select {
	case <-downstreamDone:
	case <-time.After(time.Second):
		t.Fatal("downstream relay goroutine remained after cancellation")
	}
	rebound, err := net.Listen("tcp", nodeAddress)
	if err != nil {
		t.Fatalf("active cancellation retained listener: %v", err)
	}
	rebound.Close()
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
