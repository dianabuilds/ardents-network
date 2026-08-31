package publication

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
)

const legacyIntroductionReceiptBodySize = 4 + 1 + 32 + 8 + 8 + 32 + 32 + 32
const legacyIntroductionReceiptSize = legacyIntroductionReceiptBodySize + ed25519.SignatureSize
const legacyIntroductionReceiptMagic = "ARIA"

// ValidLegacyIntroductionReceipt verifies the retained lower-level C-2
// publication receipt. The maintained Publisher start path commits its native
// slot-ready transcript instead and never uses this grammar or a local socket.
func ValidLegacyIntroductionReceipt(raw []byte, credential Credential, broker, introduction [32]byte) bool {
	if len(raw) != legacyIntroductionReceiptSize || string(raw[:4]) != legacyIntroductionReceiptMagic || raw[4] != 1 ||
		!equalReceiptField(raw[5:37], credential.Target) || binary.BigEndian.Uint64(raw[37:45]) != credential.Generation ||
		binary.BigEndian.Uint64(raw[45:53]) != uint64(credential.NotAfter) ||
		!equalReceiptField(raw[53:85], credential.NetworkID) || !equalReceiptField(raw[85:117], broker) ||
		bytes.Equal(raw[117:149], make([]byte, 32)) {
		return false
	}
	return ed25519.Verify(ed25519.PublicKey(introduction[:]), legacyIntroductionReceiptMessage(raw[:legacyIntroductionReceiptBodySize]),
		raw[legacyIntroductionReceiptBodySize:])
}

func legacyIntroductionReceiptMessage(body []byte) []byte {
	return append([]byte("ardents-service-introduction-ack-v1\x00"), body...)
}

func equalReceiptField(raw []byte, expected [32]byte) bool {
	return len(raw) == len(expected) && bytes.Equal(raw, expected[:])
}
