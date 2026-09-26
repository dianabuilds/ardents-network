package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
)

// commandMerkleLeaf hashes one signed record into its State input-root leaf.
// The former prepareCommandNetwork/commandRecord/commandEpoch/
// commandMaterialization builders belonged to the retired `entry import`
// command fixture (ADR-0106); the name-retirement State fixture is the
// remaining consumer of these exact wire writers.
func commandMerkleLeaf(record []byte) [32]byte {
	raw := make([]byte, 5+len(record))
	binary.BigEndian.PutUint32(raw[1:5], uint32(len(record)))
	copy(raw[5:], record)
	return sha256.Sum256(raw)
}

func writeCommandText(raw *bytes.Buffer, value string) {
	raw.WriteByte(byte(len(value)))
	raw.WriteString(value)
}
func writeCommandU16(raw *bytes.Buffer, value uint16) { _ = binary.Write(raw, binary.BigEndian, value) }
func writeCommandU32(raw *bytes.Buffer, value uint32) { _ = binary.Write(raw, binary.BigEndian, value) }
func writeCommandU64(raw *bytes.Buffer, value uint64) { _ = binary.Write(raw, binary.BigEndian, value) }
func writeCommandI64(raw *bytes.Buffer, value int64)  { _ = binary.Write(raw, binary.BigEndian, value) }
