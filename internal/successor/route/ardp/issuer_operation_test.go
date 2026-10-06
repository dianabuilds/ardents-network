package ardp

import (
	"bytes"
	"testing"
)

func TestIssuerEnvelopeFixedWidthsAndNonce(t *testing.T) {
	nonce := [32]byte{7}
	batch := []byte("opaque admission batch")
	body, err := EncodeIssuerRequest(nonce, batch)
	if err != nil || len(body) != 16384 || body[0] != 1 || !bytes.Equal(body[1:33], nonce[:]) || !bytes.Equal(body[33:33+len(batch)], batch) {
		t.Fatal("request envelope changed", err)
	}
	for _, value := range body[33+len(batch):] {
		if value != 0 {
			t.Fatal("request padding was not zero")
		}
	}
	actual, padded, err := DecodeIssuerRequest(body)
	if err != nil || actual != nonce || len(padded) != 16351 {
		t.Fatal("request envelope refused", err)
	}
	for _, invalid := range [][]byte{body[:16383], append(bytes.Clone(body), 0), make([]byte, 16384)} {
		if _, _, err := DecodeIssuerRequest(invalid); err == nil {
			t.Fatal("invalid operation accepted")
		}
	}
	payload := bytes.Repeat([]byte{3}, 16347)
	result, err := EncodeIssuerResult(nonce, 2, payload)
	if err != nil || len(result) != 16384 || !bytes.Equal(result[33:37], []byte{0, 0, 63, 219}) {
		t.Fatal("result envelope length changed", err)
	}
	status, decoded, err := DecodeIssuerResult(result, nonce)
	if err != nil || status != 2 || !bytes.Equal(decoded, payload) {
		t.Fatal("result payload changed", err)
	}
	for _, mutation := range []string{"nonce", "status", "length"} {
		invalid := bytes.Clone(result)
		switch mutation {
		case "nonce":
			invalid[0]++
		case "status":
			invalid[32] = 5
		case "length":
			invalid[36]--
		}
		if _, _, err := DecodeIssuerResult(invalid, nonce); err == nil {
			t.Fatal("invalid result accepted", mutation)
		}
	}
}
