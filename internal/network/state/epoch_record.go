package state

import (
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"time"
)

const maximumRecordBytes = 32 << 10

type nodeRecord struct {
	raw        []byte
	networkID  [32]byte
	nodeID     [32]byte
	generation uint64
	notBefore  time.Time
	notAfter   time.Time
	family     string
	capability byte
	endpoint   string
	capacity   uint16
	publicKey  ed25519.PublicKey
	keyID      [32]byte
	signature  []byte
}

func parseRecord(raw []byte) (nodeRecord, error) {
	if len(raw) == 0 || len(raw) > maximumRecordBytes {
		return nodeRecord{}, errors.New("record framing length is invalid")
	}
	d := newDecoder(raw)
	magic, err := d.bytes(4)
	if err != nil || string(magic) != "ARNR" {
		return nodeRecord{}, errors.New("record magic is invalid")
	}
	version, err := d.byte()
	if err != nil || version != 1 {
		return nodeRecord{}, errors.New("record schema version is invalid")
	}
	var record nodeRecord
	record.raw = append([]byte(nil), raw...)
	network, err := d.bytes(32)
	if err != nil {
		return nodeRecord{}, err
	}
	copy(record.networkID[:], network)
	node, err := d.bytes(32)
	if err != nil {
		return nodeRecord{}, err
	}
	copy(record.nodeID[:], node)
	if record.generation, err = d.uint64(); err != nil || record.generation == 0 {
		return nodeRecord{}, errors.New("record generation is invalid")
	}
	from, err := d.int64()
	if err != nil {
		return nodeRecord{}, err
	}
	until, err := d.int64()
	if err != nil || until <= from {
		return nodeRecord{}, errors.New("record validity interval is invalid")
	}
	record.notBefore, record.notAfter = time.Unix(from, 0).UTC(), time.Unix(until, 0).UTC()
	if record.family, err = d.text(32); err != nil {
		return nodeRecord{}, err
	}
	if record.capability, err = d.byte(); err != nil {
		return nodeRecord{}, err
	}
	if record.endpoint, err = d.text(96); err != nil {
		return nodeRecord{}, err
	}
	if record.capacity, err = d.uint16(); err != nil {
		return nodeRecord{}, err
	}
	public, err := d.bytes(ed25519.PublicKeySize)
	if err != nil {
		return nodeRecord{}, err
	}
	record.publicKey = append(ed25519.PublicKey(nil), public...)
	record.keyID = sha256.Sum256(public)
	record.signature, err = d.bytes(ed25519.SignatureSize)
	if err != nil {
		return nodeRecord{}, err
	}
	if !d.done() {
		return nodeRecord{}, errors.New("record contains trailing bytes")
	}
	return record, nil
}

func (record nodeRecord) signatureValid() bool {
	unsignedLength := len(record.raw) - ed25519.SignatureSize
	return ed25519.Verify(record.publicKey, record.raw[:unsignedLength], record.signature)
}
