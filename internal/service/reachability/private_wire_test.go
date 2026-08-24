package reachability

import "testing"

func TestPrivateWireBindsExactRequestAndResponse(t *testing.T) {
	request := privateRequest{network: [32]byte{1}, target: [32]byte{2}, nonce: [32]byte{3}, deadline: 2_000_200_000_000_000_000}
	raw, err := encodePrivateRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodePrivateRequest(raw)
	if err != nil || decoded != request {
		t.Fatalf("Decode request = %+v, %v", decoded, err)
	}
	padded, err := padPrivateMessage(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(padded) != privateMessageSize {
		t.Fatalf("padded length = %d", len(padded))
	}
	if restored, err := unpadPrivateMessage(padded); err != nil || string(restored) != string(raw) {
		t.Fatalf("Unpad = %x, %v", restored, err)
	}
	response := privateResponse{network: request.network, target: request.target, nonce: request.nonce, deadline: request.deadline,
		class: privateResolved, descriptor: []byte("signed-descriptor")}
	encodedResponse, err := encodePrivateResponse(response)
	if err != nil {
		t.Fatal(err)
	}
	decodedResponse, err := decodePrivateResponse(encodedResponse)
	if err != nil || decodedResponse.class != response.class || string(decodedResponse.descriptor) != string(response.descriptor) ||
		decodedResponse.network != request.network || decodedResponse.target != request.target || decodedResponse.nonce != request.nonce || decodedResponse.deadline != request.deadline {
		t.Fatalf("Decode response = %+v, %v", decodedResponse, err)
	}
}

func TestPrivateWireRejectsMalformedOrAmbiguousMessages(t *testing.T) {
	if _, err := decodePrivateRequest(make([]byte, 1)); err == nil {
		t.Fatal("Decode accepted a truncated request")
	}
	if _, err := encodePrivateResponse(privateResponse{network: [32]byte{1}, target: [32]byte{2}, nonce: [32]byte{3}, deadline: 1,
		class: privateUnavailable, descriptor: []byte("unexpected")}); err == nil {
		t.Fatal("Encode accepted a descriptor with unavailable response")
	}
	padded := make([]byte, privateMessageSize)
	padded[1] = 1
	padded[2] = 1
	padded[3] = 1
	if _, err := unpadPrivateMessage(padded); err == nil {
		t.Fatal("Unpad accepted non-canonical padding")
	}
}
