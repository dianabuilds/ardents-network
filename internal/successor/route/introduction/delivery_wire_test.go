package introduction

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/capsule"
)

// Fixed outer bytes are a grammar fixture, not an authenticated HPKE capsule.
func deliveryEnvelope(t *testing.T) capsule.Envelope {
	t.Helper()
	raw := make([]byte, 474)
	raw[0], raw[48], raw[80], raw[114] = 1, 30, 40, 50
	binary.BigEndian.PutUint64(raw[32:40], 7)
	binary.BigEndian.PutUint64(raw[40:48], uint64(time.Now().Add(8*time.Second).Unix()))
	binary.BigEndian.PutUint16(raw[112:114], 360)
	envelope, err := capsule.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return envelope
}

func TestDeliveryIndependentFixedEnvelopeAndNonceNamespaces(t *testing.T) {
	envelope := deliveryEnvelope(t)
	nonce := [32]byte{9}
	body, err := EncodeDelivery(nonce, envelope)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) != 4096 || body[0] != 4 || body[1] != 9 || !bytes.Equal(body[33:507], envelope.Bytes()) || !zeroPadding(body[507:]) {
		t.Fatal("sealed bytes or independent delivery envelope changed")
	}
	actualNonce, actualEnvelope, err := decodeDelivery(body)
	if err != nil || actualNonce != nonce || actualEnvelope != envelope {
		t.Fatal("delivery input changed", err)
	}
	for _, offset := range []int{0, 1, 507, 4095} {
		mutated := append([]byte(nil), body...)
		if offset < 507 {
			mutated[offset] = 0
		} else {
			mutated[offset] = 1
		}
		if _, _, err := decodeDelivery(mutated); err == nil {
			t.Fatal("foreign operation/nonce/padding accepted", offset)
		}
	}
	for _, candidate := range [][32]byte{{}, envelope.Header().DeliveryNonce} {
		if _, err := EncodeDelivery(candidate, envelope); err == nil {
			t.Fatal("copied or zero channel nonce accepted")
		}
	}
	if deliveryExchangeBytes != 20529 || registrationExchangeBytes != 41024 {
		t.Fatal("fixed RESULT/CLOSE or withdrawal reserve changed")
	}
}
