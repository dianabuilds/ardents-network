//go:build ignore

package main

import (
	"encoding/binary"
	"fmt"
	"io"
)

// The encoding helpers below are a local copy of the helpers that the
// maintained tests under tests/e2e/network-source/ use to build a canonical
// State fixture. The test files are package state_test, so they cannot be
// imported from a non-test Go binary. The pilot prebake runs as a regular Go
// command inside the prebake container, so it owns its own minimal copy.
// This duplication is intentional and limited to the experiment tree.
//
// IMPORTANT: writeText writes a 1-byte length prefix, not a 4-byte one. The
// production decoder in internal/network/state/decoder.go uses
// stateReader.Text which reads exactly one byte as the text length. The
// earlier 4-byte version of writeText in this package produced epochs that
// the production accept-offline path rejected with "epoch profile is
// unsupported" because the decoder read the first text's 1-byte length from
// what was actually the second byte of a 4-byte big-endian length, then
// walked off the end of the buffer and emitted "truncated canonical bytes",
// surfaced to the CLI as "epoch profile is unsupported" via the second
// text() call.

func writeU64(w io.Writer, value uint64) error {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], value)
	_, err := w.Write(buf[:])
	return err
}
func writeI64(w io.Writer, value int64) error { return writeU64(w, uint64(value)) }
func writeU32(w io.Writer, value uint32) error {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], value)
	_, err := w.Write(buf[:])
	return err
}
func writeU16(w io.Writer, value uint16) error {
	var buf [2]byte
	binary.BigEndian.PutUint16(buf[:], value)
	_, err := w.Write(buf[:])
	return err
}
func writeText(w io.Writer, value string) error {
	if len(value) > 255 {
		return fmt.Errorf("pilot: text length %d exceeds 1-byte limit", len(value))
	}
	if _, err := w.Write([]byte{byte(len(value))}); err != nil {
		return err
	}
	_, err := io.WriteString(w, value)
	return err
}
