package access

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func FuzzCanonicalArtifactParsers(f *testing.F) {
	type vector struct {
		Kind string `json:"kind"`
		Wire string `json:"wire_base64"`
	}
	var fixture struct {
		Now     string   `json:"now"`
		Vectors []vector `json:"vectors"`
	}
	raw, err := os.ReadFile(artifactVectorFixturePath())
	if err != nil || json.Unmarshal(raw, &fixture) != nil {
		f.Fatal("cannot load artifact vectors")
	}
	kinds := map[string]byte{"key_credential": 0, "access_grant": 1, "delegation": 2, "device_revocation": 3, "access_grant_revocation": 4, "delegation_revocation": 5}
	for _, item := range fixture.Vectors {
		wire, decodeErr := base64.RawStdEncoding.DecodeString(item.Wire)
		if decodeErr != nil {
			f.Fatal(decodeErr)
		}
		f.Add(kinds[item.Kind], wire)
	}
	now, err := time.Parse(time.RFC3339, fixture.Now)
	if err != nil {
		f.Fatal(err)
	}
	nodeKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x33}, ed25519.SeedSize))
	nodePublic := nodeKey.Public().(ed25519.PublicKey)
	f.Fuzz(func(t *testing.T, kind byte, wire []byte) {
		var artifact *Artifact
		switch kind % 6 {
		case 0:
			artifact, _ = ParseAndVerifyKeyCredential(wire, now)
		case 1:
			artifact, _ = ParseAndVerifyAccessGrant(wire, nodePublic, now)
		case 2:
			artifact, _ = ParseAndVerifyDelegation(wire, now)
		case 3:
			artifact, _ = ParseAndVerifyDeviceRevocation(wire, nodePublic, now)
		case 4:
			_, _ = ParseAndVerifyAccessGrantRevocation(wire, nodePublic, now, nil)
		case 5:
			artifact, _ = ParseAndVerifyDelegationRevocation(wire, now)
		}
		if artifact != nil {
			canonical, marshalErr := artifact.MarshalBinary()
			if marshalErr != nil || !bytes.Equal(canonical, wire) {
				t.Fatal("accepted noncanonical wire")
			}
		}
	})
}
