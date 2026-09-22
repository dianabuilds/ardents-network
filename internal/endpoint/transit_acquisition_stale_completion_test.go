package endpoint

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
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
