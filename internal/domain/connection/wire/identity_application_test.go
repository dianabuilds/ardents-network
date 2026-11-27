package wire

import (
	"crypto/tls"
	"testing"
)

func TestByteStreamContextAndReplacement(t *testing.T) {
	identity := LogicalIdentity{
		Network: [32]byte{1}, Target: [32]byte{2}, Instance: [32]byte{3}, InstanceGeneration: 4,
		Publication: [32]byte{5}, Profile: [32]byte{6}, ConnectionNonce: [32]byte{7}, InitiatorBinding: [32]byte{8},
		WorkSafetyNotAfter: 9, WorkSafetyMaximum: 10, NoNewRecoveryAfter: 8,
	}
	capsule := [32]byte{11}
	stream, streamAttachment, err := Contexts(identity, capsule, 1)
	if err != nil {
		t.Fatal(err)
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
	wrongContext := expectedStream
	wrongContext.Context = [32]byte{99}
	if receipt, err := VerifyInitialRequest(binding, wrongContext, request, proof); err == nil || receipt != nil {
		t.Fatal("request obtained an Instance signing receipt for a different context")
	}
	if _, err := VerifyInitialRequest(binding, expectedStream, request, proof); err != nil {
		t.Fatal("matching byte-stream request did not obtain its original receipt:", err)
	}

}
