//go:build ignore

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
)

const (
	transcriptBytes  = 256 << 10
	frameHeaderBytes = 4 + sha256.Size
)

var expectedTranscriptDigest = sha256.Sum256(experimentTranscript())

func experimentTranscript() []byte {
	result := make([]byte, transcriptBytes)
	for index := range result {
		result[index] = byte((index*31 + 17) % 251)
	}
	return result
}

func writeTranscript(writer io.Writer) error {
	payload := experimentTranscript()
	header := make([]byte, frameHeaderBytes)
	binary.BigEndian.PutUint32(header, uint32(len(payload)))
	copy(header[4:], expectedTranscriptDigest[:])
	if _, err := writer.Write(header); err != nil {
		return err
	}
	_, err := writer.Write(payload)
	return err
}

func readTranscript(reader io.Reader) error {
	header := make([]byte, frameHeaderBytes)
	if _, err := io.ReadFull(reader, header); err != nil {
		return err
	}
	if binary.BigEndian.Uint32(header) != transcriptBytes || !bytes.Equal(header[4:], expectedTranscriptDigest[:]) {
		return errors.New("transcript header is invalid")
	}
	payload := make([]byte, transcriptBytes)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return err
	}
	if digest := sha256.Sum256(payload); digest != expectedTranscriptDigest {
		return errors.New("transcript digest is invalid")
	}
	return nil
}
