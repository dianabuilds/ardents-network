package route_test

import (
	"bytes"
	"encoding/binary"
)

func text(buffer *bytes.Buffer, value string) {
	buffer.WriteByte(byte(len(value)))
	buffer.WriteString(value)
}

func u16(buffer *bytes.Buffer, value uint16) {
	var raw [2]byte
	binary.BigEndian.PutUint16(raw[:], value)
	buffer.Write(raw[:])
}

func u32(buffer *bytes.Buffer, value uint32) {
	var raw [4]byte
	binary.BigEndian.PutUint32(raw[:], value)
	buffer.Write(raw[:])
}

func u64(buffer *bytes.Buffer, value uint64) {
	var raw [8]byte
	binary.BigEndian.PutUint64(raw[:], value)
	buffer.Write(raw[:])
}

func i64(buffer *bytes.Buffer, value int64) { u64(buffer, uint64(value)) }
