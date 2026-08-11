package nativecircuit

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestRendezvousPairsOnceWithoutReceivingIntroductionKnowledge(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	manager := newRendezvousManager()
	token := randomHandle(t)
	userHandle := randomHandle(t)
	serviceHandle := randomHandle(t)
	userClient, userServer := net.Pipe()
	serviceClient, serviceServer := net.Pipe()
	defer userClient.Close()
	defer serviceClient.Close()
	userDone := make(chan error, 1)
	serviceDone := make(chan error, 1)
	go func() { userDone <- manager.join(ctx, "user", token, userHandle, userServer) }()
	go func() { serviceDone <- manager.join(ctx, "service", token, serviceHandle, serviceServer) }()
	results := make(chan error, 2)
	for _, connection := range []net.Conn{userClient, serviceClient} {
		go func(connection net.Conn) {
			result, err := readFrame(connection)
			if err == nil && (result.Type != frameRendezvousResult || string(result.Payload) != "joined") {
				err = context.Canceled
			}
			results <- err
		}(connection)
	}
	for range 2 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}

	message := []byte("opaque end-to-end TLS bytes")
	writeDone := make(chan error, 1)
	go func() { writeDone <- writeAll(userClient, message) }()
	got := make([]byte, len(message))
	if _, err := serviceClient.Read(got); err != nil {
		t.Fatal(err)
	}
	if string(got) != string(message) {
		t.Fatalf("joined stream changed bytes: got %q want %q", got, message)
	}
	if err := <-writeDone; err != nil {
		t.Fatal(err)
	}
	_ = userClient.Close()
	_ = serviceClient.Close()
	if err := <-userDone; err != nil {
		t.Fatal(err)
	}
	if err := <-serviceDone; err != nil {
		t.Fatal(err)
	}

	replayClient, replayServer := net.Pipe()
	defer replayClient.Close()
	if err := manager.join(ctx, "user", token, randomHandle(t), replayServer); err == nil {
		t.Fatal("consumed Rendezvous join token was accepted again")
	}
}

func TestRendezvousFailureClosesJoinedStreamWithinDeadline(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	manager := newRendezvousManager()
	token := randomHandle(t)
	userClient, userServer := net.Pipe()
	serviceClient, serviceServer := net.Pipe()
	defer userClient.Close()
	defer serviceClient.Close()
	done := make(chan error, 2)
	go func() { done <- manager.join(ctx, "user", token, randomHandle(t), userServer) }()
	go func() { done <- manager.join(ctx, "service", token, randomHandle(t), serviceServer) }()
	for _, connection := range []net.Conn{userClient, serviceClient} {
		if joined, err := readFrame(connection); err != nil || joined.Type != frameRendezvousResult {
			t.Fatalf("Rendezvous did not join before failure: frame=%#v err=%v", joined, err)
		}
	}
	started := time.Now()
	cancel()
	_ = userClient.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := userClient.Read(make([]byte, 1)); err == nil {
		t.Fatal("joined stream remained open after Rendezvous failure")
	}
	if time.Since(started) > time.Second {
		t.Fatal("Rendezvous failure did not propagate within the bounded deadline")
	}
	for range 2 {
		<-done
	}
}
