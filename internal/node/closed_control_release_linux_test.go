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
	request, err := terminal.EncodeDescriptorLookup([32]byte{1}, fixture.current.Credential.Target)
	if err != nil {
		t.Fatal(err)
	}
	status, _, err := fixture.exchange(t.Context(), fixture.tokens[0], [32]byte{1}, request)
	if err != nil || status != 1 {
		t.Fatalf("resolution reply = %d, %v", status, err)
	}
	handle.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	drainErr = handle.Drain(ctx)
	if !errors.Is(drainErr, releaseErr) {
		t.Fatalf("resolution drain lost Hosting release failure: %v", drainErr)
	}
	if host.reserved.Load() != 1 || host.released.Load() != 1 {
		t.Fatalf("Hosting reservation = %d release = %d", host.reserved.Load(), host.released.Load())
	}
}
