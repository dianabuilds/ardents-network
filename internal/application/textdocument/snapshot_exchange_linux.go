//go:build linux

package textdocument

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"unicode/utf8"
)

// NewSnapshot copies a previously authorized input after bounded validation.
// File selection and no-follow/stable-read import are separate trusted work.
func NewSnapshot(body []byte) (*Snapshot, error) {
	if len(body) > MaximumBytes || !utf8.Valid(body) {
		return nil, errors.New("text snapshot is invalid")
	}
	return &Snapshot{body: bytes.Clone(body)}, nil
}

// Respond accepts exactly one fixed request and directional EOF, then writes
// the single immutable response. Its caller owns authenticated stream closure.
func (snapshot *Snapshot) Respond(input io.Reader, output io.Writer) error {
	if snapshot == nil || input == nil || output == nil {
		return errors.New("text snapshot is unavailable")
	}
	var request [requestBytes + 1]byte
	n, err := io.ReadFull(input, request[:])
	if err != io.ErrUnexpectedEOF || n != requestBytes || !validDocumentRequest(request[:n]) {
		return errors.New("text request is invalid")
	}
	header := documentResponseHeader(len(snapshot.body))
	_, err = io.Copy(output, io.MultiReader(bytes.NewReader(header[:]), bytes.NewReader(snapshot.body)))
	return err
}

func validDocumentRequest(request []byte) bool {
	if len(request) != requestBytes || string(request[:8]) != magic || request[8] != 1 {
		return false
	}
	for _, value := range request[9:] {
		if value != 0 {
			return false
		}
	}
	return true
}

func documentResponseHeader(length int) [13]byte {
	var header [13]byte
	copy(header[:], magic)
	binary.BigEndian.PutUint32(header[9:], uint32(length))
	return header
}

// Snapshot is one immutable, bounded publication document. It contains no path,
// Service identity or authority, and can serve all of one worker's streams.
type Snapshot struct{ body []byte }
