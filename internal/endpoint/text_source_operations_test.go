//go:build linux

package endpoint

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

func TestTextSourcePreparationFailureRetainsStageAndCause(t *testing.T) {
	cause := errors.New("opening issuance refused")
	failure := textSourcePreparationFailureAt("issuance", cause)
	if got := textSourcePreparationFailureStage(failure); got != "issuance" {
		t.Fatalf("source preparation stage = %q", got)
	}
	if !errors.Is(failure, cause) {
		t.Fatal("source preparation failure lost its cause")
	}
}

func TestTextPrefixPreparationFailureRetainsStageAndCause(t *testing.T) {
	cause := errors.New("source carrier refused")
	failure := textPrefixPreparationFailureAt("opening", cause)
	if got := textPrefixPreparationFailureStage(failure); got != "opening" {
		t.Fatalf("prefix preparation stage = %q", got)
	}
	if !errors.Is(failure, cause) {
		t.Fatal("prefix preparation failure lost its cause")
	}
}

func TestTextTokenPresentationFailureRetainsNestedStageAndCause(t *testing.T) {
	cause := errors.New("local role conflict read unavailable")
	role := textRoleMemberFailureAt("conflict-read", cause)
	selection := textSourceSelectionFailureAt("role-members-"+textRoleMemberFailureStage(role), role)
	failure := textTokenPresentationFailureAt("selection-"+textSourceSelectionFailureStage(selection), selection)
	if got := textTokenPresentationFailureStage(failure); got != "selection-role-members-conflict-read" {
		t.Fatalf("token presentation stage = %q", got)
	}
	if !errors.Is(failure, cause) {
		t.Fatal("token presentation failure lost its cause")
	}
	transfer := textTokenTransferFailureAt("journal", cause)
	if got := textTokenTransferFailureStage(transfer); got != "journal" || !errors.Is(transfer, cause) {
		t.Fatalf("token transfer failure = %q, %v", got, transfer)
	}
}

func TestTextTokenPresentationClassifiesConcurrentRoleCommit(t *testing.T) {
	endpoint, owner, source := textSourceContextFixture(t)
	prepareTextIssuancePermission(t, owner, source)
	selection := selectTextSource(t, owner)
	writer, err := duty.Open(duty.Config{Root: endpoint.closedRoleRoot, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()

	attempt, cancel := context.WithCancel(t.Context())
	flight := &textPrefixOpeningOperation{owner: owner, context: attempt, cancelOperation: cancel, done: make(chan struct{})}
	owner.mu.Lock()
	owner.source.opening = flight
	profile := owner.permission.profile
	owner.mu.Unlock()
	defer func() {
		owner.mu.Lock()
		owner.source.opening = nil
		owner.mu.Unlock()
		cancel()
		close(flight.done)
	}()
	hello := ardp.Hello{NetworkID: profile.NetworkID, StateGeneration: profile.StateGeneration, StateDigest: profile.StateDigest,
		ProfileDigest: profile.Digest, RecipientNodeID: selection.EntryNodeID, RecipientDutyGeneration: source.view.Nodes[0].DutyGeneration,
		Purpose: ardp.PurposeForwarding, Deadline: time.Now().Add(10 * time.Second), ChannelNonce: fixtureID(199)}
	started := time.Now()
	_, err = flight.presentTextToken(selection, hello, 2)
	if got := textTokenPresentationFailureStage(err); got != "selection-role-members-conflict-read" {
		t.Fatalf("concurrent role commit stage = %q: %v", got, err)
	}
	if elapsed := time.Since(started); elapsed < 900*time.Millisecond || elapsed > 2*time.Second {
		t.Fatalf("bounded conflict read took %s", elapsed)
	}
}

func TestTextRefreshWaitsForActualSourceUse(t *testing.T) {
	for _, carrier := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			_, owner, _ := textJoinedNetworkFixture(t, carrier)
			release, err := owner.acquireTextSourceOperation(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			held := true
			defer func() {
				if held {
					release()
				}
			}()
			owner.mu.Lock()
			first, refresh := owner.registration, owner.refresh.current()
			first.refreshAt = time.Now().Add(-time.Second)
			owner.signalTextRegistrationsLocked()
			owner.mu.Unlock()
			// The scheduler may begin, but cannot treat legitimate Source ownership
			// as lost publication authority. Issue real tokens under that reservation.
			select {
			case <-refresh.done:
				t.Fatal("refresh ended while Source was in use")
			case <-time.After(40 * time.Millisecond):
			}
			selection := selectTextSource(t, owner)
			if err := owner.issueTextTokensForOpening(t.Context(), [][32]byte{selection.EntryNodeID}, 2, nil, false); err != nil {
				t.Fatal(err)
			}
			release()
			held = false
			waitTextRefreshCondition(t, owner, func() bool {
				return owner.registration != nil && owner.registration != first && !owner.registration.refreshAt.IsZero()
			})
			owner.mu.Lock()
			valid := owner.previousRegistration == first && owner.refresh.outcome(refresh) == nil && owner.permission.batches == 2
			owner.mu.Unlock()
			if !valid {
				t.Fatal("refresh lost original registration or repeated bootstrap")
			}
		})
	}
}

func TestTextSourceWaitCancellationDoesNotStealReservation(t *testing.T) {
	for _, revoke := range []bool{false, true} {
		name := "caller"
		if revoke {
			name = "context"
		}
		t.Run(name, func(t *testing.T) {
			_, owner, _ := textSourceContextFixture(t)
			release, err := owner.acquireTextSourceOperation(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			result := make(chan error, 1)
			go func() {
				unlock, err := owner.acquireTextSourceOperation(ctx)
				if unlock != nil {
					unlock()
				}
				result <- err
			}()
			select {
			case <-result:
				t.Fatal("waiter acquired another owner's reservation")
			case <-time.After(20 * time.Millisecond):
			}
			if revoke {
				if err := owner.Close(); err != nil {
					t.Fatal(err)
				}
			} else {
				cancel()
			}
			select {
			case err := <-result:
				if err == nil {
					t.Fatal("cancelled waiter succeeded")
				}
			case <-time.After(time.Second):
				t.Fatal("cancelled waiter did not join")
			}
			owner.mu.Lock()
			occupied := len(owner.sourceOperations) == 1
			owner.mu.Unlock()
			if !occupied {
				t.Fatal("cancelled waiter released the active owner's reservation")
			}
		})
	}
}
