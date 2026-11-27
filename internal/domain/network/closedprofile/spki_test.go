package closedprofile

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"testing"
	"time"
)

func TestSignedProfileRejectsNoncanonicalTokenKeys(t *testing.T) {
	now := time.Unix(1_800_003_600, 0).UTC()
	signer := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{9}, ed25519.SeedSize))
	authority := signer.Public().(ed25519.PublicKey)
	network, generation, digest := [32]byte{1}, [32]byte{2}, [32]byte{3}
	nodes := []Node{{NodeID: [32]byte{4}, RecordDigest: [32]byte{5}, RoleDomain: 2, Subrole: 6, DutyGeneration: 1}}
	raw := testClosedProfile(t, signer, network, generation, digest, now, nodes)
	if _, err := verifyFixture(raw, generation, network, digest, 9, authority, now); err != nil {
		t.Fatalf("canonical signed profile refused: %v", err)
	}
	body := raw[:len(raw)-ed25519.SignatureSize]
	// The independent fixture ends with one length-prefixed 346-byte key.
	spki := body[len(body)-346:]
	prefix := body[:len(body)-346-2]
	changed := bytes.Clone(spki)
	changed[len(changed)-1] ^= 1
	for name, key := range map[string][]byte{
		"truncated": spki[:len(spki)-1],
		"extended":  append(bytes.Clone(spki), 0),
		"same size": changed,
	} {
		t.Run(name, func(t *testing.T) {
			candidate := binary.BigEndian.AppendUint16(bytes.Clone(prefix), uint16(len(key)))
			candidate = append(candidate, key...)
			signature := ed25519.Sign(signer, append([]byte("ardents-closed-profile-v3\x00"), candidate...))
			if _, err := verifyFixture(append(candidate, signature...), generation, network, digest, 9, authority, now); err == nil {
				t.Fatal("accepted signed profile with a noncanonical token key")
			}
		})
	}
}
