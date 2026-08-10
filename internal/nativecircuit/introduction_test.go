package nativecircuit

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestIntroductionForwardsOpaqueInvitationOnlyToRegisteredSlot(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	manager := newIntroductionManager()
	slot := randomHandle(t)
	sealed := []byte("sealed-hpke-invitation")
	serviceClient, serviceServer := net.Pipe()
	defer serviceClient.Close()
	serviceDone := make(chan error, 1)
	go func() { serviceDone <- manager.register(ctx, slot, serviceServer) }()
	registered, err := readFrame(serviceClient)
	if err != nil || registered.Type != frameIntroductionAcknowledge || string(registered.Payload) != "registered" {
		t.Fatalf("Introduction slot was not acknowledged: frame=%#v err=%v", registered, err)
	}

	delivered := make(chan frame, 1)
	readDone := make(chan error, 1)
	go func() {
		value, err := readFrame(serviceClient)
		if err == nil {
			delivered <- value
			err = writeFrame(serviceClient, frame{Type: frameIntroductionAcknowledge, Payload: []byte("accepted")})
		}
		readDone <- err
	}()
	if err := manager.deliver(ctx, slot, sealed); err != nil {
		t.Fatal(err)
	}
	got := <-delivered
	if err := <-readDone; err != nil {
		t.Fatal(err)
	}
	if got.Type != frameIntroductionDeliver || string(got.Payload) != string(sealed) {
		t.Fatalf("Introduction changed sealed bytes: got %#v want %q", got, sealed)
	}
	_ = serviceClient.Close()
	if err := <-serviceDone; err != nil {
		t.Fatal(err)
	}
	if err := manager.deliver(ctx, slot, sealed); err == nil {
		t.Fatal("consumed Introduction slot accepted another invitation")
	}
}

func TestIntroductionRejectsInvalidStateTransition(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	node := newTestNode(t, "introduction-node")
	done := make(chan error, 1)
	go func() { done <- serveIntroduction(ctx, node.listener, node.certificate, 1) }()
	connection, err := dialTelescopedCircuit(ctx, []circuitHop{{Address: node.address, CertificateSHA256: node.digest}})
	if err != nil {
		t.Fatal(err)
	}
	if err := writeFrame(connection, frame{Type: frameClose}); err != nil {
		t.Fatal(err)
	}
	_ = connection.Close()
	if err := <-done; err == nil {
		t.Fatal("Introduction accepted a close frame before register or deliver")
	}
}
