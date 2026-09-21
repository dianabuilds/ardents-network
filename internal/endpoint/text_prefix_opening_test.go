//go:build linux

package endpoint

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestTextPrefixReservesOpeningBeforeIssuanceAndJoinsCancellation(t *testing.T) {
	_, owner, source := textSourceContextFixture(t)
	prepareTextIssuancePermission(t, owner, source)
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
		prefix, err := owner.openTextPrefix(ctx)
		if prefix != nil {
			_ = prefix.Close()
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
	flight := owner.prefixOpening
	reserved := flight != nil && owner.issuance != nil && owner.permission.batches == 1
	owner.mu.Unlock()
	if !reserved {
		cancel()
		<-done
		t.Fatal("issuance started without reserving the prefix transition")
	}
	if prefix, err := owner.openTextPrefix(t.Context()); prefix != nil || err == nil {
		if prefix != nil {
			_ = prefix.Close()
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
	select {
	case <-flight.done:
	default:
		t.Fatal("prefix returned before its opening completed")
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.prefixOpening != nil || owner.issuance != nil || owner.prefix != nil || owner.permission.batches != 1 {
		t.Fatal("cancellation leaked ownership or another batch debit")
	}
}

func TestTextPrefixOpeningExcludesUnrelatedIssuanceBetweenBatches(t *testing.T) {
	_, owner, source := textSourceContextFixture(t)
	prepareTextIssuancePermission(t, owner, source)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan struct{})
	close(done)
	// The reserved transition can be between its two network flights. The
	// absence of a current issuance must not permit an unrelated batch.
	flight := &textPrefixOpeningOperation{owner: owner, context: ctx, cancelOperation: cancel, done: done}
	owner.prefixOpening = flight
	attempt, stop := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer stop()
	if err := owner.issueTextTokens(attempt, [][32]byte{source.view.Nodes[0].NodeID}, 2); err == nil {
		t.Fatal("unrelated issuance entered prefix transition")
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.permission.pending != nil || owner.permission.batches != 0 || owner.permission.reserved != [3]uint32{} {
		t.Fatal("unrelated issuance consumed a batch during reserved prefix opening")
	}
}

func TestTextPrefixOpeningRejectsObsoleteCompletionWithoutTouchingReplacement(t *testing.T) {
	_, owner, source := textSourceContextFixture(t)
	prepareTextIssuancePermission(t, owner, source)
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
		prefix, openErr := owner.openTextPrefix(ctx)
		if prefix != nil {
			_ = prefix.Close()
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
	replacement := &textPrefixOpeningOperation{owner: owner, context: replacementContext, cancelOperation: replacementCancel, done: replacementDone}
	owner.mu.Lock()
	original := owner.prefixOpening
	if original == nil {
		owner.mu.Unlock()
		cancel()
		<-result
		t.Fatal("opening transport started without a retained reservation")
	}
	owner.prefixOpening = replacement
	permission := owner.permission
	pending := permission.pending
	batches, reserved, stock := permission.batches, permission.reserved, len(permission.stock)
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

	owner.mu.Lock()
	retained := owner.prefixOpening == replacement && owner.prefix == nil && owner.permission == permission &&
		permission.pending == pending && permission.batches == batches && permission.reserved == reserved && len(permission.stock) == stock
	if owner.prefixOpening == replacement {
		owner.prefixOpening = nil
	}
	owner.mu.Unlock()
	replacementCancel()
	if !retained {
		t.Fatal("obsolete completion cleared or published over the replacement reservation")
	}
}
