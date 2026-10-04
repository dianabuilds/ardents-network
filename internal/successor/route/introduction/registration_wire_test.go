package introduction

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"
)

func TestRegistrationWireHasIndependentCanonicalBytes(t *testing.T) {
	for _, withdraw := range []bool{false, true} {
		request := Request{Nonce: [32]byte{1}, Slot: [32]byte{2}, Revision: 0x0102030405060708, Withdraw: withdraw}
		manual := make([]byte, 4096)
		manual[0], manual[1], manual[33] = 3, 1, 2
		copy(manual[65:73], []byte{1, 2, 3, 4, 5, 6, 7, 8})
		if withdraw {
			manual[0] = 7
		} else {
			request.Expiry = time.Unix(1_800_000_300, 0).UTC()
			binary.BigEndian.PutUint64(manual[73:81], 1_800_000_300)
		}
		encoded, err := EncodeRequest(request)
		if err != nil || !bytes.Equal(encoded, manual) {
			t.Fatal("REGISTER/WITHDRAW differs from independent bytes", err)
		}
		decoded, err := DecodeRequest(manual)
		if err != nil || decoded != request {
			t.Fatal("independent request not decoded exactly", err)
		}
		for _, fault := range []string{"operation", "nonce", "slot", "revision", "padding", "length", "extra operation"} {
			body := bytes.Clone(manual)
			switch fault {
			case "operation":
				body[0] = 4
			case "nonce":
				clear(body[1:33])
			case "slot":
				clear(body[33:65])
			case "revision":
				clear(body[65:73])
			case "padding":
				body[4095] = 1
			case "length":
				body = body[:4095]
			case "extra operation":
				body = append(body, manual...)
			}
			if _, err := DecodeRequest(body); err == nil {
				t.Fatalf("accepted %s", fault)
			}
		}
	}
}

func TestRegistrationResultHasExactEmptyCanonicalPayload(t *testing.T) {
	nonce := [32]byte{9}
	for status := uint8(0); status <= 4; status++ {
		manual := make([]byte, 16384)
		manual[0], manual[32] = 9, status
		encoded, err := EncodeResult(nonce, status)
		if err != nil || !bytes.Equal(encoded, manual) {
			t.Fatal("RESULT differs from independent bytes", err)
		}
		if got, err := DecodeResult(manual, nonce); err != nil || got != status {
			t.Fatal("independent result not decoded", err)
		}
		for _, fault := range []string{"nonce", "outcome", "payload length", "payload", "padding", "length"} {
			body := bytes.Clone(manual)
			switch fault {
			case "nonce":
				body[0]++
			case "outcome":
				body[32] = 5
			case "payload length":
				body[36] = 1
			case "payload":
				body[37] = 1
			case "padding":
				body[16383] = 1
			case "length":
				body = body[:16383]
			}
			if _, err := DecodeResult(body, nonce); err == nil {
				t.Fatalf("accepted %s", fault)
			}
		}
	}
}
