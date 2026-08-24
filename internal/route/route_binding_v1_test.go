package route

import (
	"bytes"
	"encoding/hex"
	"testing"
	"time"
)

func TestLegBindingV1CanonicalVectorAndReciprocalPeer(t *testing.T) {
	first := bindingFixture()
	raw, err := EncodeLegBinding(first)
	if err != nil {
		t.Fatal(err)
	}
	const want = "617264656e74732d696e7465726163746976652d726f7574652d76310000d20001021c617264656e74732d696e7465726163746976652d726f7574652d7631010000000000000000000000000000000000000000000000000000000000000000000000000000070200000000000000000000000000000000000000000000000000000000000000030000000000000000000000000000000000000000000000000000000000000001040000000000000000000000000000000000000000000000000000000000000003050000000000000000000000000000000000000000000000000000000000000000000000684ee180"
	if hex.EncodeToString(raw) != want {
		t.Fatalf("canonical vector = %x, want %s", raw, want)
	}
	decoded, err := DecodeLegBinding(raw)
	if err != nil || decoded != first {
		t.Fatalf("decoded binding = %+v, %v", decoded, err)
	}
	peer := first
	peer.SenderRole, peer.PeerRole = first.PeerRole, first.SenderRole
	peer.SenderNodeID, peer.PeerNodeID = first.PeerNodeID, first.SenderNodeID
	if err := first.VerifyReciprocal(peer); err != nil {
		t.Fatal(err)
	}
	peer.AttachmentID[0] ^= 1
	if err := first.VerifyReciprocal(peer); err == nil {
		t.Fatal("mismatched attachment was accepted as reciprocal")
	}
}

func TestLegBindingV1RejectsMalformedAndDowngradedBytes(t *testing.T) {
	raw, err := EncodeLegBinding(bindingFixture())
	if err != nil {
		t.Fatal(err)
	}
	mutations := [][]byte{
		nil,
		raw[:len(raw)-1],
		append(append([]byte(nil), raw...), 0),
		append([]byte("ardents-h3-route-tracer-v1"), raw...),
	}
	unknownRole := append([]byte(nil), raw...)
	unknownRole[len(unknownRole)-74] = 0
	mutations = append(mutations, unknownRole)
	for index, value := range mutations {
		if _, err := DecodeLegBinding(value); err == nil {
			t.Fatalf("mutation %d was accepted", index)
		}
	}
}

func bindingFixture() LegBinding {
	return LegBinding{NetworkID: identifier(1), Digest: identifier(2), AttachmentID: identifier(3), Epoch: 7,
		SenderRole: InitiatorRole, PeerRole: RendezvousRole, SenderNodeID: identifier(4), PeerNodeID: identifier(5),
		NotAfter: time.Unix(1_750_000_000, 0).UTC()}
}

func identifier(value byte) [32]byte {
	return [32]byte{value}
}

func equalIntroduction(left, right SealedIntroduction) bool {
	return left.NetworkID == right.NetworkID && left.Digest == right.Digest && left.Epoch == right.Epoch &&
		left.IntroductionNodeID == right.IntroductionNodeID && left.RendezvousNodeID == right.RendezvousNodeID &&
		left.Reachability == right.Reachability && left.NotAfter.Equal(right.NotAfter) && left.JoinHandle == right.JoinHandle &&
		left.EndpointHandshake == right.EndpointHandshake && bytes.Equal(left.Enc, right.Enc) && bytes.Equal(left.Ciphertext, right.Ciphertext)
}
