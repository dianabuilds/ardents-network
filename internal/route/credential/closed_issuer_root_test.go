package credential

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func TestInitializeClosedIssuerRootPublishesExactSPKIInventory(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	config := ClosedIssuerRootConfig{Root: t.TempDir(), NetworkID: credentialID(1), NodeID: credentialID(2), IdentityKey: private,
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
	config := ClosedIssuerRootConfig{Root: t.TempDir(), NetworkID: credentialID(11), NodeID: credentialID(12), IdentityKey: private,
		NotBefore: now, NotAfter: now.Add(time.Hour), Clock: func() time.Time { return now.Add(time.Minute) }}
	receipt, err := InitializeClosedIssuerRoot(config)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := DecodeClosedIssuerProfile(receipt.Profile, public)
	if err != nil {
		t.Fatal(err)
	}
	profile.Keys[0], profile.Keys[1] = profile.Keys[1], profile.Keys[0]
	changed, err := encodeClosedIssuerProfile(profile, private)
	if err == nil {
		t.Fatal("encoded reordered closed issuer keys")
	}
	if _, err := DecodeClosedIssuerProfile(changed, public); err == nil {
		t.Fatal("decoded reordered closed issuer keys")
	}
}
