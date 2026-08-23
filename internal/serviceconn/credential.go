package serviceconn

import (
	"encoding/binary"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

// Credential is the Authority-signed public delegation owned by publication.
// The alias preserves this temporary adapter's product vocabulary while M9
// replaces the former serviceconn owner.
type Credential = publication.Credential

func validateCredential(value Credential, authority, network [32]byte, at time.Time, capability uint32) error {
	return publication.Validate(value, authority, network, at, capability)
}

func deriveTarget(authority [32]byte) [32]byte { return publication.Target(authority) }

// legacyCredentialBody exists only while this temporary serviceconn adapter
// still characterizes old H3 records. M9 removes it with the native
// connection cutover; it is not a publication writer or validator.
func legacyCredentialBody(value Credential) []byte {
	encoded := make([]byte, 4+1+32+32+32+8+8+8+32+4)
	copy(encoded[:4], "ASCR")
	encoded[4] = 1
	offset := 5
	for _, field := range [][32]byte{value.AuthorityPublic, value.Target, value.InstancePublic} {
		copy(encoded[offset:offset+32], field[:])
		offset += 32
	}
	binary.BigEndian.PutUint64(encoded[offset:offset+8], value.Generation)
	offset += 8
	binary.BigEndian.PutUint64(encoded[offset:offset+8], uint64(value.NotBefore))
	offset += 8
	binary.BigEndian.PutUint64(encoded[offset:offset+8], uint64(value.NotAfter))
	offset += 8
	copy(encoded[offset:offset+32], value.NetworkID[:])
	offset += 32
	binary.BigEndian.PutUint32(encoded[offset:offset+4], value.Capabilities)
	return encoded
}
