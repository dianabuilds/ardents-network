package node

import (
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestClosedForwardingTransportFailureIsNotPeerTerminal(t *testing.T) {
	reverse := newClosedForwardingQueue(32)
	reverse.close()
	session := &closedForwardingSession{closed: true}
	if written, err := session.writeChildFrame(route.ClosedLaneFrame{Kind: 9, Lane: 1, Body: []byte{5}}, time.Now().Add(time.Second), reverse); err == nil || written {
		t.Fatal("physical EOF was treated as successful child terminal")
	}
}

func TestClosedForwardingPartialWriteFailureSurvivesPeerTerminal(t *testing.T) {
	local, peer := net.Pipe()
	defer local.Close()
	defer peer.Close()
	reverse := newClosedForwardingQueue(32)
	session := &closedForwardingSession{carrier: local}
	observed := make(chan error, 1)
	go func() {
		var first [1]byte
		_, err := peer.Read(first[:])
		if err == nil {
			err = reverse.push(route.ClosedLaneFrame{Kind: 9, Lane: 1, Body: []byte{0}}, nil)
		}
		_ = peer.Close()
		observed <- err
	}()
	written, err := session.writeChildFrame(route.ClosedLaneFrame{Kind: 6, Lane: 1, Body: []byte("payload")}, time.Now().Add(time.Second), reverse)
	if peerErr := <-observed; peerErr != nil {
		t.Fatal(peerErr)
	}
	if !reverse.peerClosed() || written || err == nil {
		t.Fatal("received terminal hid a partially emitted frame failure")
	}
}
