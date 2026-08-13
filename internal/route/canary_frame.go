package route

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
)

const canaryLength = 32

func writeCanary(writer io.Writer, value []byte) error {
	if len(value) != canaryLength {
		return errors.New("canary must contain exactly 32 bytes")
	}
	frame := make([]byte, 0, 9+len(value))
	frame = append(frame, "ARCN"...)
	frame = append(frame, 1)
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(value)))
	frame = append(frame, length[:]...)
	frame = append(frame, value...)
	return writeAll(writer, frame)
}

func readCanary(reader io.Reader) ([]byte, error) {
	header := make([]byte, 9)
	if _, err := io.ReadFull(reader, header); err != nil {
		return nil, err
	}
	if string(header[:4]) != "ARCN" || header[4] != 1 || binary.BigEndian.Uint32(header[5:]) != canaryLength {
		return nil, errors.New("canary frame is malformed or outside its bound")
	}
	value := make([]byte, canaryLength)
	_, err := io.ReadFull(reader, value)
	return value, err
}

func writeReceipt(writer io.Writer, value []byte) error {
	digest := sha256.Sum256(value)
	frame := make([]byte, 0, 8+len(value)+len(digest))
	frame = append(frame, "AROK"...)
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(value)))
	frame = append(frame, length[:]...)
	frame = append(frame, value...)
	frame = append(frame, digest[:]...)
	return writeAll(writer, frame)
}

type canaryReceipt struct {
	length uint32
	digest [32]byte
	bytes  []byte
}

func readReceipt(reader io.Reader, expected []byte) (canaryReceipt, error) {
	header := make([]byte, 8)
	if _, err := io.ReadFull(reader, header); err != nil {
		return canaryReceipt{}, err
	}
	length := binary.BigEndian.Uint32(header[4:])
	if string(header[:4]) != "AROK" || length != canaryLength {
		return canaryReceipt{}, errors.New("canary receipt is malformed")
	}
	value := make([]byte, int(length))
	var claimed [32]byte
	if _, err := io.ReadFull(reader, value); err != nil {
		return canaryReceipt{}, err
	}
	if _, err := io.ReadFull(reader, claimed[:]); err != nil {
		return canaryReceipt{}, err
	}
	digest := sha256.Sum256(value)
	if !bytes.Equal(value, expected) || digest != claimed {
		return canaryReceipt{}, errors.New("publisher receipt does not match the exact canary")
	}
	return canaryReceipt{length: length, digest: digest, bytes: value}, nil
}

func writeAll(writer io.Writer, value []byte) error {
	for len(value) > 0 {
		written, err := writer.Write(value)
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrNoProgress
		}
		value = value[written:]
	}
	return nil
}
