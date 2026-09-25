package capsule

import "time"

// Capsule is the Introduction-visible envelope. It has no
// Target, Rendezvous, logical Connection label or callback address.
type Capsule struct {
	Slot                         [32]byte
	Revision                     uint64
	Expiry                       time.Time
	DeliveryNonce, Encapsulation [32]byte
	Ciphertext                   []byte
}

// Recipient performs only this generation's fixed HPKE
// operation. The Instance owns its private key and retirement checks.
type Recipient interface {
	OpenPrivateIntroduction(encapsulation, info, authenticatedHeader, ciphertext []byte, at time.Time) ([]byte, error)
}

func validClosedIntroductionHeader(input Capsule) bool {
	return input.Slot != [32]byte{} && input.Revision != 0 && input.DeliveryNonce != [32]byte{} && input.Expiry.Unix() > 0 &&
		input.Expiry.Equal(input.Expiry.UTC().Truncate(time.Second))
}
