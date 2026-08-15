package streamworkload

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"time"
)

type generator struct {
	seed   [32]byte
	offset uint64
}

func (generator *generator) fill(value []byte) {
	for len(value) > 0 {
		counter, within := generator.offset/32, generator.offset%32
		var block [40]byte
		copy(block[:], generator.seed[:])
		binary.BigEndian.PutUint64(block[32:], counter)
		digest := sha256.Sum256(block[:])
		count := min(len(value), len(digest)-int(within))
		copy(value[:count], digest[within:uint64(count)+within])
		value = value[count:]
		generator.offset += uint64(count)
	}
}

// PacingWriter bounds writes to non-record-aligned chunks and optionally
// delays each successful chunk.
func PacingWriter(destination io.Writer, encodedDelay string) (func([]byte) (int, error), error) {
	if encodedDelay == "" {
		return destination.Write, nil
	}
	delay, err := time.ParseDuration(encodedDelay)
	if err != nil || delay <= 0 || delay > 3*time.Second {
		return nil, errors.New("stream chunk delay is outside its bound")
	}
	return func(value []byte) (int, error) {
		if len(value) > 16_381 {
			value = value[:16_381]
		}
		written, writeErr := destination.Write(value)
		if written > 0 {
			time.Sleep(delay)
		}
		return written, writeErr
	}, nil
}
