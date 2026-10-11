package introduction

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/successor/route/capsule"
)

const registrationExchangeBytes = uint64(2 * (16 + 4096 + 16 + 16384))
const deliveryExchangeBytes = uint64(16 + 4096 + 16 + 16384 + 16 + 1)

// EncodeDelivery preserves the sealed capsule and adds only this channel's
// fresh request nonce. Its fixed envelope grants no recipient acceptance.
func EncodeDelivery(nonce [32]byte, envelope capsule.Envelope) ([]byte, error) {
	if nonce == [32]byte{} || nonce == envelope.Header().DeliveryNonce {
		return nil, errors.New("delivery channel nonce invalid")
	}
	raw := envelope.Bytes()
	if _, err := capsule.Parse(raw); err != nil {
		return nil, err
	}
	body := make([]byte, 4096)
	body[0] = 4
	copy(body[1:33], nonce[:])
	copy(body[33:507], raw)
	return body, nil
}

func decodeDelivery(body []byte) ([32]byte, capsule.Envelope, error) {
	var nonce [32]byte
	if len(body) != 4096 || body[0] != 4 || !zeroPadding(body[507:]) {
		return nonce, capsule.Envelope{}, errors.New("delivery operation invalid")
	}
	copy(nonce[:], body[1:33])
	envelope, err := capsule.Parse(body[33:507])
	if err != nil || nonce == [32]byte{} || nonce == envelope.Header().DeliveryNonce {
		return [32]byte{}, capsule.Envelope{}, errors.Join(errors.New("delivery channel binding invalid"), err)
	}
	return nonce, envelope, nil
}
