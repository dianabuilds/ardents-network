package wire

import (
	"crypto/tls"
	"encoding/hex"
	"testing"
)

func TestApplicationProfileContexts(t *testing.T) {
	identity := LogicalIdentity{
		Network: [32]byte{1}, Target: [32]byte{2}, Instance: [32]byte{3}, InstanceGeneration: 4,
		Publication: [32]byte{5}, Profile: [32]byte{6}, ConnectionNonce: [32]byte{7}, InitiatorBinding: [32]byte{8},
		WorkSafetyNotAfter: 9, WorkSafetyMaximum: 10, NoNewRecoveryAfter: 8,
	}
	capsule := [32]byte{11}
	document, documentAttachment, err := Contexts(identity, capsule, 1)
	if err != nil {
		t.Fatal(err)
	}
	const originalDocument = "a463df85f6dc6ad4aaac61bdd52c02f105f27a6f61cc3e92cf555c3e3dbefea2"
	if hex.EncodeToString(document[:]) != originalDocument {
		t.Fatalf("original document context changed: %x", document)
	}

	identity.Application = ByteStreamApplication
	stream, streamAttachment, err := Contexts(identity, capsule, 1)
	if err != nil || stream == document || streamAttachment == documentAttachment {
		t.Fatalf("profiles share an authenticated context: %v", err)
	}
	recovered, replacement, err := Contexts(identity, [32]byte{12}, 2)
	if err != nil || recovered != stream || replacement == streamAttachment {
		t.Fatalf("replacement changed the logical binding or reused the attachment: %v", err)
	}

	binding := &TLSBinding{connection: &tls.Conn{}, key: [32]byte{13}, exporter: [32]byte{14}}
	request := Challenge{
		Network: identity.Network, Target: identity.Target, InstanceGeneration: identity.InstanceGeneration,
		Context: stream, Nonce: [32]byte{15},
	}
	proof, err := NewContinuity(binding.key, RoleInitiator, 1, 0, 0, 0, stream, binding.exporter)
	if err != nil {
		t.Fatal(err)
	}
	expectedStream := request
	expectedStream.Nonce = [32]byte{}
	expectedDocument := expectedStream
	expectedDocument.Context = document
	if receipt, err := VerifyInitialRequest(binding, expectedDocument, request, proof); err == nil || receipt != nil {
		t.Fatal("byte-stream request obtained an Instance signing receipt for the document profile")
	}
	if _, err := VerifyInitialRequest(binding, expectedStream, request, proof); err != nil {
		t.Fatal("matching byte-stream request did not obtain its original receipt:", err)
	}

	identity.Application = 255
	if logical, attachment, err := Contexts(identity, capsule, 1); err == nil || logical != [32]byte{} || attachment != [32]byte{} {
		t.Fatal("unknown Application profile supplied contexts")
	}
}
