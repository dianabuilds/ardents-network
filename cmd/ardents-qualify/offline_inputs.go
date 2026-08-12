package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/planfile"
)

func readMaterials(paths string) ([][]byte, error) {
	if paths == "" {
		return nil, errors.New("materializations are required")
	}
	parts := strings.Split(paths, ",")
	if len(parts) > 64 {
		return nil, errors.New("too many materializations")
	}
	materials := make([][]byte, len(parts))
	for index, path := range parts {
		var err error
		materials[index], err = planfile.Read(path, 35<<10)
		if err != nil {
			return nil, fmt.Errorf("read materialization %d: %w", index, err)
		}
	}
	return materials, nil
}

func decodeHex(encoded string, destination []byte) error {
	decoded, err := hex.DecodeString(encoded)
	if err != nil {
		return err
	}
	if len(decoded) != len(destination) {
		return fmt.Errorf("decoded length is %d, want %d", len(decoded), len(destination))
	}
	copy(destination, decoded)
	return nil
}
