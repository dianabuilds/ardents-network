package credential

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"path/filepath"
	"testing"
	"time"
)

func TestInitializeIssuerRootPublishesOneStableStateBindableProfile(t *testing.T) {
	now := time.Unix(2_000_700_000, 0).UTC()
	nodePublic, nodePrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	initiatorPublic, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "transit-issuer")
	config := IssuerRootConfig{
		Root:               root,
		NetworkID:          credentialID(70),
		NodeID:             credentialID(71),
		IdentityKey:        nodePrivate,
		InitiatorNodeID:    credentialID(72),
		InitiatorPublicKey: publicIdentifier(initiatorPublic),
		AssignmentNotAfter: now.Add(time.Hour),
		Budget:             4,
		Clock:              func() time.Time { return now },
	}
	first, err := InitializeIssuerRoot(config)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := DecodeProfile(first.Profile)
	if err != nil {
		t.Fatal(err)
	}
	if profile.Version != 3 || profile.NetworkID != config.NetworkID || profile.NodeID != config.NodeID ||
		profile.InitiatorNodeID != config.InitiatorNodeID || profile.InitiatorPublicKey != config.InitiatorPublicKey ||
		profile.GrantSignerPublicKey == [32]byte{} || profile.GrantSignerPublicKey == publicIdentifier(nodePublic) ||
		profile.GrantSignerID != sha256.Sum256(profile.GrantSignerPublicKey[:]) || first.ProfileDigest != sha256.Sum256(first.Profile) {
		t.Fatalf("issuer root public profile = %+v, digest=%x", profile, first.ProfileDigest)
	}
	if err := VerifyProfile(profile, config.NetworkID, config.NodeID, publicIdentifier(nodePublic), now, now.Add(15*time.Second)); err != nil {
		t.Fatal(err)
	}

	reopened, err := InitializeIssuerRoot(config)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(reopened.Profile, first.Profile) || reopened.ProfileDigest != first.ProfileDigest {
		t.Fatal("issuer root reopen changed its public profile")
	}

	changed := config
	changed.InitiatorNodeID = credentialID(73)
	if _, err := InitializeIssuerRoot(changed); err == nil {
		t.Fatal("existing issuer root accepted a different Initiator binding")
	}
	again, err := InitializeIssuerRoot(config)
	if err != nil || !bytes.Equal(again.Profile, first.Profile) {
		t.Fatalf("rejected replacement changed issuer root: same=%t, err=%v", bytes.Equal(again.Profile, first.Profile), err)
	}
}
