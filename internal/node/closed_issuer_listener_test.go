package node

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

func TestRunServesClosedIssuerThenDrainsOnClosedProfileSuccessor(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	until := now.Truncate(time.Hour).Add(time.Hour)
	certificate, public := rendezvousCertificate(t, 211, "closed-issuer")
	network, issuerID := [32]byte{41}, [32]byte{42}
	generation, digest := sha256.Sum256([]byte("closed node generation")), sha256.Sum256([]byte("closed node digest"))
	root := t.TempDir()
	receipt, err := credential.InitializeClosedIssuerRoot(credential.ClosedIssuerRootConfig{Root: root, NetworkID: network, NodeID: issuerID,
		IdentityKey: certificate.PrivateKey.(ed25519.PrivateKey), NotBefore: now.Truncate(time.Hour), NotAfter: now.Truncate(time.Hour).Add(time.Hour), Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	issuerProfile, err := credential.DecodeClosedIssuerProfile(receipt.Profile, ed25519.PublicKey(public[:]))
	if err != nil {
		t.Fatal(err)
	}
	profile := state.ClosedProfileView{NetworkID: network, StateGeneration: generation, StateDigest: digest, Digest: sha256.Sum256([]byte("closed profile")),
		IssuanceAuthorityKey: [32]byte{43}, IssuerNodeID: issuerID, IssuerDutyGeneration: 8, Epoch: 4,
		NotBefore: now.Truncate(time.Hour), NotAfter: now.Truncate(time.Hour).Add(time.Hour), TokenKeyCount: uint8(len(issuerProfile.Keys))}
	for index, key := range issuerProfile.Keys {
		profile.TokenKeys[index].WindowStart, profile.TokenKeys[index].Class = key.WindowStart, uint8(key.Class)
		copy(profile.TokenKeys[index].SPKI[:], key.SPKI)
	}
	snapshot := dutyFacts{Generation: hex.EncodeToString(generation[:]), NetworkID: network, Epoch: profile.Epoch, Digest: digest,
		EpochValidFrom: profile.NotBefore, ValidUntil: until, Profile: route.ClosedRouteProfile, Fresh: true, RecordPresent: true,
		NodeID: issuerID, NodePublicKey: public, RecordValidFrom: now.Add(-time.Second), RecordValidUntil: until,
		DeclaredFamily: "closed-issuer-family", ProbeEndpoint: reserveAddress(t), CarrierProfile: string(route.ClosedCarrierTCP), Assignment: "rendezvous", AssignmentDigest: [32]byte{44}}
	var lock sync.RWMutex
	events := make(chan Event, 16)
	config := Config{NetworkID: network, NodeID: issuerID, IdentityKey: certificate.PrivateKey.(ed25519.PrivateKey),
		Current:              func() (DutyView, error) { lock.RLock(); defer lock.RUnlock(); return snapshot, nil },
		CurrentClosedProfile: func() (state.ClosedProfileView, bool) { return profile, true },
		ClosedIssuer:         ClosedIssuerProfile{Root: root, Certificate: certificate, ConnectionLimit: 1, DrainTimeout: time.Second},
		PollInterval:         10 * time.Millisecond, Quarantine: time.Millisecond, LocalRoleStateRoot: localRoleStateRoot(t), CheckPlacement: func() error { return nil },
		Emit: func(_ context.Context, event Event) error { events <- event; return nil }}
	resolved, err := resolveConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	if admission := assessAdmission(resolved, snapshot); admission.kind != admissionReady {
		t.Fatalf("closed issuer admission = %+v", admission)
	}
	results := make(chan Result, 1)
	runErrors := make(chan error, 1)
	go func() { result, runErr := Run(context.Background(), config); results <- result; runErrors <- runErr }()
	waitForStateEvent(t, events, "READY")
	lock.Lock()
	snapshot.Generation = strings.Repeat("b", 64)
	lock.Unlock()
	waitForStateEvent(t, events, "DRAINING")
	select {
	case result := <-results:
		if result.State != "WITHDRAWN" {
			t.Fatalf("closed successor result = %+v", result)
		}
	case <-time.After(testLifecycleWait):
		t.Fatal("closed profile successor did not drain issuer")
	}
	if err := <-runErrors; err != nil {
		t.Fatal(err)
	}
}
