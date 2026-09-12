//go:build linux

package reachability

import (
	"crypto/ed25519"
	"encoding/binary"
	"errors"
)

func encodePrivateDescriptorBody(value Descriptor) ([]byte, error) {
	if value.Version != privateDescriptorVersion || len(value.Publication) == 0 ||
		privateDescriptorHeader+len(value.Publication)+ed25519.SignatureSize > MaximumPrivateDescriptorSize {
		return nil, errors.New("private reachability proof exceeds bound")
	}
	body := make([]byte, 0, privateDescriptorHeader+len(value.Publication))
	body = binary.BigEndian.AppendUint16(body, privateDescriptorVersion)
	for _, field := range [][32]byte{value.NetworkID, value.Target, value.AuthorityPublic, value.PublicationDigest, value.ProfileDigest} {
		body = append(body, field[:]...)
	}
	body = appendPrivateRecipient(body, value.Private)
	body = binary.BigEndian.AppendUint16(body, uint16(len(value.Publication)))
	return append(body, value.Publication...), nil
}
