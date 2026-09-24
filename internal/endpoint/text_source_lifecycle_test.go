//go:build linux

package endpoint

import (
	"context"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// These helpers let package tests force the Route's finite lifetime. They are
// deliberately absent from the production read-only Source handle.
func (handle *textSourceHandle) Close() error {
	prefix, err := handle.routePrefix()
	if err != nil {
		return err
	}
	return prefix.Close()
}

func (handle *textSourceHandle) Done() <-chan struct{} {
	prefix, err := handle.routePrefix()
	if err != nil {
		done := make(chan struct{})
		close(done)
		return done
	}
	return prefix.Done()
}

func (handle *textSourceHandle) ResolutionRecipient() ([32]byte, error) {
	return handle.resolutionRecipient()
}

func (handle *textSourceHandle) ExchangeDescriptor(ctx context.Context, present route.ClosedTokenPresenter,
	target [32]byte, descriptor []byte) (uint8, []byte, error) {
	return handle.exchangeDescriptor(ctx, present, target, descriptor)
}

func (handle *textSourceHandle) SubmissionRecipient() ([32]byte, error) {
	return handle.submissionRecipient()
}

func (handle *textSourceHandle) SubmitIntroduction(ctx context.Context, present route.ClosedTokenPresenter,
	operation []byte) (uint8, error) {
	return handle.submitIntroduction(ctx, present, operation)
}

func (handle *textSourceHandle) DataJoinRecipient() ([32]byte, uint64, time.Time, error) {
	return handle.dataJoinRecipient()
}

func (handle *textSourceHandle) Join(ctx context.Context, present route.ClosedTokenPresenter,
	intent route.ClosedJoinIntent) (*route.ClosedJoinedStream, error) {
	return handle.join(ctx, present, intent)
}

func (handle *textSourceHandle) ExchangeIssuer(ctx context.Context, present route.ClosedTokenPresenter,
	batch []byte) (route.ClosedIssuanceExchangeResult, error) {
	return handle.exchangeIssuer(ctx, present, batch)
}

func (handle *textSourceHandle) Replenish(ctx context.Context, present route.ClosedTokenPresenter) error {
	return handle.replenish(ctx, present)
}

func TestTextSourceHandleRejectsUseAfterIdleRetirement(t *testing.T) {
	endpoint, owner, _ := startTextRoleNetwork(t, textRoleNetworkFixture{carrier: route.ClosedCarrierTCP})
	defer func() { _ = endpoint.Close() }()
	handle, err := owner.openTextPrefix(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := handle.Close(); err != nil {
		t.Fatal(err)
	}
	owner.mu.Lock()
	err = owner.retireTextPrefixLocked()
	retired := owner.currentTextSourceLocked() == nil
	owner.mu.Unlock()
	if err != nil || !retired {
		t.Fatalf("idle Source retirement failed: retired=%v err=%v", retired, err)
	}
	if _, _, _, err := handle.dataJoinRecipient(); err == nil {
		t.Fatal("retired Source handle remained usable")
	}
}
