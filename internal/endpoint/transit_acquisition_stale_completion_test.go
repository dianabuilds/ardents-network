package endpoint

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

func TestTransitCredentialLifecycleIgnoresStaleIssuerOutcomes(t *testing.T) {
	for _, outcome := range []credential.Outcome{credential.Issued, credential.Exhausted, credential.Withdrawn, credential.Unavailable} {
		t.Run(string(outcome), func(t *testing.T) {
			now := time.Unix(2_000_001_200, 0).UTC()
			_, signer, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			var signerPublic [32]byte
			copy(signerPublic[:], signer.Public().(ed25519.PublicKey))
			base := transitAcquisitionScope{NetworkID: acquisitionID(81), Digest: acquisitionID(82), Epoch: 83,
				IssuerNodeID: acquisitionID(84), IssuerPublicKey: acquisitionID(85), IssuerProfileDigest: acquisitionID(86),
				GrantSignerPublicKey: signerPublic, TransitNodeID: acquisitionID(87), TransitRole: route.IntroductionRole,
				NotAfter: now.Add(10 * time.Second)}
			first, second, third := base, base, base
			first.AttachmentID, second.AttachmentID, third.AttachmentID = acquisitionID(88), acquisitionID(89), acquisitionID(90)

			owner, err := openTransitAcquisition(transitAcquisitionConfig{Root: filepath.Join(t.TempDir(), "transit-acquisition"), Create: true,
				Clock: func() time.Time { return now }})
			if err != nil {
				t.Fatal(err)
			}
			defer owner.Close()
			endpoint := &endpoint{}
			started := make(chan credential.Request, 1)
			result := make(chan credential.Result, 1)
			firstDone := make(chan error, 1)
			go func() {
				_, err := endpoint.acquireTransitCredentialLifecycle(t.Context(), owner, first, func(_ context.Context, request credential.Request) (credential.Result, error) {
					started <- request
					return <-result, nil
				})
				firstDone <- err
			}()
			firstRequest := <-started
			if _, err := owner.begin(second); err == nil {
				t.Fatal("replacement acquisition did not invalidate the first attempt")
			}
			acquiredThird, err := endpoint.acquireTransitCredentialLifecycle(t.Context(), owner, third, func(_ context.Context, request credential.Request) (credential.Result, error) {
				return credential.Result{Outcome: credential.Issued, Grant: acquisitionGrant(t, third, request, signer)}, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			late := credential.Result{Outcome: outcome}
			if outcome == credential.Issued {
				late.Grant = acquisitionGrant(t, first, firstRequest, signer)
			}
			result <- late
			if err := <-firstDone; !errors.Is(err, errTransitAcquisitionStale) {
				t.Fatalf("stale %s outcome error = %v, want stale-attempt rejection", outcome, err)
			}
			if state := owner.stateForTest(); state.Phase != transitPresenting || state.RequestID != acquiredThird.attempt.Request.RequestID ||
				!bytes.Equal(state.Grant, acquiredThird.attempt.Grant) || len(state.PrivateKey) == 0 {
				t.Fatalf("stale %s outcome changed the third acquisition: %+v", outcome, state)
			}
			if err := acquiredThird.finish(true); err != nil {
				t.Fatalf("third completion failed: %v", err)
			}
		})
	}
}

func TestTransitAcquisitionRejectsStaleReadyPresentation(t *testing.T) {
	now := time.Unix(2_000_001_300, 0).UTC()
	_, signer, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var signerPublic [32]byte
	copy(signerPublic[:], signer.Public().(ed25519.PublicKey))
	base := transitAcquisitionScope{NetworkID: acquisitionID(101), Digest: acquisitionID(102), Epoch: 103,
		IssuerNodeID: acquisitionID(104), IssuerPublicKey: acquisitionID(105), IssuerProfileDigest: acquisitionID(106),
		GrantSignerPublicKey: signerPublic, TransitNodeID: acquisitionID(107), TransitRole: route.IntroductionRole,
		NotAfter: now.Add(10 * time.Second)}
	first, second, third := base, base, base
	first.AttachmentID, second.AttachmentID, third.AttachmentID = acquisitionID(108), acquisitionID(109), acquisitionID(110)
	owner, err := openTransitAcquisition(transitAcquisitionConfig{Root: filepath.Join(t.TempDir(), "transit-acquisition"), Create: true,
		Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	firstAttempt, err := owner.begin(first)
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.commit(firstAttempt.Request.RequestID, credential.Result{Outcome: credential.Issued,
		Grant: acquisitionGrant(t, first, firstAttempt.Request, signer)}); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.begin(second); err == nil {
		t.Fatal("replacement acquisition did not invalidate the first attempt")
	}
	thirdAttempt, err := owner.begin(third)
	if err != nil {
		t.Fatal(err)
	}
	thirdGrant := acquisitionGrant(t, third, thirdAttempt.Request, signer)
	if err := owner.commit(thirdAttempt.Request.RequestID, credential.Result{Outcome: credential.Issued, Grant: thirdGrant}); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.present(thirdAttempt.Request.RequestID, third); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.present(firstAttempt.Request.RequestID, first); !errors.Is(err, errTransitAcquisitionStale) {
		t.Fatalf("stale ready-to-present transition = %v, want stale-attempt rejection", err)
	}
	if state := owner.stateForTest(); state.Phase != transitPresenting || state.RequestID != thirdAttempt.Request.RequestID ||
		!bytes.Equal(state.Grant, thirdGrant) || len(state.PrivateKey) == 0 {
		t.Fatalf("stale ready-to-present transition changed the third acquisition: %+v", state)
	}
}

func TestPublisherAndUserShareOneIntroductionCompletionOwner(t *testing.T) {
	fixture := openUserRouteCredentialFixture(t, route.IntroductionDelivered)
	defer fixture.close()
	introductionRelay := fixture.entry.handlers[2]
	fixture.entry.handlers = []func(net.Conn) error{fixture.resolutionRelay, fixture.resolutionRelay, fixture.credentialRelay, introductionRelay}
	fixture.entryCalls = len(fixture.entry.handlers)
	deadline := fixture.now.Add(15 * time.Second)
	publisherEntry := &userRouteCredentialEntry{contact: fixture.entry.contact, handlers: []func(net.Conn) error{
		fixture.credentialRelay, fixture.credentialRelay,
	}, errs: make(chan error, 2)}
	publisherView := userRoutePublisherView{userRouteCredentialState: fixture.view, attachment: state.PublisherAttachment{
		NetworkID: fixture.network, Digest: fixture.view.epoch.Digest, Epoch: fixture.view.epoch.Number, NotAfter: deadline,
		Introduction: state.PublisherTransitPeer{NodeID: fixture.view.introduction.NodeID, PublicKey: fixture.view.introduction.PublicKey,
			Family: [32]byte{91}, Endpoint: fixture.view.introduction.Endpoint},
		Rendezvous: state.PublisherTransitPeer{NodeID: fixture.view.rendezvous.NodeID, PublicKey: fixture.view.rendezvous.PublicKey,
			Family: [32]byte{92}, Endpoint: fixture.view.rendezvous.Endpoint},
		Responder: state.PublisherTransitPeer{NodeID: [32]byte{93}, PublicKey: [32]byte{94}, Family: [32]byte{95}, Endpoint: "127.0.0.1:3"},
	}}
	publisher, err := fixture.endpoint.acquirePublisherProfile(t.Context(), publisherView, publisherEntry, fixture.credential, fixture.now)
	if err != nil {
		t.Fatal(err)
	}
	publisherEntry.wait(t, 2)
	firstRequestID := fixture.endpoint.transitAcquire.introduction.stateForTest().RequestID
	if _, err := fixture.route.Attach(t.Context(), route.Intent{Target: fixture.target}); err == nil {
		t.Fatal("User replacement did not refuse the live Publisher Introduction attempt")
	}
	attachment, err := fixture.route.Attach(t.Context(), route.Intent{Target: fixture.target})
	if err != nil || attachment == nil {
		t.Fatalf("User successor attachment = %v, %v", attachment, err)
	}
	if err := attachment.Close(); err != nil {
		t.Fatal(err)
	}
	fixture.wait(t)
	current := fixture.endpoint.transitAcquire.introduction.stateForTest()
	if current.Phase != transitSpent || current.RequestID == firstRequestID {
		t.Fatalf("User successor did not own the spent Introduction attempt: %+v", current)
	}
	if err := publisher.credentials.introduction(true); !errors.Is(err, errTransitAcquisitionStale) {
		t.Fatalf("late Publisher Introduction completion = %v, want stale-attempt rejection", err)
	}
	if state := fixture.endpoint.transitAcquire.introduction.stateForTest(); state.Phase != transitSpent || state.RequestID != current.RequestID {
		t.Fatalf("late Publisher completion changed User result: %+v", state)
	}
	if err := publisher.credentials.responder(false); err != nil {
		t.Fatalf("Publisher Responder cleanup = %v", err)
	}

	exhaustedEntry := &userRouteCredentialEntry{contact: fixture.entry.contact, handlers: []func(net.Conn) error{fixture.credentialRelay}, errs: make(chan error, 1)}
	slot := reachability.Introduction{StateDigest: fixture.view.epoch.Digest, Epoch: fixture.view.epoch.Number,
		IntroductionNodeID: fixture.view.introduction.NodeID, RendezvousNodeID: fixture.view.rendezvous.NodeID,
		NotAfter: deadline, SubmissionMode: reachability.SubmissionMembershipGrant}
	_, err = fixture.endpoint.acquireTransitCredential(t.Context(), fixture.view, fixture.view.epoch, exhaustedEntry,
		transitPeer{NodeID: fixture.view.initiator.NodeID, PublicKey: fixture.view.initiator.PublicKey, Family: fixture.entry.contact.FamilyID, Endpoint: fixture.view.initiator.Endpoint},
		transitPeer{NodeID: fixture.view.introduction.NodeID, PublicKey: fixture.view.introduction.PublicKey, Family: [32]byte{91}, Endpoint: fixture.view.introduction.Endpoint},
		route.IntroductionRole, slot, fixture.now, deadline)
	exhaustedEntry.wait(t, 1)
	var outcome transitAcquisitionOutcomeError
	if !errors.As(err, &outcome) || outcome.outcome != credential.Exhausted {
		t.Fatalf("fourth issuer request = %v, want finite-budget exhaustion", err)
	}
}

type userRoutePublisherView struct {
	userRouteCredentialState
	attachment state.PublisherAttachment
}

func (view userRoutePublisherView) PublisherAttachment(time.Time, time.Time) (state.PublisherAttachment, bool) {
	return view.attachment, true
}
