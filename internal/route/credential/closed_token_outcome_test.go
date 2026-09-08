package credential

import "testing"

func TestClosedTokenBatchResultsShareFixedPlaintextShape(t *testing.T) {
	signature := make([]byte, closedTokenBlindElementSize)
	signature[0] = 1
	for _, result := range []ClosedTokenBatchResult{
		{Status: ClosedTokenIssued, Signatures: [][]byte{signature}},
		{Status: ClosedTokenExhausted}, {Status: ClosedTokenWithdrawn}, {Status: ClosedTokenUnavailable},
	} {
		raw, err := EncodeClosedTokenBatchResult(result)
		if err != nil || len(raw) != closedTokenOutcomeSize || len(raw)+32+1+4 != closedTokenTerminalResultSize {
			t.Fatalf("encode outcome %d = %d / %v", result.Status, len(raw), err)
		}
		decoded, err := DecodeClosedTokenBatchResult(raw)
		if err != nil || decoded.Status != result.Status || len(decoded.Signatures) != len(result.Signatures) {
			t.Fatalf("decode outcome %d = %+v / %v", result.Status, decoded, err)
		}
	}
	raw, err := EncodeClosedTokenBatchResult(ClosedTokenBatchResult{Status: ClosedTokenUnavailable})
	if err != nil {
		t.Fatal(err)
	}
	raw[len(raw)-1] = 1
	if _, err := DecodeClosedTokenBatchResult(raw); err == nil {
		t.Fatal("accepted a noncanonical outcome padding byte")
	}
}
