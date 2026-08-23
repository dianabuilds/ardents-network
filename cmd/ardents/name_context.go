package main

import (
	"encoding/hex"
	"errors"
)

const maxResolutionInput = 1 << 20

func decodeContext(raw string) ([32]byte, error) {
	var contextID [32]byte
	decoded, err := hex.DecodeString(raw)
	if err != nil || len(decoded) != len(contextID) || hex.EncodeToString(decoded) != raw {
		return contextID, errors.New("isolation Context must be 32 canonical hexadecimal bytes")
	}
	copy(contextID[:], decoded)
	if contextID == [32]byte{} {
		return contextID, errors.New("isolation Context must be non-zero")
	}
	return contextID, nil
}
