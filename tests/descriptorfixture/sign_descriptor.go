package descriptorfixture

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"time"
)

// Input selects deterministic test keys and the complete public fixture binding.
type Input struct {
	AuthorityMarker, InstanceMarker         byte
	Network, Profile                        [32]byte
	Introduction, Slot, RecipientKey        [32]byte
	ValidFrom, PublicationEnd, RecipientEnd time.Time
	Capabilities                            uint32
}

// Signed constructs the independent test transcript used by actual consumers.
func Signed(input Input) ([]byte, [32]byte) {
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{input.AuthorityMarker}, ed25519.SeedSize))
	instance := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{input.InstanceMarker}, ed25519.SeedSize))
	defer clear(authority)
	defer clear(instance)
	public := authority.Public().(ed25519.PublicKey)
	target := sha256.Sum256(append([]byte("ardents-service-target-v3\x00"), public...))
	network, profile := input.Network, input.Profile
	credential := []byte{0, 3}
	credential = append(credential, public...)
	credential = append(credential, target[:]...)
	credential = append(credential, instance.Public().(ed25519.PublicKey)...)
	for _, number := range []uint64{7, uint64(input.ValidFrom.Unix()), uint64(input.PublicationEnd.Unix())} {
		credential = binary.BigEndian.AppendUint64(credential, number)
	}
	credential = append(credential, network[:]...)
	credential = binary.BigEndian.AppendUint32(credential, input.Capabilities)
	credential = append(credential, ed25519.Sign(authority, credential)...)
	record := append([]byte("ardents-service-publication-v3\x00"), credential...)
	record = append(record, make([]byte, 32)...)
	commitment := sha256.Sum256(record)
	record = append(record, ed25519.Sign(instance, commitment[:])...)
	digest := sha256.Sum256(record)
	recipient := binary.BigEndian.AppendUint64(nil, 9)
	for _, key := range [][32]byte{input.Introduction, input.Slot, input.RecipientKey} {
		recipient = append(recipient, key[:]...)
	}
	for _, seconds := range []uint64{uint64(input.ValidFrom.Unix()), uint64(input.RecipientEnd.Unix())} {
		recipient = binary.BigEndian.AppendUint64(recipient, seconds)
	}
	transcript := []byte("ardents-private-reachability-v3\x00")
	transcript = append(transcript, network[:]...)
	transcript = append(transcript, profile[:]...)
	transcript = append(transcript, digest[:]...)
	transcript = append(transcript, recipient...)
	body := []byte{0, 3}
	body = append(body, network[:]...)
	body = append(body, target[:]...)
	body = append(body, public...)
	body = append(body, digest[:]...)
	body = append(body, profile[:]...)
	body = append(body, recipient...)
	body = binary.BigEndian.AppendUint16(body, uint16(len(record)))
	body = append(body, record...)
	return append(body, ed25519.Sign(instance, transcript)...), target
}

// Fixed produces the portable owner tests' fixture at a selected signed time.
func Fixed(capabilities uint32, authority byte) ([]byte, [32]byte, [32]byte, [32]byte, time.Time) {
	at := time.Unix(1900000000, 0).UTC()
	input := Input{AuthorityMarker: authority, InstanceMarker: 0x22,
		Network: [32]byte{0x33}, Profile: [32]byte{0x44},
		Introduction: [32]byte{0x55}, Slot: [32]byte{0x66}, RecipientKey: [32]byte{0x77},
		ValidFrom: at, PublicationEnd: at.Add(time.Hour), RecipientEnd: at.Add(10 * time.Minute), Capabilities: capabilities}
	body, target := Signed(input)
	return body, target, input.Network, input.Profile, at
}

// ResignFixed applies a controlled mutation to a default-authority fixture.
func ResignFixed(raw []byte, change func([]byte)) []byte {
	body := bytes.Clone(raw)
	change(body)
	start := 284
	end := len(body) - 64
	credentialStart := start + len("ardents-service-publication-v3\x00")
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x11}, 32))
	instance := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x22}, 32))
	copy(body[credentialStart+158:credentialStart+222], ed25519.Sign(authority, body[credentialStart:credentialStart+158]))
	commitment := sha256.Sum256(body[start : end-64])
	copy(body[end-64:end], ed25519.Sign(instance, commitment[:]))
	digest := sha256.Sum256(body[start:end])
	copy(body[98:130], digest[:])
	transcript := []byte("ardents-private-reachability-v3\x00")
	transcript = append(transcript, body[2:34]...)
	transcript = append(transcript, body[130:162]...)
	transcript = append(transcript, body[98:130]...)
	transcript = append(transcript, body[162:282]...)
	copy(body[end:], ed25519.Sign(instance, transcript))
	return body
}
