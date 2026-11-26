package processfixture

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
)

// SignedDescriptor builds independent signed bytes for descriptor verification tests.
func SignedDescriptor(t testing.TB, facts admission.AuthorityFacts, caps uint32, introduction, profile [32]byte) ([]byte, [32]byte) {
	t.Helper()
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x71}, 32))
	instance := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x72}, 32))
	defer clear(authority)
	defer clear(instance)
	public := authority.Public().(ed25519.PublicKey)
	target := sha256.Sum256(append([]byte("ardents-service-target-v3\x00"), public...))
	at := time.Now().UTC().Truncate(time.Second)
	end := at.Add(60 * time.Second)
	if facts.NotAfter.Before(end) {
		end = facts.NotAfter
	}
	credential := []byte{0, 3}
	credential = append(credential, public...)
	credential = append(credential, target[:]...)
	credential = append(credential, instance.Public().(ed25519.PublicKey)...)
	for _, value := range []uint64{7, uint64(at.Unix()), uint64(end.Unix())} {
		credential = binary.BigEndian.AppendUint64(credential, value)
	}
	credential = append(credential, facts.NetworkID[:]...)
	credential = binary.BigEndian.AppendUint32(credential, caps)
	credential = append(credential, ed25519.Sign(authority, credential)...)
	publication := append([]byte("ardents-service-publication-v3\x00"), credential...)
	publication = append(publication, make([]byte, 32)...)
	commitment := sha256.Sum256(publication)
	publication = append(publication, ed25519.Sign(instance, commitment[:])...)
	digest := sha256.Sum256(publication)
	recipient := binary.BigEndian.AppendUint64(nil, 9)
	recipient = append(recipient, introduction[:]...)
	for _, key := range [][32]byte{{0x73}, {0x74}} {
		recipient = append(recipient, key[:]...)
	}
	for _, seconds := range []uint64{uint64(at.Unix()), uint64(end.Unix())} {
		recipient = binary.BigEndian.AppendUint64(recipient, seconds)
	}
	transcript := []byte("ardents-private-reachability-v3\x00")
	transcript = append(transcript, facts.NetworkID[:]...)
	transcript = append(transcript, profile[:]...)
	transcript = append(transcript, digest[:]...)
	transcript = append(transcript, recipient...)
	body := []byte{0, 3}
	body = append(body, facts.NetworkID[:]...)
	body = append(body, target[:]...)
	body = append(body, public...)
	body = append(body, digest[:]...)
	body = append(body, profile[:]...)
	body = append(body, recipient...)
	body = binary.BigEndian.AppendUint16(body, uint16(len(publication)))
	body = append(body, publication...)
	return append(body, ed25519.Sign(instance, transcript)...), target
}
