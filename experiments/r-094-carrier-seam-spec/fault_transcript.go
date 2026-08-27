//go:build ignore

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"hash"
	"io"
)

const (
	faultPayloadSize = 256 << 10
	faultMaximumSize = 1 << 20
	faultHeaderSize  = 8 + 4 + sha256.Size
)

var (
	faultRequestMagic  = [8]byte{'R', '0', '9', '4', 'F', '0', '0', '1'}
	faultResponseMagic = [8]byte{'R', '0', '9', '4', 'O', 'K', '0', '1'}
)

func faultPayload() []byte {
	payload := make([]byte, faultPayloadSize)
	for index := range payload {
		payload[index] = byte((index*31 + 7) % 251)
	}
	return payload
}

func faultExchangeClient(lane io.ReadWriter) ([32]byte, error) {
	payload := faultPayload()
	digest := sha256.Sum256(payload)
	header := faultHeader(faultRequestMagic, uint32(len(payload)), digest)
	if err := faultWriteExact(lane, header); err != nil {
		return digest, err
	}
	if err := faultWriteExact(lane, payload); err != nil {
		return digest, err
	}
	response := make([]byte, faultHeaderSize)
	if _, err := io.ReadFull(lane, response); err != nil {
		return digest, err
	}
	if !bytes.Equal(response, faultHeader(faultResponseMagic, uint32(len(payload)), digest)) {
		return digest, errors.New("fault peer returned a noncanonical transcript proof")
	}
	return digest, nil
}

func faultExchangeServer(lane io.ReadWriter) error {
	header := make([]byte, faultHeaderSize)
	if _, err := io.ReadFull(lane, header); err != nil {
		return err
	}
	if !bytes.Equal(header[:8], faultRequestMagic[:]) {
		return errors.New("fault peer received an invalid transcript magic")
	}
	length := binary.BigEndian.Uint32(header[8:12])
	if length == 0 || length > faultMaximumSize {
		return errors.New("fault peer received an invalid transcript length")
	}
	expected := [32]byte{}
	copy(expected[:], header[12:])
	actual, err := faultReadDigest(lane, int64(length), sha256.New())
	if err != nil {
		return err
	}
	if actual != expected {
		return errors.New("fault peer received a corrupted transcript")
	}
	return faultWriteExact(lane, faultHeader(faultResponseMagic, length, actual))
}

func faultReadDigest(reader io.Reader, length int64, digest hash.Hash) ([32]byte, error) {
	if _, err := io.CopyN(digest, reader, length); err != nil {
		return [32]byte{}, err
	}
	var result [32]byte
	copy(result[:], digest.Sum(nil))
	return result, nil
}

func faultHeader(magic [8]byte, length uint32, digest [32]byte) []byte {
	result := make([]byte, faultHeaderSize)
	copy(result[:8], magic[:])
	binary.BigEndian.PutUint32(result[8:12], length)
	copy(result[12:], digest[:])
	return result
}

func faultWriteExact(writer io.Writer, value []byte) error {
	for len(value) != 0 {
		count, err := writer.Write(value)
		if err != nil {
			return err
		}
		if count <= 0 || count > len(value) {
			return io.ErrShortWrite
		}
		value = value[count:]
	}
	return nil
}
