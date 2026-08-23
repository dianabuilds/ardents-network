package endpoint

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
)

const acknowledgementBodySize = 4 + 1 + 32 + 8 + 8 + 32 + 32 + 32
const acknowledgementSize = acknowledgementBodySize + ed25519.SignatureSize
const acknowledgementMagic = "ARIA"

// validAcknowledgement verifies the native Introduction receipt before it
// enters the target-owned publication record.
func validAcknowledgement(raw []byte, credential Credential, network, broker, introduction [32]byte) bool {
	if len(raw) != acknowledgementSize || string(raw[:4]) != acknowledgementMagic || raw[4] != 1 ||
		!equal32(raw[5:37], credential.Target) || binary.BigEndian.Uint64(raw[37:45]) != credential.Generation ||
		binary.BigEndian.Uint64(raw[45:53]) != uint64(credential.NotAfter) || !equal32(raw[53:85], network) ||
		!equal32(raw[85:117], broker) || bytes.Equal(raw[117:149], make([]byte, 32)) {
		return false
	}
	return ed25519.Verify(ed25519.PublicKey(introduction[:]), acknowledgementMessage(raw[:acknowledgementBodySize]),
		raw[acknowledgementBodySize:])
}

func acknowledgementMessage(body []byte) []byte {
	return append([]byte("ardents-service-introduction-ack-v1\x00"), body...)
}
