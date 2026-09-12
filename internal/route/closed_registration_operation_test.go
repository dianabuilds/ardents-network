//go:build linux

package route

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"
)

func TestClosedRegistrationExactEnvelope(t *testing.T) {
	for _, withdraw := range []bool{false, true} {
		request := ClosedRegistrationRequest{Nonce: [32]byte{1}, Slot: [32]byte{2}, Revision: 3, Withdraw: withdraw}
		if !withdraw {
			request.Expiry = time.Unix(1800000000, 0).UTC()
		}
		body, err := EncodeClosedRegistrationRequest(request)
		if err != nil || len(body) != 4096 {
			t.Fatalf("exact envelope: %d %v", len(body), err)
		}
		decoded, err := DecodeClosedRegistrationRequest(body)
		if err != nil || decoded != request {
			t.Fatalf("roundtrip: %+v %v", decoded, err)
		}
		for _, invalid := range [][]byte{body[:4095], append(bytes.Clone(body), 0)} {
			if _, err := DecodeClosedRegistrationRequest(invalid); err == nil {
				t.Fatal("invalid fixed length accepted")
			}
		}
		for _, offset := range []int{0, 4095} {
			invalid := bytes.Clone(body)
			invalid[offset] = 255
			if _, err := DecodeClosedRegistrationRequest(invalid); err == nil {
				t.Fatalf("invalid byte %d accepted", offset)
			}
		}
		invalid := bytes.Clone(body)
		clear(invalid[65:73])
		if _, err := DecodeClosedRegistrationRequest(invalid); err == nil {
			t.Fatal("zero revision accepted")
		}
		if !withdraw {
			binary.BigEndian.PutUint64(invalid[65:73], 1)
			binary.BigEndian.PutUint64(invalid[73:81], ^uint64(0))
			if _, err := DecodeClosedRegistrationRequest(invalid); err == nil {
				t.Fatal("expiry overflow accepted")
			}
		}
	}
}
