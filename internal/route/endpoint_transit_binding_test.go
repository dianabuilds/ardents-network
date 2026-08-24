package route

import (
	"bytes"
	"encoding/hex"
	"testing"
	"time"
)

func TestEndpointTransitBindingV1CanonicalVector(t *testing.T) {
	input := endpointTransitBindingFixture()
	raw, err := EncodeEndpointTransitBinding(input)
	if err != nil {
		t.Fatal(err)
	}
	const want = "617264656e74732d696e7465726163746976652d726f7574652d76310000d70001061c617264656e74732d696e7465726163746976652d726f7574652d76313d0000000000000000000000000000000000000000000000000000000000000000000000000000413e000000000000000000000000000000000000000000000000000000000000003f0000000000000000000000000000000000000000000000000000000000000002400000000000000000000000000000000000000000000000000000000000000000000000684ee180420000000000000000000000000000000000000000000000000000000000000000040708090a"
	if hex.EncodeToString(raw) != want {
		t.Fatalf("canonical EndpointTransitBinding vector = %x, want %s", raw, want)
	}
	decoded, err := DecodeEndpointTransitBinding(raw)
	if err != nil || !equalEndpointTransitBinding(decoded, input) {
		t.Fatalf("decoded EndpointTransitBinding = %+v, %v", decoded, err)
	}
}

func TestEndpointTransitBindingV1RejectsWrongRoleAndMalformedBytes(t *testing.T) {
	raw, err := EncodeEndpointTransitBinding(endpointTransitBindingFixture())
	if err != nil {
		t.Fatal(err)
	}
	badRole := append([]byte(nil), raw...)
	badRole[len(routeWireMagic)+2+2+1+1+len(Profile)+32+8+32+32] = InitiatorRole
	wrongKind := append([]byte(nil), raw...)
	wrongKind[len(routeWireMagic)+2+2] = entryBindingKind
	for index, value := range [][]byte{nil, raw[:len(raw)-1], append(raw, 0), badRole, wrongKind} {
		if _, err := DecodeEndpointTransitBinding(value); err == nil {
			t.Fatalf("mutation %d was accepted", index)
		}
	}
}

func endpointTransitBindingFixture() EndpointTransitBinding {
	return EndpointTransitBinding{NetworkID: identifier(61), Digest: identifier(62), AttachmentID: identifier(63),
		TransitNodeID: identifier(64), Epoch: 65, TransitRole: IntroductionRole,
		NotAfter: time.Unix(1_750_000_000, 0).UTC(), ClientKeyDigest: identifier(66), Authorization: []byte{7, 8, 9, 10}}
}

func equalEndpointTransitBinding(left, right EndpointTransitBinding) bool {
	return left.NetworkID == right.NetworkID && left.Digest == right.Digest && left.AttachmentID == right.AttachmentID &&
		left.TransitNodeID == right.TransitNodeID && left.Epoch == right.Epoch && left.TransitRole == right.TransitRole &&
		left.NotAfter.Equal(right.NotAfter) && left.ClientKeyDigest == right.ClientKeyDigest && bytes.Equal(left.Authorization, right.Authorization)
}
