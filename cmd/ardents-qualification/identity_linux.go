package main

import (
	"encoding/hex"
	"errors"
)

func decodeIdentity(value string) ([32]byte, error) {
	var identity [32]byte
	bytes, err := hex.DecodeString(value)
	if err != nil || len(bytes) != 32 || hex.EncodeToString(bytes) != value {
		return identity, errors.New("qualification identity must be canonical SHA-256-width hex")
	}
	copy(identity[:], bytes)
	if identity == [32]byte{} {
		return identity, errors.New("qualification identity absent")
	}
	return identity, nil
}
