package state

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"sort"
	"time"
)

var alphaGenesisDomains = []string{"initiator", "introduction", "rendezvous", "responder"}

func encodeAlphaGenesis(networkID, assignmentSeed [32]byte, notBefore, notAfter time.Time, private ed25519.PrivateKey) ([]byte, [32]byte) {
	buffer := new(bytes.Buffer)
	buffer.WriteString("AREP")
	buffer.WriteByte(1)
	buffer.Write(networkID[:])
	writeAlphaU64(buffer, 1)
	buffer.Write(make([]byte, 32))
	writeAlphaI64(buffer, notBefore.Unix())
	writeAlphaI64(buffer, notAfter.Unix())
	writeAlphaU32(buffer, 0)
	writeAlphaText(buffer, interactiveRouteProfile)
	inputRoot := epochCommitmentRoot(nil, emptyInputTag)
	viewRoot := epochCommitmentRoot(nil, emptyViewTag)
	rejectedRoot := epochHashedCommitmentRoot(nil, emptyRejectionTag)
	buffer.Write(inputRoot[:])
	buffer.Write(viewRoot[:])
	writeAlphaU32(buffer, 0)
	buffer.Write(rejectedRoot[:])
	writeAlphaU32(buffer, 0)
	buffer.Write(assignmentSeed[:])
	writeAlphaText(buffer, assignmentV1)
	writeAlphaU32(buffer, 0)
	writeAlphaU32(buffer, 0)
	writeAlphaU16(buffer, 0)
	writeAlphaU16(buffer, 0)
	writeAlphaU32(buffer, 0)
	domains := append([]string(nil), alphaGenesisDomains...)
	sort.Strings(domains)
	buffer.WriteByte(byte(len(domains)))
	for _, domain := range domains {
		writeAlphaText(buffer, domain)
		writeAlphaU16(buffer, 0)
		writeAlphaU32(buffer, 0)
	}
	unsigned := buffer.Bytes()
	digest := sha256.Sum256(unsigned)
	public := private.Public().(ed25519.PublicKey)
	authorityID := sha256.Sum256(public)
	buffer.WriteByte(1)
	buffer.Write(authorityID[:])
	buffer.Write(ed25519.Sign(private, digest[:]))
	return buffer.Bytes(), digest
}

func writeAlphaText(buffer *bytes.Buffer, value string) {
	buffer.WriteByte(byte(len(value)))
	buffer.WriteString(value)
}

func writeAlphaU16(buffer *bytes.Buffer, value uint16) {
	_ = binary.Write(buffer, binary.BigEndian, value)
}
func writeAlphaU32(buffer *bytes.Buffer, value uint32) {
	_ = binary.Write(buffer, binary.BigEndian, value)
}
func writeAlphaU64(buffer *bytes.Buffer, value uint64) {
	_ = binary.Write(buffer, binary.BigEndian, value)
}
func writeAlphaI64(buffer *bytes.Buffer, value int64) { writeAlphaU64(buffer, uint64(value)) }
