package credential

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	marker, err := os.ReadFile(filepath.Join(root, ".ardents-local-roles-v2"))
	if err != nil || string(marker) != "ardents-local-roles-v2\n" {
		t.Fatalf("issuer root v2 marker = %q, %v", marker, err)
	}
	pointer, err := os.ReadFile(filepath.Join(root, "current"))
	if err != nil {
		t.Fatal(err)
	}
	stateRaw, err := os.ReadFile(filepath.Join(root, "state-"+strings.TrimSuffix(string(pointer), "\n")))
	if err != nil {
		t.Fatal(err)
	}
	var state issuerRootState
	if err := json.Unmarshal(stateRaw, &state); err != nil || state.Version != 2 {
		t.Fatalf("issuer root JSON version = %d, %v", state.Version, err)
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

func TestIssuerRootRejectsEveryV1FormWithoutMutation(t *testing.T) {
	now := time.Unix(2_000_700_000, 0).UTC()
	_, identity, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name  string
		state *issuerRootState
	}{
		{name: "marker only"},
		{name: "unbound", state: &issuerRootState{Version: 1, Generation: 1, Duties: []json.RawMessage{}, TransitGrantSpends: []json.RawMessage{}}},
		{name: "bound", state: &issuerRootState{Version: 1, Generation: 1, Duties: []json.RawMessage{}, TransitGrantSpends: []json.RawMessage{},
			TransitGrantIssuer: &issuerRootRecord{StateGeneration: testStateGeneration, ProfileDigest: credentialID(1), Profile: []byte{1},
				NetworkID: credentialID(2), Digest: credentialID(3), IssuerNodeID: credentialID(4), GrantSignerID: credentialID(5), Epoch: 6, NotAfter: now.Add(time.Hour).Unix(), Budget: 1, PrivateMaterial: []byte{1}}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "legacy-root")
			if err := os.Mkdir(root, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, ".ardents-local-roles-v1"), []byte("ardents-local-roles-v1\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if test.state != nil {
				raw, marshalErr := json.Marshal(test.state)
				if marshalErr != nil {
					t.Fatal(marshalErr)
				}
				name := issuerStateDigest(raw)
				if err := os.WriteFile(filepath.Join(root, "state-"+name), raw, 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "current"), []byte(name+"\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "watermark"), []byte(fmt.Sprintf("1 %s\n", name)), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			before := issuerRootBytes(t, root)
			if _, err := InitializeIssuerRoot(IssuerRootConfig{Root: root, NetworkID: credentialID(10), NodeID: credentialID(11),
				IdentityKey: identity, InitiatorNodeID: credentialID(12), InitiatorPublicKey: credentialID(13),
				AssignmentNotAfter: now.Add(time.Hour), Budget: 1, Clock: func() time.Time { return now }}); err == nil {
				t.Fatal("v1 issuer root was accepted")
			}
			if _, statErr := os.Lstat(filepath.Join(root, issuerRootLockName)); !os.IsNotExist(statErr) {
				t.Fatalf("v1 issuer root acquired a lease: %v", statErr)
			}
			if after := issuerRootBytes(t, root); !bytes.Equal(after, before) {
				t.Fatal("v1 issuer root changed while being rejected")
			}
		})
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
	duty := StateDuty{Generation: testStateGeneration, NetworkID: config.NetworkID, Digest: credentialID(77), IssuerNodeID: config.NodeID,
		IssuerPublicKey: publicIdentifier(nodePublic), InitiatorNodeID: config.InitiatorNodeID,
		InitiatorPublicKey: config.InitiatorPublicKey, GrantSignerPublicKey: profile.GrantSignerPublicKey,
		ProfileDigest: receipt.ProfileDigest, Epoch: 78, NotAfter: config.AssignmentNotAfter, Fresh: true}
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
	current.Generation = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if _, err := open(); err == nil {
		t.Fatal("bound issuer root started under a generation-only State successor")
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

const testStateGeneration = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
