//go:build live

package network_test

import (
	"bytes"
	"encoding/binary"
)

func writeBytes(raw *bytes.Buffer, value []byte, width int) {
	if width == 1 {
		raw.WriteByte(byte(len(value)))
	} else {
		writeU16(raw, uint16(len(value)))
	}
	raw.Write(value)
}

func writeText(raw *bytes.Buffer, value string) { writeBytes(raw, []byte(value), 1) }
func writeU16(raw *bytes.Buffer, value uint16)  { _ = binary.Write(raw, binary.BigEndian, value) }
func writeU32(raw *bytes.Buffer, value uint32)  { _ = binary.Write(raw, binary.BigEndian, value) }
func writeU64(raw *bytes.Buffer, value uint64)  { _ = binary.Write(raw, binary.BigEndian, value) }
func writeI64(raw *bytes.Buffer, value int64)   { _ = binary.Write(raw, binary.BigEndian, value) }
