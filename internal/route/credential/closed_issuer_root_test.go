package credential

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

// closedIssuerFixtureRoot makes the private-root precondition explicit rather
// than depending on a platform's temporary-directory defaults.
func closedIssuerFixtureRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "closed-issuer")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestInitializeClosedIssuerRootPublishesExactSPKIInventory(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	config := ClosedIssuerRootConfig{Root: closedIssuerFixtureRoot(t), NetworkID: credentialID(1), NodeID: credentialID(2), IdentityKey: private,
		NotBefore: now, NotAfter: now.Add(time.Hour), Clock: func() time.Time { return now.Add(time.Minute) }}
	first, err := InitializeClosedIssuerRoot(config)
	if err != nil || len(first.Profile) == 0 {
		t.Fatalf("initialize closed issuer root = %d bytes, %v", len(first.Profile), err)
	}
	second, err := InitializeClosedIssuerRoot(config)
	if err != nil || !bytes.Equal(first.Profile, second.Profile) || first.ProfileDigest != second.ProfileDigest {
		t.Fatalf("reopen closed issuer root changed public profile: %v", err)
	}
	profile, err := DecodeClosedIssuerProfile(first.Profile, public)
	if err != nil || profile.NetworkID != config.NetworkID || profile.NodeID != config.NodeID || len(profile.Keys) != 3 {
		t.Fatalf("decode closed issuer profile = %+v, %v", profile, err)
	}
	seen := map[[32]byte]bool{}
	for index, key := range profile.Keys {
		if key.WindowStart != now || key.Class != byte(index+1) || len(key.SPKI) != 346 || !state.ValidateClosedTokenSPKI(key.SPKI) ||
			key.KeyID != credentialID(0) && seen[key.KeyID] {
			t.Fatalf("closed issuer key %d = %+v", index, key)
		}
		seen[key.KeyID] = true
	}
	_, replacement, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	config.IdentityKey = replacement
	if _, err := InitializeClosedIssuerRoot(config); err == nil {
		t.Fatal("closed issuer root accepted replacement signing identity")
	}
}

func TestDecodeClosedIssuerProfileRejectsReorderedKey(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	config := ClosedIssuerRootConfig{Root: closedIssuerFixtureRoot(t), NetworkID: credentialID(11), NodeID: credentialID(12), IdentityKey: private,
		NotBefore: now, NotAfter: now.Add(time.Hour), Clock: func() time.Time { return now.Add(time.Minute) }}
	receipt, err := InitializeClosedIssuerRoot(config)
	if err != nil {
		t.Fatal(err)
	}
	body := append([]byte(nil), receipt.Profile[:len(receipt.Profile)-ed25519.SignatureSize]...)
	const profileHeader = 8 + 32 + 32 + 8 + 8 + 2
	const keyEntry = 8 + 1 + 2 + 346
	first, second := append([]byte(nil), body[profileHeader:profileHeader+keyEntry]...), append([]byte(nil), body[profileHeader+keyEntry:profileHeader+2*keyEntry]...)
	copy(body[profileHeader:profileHeader+keyEntry], second)
	copy(body[profileHeader+keyEntry:profileHeader+2*keyEntry], first)
	changed := append(body, ed25519.Sign(private, closedIssuerTranscript(body))...)
	if _, err := DecodeClosedIssuerProfile(changed, public); err == nil {
		t.Fatal("decoded reordered closed issuer keys")
	}
}

func TestDecodeClosedIssuerProfileRejectsSignedEmptyInvalidInterval(t *testing.T) {
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{19}, ed25519.SeedSize))
	body := make([]byte, 8+32+32+8+8+2)
	copy(body, closedIssuerProfileMagic)
	body[8], body[40] = 1, 2
	// An empty interval makes the expected key count zero; it must not make
	// an equally empty inventory valid merely because the signature verifies.
	binary.BigEndian.PutUint64(body[72:80], 1_800_000_000)
	binary.BigEndian.PutUint64(body[80:88], 1_800_000_000)
	raw := append(body, ed25519.Sign(private, closedIssuerTranscript(body))...)
	if _, err := DecodeClosedIssuerProfile(raw, private.Public().(ed25519.PublicKey)); err == nil {
		t.Fatal("accepted a signed empty inventory with an invalid interval")
	}
}
