package provisioning

import "github.com/dianabuilds/ardents-network/internal/platform/artifacts"

// controlPublic32 decodes one non-zero public 32-byte identifier for the
// offline Forwarding, JOIN and Descriptor preparations. The closed role plans
// keep their own schemas and validators; this shared decoder only enforces the
// common canonical hex shape and rejects the zero identifier.
func controlPublic32(raw string) ([32]byte, error) {
	value, ok := decodeHex32(raw)
	if !ok || value == [32]byte{} {
		return value, artifacts.ErrInput
	}
	return value, nil
}
