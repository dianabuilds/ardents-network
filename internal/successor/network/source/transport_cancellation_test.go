package source

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"sync"
	"testing"
	"time"
)

func TestFetchCancelsResponseReadAfterTLSHandshake(t *testing.T) {
	now := time.Date(2032, time.February, 3, 4, 5, 6, 0, time.UTC)
	fixture := newSourceTLSFixture(t, now)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	requestRead := make(chan struct{})
	releaseResponse := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(releaseResponse) }) }
	defer unblock()
	serverDone := make(chan error, 1)
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverDone <- acceptErr
			return
		}
		defer connection.Close()
		secured := tls.Server(connection, serverTLSConfig(server{
			certificate: fixture.server, clientRoots: fixture.authority.pool,
			clientDigests: map[[32]byte]bool{fixture.clientPin: true},
			clock:         func() time.Time { return now },
		}))
		if deadlineErr := secured.SetDeadline(time.Now().Add(3 * time.Second)); deadlineErr != nil {
			serverDone <- deadlineErr
			return
		}
		if handshakeErr := secured.Handshake(); handshakeErr != nil {
			serverDone <- handshakeErr
			return
		}
		if _, requestErr := readRequest(secured); requestErr != nil {
			serverDone <- requestErr
			return
		}
		close(requestRead)
		<-releaseResponse
		serverDone <- writeResponse(secured, Message{Status: "not-found"})
	}()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	type result struct {
		response Message
		err      error
	}
	received := make(chan result, 1)
	go func() {
		response, fetchErr := fetch(ctx, client{
			address: listener.Addr().String(), roots: fixture.authority.pool,
			serverName: fixture.serverName, leafKeyDigest: fixture.serverPin,
			certificate: fixture.client, clock: func() time.Time { return now },
		}, Message{Operation: "latest"})
		received <- result{response: response, err: fetchErr}
	}()
	select {
	case <-requestRead:
	case serverErr := <-serverDone:
		t.Fatalf("server did not receive authenticated request: %v", serverErr)
	case <-time.After(2 * time.Second):
		t.Fatal("server did not receive authenticated request")
	}
	cancel()
	select {
	case got := <-received:
		if !errors.Is(got.err, context.Canceled) || (got.response.Status != "" || got.response.ObjectDigest != [32]byte{} || len(got.response.Payload) != 0) {
			t.Fatalf("canceled fetch = (%+v, %v), want empty response and context cancellation", got.response, got.err)
		}
	case <-time.After(time.Second):
		unblock()
		<-received
		t.Fatal("canceled fetch waited for the Source response")
	}
	unblock()
	<-serverDone
}

func TestCanceledFetchFailureDiscardsPartialObjectDigest(t *testing.T) {
	partial := Message{Status: "ok", ObjectDigest: [32]byte{1}}
	framing := errors.New("partial distribution response")
	liveResponse, liveErr := fetchFailure(t.Context(), partial, framing)
	if liveResponse.ObjectDigest != partial.ObjectDigest || !errors.Is(liveErr, framing) {
		t.Fatalf("live partial response = (%+v, %v), want retained digest and framing failure", liveResponse, liveErr)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	canceledResponse, canceledErr := fetchFailure(ctx, partial, framing)
	if canceledResponse.ObjectDigest != [32]byte{} || canceledResponse.Status != "" ||
		!errors.Is(canceledErr, context.Canceled) {
		t.Fatalf("canceled partial response = (%+v, %v), want no fallback selector", canceledResponse, canceledErr)
	}
}
