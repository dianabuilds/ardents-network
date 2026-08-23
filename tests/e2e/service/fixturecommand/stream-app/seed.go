package main

import (
	"encoding/hex"
	"errors"
	"os"
)

func readSeed(path string) ([32]byte, error) {
	var seed [32]byte
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) != 64 {
		return seed, errors.Join(err, errors.New("workload seed is invalid"))
	}
	_, err = hex.Decode(seed[:], raw)
	return seed, err
}
