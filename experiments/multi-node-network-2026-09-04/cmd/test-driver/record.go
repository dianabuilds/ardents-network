//go:build ignore

package main

import (
	"bytes"
	"crypto/ed25519"
	"errors"
)

// Record is one closed-alpha signed Node Record. It is the local copy of
// the type the maintained tests under tests/e2e/network-source/ produce, kept
// inside the experiment tree so a non-test Go binary can build fixtures.
type Record struct {
	Raw      []byte
	NodeID   [32]byte
	Family   string
	Capacity uint16
}

// BuildRecord returns one canonical signed Node Record. It is intentionally
// byte-compatible with the maintained BuildRecord so a downstream consumer
// (the source-server accept-offline path) parses the produced bytes exactly
// the same way as a maintained-fixture byte.
func BuildRecord(network [32]byte, nodeID [32]byte, generation uint64, validFrom, validUntil int64,
	family, endpoint string, capability byte, capacity uint16, private ed25519.PrivateKey) (Record, error) {
	if generation == 0 || validFrom == 0 || validUntil <= validFrom || family == "" || len(family) > 32 ||
		endpoint == "" || len(endpoint) > 96 || len(private) != ed25519.PrivateKeySize {
		return Record{}, errors.New("pilot: record specification is invalid")
	}
	public := private.Public().(ed25519.PublicKey)
	buffer := new(bytes.Buffer)
	buffer.WriteString("ARNR")
	buffer.WriteByte(1)
	buffer.Write(network[:])
	buffer.Write(nodeID[:])
	if err := writeU64(buffer, generation); err != nil {
		return Record{}, err
	}
	if err := writeI64(buffer, validFrom); err != nil {
		return Record{}, err
	}
	if err := writeI64(buffer, validUntil); err != nil {
		return Record{}, err
	}
	if err := writeText(buffer, family); err != nil {
		return Record{}, err
	}
	buffer.WriteByte(capability)
	if err := writeText(buffer, endpoint); err != nil {
		return Record{}, err
	}
	if err := writeU16(buffer, capacity); err != nil {
		return Record{}, err
	}
	buffer.Write(public)
	buffer.Write(ed25519.Sign(private, buffer.Bytes()))
	return Record{Raw: buffer.Bytes(), NodeID: nodeID, Family: family, Capacity: capacity}, nil
}
