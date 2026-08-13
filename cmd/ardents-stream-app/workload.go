package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"os"
)

func workload(count int, seed [32]byte) []byte {
	value := make([]byte, 0, count)
	for counter := uint64(0); len(value) < count; counter++ {
		block := make([]byte, 40)
		copy(block, seed[:])
		binary.BigEndian.PutUint64(block[32:], counter)
		digest := sha256.Sum256(block)
		value = append(value, digest[:]...)
	}
	return value[:count]
}

func readSeed(path string) ([32]byte, error) {
	var seed [32]byte
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) != 64 {
		return seed, errors.Join(err, errors.New("workload seed is invalid"))
	}
	_, err = hex.Decode(seed[:], raw)
	return seed, err
}
