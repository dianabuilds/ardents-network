//go:build linux

package endpoint

import (
	"context"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/carrier"
	"github.com/dianabuilds/ardents-network/internal/route/client"
)

// These helpers let package tests force the Route's finite lifetime. They are
// deliberately absent from the production read-only Source handle.
func (handle *sourceHandle) Close() error {
	prefix, err := handle.routePrefix()
	if err != nil {
		return err
	}
	return prefix.Close()
}

func (handle *sourceHandle) Done() <-chan struct{} {
	prefix, err := handle.routePrefix()
	if err != nil {
		done := make(chan struct{})
		close(done)
		return done
	}
	return prefix.Done()
}

func (handle *sourceHandle) ResolutionRecipient() ([32]byte, error) {
	return handle.resolutionRecipient()
}

func (handle *sourceHandle) ExchangeDescriptor(ctx context.Context, present client.ClosedTokenPresenter,
	target [32]byte, descriptor []byte) (uint8, []byte, error) {
	return handle.exchangeDescriptor(ctx, present, target, descriptor)
}

func (handle *sourceHandle) SubmissionRecipient() ([32]byte, error) {
	return handle.submissionRecipient()
}

func (handle *sourceHandle) SubmitIntroduction(ctx context.Context, present client.ClosedTokenPresenter,
	operation []byte) (uint8, error) {
	return handle.submitIntroduction(ctx, present, operation)
}

func (handle *sourceHandle) DataJoinRecipient() ([32]byte, uint64, time.Time, error) {
	return handle.dataJoinRecipient()
}

func (handle *sourceHandle) Join(ctx context.Context, present client.ClosedTokenPresenter,
	intent client.ClosedJoinIntent) (*client.ClosedJoinedStream, error) {
	return handle.join(ctx, present, intent)
}

func (handle *sourceHandle) Replenish(ctx context.Context, present client.ClosedTokenPresenter) error {
	return handle.replenish(ctx, present)
}

func TestTextSourceHandleRejectsUseAfterIdleRetirement(t *testing.T) {
	endpoint, owner, _ := startRoleNetwork(t, roleNetworkFixture{carrier: carrier.ClosedCarrierTCP})
	defer func() { _ = endpoint.Close() }()
	handle, err := owner.openPrefix(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := handle.Close(); err != nil {
		t.Fatal(err)
	}
	owner.mu.Lock()
	err = owner.retirePrefixLocked()
	retired := owner.source.currentLocked() == nil
	owner.mu.Unlock()
	if err != nil || !retired {
		t.Fatalf("idle Source retirement failed: retired=%v err=%v", retired, err)
	}
	if _, _, _, err := handle.dataJoinRecipient(); err == nil {
		t.Fatal("retired Source handle remained usable")
	}
}
