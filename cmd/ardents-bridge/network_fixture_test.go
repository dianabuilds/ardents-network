package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

type commandNetwork struct {
	root            string
	snapshot        state.Snapshot
	authorityPublic ed25519.PublicKey
	nodePrivate     ed25519.PrivateKey
	domainProof     []byte
}

func prepareCommandNetwork(t *testing.T, directory string, now time.Time) commandNetwork {
	t.Helper()
	networkID := sha256.Sum256([]byte("stage-5-command-network"))
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0xa9}, ed25519.SeedSize))
	node := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x11}, ed25519.SeedSize))
	record := commandRecord(networkID, node, now)
	epoch, digest := commandEpoch(networkID, authority, record, now)
	material := commandMaterialization(digest, record)
	root := filepath.Join(directory, "network-state")
	public := authority.Public().(ed25519.PublicKey)
	authorityID := sha256.Sum256(public)
	owner, err := state.Open(state.Config{
		Root: root, NetworkID: networkID, Authorities: map[[32]byte]ed25519.PublicKey{authorityID: public},
		Threshold: 1, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, acceptErr := owner.Accept(t.Context(), epoch, [][]byte{record}, [][]byte{material})
	closeErr := owner.Close()
	if acceptErr != nil {
		t.Fatal(acceptErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	return commandNetwork{root: root, snapshot: snapshot, authorityPublic: public, nodePrivate: node, domainProof: material}
}

func commandRecord(network [32]byte, private ed25519.PrivateKey, now time.Time) []byte {
	var raw bytes.Buffer
	raw.WriteString("ARNR")
	raw.WriteByte(1)
	raw.Write(network[:])
	nodeID := sha256.Sum256([]byte("stage-5-command-node"))
	raw.Write(nodeID[:])
	writeCommandU64(&raw, 1)
	writeCommandI64(&raw, now.Add(-time.Hour).Unix())
	writeCommandI64(&raw, now.Add(time.Hour).Unix())
	writeCommandText(&raw, "stage-5-family")
	raw.WriteByte(1)
	writeCommandText(&raw, "127.0.0.1:4101")
	writeCommandU16(&raw, 4)
	raw.Write(private.Public().(ed25519.PublicKey))
	raw.Write(ed25519.Sign(private, raw.Bytes()))
	return raw.Bytes()
}

func commandEpoch(network [32]byte, authority ed25519.PrivateKey, record []byte, now time.Time) ([]byte, [32]byte) {
	var raw bytes.Buffer
	raw.WriteString("AREP")
	raw.WriteByte(1)
	raw.Write(network[:])
	writeCommandU64(&raw, 1)
	raw.Write(make([]byte, 32))
	writeCommandI64(&raw, now.Add(-time.Minute).Unix())
	writeCommandI64(&raw, now.Add(time.Hour).Unix())
	writeCommandU32(&raw, 1)
	writeCommandText(&raw, "h3-role-probe-v1")
	inputRoot := commandMerkleLeaf(record)
	viewRoot := inputRoot
	raw.Write(inputRoot[:])
	raw.Write(viewRoot[:])
	writeCommandU32(&raw, 1)
	emptyRejected := sha256.Sum256([]byte{0x12})
	raw.Write(emptyRejected[:])
	writeCommandU32(&raw, 0)
	seed := sha256.Sum256([]byte("stage-5-command-assignment"))
	raw.Write(seed[:])
	writeCommandText(&raw, "ardents-h3-role-domain-v1")
	writeCommandU32(&raw, 1)
	writeCommandU32(&raw, 4)
	writeCommandU16(&raw, 1)
	writeCommandU16(&raw, 1)
	writeCommandU32(&raw, 4)
	raw.WriteByte(1)
	writeCommandText(&raw, "initiator")
	writeCommandU16(&raw, 1)
	writeCommandU32(&raw, 4)
	digest := sha256.Sum256(raw.Bytes())
	public := authority.Public().(ed25519.PublicKey)
	raw.WriteByte(1)
	authorityID := sha256.Sum256(public)
	raw.Write(authorityID[:])
	raw.Write(ed25519.Sign(authority, digest[:]))
	return raw.Bytes(), digest
}

func commandMaterialization(digest [32]byte, record []byte) []byte {
	var raw bytes.Buffer
	raw.Write(digest[:])
	writeCommandU32(&raw, 0)
	writeCommandU32(&raw, uint32(len(record)))
	raw.Write(record)
	writeCommandU16(&raw, 0)
	return raw.Bytes()
}

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
