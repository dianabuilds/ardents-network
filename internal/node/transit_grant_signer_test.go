package node

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

func TestNodeProjectsOnlyPurposeScopedSignerFromStateIssuerProfile(t *testing.T) {
	now := time.Unix(2_000_700_000, 0).UTC()
	network, digest, issuerID := [32]byte{1}, [32]byte{2}, [32]byte{3}
	issuerPublic, issuerPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	initiatorPublic, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	issuer := openNodeTestIssuer(t, network, issuerID, issuerPrivate, [32]byte{4}, [32]byte(initiatorPublic), now.Add(time.Minute), 2,
		func() time.Time { return now }, func(profile credential.Profile, profileDigest [32]byte) (credential.StateDuty, bool) {
			return credential.StateDuty{Generation: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", NetworkID: network, Digest: digest, IssuerNodeID: issuerID,
				IssuerPublicKey: [32]byte(issuerPublic), InitiatorNodeID: [32]byte{4}, InitiatorPublicKey: [32]byte(initiatorPublic),
				GrantSignerPublicKey: profile.GrantSignerPublicKey, ProfileDigest: profileDigest,
				Epoch: 5, NotAfter: now.Add(time.Minute), Fresh: true}, true
		})
	defer func() { _ = issuer.Close() }()
	issuerProfile := issuer.Profile()
	grantPublic := ed25519.PublicKey(issuerProfile.GrantSignerPublicKey[:])
	profile, err := credential.EncodeProfile(issuer.Profile())
	if err != nil {
		t.Fatal(err)
	}
	statePublic, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := dutyFacts{Generation: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", NetworkID: network, Epoch: 5, Digest: digest,
		NodeID: issuerID, NodePublicKey: [32]byte(issuerPublic), Assignment: "transit-issuance", Fresh: true,
		ValidUntil: now.Add(time.Minute), RecordValidUntil: now.Add(time.Minute),
		TransitIssuerNodeID: issuerID, TransitIssuerProfile: profile, CandidateCount: 2,
		Candidates: [64]dutyCandidate{
			{NodeID: issuerID, PublicKey: [32]byte(issuerPublic), Assignment: "transit-issuance",
				ValidUntil: now.Add(time.Minute), AssignmentNotAfter: now.Add(time.Minute)},
			{NodeID: [32]byte{4}, PublicKey: [32]byte(initiatorPublic), Assignment: "initiator",
				ValidUntil: now.Add(time.Minute), AssignmentNotAfter: now.Add(time.Minute)},
		},
		Authorities: [16]dutyAuthority{{ID: sha256.Sum256(statePublic), PublicKey: [32]byte(statePublic)}}, AuthorityCount: 1}
	if err := attachTransitGrantSigner(&snapshot, now); err != nil {
		t.Fatal(err)
	}
	if snapshot.TransitGrantSignerID != sha256.Sum256(grantPublic) || snapshot.TransitGrantSignerPublicKey != [32]byte(grantPublic) {
		t.Fatalf("projected signer = (%x, %x)", snapshot.TransitGrantSignerID, snapshot.TransitGrantSignerPublicKey)
	}
	if snapshot.TransitGrantSignerID == snapshot.Authorities[0].ID {
		t.Fatal("purpose-scoped Grant signer collapsed into the Epoch authority")
	}
	if _, available := transitIssuerStateDuty(snapshot, now); !available {
		t.Fatal("fresh non-conflicting State issuer duty was unavailable")
	}
	for _, unavailable := range []struct {
		name   string
		mutate func(*dutyFacts)
	}{
		{name: "stale", mutate: func(duty *dutyFacts) { duty.Fresh = false }},
		{name: "conflicting", mutate: func(duty *dutyFacts) { duty.Conflicting = true }},
	} {
		t.Run(unavailable.name, func(t *testing.T) {
			changedDuty := snapshot
			unavailable.mutate(&changedDuty)
			if _, available := transitIssuerStateDuty(changedDuty, now); available {
				t.Fatal("unavailable State projected a transit issuer duty")
			}
		})
	}

	changed := snapshot
	changed.TransitGrantSignerID, changed.TransitGrantSignerPublicKey = [32]byte{}, [32]byte{}
	changed.TransitIssuerProfile = append([]byte(nil), profile...)
	changed.TransitIssuerProfile[len(changed.TransitIssuerProfile)-1] ^= 1
	if err := attachTransitGrantSigner(&changed, now); err == nil {
		t.Fatal("Node accepted a substituted State issuer profile")
	}

	changed = snapshot
	changed.TransitGrantSignerID, changed.TransitGrantSignerPublicKey = [32]byte{}, [32]byte{}
	changed.Candidates[1].PublicKey = [32]byte{99}
	if err := attachTransitGrantSigner(&changed, now); err == nil {
		t.Fatal("Node accepted an issuer profile whose permitted Initiator did not match State")
	}
}

func openNodeTestIssuer(t *testing.T, network, issuerID [32]byte, identity ed25519.PrivateKey,
	initiatorID, initiatorPublic [32]byte, notAfter time.Time, budget uint16, clock func() time.Time,
	current func(credential.Profile, [32]byte) (credential.StateDuty, bool),
) *credential.Issuer {
	t.Helper()
	root := transitIssuerStoreRoot(t)
	receipt, err := credential.InitializeIssuerRoot(credential.IssuerRootConfig{Root: root, NetworkID: network, NodeID: issuerID,
		IdentityKey: identity, InitiatorNodeID: initiatorID, InitiatorPublicKey: initiatorPublic,
		AssignmentNotAfter: notAfter, Budget: budget, Clock: clock})
	if err != nil {
		t.Fatal(err)
	}
	profile, err := credential.DecodeProfile(receipt.Profile)
	if err != nil {
		t.Fatal(err)
	}
	issuer, err := credential.OpenIssuerFromRoot(credential.RootIssuerConfig{Root: root, NetworkID: network, NodeID: issuerID,
		IdentityKey: identity, CurrentDuty: func() (credential.StateDuty, bool) { return current(profile, receipt.ProfileDigest) }, Clock: clock})
	if err != nil {
		t.Fatal(err)
	}
	return issuer
}
