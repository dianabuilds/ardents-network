package source

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestServingCancellationJoinsOriginalBlockedHandshake(t *testing.T) {
	fixture := newSourceTLSFixture(t, time.Now())
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	plan, _, err := New(Config{ServeAddress: address, ServeCertificate: fixture.server,
		ServeClientRootPEM: fixture.authority.rootPEM, ServeClientKeyDigests: [][32]byte{fixture.clientPin}, VerificationClock: time.Now}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	ready, joined := make(chan error, 1), make(chan error, 1)
	active := make(chan int, 2)
	go func() {
		joined <- plan.Serve(ctx, ready, nil, func(delta int) { active <- delta },
			func(context.Context, Message) Message {
				t.Error("incomplete handshake reached resolver")
				return Message{}
			})
	}()
	if err := <-ready; err != nil {
		t.Fatal(err)
	}
	peer, err := net.Dial("tcp", address)
	if err != nil {
		cancel()
		<-joined
		t.Fatal(err)
	}
	defer peer.Close()
	if delta := <-active; delta != 1 {
		t.Fatal("original handler absent")
	}
	cancel()
	select {
	case err := <-joined:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled original handshake retained socket until deadline")
	}
	if delta := <-active; delta != -1 {
		t.Fatal("handler release not joined")
	}
}
