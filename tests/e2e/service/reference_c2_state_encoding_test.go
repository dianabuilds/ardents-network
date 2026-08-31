//go:build referencec2

package service_test

import (
	"bytes"
	"encoding/binary"
)

func referenceC2Text(buffer *bytes.Buffer, value string) {
	buffer.WriteByte(byte(len(value)))
	buffer.WriteString(value)
}

func referenceC2U16(buffer *bytes.Buffer, value uint16) {
	var raw [2]byte
	binary.BigEndian.PutUint16(raw[:], value)
	buffer.Write(raw[:])
}

func referenceC2U32(buffer *bytes.Buffer, value uint32) {
	var raw [4]byte
	binary.BigEndian.PutUint32(raw[:], value)
	buffer.Write(raw[:])
}

func referenceC2U64(buffer *bytes.Buffer, value uint64) {
	var raw [8]byte
	binary.BigEndian.PutUint64(raw[:], value)
	buffer.Write(raw[:])
}

func referenceC2I64(buffer *bytes.Buffer, value int64) { referenceC2U64(buffer, uint64(value)) }
