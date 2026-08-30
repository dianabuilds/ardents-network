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

func openTestRootIssuer(t *testing.T, root string, network, nodeID [32]byte, identity ed25519.PrivateKey,
	initiatorID, initiatorPublic [32]byte, notAfter time.Time, budget uint16, clock func() time.Time,
	current func(Profile, [32]byte) (StateDuty, bool),
) *Issuer {
	t.Helper()
	receipt, err := InitializeIssuerRoot(IssuerRootConfig{Root: root, NetworkID: network, NodeID: nodeID,
		IdentityKey: identity, InitiatorNodeID: initiatorID, InitiatorPublicKey: initiatorPublic,
		AssignmentNotAfter: notAfter, Budget: budget, Clock: clock})
	if err != nil {
		t.Fatal(err)
	}
	profile, err := DecodeProfile(receipt.Profile)
	if err != nil {
		t.Fatal(err)
	}
	issuer, err := OpenIssuerFromRoot(RootIssuerConfig{Root: root, NetworkID: network, NodeID: nodeID,
		IdentityKey: identity, CurrentDuty: func() (StateDuty, bool) { return current(profile, receipt.ProfileDigest) }, Clock: clock})
	if err != nil {
		t.Fatal(err)
	}
	return issuer
}

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

func TestOpenIssuerFromRootBindsOnlyItsExactFirstStateDuty(t *testing.T) {
	now := time.Unix(2_000_700_000, 0).UTC()
	nodePublic, nodePrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	initiatorPublic, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	config := IssuerRootConfig{Root: filepath.Join(t.TempDir(), "transit-issuer"), NetworkID: credentialID(74), NodeID: credentialID(75),
		IdentityKey: nodePrivate, InitiatorNodeID: credentialID(76), InitiatorPublicKey: publicIdentifier(initiatorPublic),
		AssignmentNotAfter: now.Add(time.Hour), Budget: 2, Clock: func() time.Time { return now }}
	receipt, err := InitializeIssuerRoot(config)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := DecodeProfile(receipt.Profile)
	if err != nil {
		t.Fatal(err)
	}
	duty := StateDuty{NetworkID: config.NetworkID, Digest: credentialID(77), IssuerNodeID: config.NodeID,
		IssuerPublicKey: publicIdentifier(nodePublic), InitiatorNodeID: config.InitiatorNodeID,
		InitiatorPublicKey: config.InitiatorPublicKey, GrantSignerPublicKey: profile.GrantSignerPublicKey,
		ProfileDigest: receipt.ProfileDigest, Epoch: 78, NotAfter: config.AssignmentNotAfter}
	current := duty
	open := func() (*Issuer, error) {
		return OpenIssuerFromRoot(RootIssuerConfig{Root: config.Root, NetworkID: config.NetworkID, NodeID: config.NodeID,
			IdentityKey: nodePrivate, CurrentDuty: func() (StateDuty, bool) { return current, true }, Clock: func() time.Time { return now }})
	}
	issuer, err := open()
	if err != nil {
		t.Fatal(err)
	}
	openedProfile, err := EncodeProfile(issuer.Profile())
	if err != nil || !bytes.Equal(openedProfile, receipt.Profile) {
		t.Fatalf("root-backed issuer profile changed: same=%t, err=%v", bytes.Equal(openedProfile, receipt.Profile), err)
	}
	if err := issuer.Close(); err != nil {
		t.Fatal(err)
	}
	issuer, err = open()
	if err != nil {
		t.Fatal(err)
	}
	if err := issuer.Close(); err != nil {
		t.Fatal(err)
	}

	current = duty
	current.Digest = credentialID(79)
	current.Epoch++
	if _, err := open(); err == nil {
		t.Fatal("bound issuer root started under a State successor")
	}
	current = duty
	current.InitiatorPublicKey = credentialID(80)
	if _, err := open(); err == nil {
		t.Fatal("bound issuer root accepted a substituted State Initiator")
	}
	current = duty
	current.ProfileDigest = credentialID(81)
	if _, err := open(); err == nil {
		t.Fatal("bound issuer root accepted a different State-authenticated profile")
	}
}
