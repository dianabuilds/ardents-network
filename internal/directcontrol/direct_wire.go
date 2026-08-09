package directcontrol

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"io"
)

func writeDirectRecord(writer io.Writer, payload []byte) error {
	if len(payload) > directRecordLimit {
		return errors.New("direct TLS Application record exceeds its fixed limit")
	}
	header := [4]byte{}
	binary.BigEndian.PutUint32(header[:], uint32(len(payload)))
	if err := writeDirectBytes(writer, header[:]); err != nil {
		return err
	}
	return writeDirectBytes(writer, payload)
}

func readDirectRecord(reader io.Reader, maximum int) ([]byte, error) {
	header := [4]byte{}
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return nil, err
	}
	size := int(binary.BigEndian.Uint32(header[:]))
	if size < 0 || size > maximum {
		return nil, errors.New("direct TLS Application record has an invalid length")
	}
	payload := make([]byte, size)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func directPayload(seed string, size int) []byte {
	payload := make([]byte, size)
	var counter uint64
	for offset := 0; offset < len(payload); {
		input := make([]byte, len(seed)+8)
		copy(input, seed)
		binary.BigEndian.PutUint64(input[len(seed):], counter)
		block := sha256.Sum256(input)
		offset += copy(payload[offset:], block[:])
		counter++
	}
	return payload
}

func equalDirectBytes(left, right []byte) bool {
	return len(left) == len(right) && subtle.ConstantTimeCompare(left, right) == 1
}

func writeDirectBytes(writer io.Writer, data []byte) error {
	for len(data) > 0 {
		written, err := writer.Write(data)
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
		data = data[written:]
	}
	return nil
}
