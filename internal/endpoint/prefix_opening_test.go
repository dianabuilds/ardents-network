//go:build linux

package endpoint

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestTextPrefixReservesOpeningBeforeIssuanceAndJoinsCancellation(t *testing.T) {
	_, owner, source := sourceContextFixture(t)
	prepareIssuancePermission(t, owner, source)
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := listener.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	source.mu.Lock()
	for index := 0; index < 2; index++ {
		source.snapshot.Candidates[index].Endpoint = listener.Addr().String()
	}
	source.mu.Unlock()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		prefix, err := owner.openPrefix(ctx)
		if prefix != nil {
			_ = closeSourceHandle(prefix)
		}
		done <- err
	}()
	// The actual bootstrap transport is held before TLS completion. Preparing
	// its batch must already own the entire prefix-opening reservation.
	accepted, err := listener.Accept()
	if err != nil {
		cancel()
		<-done
		t.Fatal(err)
	}
	defer accepted.Close()
	owner.mu.Lock()
	reserved := owner.source.OpeningInProgressLocked() && owner.tokens.BusyLocked() && (2-owner.tokens.PermissionLocked().BootstrapAllowance()) == 1
	owner.mu.Unlock()
	if !reserved {
		cancel()
		<-done
		t.Fatal("issuance started without reserving the prefix transition")
	}
	if prefix, err := owner.openPrefix(t.Context()); prefix != nil || err == nil {
		if prefix != nil {
			_ = closeSourceHandle(prefix)
		}
		cancel()
		<-done
		t.Fatal("concurrent prefix opening acquired a second reservation")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled prefix returned success")
		}
	case <-time.After(3 * time.Second):
		_ = accepted.Close()
		<-done
		t.Fatal("cancelled prefix did not join bootstrap")
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	// The prefix returned only after its completion cleared the reservation:
	// FinishOpeningLocked runs before openPrefix hands the caller its outcome.
	if owner.source.OpeningInProgressLocked() || owner.tokens.BusyLocked() || owner.source.CurrentLocked() != nil || (2-owner.tokens.PermissionLocked().BootstrapAllowance()) != 1 {
		t.Fatal("cancellation leaked ownership or another batch debit")
	}
}

func TestTextPrefixOpeningExcludesUnrelatedIssuanceBetweenBatches(t *testing.T) {
	_, owner, source := sourceContextFixture(t)
	prepareIssuancePermission(t, owner, source)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan struct{})
	close(done)
	// The reserved transition can be between its two network flights. The
	// absence of a current issuance must not permit an unrelated batch.
	flight := &operationFlight{owner: owner, context: ctx, cancelOperation: cancel, done: done}
	if !owner.source.ReserveOpeningLocked(flight) {
		t.Fatal("Source lifecycle refused the planted prefix transition")
	}
	attempt, stop := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer stop()
	if err := owner.issueTokens(attempt, [][32]byte{source.view.Nodes[0].NodeID}, 2); err == nil {
		t.Fatal("unrelated issuance entered prefix transition")
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.tokens.PermissionLocked().HasPending() || (2-owner.tokens.PermissionLocked().BootstrapAllowance()) != 0 || reservedStockAllocation(owner.tokens.PermissionLocked()) != [3]uint32{} {
		t.Fatal("unrelated issuance consumed a batch during reserved prefix opening")
	}
}

func TestTextPrefixOpeningRejectsObsoleteCompletionWithoutTouchingReplacement(t *testing.T) {
	_, owner, source := sourceContextFixture(t)
	prepareIssuancePermission(t, owner, source)
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := listener.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	source.mu.Lock()
	for index := 0; index < 2; index++ {
		source.snapshot.Candidates[index].Endpoint = listener.Addr().String()
	}
	source.mu.Unlock()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		prefix, openErr := owner.openPrefix(ctx)
		if prefix != nil {
			_ = closeSourceHandle(prefix)
		}
		result <- openErr
	}()
	accepted, err := listener.Accept()
	if err != nil {
		cancel()
		<-result
		t.Fatal(err)
	}
	defer accepted.Close()

	replacementContext, replacementCancel := context.WithCancel(t.Context())
	replacementDone := make(chan struct{})
	close(replacementDone)
	replacement := &operationFlight{owner: owner, context: replacementContext, cancelOperation: replacementCancel, done: replacementDone}
	owner.mu.Lock()
	if !owner.source.OpeningInProgressLocked() {
		owner.mu.Unlock()
		cancel()
		<-result
		t.Fatal("opening transport started without a retained reservation")
	}
	// Displace the reservation the same way context stop revokes it: the
	// detached original keeps running, and its completion must find the
	// replacement slot and refuse to publish or clear it.
	retirement := owner.source.StopLocked()
	if !owner.source.ReserveOpeningLocked(replacement) {
		owner.mu.Unlock()
		retirement.JoinOpening()
		cancel()
		<-result
		t.Fatal("Source lifecycle refused the replacement reservation")
	}
	permission := owner.tokens.PermissionLocked()
	pending := permission.HasPending()
	batches, reserved, stock := (2 - permission.BootstrapAllowance()), reservedStockAllocation(permission), usableStockCountLocked(t, owner)
	owner.mu.Unlock()
	cancel()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("obsolete opening completed successfully")
		}
	case <-time.After(3 * time.Second):
		_ = accepted.Close()
		<-result
		t.Fatal("obsolete opening did not join")
	}
	retirement.JoinOpening()

	owner.mu.Lock()
	retained := owner.source.OpeningAdmittedLocked(replacement) && owner.source.CurrentLocked() == nil && owner.tokens.PermissionLocked() == permission &&
		permission.HasPending() == pending && (2-permission.BootstrapAllowance()) == batches && reservedStockAllocation(permission) == reserved && usableStockCountLocked(t, owner) == stock
	if owner.source.OpeningAdmittedLocked(replacement) {
		owner.source.FinishOpeningLocked(replacement, nil, nil, false)
	}
	owner.mu.Unlock()
	replacementCancel()
	if !retained {
		t.Fatal("obsolete completion cleared or published over the replacement reservation")
	}
}
