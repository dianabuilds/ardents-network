package recoverysmoke

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
)

func refreshWorkload(generationRoot string) error {
	for _, name := range []string{"client-seed.hex", "publisher-seed.hex"} {
		seed := make([]byte, 32)
		if _, err := rand.Read(seed); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(generationRoot, name), []byte(hex.EncodeToString(seed)), 0o600); err != nil {
			return err
		}
	}
	return nil
}

func recoveryDirectionSeed(root, direction string) ([32]byte, error) {
	name := "client-seed.hex"
	if direction == "publisher-to-client" {
		name = "publisher-seed.hex"
	}
	var seed [32]byte
	raw, err := byteio.ReadFile(filepath.Join(root, name), 64)
	if err != nil || len(raw) != 64 {
		return seed, errors.Join(err, errors.New("recovery workload seed is invalid"))
	}
	_, err = hex.Decode(seed[:], raw)
	return seed, err
}

func recoveryFaultOffset(seed [32]byte) uint32 {
	return (uint32(17) + uint32(seed[0]%8)) * 16_381
}

func workloadDigest(seed [32]byte, count uint32) [32]byte {
	hash := sha256.New()
	for offset := uint32(0); offset < count; offset += 32 {
		block := workloadBlock(seed, uint64(offset/32))
		length := min(uint32(32), count-offset)
		_, _ = hash.Write(block[:length])
	}
	var digest [32]byte
	copy(digest[:], hash.Sum(nil))
	return digest
}

func workloadCanary(seed [32]byte, offset uint32) [32]byte {
	var canary [32]byte
	for index := range canary {
		block := workloadBlock(seed, uint64((offset+uint32(index))/32))
		canary[index] = block[(offset+uint32(index))%32]
	}
	return canary
}

func workloadBlock(seed [32]byte, counter uint64) [32]byte {
	input := make([]byte, 40)
	copy(input, seed[:])
	binary.BigEndian.PutUint64(input[32:], counter)
	return sha256.Sum256(input)
}
