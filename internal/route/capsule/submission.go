package capsule

import (
	"encoding/binary"
	"errors"
	"time"
)

const submissionSize = 4 << 10

func validPadding(raw []byte) bool {
	for _, value := range raw {
		if value != 0 {
			return false
		}
	}
	return true
}

// EncodeSubmission returns one fixed 4096-byte operation.
// The request nonce belongs to this channel; the capsule delivery nonce is
// separately generated and survives only inside the opaque forwarded capsule.
func EncodeSubmission(nonce [32]byte, capsule Capsule) ([]byte, error) {
	if nonce == [32]byte{} || nonce == capsule.DeliveryNonce || !validClosedIntroductionHeader(capsule) ||
		capsule.Encapsulation == [32]byte{} || len(capsule.Ciphertext) != closedIntroductionCiphertextSize {
		return nil, errors.New("closed Introduction submission invalid")
	}
	raw := make([]byte, submissionSize)
	raw[0] = 4
	copy(raw[1:33], nonce[:])
	copy(raw[33:113], closedIntroductionHeader(capsule))
	copy(raw[113:145], capsule.Encapsulation[:])
	binary.BigEndian.PutUint16(raw[145:147], uint16(len(capsule.Ciphertext)))
	copy(raw[147:], capsule.Ciphertext)
	return raw, nil
}

func DecodeSubmission(raw []byte) ([32]byte, Capsule, error) {
	var nonce [32]byte
	var capsule Capsule
	if len(raw) != submissionSize || raw[0] != 4 || binary.BigEndian.Uint16(raw[145:147]) != closedIntroductionCiphertextSize ||
		!validPadding(raw[147+closedIntroductionCiphertextSize:]) {
		return nonce, capsule, errors.New("closed Introduction submission framing invalid")
	}
	copy(nonce[:], raw[1:33])
	copy(capsule.Slot[:], raw[33:65])
	capsule.Revision = binary.BigEndian.Uint64(raw[65:73])
	seconds := binary.BigEndian.Uint64(raw[73:81])
	if seconds == 0 || seconds > 1<<63-1 {
		return nonce, capsule, errors.New("closed Introduction expiry invalid")
	}
	capsule.Expiry = time.Unix(int64(seconds), 0).UTC()
	copy(capsule.DeliveryNonce[:], raw[81:113])
	copy(capsule.Encapsulation[:], raw[113:145])
	if !validClosedIntroductionHeader(capsule) || capsule.Encapsulation == [32]byte{} || nonce == [32]byte{} || nonce == capsule.DeliveryNonce {
		return [32]byte{}, Capsule{}, errors.New("closed Introduction submission binding invalid")
	}
	capsule.Ciphertext = append([]byte(nil), raw[147:147+closedIntroductionCiphertextSize]...)
	return nonce, capsule, nil
}

func closedIntroductionHeader(capsule Capsule) []byte {
	raw := make([]byte, 0, 80)
	raw = append(raw, capsule.Slot[:]...)
	raw = binary.BigEndian.AppendUint64(raw, capsule.Revision)
	raw = binary.BigEndian.AppendUint64(raw, uint64(capsule.Expiry.Unix()))
	return append(raw, capsule.DeliveryNonce[:]...)
}
