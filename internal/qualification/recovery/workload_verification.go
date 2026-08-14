package recovery

import (
	"crypto/sha256"
	"encoding/binary"
)

func workloadDigest(seed [32]byte, count uint32) [32]byte {
	hash := sha256.New()
	for offset := uint32(0); offset < count; {
		block := workloadBlock(seed, uint64(offset/32))
		length := min(uint32(len(block)), count-offset)
		_, _ = hash.Write(block[:length])
		offset += length
	}
	var value [32]byte
	copy(value[:], hash.Sum(nil))
	return value
}

func workloadRange(seed [32]byte, offset uint32) [32]byte {
	var value [32]byte
	for index := uint32(0); index < uint32(len(value)); index++ {
		block := workloadBlock(seed, uint64((offset+index)/32))
		value[index] = block[(offset+index)%32]
	}
	return value
}

func workloadBlock(seed [32]byte, counter uint64) [32]byte {
	input := make([]byte, 40)
	copy(input, seed[:])
	binary.BigEndian.PutUint64(input[32:], counter)
	return sha256.Sum256(input)
}
