package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
)

const maxRecordInput, maxResolutionInput = 16 << 20, 1 << 20

func readBoundedRecord(path string) ([]byte, error) {
	return readBounded(path, maxRecordInput, "name record")
}

func readBounded(path string, limit int64, kind string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	wire, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if len(wire) == 0 || int64(len(wire)) > limit {
		return nil, fmt.Errorf("%s input is empty or exceeds command bound", kind)
	}
	return wire, nil
}

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
