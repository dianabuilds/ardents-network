package endpoint

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRecoveryCannotMintFreshProofFromOriginalObservations(t *testing.T) {
	selected, binding, files := installedObservationFixture(t)
	request, err := decodeInstallationRequest(files["request.json"])
	if err != nil {
		t.Fatal(err)
	}
	intent := initialTransitionIntent{Schema: "ardents-endpoint-installation-initial-v1", Candidate: selected, CandidateBinding: binding, Request: request}
	reference, err := time.Parse(time.RFC3339Nano, request.ReferenceTime)
	if err != nil {
		t.Fatal(err)
	}
	// Original observations cannot populate the separate owner's private pair.
	observed := Authorization{}
	if _, err := recoveryGeneration(t.Context(), intent, reference, observed); !errors.Is(err, ErrAuthorization) {
		t.Fatal("original binding and exact observed bytes became fresh recovery proof", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := recoveryGeneration(ctx, intent, reference, observed); !errors.Is(err, context.Canceled) {
		t.Fatal("recovery lost original cancellation", err)
	}
}

func TestZeroRecoveryAndOriginalCancellationGrantNoNativeOperation(t *testing.T) {
	var owner Recovery
	if _, err := owner.Complete(Authorization{}); !errors.Is(err, ErrInput) {
		t.Fatal("zero retained operation accepted", err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if owner, err := OpenInitialRecovery(ctx, "/root/never-opened", time.Now().UTC()); owner != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled operation acquired native ownership", err)
	}
}
