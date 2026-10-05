//go:build linux

package transport

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route/selection"
)

// This is a lifecycle oracle only. No fabricated Prefix in this test performs
// admission, Network verification or a successful JOIN.
func TestJoinAcquisitionKeepsOriginalSourceAndJoinsOpening(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		prefix := func(domain uint8) *Prefix {
			ctx, cancel := context.WithCancel(t.Context())
			return &Prefix{ctx: ctx, caller: ctx, cancel: cancel, closing: make(chan struct{}), activity: make(chan struct{}, 1), config: PrefixConfig{Leg: selection.Leg{EntryMember: network.Member{RoleDomain: domain}}}}
		}
		source, responder := prefix(1), prefix(3)
		responder.source = source
		acquisition, err := responder.AcquireResponderJoin(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		acquisition.mu.Lock()
		acquisition.opening = make(chan struct{})
		opening := acquisition.opening
		acquisition.mu.Unlock()
		closed := make(chan error, 1)
		go func() { closed <- acquisition.Close() }()
		synctest.Wait()
		if acquisition.ctx.Err() == nil {
			t.Fatal("close did not interrupt original opening")
		}
		select {
		case <-closed:
			t.Fatal("returned acquisition before opening joined")
		default:
		}
		if _, ok := source.joins[acquisition]; !ok {
			t.Fatal("released Source before join")
		}
		if _, ok := responder.joins[acquisition]; !ok {
			t.Fatal("released Responder before join")
		}
		close(opening)
		if err := <-closed; err != nil {
			t.Fatal(err)
		}
		if len(source.joins) != 0 || len(responder.joins) != 0 {
			t.Fatal("original borrow retained after join")
		}
		if err := acquisition.current(); !errors.Is(err, context.Canceled) {
			t.Fatal("retired acquisition regained authority", err)
		}
		if err := acquisition.Close(); err != nil {
			t.Fatal(err)
		}
		close(source.closing)
		if _, err := responder.AcquireResponderJoin(t.Context()); err == nil {
			t.Fatal("closed Source authorized stocked Responder")
		}
		if _, err := responder.AcquireSourceJoin(t.Context()); err == nil {
			t.Fatal("Responder substituted Source")
		}
	})
}

func TestJoinAcquisitionUnpublishedSetupCleanupRetainsFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		owner, generation := joinContextLifetime(t)
		generation.source = joinContextRetiringPrefix(t, generation.ctx, nil)
		generation.source.config.Leg.EntryMember.RoleDomain = 1
		generation.ready = true
		close(generation.opened)
		acquisition, err := generation.source.AcquireSourceJoin(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		acquisition.context, acquisition.generation = owner, generation
		generation.joins[acquisition] = struct{}{}
		acquisition.opening = make(chan struct{})
		closed := make(chan error, 1)
		go func() { closed <- acquisition.Close() }()
		synctest.Wait()
		select {
		case <-closed:
			t.Fatal("Close did not join unpublished setup")
		default:
		}
		cause := errors.New("unpublished physical setup close failed")
		physical := newLifecycleConn(false)
		physical.closeFailure = cause
		returns := 0
		retirement := acquisition.cleanupJoin(nil, &retiredConn{Conn: physical}, nil, func() { returns++ })
		if !errors.Is(retirement, cause) {
			t.Fatal("immediate setup cleanup lost failure", retirement)
		}
		close(acquisition.opening)
		first := <-closed
		if !errors.Is(first, cause) || acquisition.Close() != first || returns != 1 {
			t.Fatal("joined acquisition discarded or replaced setup cleanup failure", first, returns)
		}
		if _, err := owner.Open(t.Context()); !errors.Is(err, cause) {
			t.Fatal("cleanup failure permitted context reopen", err)
		}
		firstContext := owner.Close()
		if !errors.Is(firstContext, cause) || owner.Close() != firstContext || returns != 1 {
			t.Fatal("context lost setup terminal failure or released twice", firstContext, returns)
		}
	})
}

func TestJoinAcquisitionCleanSetupRetirementDoesNotSealContext(t *testing.T) {
	owner, generation := joinContextLifetime(t)
	generation.source = joinContextRetiringPrefix(t, generation.ctx, nil)
	generation.source.config.Leg.EntryMember.RoleDomain = 1
	generation.ready = true
	close(generation.opened)
	acquisition, err := generation.source.AcquireSourceJoin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	acquisition.context, acquisition.generation = owner, generation
	generation.joins[acquisition] = struct{}{}
	returns := 0
	if err := acquisition.cleanupJoin(nil, nil, nil, func() { returns++ }); err != nil {
		t.Fatal(err)
	}
	if err := acquisition.Close(); err != nil || owner.failure != nil || generation.closing || returns != 1 {
		t.Fatal("clean setup retirement fabricated terminal context failure", err, owner.failure, returns)
	}
	if err := owner.currentLocked(generation); err != nil {
		t.Fatal("clean original retirement denied local context", err)
	}
	if err := owner.Close(); err != nil || returns != 1 {
		t.Fatal("clean context joined incorrectly", err, returns)
	}
}
