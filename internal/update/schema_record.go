package update

import (
	"crypto/sha256"
	"encoding/binary"
)

const (
	schemaRecordBodyBytes = 160
	maximumSchemaBytes    = 64 << 20
	maximumSchemaEntries  = 4096
)

type schemaCurrent struct {
	Transaction uint64
	Selection   SchemaSelection
	Predecessor [32]byte
}

// schemaSelectionIdentity returns the one canonical opaque identity for the
// bounded descriptor returned by SchemaWork. Paths and foreign-root bytes are
// deliberately absent from this transaction-owned commitment.
func schemaSelectionIdentity(selection SchemaSelection) [32]byte {
	buffer := make([]byte, 0, 32+32+8+32+8+8)
	buffer = append(buffer, "ardents-schema-selection-v1\x00"...)
	buffer = append(buffer, selection.Owner[:]...)
	var number [8]byte
	binary.BigEndian.PutUint64(number[:], selection.Generation)
	buffer = append(buffer, number[:]...)
	buffer = append(buffer, selection.Content[:]...)
	binary.BigEndian.PutUint64(number[:], selection.Bytes)
	buffer = append(buffer, number[:]...)
	binary.BigEndian.PutUint64(number[:], selection.Entries)
	buffer = append(buffer, number[:]...)
	return sha256.Sum256(buffer)
}

func encodeSchemaCurrent(current schemaCurrent) ([]byte, error) {
	selection := current.Selection
	if selection.Owner == ([32]byte{}) || selection.Content == ([32]byte{}) ||
		selection.Bytes > maximumSchemaBytes || selection.Entries > maximumSchemaEntries ||
		selection.Identity != schemaSelectionIdentity(selection) {
		return nil, errRecordInvalid
	}
	body := make([]byte, schemaRecordBodyBytes)
	binary.BigEndian.PutUint64(body[0:8], current.Transaction)
	copy(body[8:40], selection.Owner[:])
	binary.BigEndian.PutUint64(body[40:48], selection.Generation)
	copy(body[48:80], selection.Identity[:])
	copy(body[80:112], selection.Content[:])
	binary.BigEndian.PutUint64(body[112:120], selection.Bytes)
	binary.BigEndian.PutUint64(body[120:128], selection.Entries)
	copy(body[128:160], current.Predecessor[:])
	return encodeRecord(recordSchemaCurrent, body, maximumRecordBytes)
}

func decodeSchemaCurrent(raw []byte) (schemaCurrent, error) {
	var current schemaCurrent
	body, err := decodeRecord(raw, recordSchemaCurrent, maximumRecordBytes)
	if err != nil || len(body) != schemaRecordBodyBytes {
		return current, errRecordInvalid
	}
	current.Transaction = binary.BigEndian.Uint64(body[0:8])
	copy(current.Selection.Owner[:], body[8:40])
	current.Selection.Generation = binary.BigEndian.Uint64(body[40:48])
	copy(current.Selection.Identity[:], body[48:80])
	copy(current.Selection.Content[:], body[80:112])
	current.Selection.Bytes = binary.BigEndian.Uint64(body[112:120])
	current.Selection.Entries = binary.BigEndian.Uint64(body[120:128])
	copy(current.Predecessor[:], body[128:160])
	if _, err := encodeSchemaCurrent(current); err != nil {
		return schemaCurrent{}, err
	}
	return current, nil
}
