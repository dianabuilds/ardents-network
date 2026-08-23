package route

import (
	"bytes"
	"net"
	"testing"
)

func TestNativeNodeLegBindingExchangesOnlyReciprocalV1Records(t *testing.T) {
	initiator := bindingFixture()
	responder := initiator
	responder.SenderRole, responder.PeerRole = initiator.PeerRole, initiator.SenderRole
	responder.SenderNodeID, responder.PeerNodeID = initiator.PeerNodeID, initiator.SenderNodeID
	left, right := net.Pipe()
	result := make(chan error, 1)
	go func() { result <- AcceptNodeLegBinding(right, responder) }()
	if err := ConfirmNodeLegBinding(left, initiator); err != nil {
		t.Fatal(err)
	}
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	if err := left.Close(); err != nil {
		t.Fatal(err)
	}
	if err := right.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestNativeNodeLegBindingRefusesH3AndNonReciprocalRecords(t *testing.T) {
	local := bindingFixture()
	peer := local
	peer.SenderRole, peer.PeerRole = local.PeerRole, local.SenderRole
	peer.SenderNodeID, peer.PeerNodeID = local.PeerNodeID, local.SenderNodeID
	raw, err := EncodeLegBinding(peer)
	if err != nil {
		t.Fatal(err)
	}
	if err := AcceptNodeLegBinding(bytes.NewBuffer(append([]byte("ARLG"), raw...)), local); err == nil {
		t.Fatal("legacy H3 leg frame was accepted")
	}
	peer.AttachmentID[0] ^= 1
	raw, err = EncodeLegBinding(peer)
	if err != nil {
		t.Fatal(err)
	}
	if err := AcceptNodeLegBinding(bytes.NewBuffer(raw), local); err == nil {
		t.Fatal("non-reciprocal native leg was accepted")
	}
}
