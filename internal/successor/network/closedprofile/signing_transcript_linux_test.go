//go:build linux

package closedprofile

import (
	"bytes"
	"crypto/ed25519"
	"testing"
	"time"
)

// The public producer Interface follows its genuine Linux-only offline signing
// consumer. Common verification and the canonical vector remain portable.
func TestSigningTranscriptIndependentVector(t *testing.T) {
	now := time.Unix(1_800_003_600, 0).UTC()
	input := Input{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2},
		EpochDigest: [32]byte{3}, IssuerNodeID: [32]byte{4}, IssuanceAuthorityKey: [32]byte{5},
		Epoch: 7, NotBefore: now, NotAfter: now.Add(time.Hour),
		Nodes: []NodeInput{{NodeID: [32]byte{4}, RecordDigest: [32]byte{6},
			RoleDomain: 2, Subrole: 6, DutyGeneration: 1}},
		TokenKeys: []TokenKeyInput{{WindowStart: now, Class: 1, SPKI: fixedTokenSPKI(t)}}}
	signer := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{9}, ed25519.SeedSize))
	body, err := Prepare(input)
	if err != nil {
		t.Fatal(err)
	}
	raw := append(bytes.Clone(body), ed25519.Sign(signer, append([]byte("ardents-closed-profile-v3\x00"), body...))...)
	context := Context{StateGeneration: input.StateGeneration, NetworkID: input.NetworkID, EpochDigest: input.EpochDigest, Epoch: input.Epoch, Authority: signer.Public().(ed25519.PublicKey), Now: now}
	transcript, err := SigningTranscript(body, context)
	if err != nil || !bytes.Equal(transcript, append([]byte("ardents-closed-profile-v3\x00"), body...)) || !bytes.Equal(ed25519.Sign(signer, transcript), raw[len(body):]) {
		t.Fatal("public transcript differs from independent fixed vector", err)
	}
	for _, fault := range []string{"network", "generation", "digest", "epoch", "authority", "expiry", "body", "signed input", "oversize"} {
		t.Run("transcript refusal "+fault, func(t *testing.T) {
			c, b := context, bytes.Clone(body)
			switch fault {
			case "network":
				c.NetworkID[0] ^= 1
			case "generation":
				c.StateGeneration[0] ^= 1
			case "digest":
				c.EpochDigest[0] ^= 1
			case "epoch":
				c.Epoch++
			case "authority":
				c.Authority = nil
			case "expiry":
				c.Now = input.NotAfter
			case "body":
				b[0] ^= 1
			case "signed input":
				b = raw
			case "oversize":
				b = make([]byte, MaxSize)
			}
			if message, e := SigningTranscript(b, c); e == nil || message != nil {
				t.Fatal("invalid transcript admitted")
			}
		})
	}
	transcript[0] ^= 1
	if body[0] != 'A' {
		t.Fatal("transcript aliases prepared body")
	}
}
