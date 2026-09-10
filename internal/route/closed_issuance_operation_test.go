//go:build linux

package route

import "testing"

func TestClosedIssuanceOperationAndResultUseOneTerminalSize(t *testing.T) {
	nonce := [32]byte{1}
	batch := []byte("ARDIBR01canonical batch")
	requestBody, err := EncodeClosedIssuanceRequest(nonce, batch)
	if err != nil || len(requestBody) != closedTerminalOperationSize {
		t.Fatalf("encode issuance request = %d / %v", len(requestBody), err)
	}
	request, err := DecodeClosedIssuanceRequest(requestBody)
	if err != nil || request.Nonce != nonce || string(request.Payload[:len(batch)]) != string(batch) {
		t.Fatalf("decode issuance request = %+v / %v", request, err)
	}
	payload := make([]byte, closedIssuanceResultPayload)
	payload[0] = 2
	resultBody, err := EncodeClosedIssuanceResult(nonce, 2, payload)
	if err != nil || len(resultBody) != closedTerminalOperationSize {
		t.Fatalf("encode issuance result = %d / %v", len(resultBody), err)
	}
	result, err := DecodeClosedIssuanceResult(resultBody, nonce)
	if err != nil || result.Status != 2 || string(result.Payload) != string(payload) {
		t.Fatalf("decode issuance result = %+v / %v", result, err)
	}
	if _, err := DecodeClosedIssuanceResult(resultBody, [32]byte{2}); err == nil {
		t.Fatal("accepted result bound to another request nonce")
	}
}
