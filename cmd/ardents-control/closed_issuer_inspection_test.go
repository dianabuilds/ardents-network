package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

func TestClosedIssuerInspectionRejectsUnverifiedInventoryWithoutOutput(t *testing.T) {
	now := time.Unix(2_000_401_600, 0).UTC().Truncate(time.Hour)
	network, node := [32]byte{1}, [32]byte{2}
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{4}, ed25519.SeedSize))
	receipt, err := credential.InitializeClosedIssuerRoot(credential.ClosedIssuerRootConfig{Root: filepath.Join(t.TempDir(), "issuer"),
		NetworkID: network, NodeID: node, IdentityKey: private, NotBefore: now, NotAfter: now.Add(time.Hour), Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	type issuerExport struct {
		Schema  string `json:"schema"`
		Profile []byte `json:"profile"`
		Digest  string `json:"profile_sha256"`
	}
	valid := issuerExport{"ardents-closed-issuer-profile-v1", receipt.Profile, hex.EncodeToString(receipt.ProfileDigest[:])}
	encode := func(value issuerExport) []byte {
		t.Helper()
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	validRaw := encode(valid)
	badSignature := valid
	badSignature.Profile = append([]byte(nil), receipt.Profile...)
	badSignature.Profile[len(badSignature.Profile)-1] ^= 1
	changedDigest := sha256.Sum256(badSignature.Profile)
	badSignature.Digest = hex.EncodeToString(changedDigest[:])
	badDigest := valid
	badDigest.Digest = hex.EncodeToString(make([]byte, 32))
	badSchema := valid
	badSchema.Schema = "ardents-transit-issuer-profile-v1"
	for _, test := range []struct {
		name        string
		raw         []byte
		flag, value string
	}{
		{name: "wrong-signature-with-correct-digest", raw: encode(badSignature)},
		{name: "wrong-digest", raw: encode(badDigest)},
		{name: "wrong-schema", raw: encode(badSchema)},
		{name: "trailing-json", raw: append(append([]byte(nil), validRaw...), []byte("{}")...)},
		{name: "unknown-field", raw: append([]byte("{\"extra\":true,"), validRaw[1:]...)},
		{name: "oversized", raw: bytes.Repeat([]byte(" "), maximumClosedProfilePlanBytes+1)},
		{name: "wrong-network", raw: validRaw, flag: "--network", value: hex.EncodeToString(bytes.Repeat([]byte{9}, 32))},
		{name: "wrong-node", raw: validRaw, flag: "--node", value: hex.EncodeToString(bytes.Repeat([]byte{9}, 32))},
		{name: "wrong-key", raw: validRaw, flag: "--node-key", value: hex.EncodeToString(bytes.Repeat([]byte{9}, 32))},
		{name: "missing-key", raw: validRaw, flag: "--node-key", value: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "public.json")
			if err := os.WriteFile(path, test.raw, 0o600); err != nil {
				t.Fatal(err)
			}
			arguments := []string{"inspect-closed-issuer-profile", "--profile", path, "--network", hex.EncodeToString(network[:]),
				"--node", hex.EncodeToString(node[:]), "--node-key", hex.EncodeToString(private.Public().(ed25519.PublicKey))}
			for index := 1; index < len(arguments); index += 2 {
				if arguments[index] == test.flag {
					arguments[index+1] = test.value
				}
			}
			var output bytes.Buffer
			if err := run(arguments, &output); err == nil || output.Len() != 0 {
				t.Fatalf("unverified inventory produced output: %q / %v", output.String(), err)
			}
			retained, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(retained, test.raw) {
				t.Fatalf("inspection changed public input: %v", err)
			}
		})
	}
}
