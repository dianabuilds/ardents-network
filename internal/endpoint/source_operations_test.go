//go:build linux

package endpoint

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint/introduction"
	"github.com/dianabuilds/ardents-network/internal/endpoint/source"
	"github.com/dianabuilds/ardents-network/internal/endpoint/tokens"
	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
)

func TestTextTokenPresentationFailureRetainsNestedStageAndCause(t *testing.T) {
	cause := errors.New("local role conflict read unavailable")
	role := roleMemberFailureAt("conflict-read", cause)
	selection := interiorSelectionFailureAt("role-members-"+roleMemberFailureStage(role), role)
	failure := source.TokenPresentationFailureAt("selection-"+interiorSelectionFailureStage(selection), selection)
	if got := source.TokenPresentationFailureStage(failure); got != "selection-role-members-conflict-read" {
		t.Fatalf("token presentation stage = %q", got)
	}
	if !errors.Is(failure, cause) {
		t.Fatal("token presentation failure lost its cause")
	}
	transfer := tokens.TransferFailureAt("journal", cause)
	if got := tokens.TransferFailureStage(transfer); got != "journal" || !errors.Is(transfer, cause) {
		t.Fatalf("token transfer failure = %q, %v", got, transfer)
	}
}

func TestTextTokenPresentationClassifiesConcurrentRoleCommit(t *testing.T) {
	endpoint, owner, sourceState := sourceContextFixture(t)
	prepareIssuancePermission(t, owner, sourceState)
	selection := selectSource(t, owner)
	writer, err := duty.Open(duty.Config{Root: endpoint.closedRoleRoot, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()

	attempt, cancel := context.WithCancel(t.Context())
	flight := &operationFlight{owner: owner, context: attempt, cancelOperation: cancel, done: make(chan struct{})}
	owner.mu.Lock()
	if !owner.source.ReserveOpeningLocked(flight) {
		owner.mu.Unlock()
		t.Fatal("Source lifecycle refused the planted opening")
	}
	profile := owner.tokens.Permission.Profile
	owner.mu.Unlock()
	defer func() {
		owner.mu.Lock()
		owner.source.FinishOpeningLocked(flight, nil, nil, false)
		owner.mu.Unlock()
		cancel()
		close(flight.done)
	}()
	hello := ardp.Hello{NetworkID: profile.NetworkID, StateGeneration: profile.StateGeneration, StateDigest: profile.StateDigest,
		ProfileDigest: profile.Digest, RecipientNodeID: selection.EntryNodeID, RecipientDutyGeneration: sourceState.view.Nodes[0].DutyGeneration,
		Purpose: ardp.PurposeForwarding, Deadline: time.Now().Add(10 * time.Second), ChannelNonce: fixtureID(199)}
	started := time.Now()
	_, err = flight.presentToken(selection, hello, 2)
	if got := source.TokenPresentationFailureStage(err); got != "selection-role-members-conflict-read" {
		t.Fatalf("concurrent role commit stage = %q: %v", got, err)
	}
	if elapsed := time.Since(started); elapsed < 900*time.Millisecond || elapsed > 2*time.Second {
		t.Fatalf("bounded conflict read took %s", elapsed)
	}
}

func TestTextRefreshWaitsForActualSourceUse(t *testing.T) {
	for _, carrier := range []routecarrier.CarrierProfile{routecarrier.ClosedCarrierTCP, routecarrier.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			_, owner, _ := joinedNetworkFixture(t, carrier)
			release, err := owner.acquireSourceOperation(t.Context())
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
			first, refresh := owner.publication.pair.CurrentLocked(), owner.publication.refresh.Current()
			introduction.ForceRefreshAt(first, time.Now().Add(-time.Second))
			owner.publication.signalRegistrationsLocked()
			owner.mu.Unlock()
			// The scheduler may begin, but cannot treat legitimate Source ownership
			// as lost publication authority. Issue real tokens under that reservation.
			select {
			case <-refresh.Done:
				t.Fatal("refresh ended while Source was in use")
			case <-time.After(40 * time.Millisecond):
			}
			selection := selectSource(t, owner)
			if err := owner.issueTokensForOpening(t.Context(), [][32]byte{selection.EntryNodeID}, 2, nil, false); err != nil {
				t.Fatal(err)
			}
			release()
			held = false
			waitRefreshCondition(t, owner, func() bool {
				current := owner.publication.pair.CurrentLocked()
				if current == nil || current == first {
					return false
				}
				scheduled, _ := current.RefreshScheduleLocked()
				return !scheduled.IsZero()
			})
			owner.mu.Lock()
			sourceOpsPrevious, _ := owner.publication.pair.PreviousLocked()
			valid := sourceOpsPrevious == first && owner.publication.refresh.Outcome(refresh) == nil && owner.tokens.Permission.Batches == 2
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
			_, owner, _ := sourceContextFixture(t)
			release, err := owner.acquireSourceOperation(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			result := make(chan error, 1)
			go func() {
				unlock, err := owner.acquireSourceOperation(ctx)
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
			occupied := len(owner.sourceOperations.busy) == 1
			owner.mu.Unlock()
			if !occupied {
				t.Fatal("cancelled waiter released the active owner's reservation")
			}
		})
	}
}
