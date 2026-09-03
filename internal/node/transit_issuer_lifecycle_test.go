package node

import (
	"context"
	"crypto/ed25519"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

func TestTransitIssuerCleanupReportsHTTPAndRootFailures(t *testing.T) {
	shutdownErr := errors.New("injected HTTP shutdown failure")
	rootErr := errors.New("injected issuer root close failure")
	err := runTransitIssuerCleanup(context.Background(), time.Second, func() error { return nil },
		func(context.Context) error { return shutdownErr }, func() error { return rootErr })
	if !errors.Is(err, shutdownErr) || !errors.Is(err, rootErr) {
		t.Fatalf("Transit issuer cleanup = %v", err)
	}
}

func TestRunServesRootBackedTransitIssuerThenStopsOnStateSuccessor(t *testing.T) {
	issuerCertificate, issuerPublic := rendezvousCertificate(t, 181, "transit-issuer")
	initiatorCertificate, initiatorPublic := rendezvousCertificate(t, 182, "transit-initiator")
	now := time.Now().UTC().Truncate(time.Second)
	until := now.Add(time.Minute)
	root := transitIssuerStoreRoot(t)
	network, issuerID, initiatorID := [32]byte{1}, [32]byte{2}, [32]byte{3}
	receipt, err := credential.InitializeIssuerRoot(credential.IssuerRootConfig{Root: root, NetworkID: network, NodeID: issuerID,
		IdentityKey: issuerCertificate.PrivateKey.(ed25519.PrivateKey), InitiatorNodeID: initiatorID, InitiatorPublicKey: initiatorPublic,
		AssignmentNotAfter: until, Budget: 2, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	profile, err := credential.DecodeProfile(receipt.Profile)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := dutyFacts{Generation: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", NetworkID: network, Epoch: 4, Digest: [32]byte{5},
		EpochValidFrom: now.Add(-time.Second), ValidUntil: until, Profile: route.Profile, Fresh: true, RecordPresent: true,
		NodeID: issuerID, NodePublicKey: issuerPublic, RecordValidFrom: now.Add(-time.Second), RecordValidUntil: until,
		DeclaredFamily: "issuer-family", ProbeEndpoint: reserveAddress(t), Assignment: "transit-issuance", AssignmentDigest: [32]byte{6},
		TransitIssuerNodeID: issuerID, TransitIssuerProfile: receipt.Profile, CandidateCount: 2, Candidates: [64]dutyCandidate{
			{NodeID: issuerID, PublicKey: issuerPublic, Assignment: "transit-issuance", ValidFrom: now.Add(-time.Second), ValidUntil: until, AssignmentNotAfter: until},
			{NodeID: initiatorID, PublicKey: initiatorPublic, Assignment: "initiator", ValidFrom: now.Add(-time.Second), ValidUntil: until, AssignmentNotAfter: until},
		}}
	var lock sync.RWMutex
	events := make(chan Event, 16)
	config := Config{NetworkID: network, NodeID: issuerID, IdentityKey: issuerCertificate.PrivateKey.(ed25519.PrivateKey),
		Current:       func() (DutyView, error) { lock.RLock(); defer lock.RUnlock(); return snapshot, nil },
		TransitIssuer: TransitIssuerProfile{Root: root, Certificate: issuerCertificate, ConnectionLimit: 2, DrainTimeout: time.Second},
		PollInterval:  10 * time.Millisecond, Quarantine: time.Millisecond, LocalRoleStateRoot: localRoleStateRoot(t), CheckPlacement: func() error { return nil },
		Emit: func(_ context.Context, event Event) error { events <- event; return nil }}
	results := make(chan Result, 1)
	errors := make(chan error, 1)
	go func() { result, runErr := Run(context.Background(), config); results <- result; errors <- runErr }()
	waitForStateEvent(t, events, "READY")

	httpClient, err := credential.HTTPClient(issuerPublic, initiatorCertificate)
	if err != nil {
		t.Fatal(err)
	}
	defer httpClient.CloseIdleConnections()
	client, err := credential.OpenClient(credential.ClientConfig{NetworkID: network, IssuerPublic: issuerPublic, Profile: profile,
		At: now, Deadline: now.Add(15 * time.Second), Exchange: func(ctx context.Context, envelope []byte) ([]byte, error) {
			return credential.ForwardOHTTP(ctx, "https://"+snapshot.ProbeEndpoint, httpClient, envelope)
		}})
	if err != nil {
		t.Fatal(err)
	}
	request := credential.Request{RequestID: [32]byte{7}, NetworkID: network, Digest: snapshot.Digest,
		TransitNodeID: [32]byte{8}, AttachmentID: [32]byte{9}, ClientKeyDigest: [32]byte{10}, Epoch: snapshot.Epoch,
		TransitRole: route.IntroductionRole, NotAfter: now.Add(10 * time.Second)}
	issued, err := client.Issue(context.Background(), request)
	if err != nil || issued.Outcome != credential.Issued {
		t.Fatalf("root-backed issuance = %q, %v", issued.Outcome, err)
	}
	if _, err := route.VerifyTransitGrant(issued.Grant, ed25519.PublicKey(profile.GrantSignerPublicKey[:])); err != nil {
		t.Fatal(err)
	}

	lock.Lock()
	snapshot.Generation = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	lock.Unlock()
	waitForStateEvent(t, events, "DRAINING")
	select {
	case result := <-results:
		if result.State != "WITHDRAWN" {
			t.Fatalf("successor terminal result = %+v", result)
		}
	case <-time.After(testLifecycleWait):
		t.Fatal("State successor did not stop the Transit Grant issuer")
	}
	if err := <-errors; err != nil {
		t.Fatal(err)
	}
}
