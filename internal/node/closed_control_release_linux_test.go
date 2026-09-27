//go:build linux

package node

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
	"github.com/dianabuilds/ardents-network/internal/route/terminal"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

func TestClosedResolutionRetainsHostingReleaseFailureAfterReply(t *testing.T) {
	releaseErr := errors.New("hosting release failed")
	host := &cleanupFailureHost{release: releaseErr}
	var handle *dutyHandle
	var stopOnce sync.Once
	var drainErr error
	fixture := newPrivateRecipientNetworkFixtureWithStart(t, carrier.ClosedCarrierTCP, ardp.PurposeReachability, 1,
		func(config Config) (func() error, error) {
			resolved, err := resolveConfig(config)
			if err != nil {
				return nil, err
			}
			resolved.host = host
			snapshot, err := currentFacts(resolved)
			if err != nil {
				return nil, err
			}
			handle, err = startClosedResolution(resolved, snapshot)
			if err != nil {
				return nil, err
			}
			return func() error {
				stopOnce.Do(func() {
					handle.Stop()
					ctx, cancel := context.WithTimeout(context.Background(), time.Second)
					defer cancel()
					drainErr = handle.Drain(ctx)
				})
				return nil
			}, nil
		})
	// The finding scenario is a successful operation/reply followed by a
	// failed Hosting release: publish one real Descriptor, then read it back.
	issued, _, err := reachability.IssuePrivate(reachability.PrivateIssueInput{Current: fixture.current,
		ProfileDigest: fixture.profile.Digest, Introduction: fixture.introduction, InstanceSigner: fixture.signer})
	if err != nil {
		t.Fatal(err)
	}
	nonce := [32]byte{1}
	publication, err := terminal.EncodeDescriptorPublication(nonce, issued)
	if err != nil {
		t.Fatal(err)
	}
	lookup, err := terminal.EncodeDescriptorLookup(nonce, fixture.current.Credential.Target)
	if err != nil {
		t.Fatal(err)
	}
	status, _, err := fixture.exchange(t.Context(), fixture.tokens[0], nonce, publication)
	if err != nil || status != 0 {
		t.Fatalf("resolution publication = %d, %v", status, err)
	}
	status, proof, err := fixture.exchange(t.Context(), fixture.tokens[1], nonce, lookup)
	if err != nil || status != 0 || len(proof) == 0 {
		t.Fatalf("resolution lookup = %d, proof=%d, %v", status, len(proof), err)
	}
	handle.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	drainErr = handle.Drain(ctx)
	if !errors.Is(drainErr, releaseErr) {
		t.Fatalf("resolution drain lost Hosting release failure: %v", drainErr)
	}
	if host.reserved.Load() != 2 || host.released.Load() != 2 {
		t.Fatalf("Hosting reservation = %d release = %d", host.reserved.Load(), host.released.Load())
	}
}
