package route

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

func closedAdmissionClaimFixture(t *testing.T) (*ClosedDutyLimits, ClosedAdmission, time.Time) {
	t.Helper()
	now := time.Unix(1_800_000_000, 0).UTC()
	limits, err := NewClosedDutyLimits(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	reservation, err := limits.reserveChannel()
	if err != nil {
		t.Fatal(err)
	}
	return limits, ClosedAdmission{Class: 2, Bytes: 32 << 20, Deadline: now.Add(time.Minute), claim: newClosedAdmissionClaim(reservation, nil)}, now
}

func closedAdmissionClaimCounts(limits *ClosedDutyLimits) (uint16, uint16) {
	limits.mu.Lock()
	defer limits.mu.Unlock()
	return limits.channels, limits.children
}

func TestClosedAdmissionCopiedHandleCannotReleaseForwardingOwner(t *testing.T) {
	limits, lease, now := closedAdmissionClaimFixture(t)
	stale := lease
	channel, err := newClosedForwardingChannel(&lease, func(ClosedOpen) error { return nil }, nil, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = channel.Cancel() }()
	if err := stale.Release(); err != nil {
		t.Fatal(err)
	}
	held := make([]*closedDutyChannel, 0, closedDutyChannels-1)
	for range closedDutyChannels - 1 {
		reservation, reserveErr := limits.reserveChannel()
		if reserveErr != nil {
			t.Fatal(reserveErr)
		}
		held = append(held, reservation)
	}
	defer func() {
		for _, reservation := range held {
			reservation.release()
		}
	}()
	if unexpected, reserveErr := limits.reserveChannel(); reserveErr == nil {
		unexpected.release()
		t.Fatal("stale copied admission released the live forwarding reservation")
	}
}

func TestClosedAdmissionMintedCopyLeavesForwardingOwnerLive(t *testing.T) {
	_, _, lease, now := closedOuterAdmissionFixture(t)
	stale := *lease
	channel, err := newClosedForwardingChannel(lease, func(ClosedOpen) error { return nil }, nil, func() time.Time { return *now })
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = channel.Cancel() }()
	if err := stale.Release(); err != nil {
		t.Fatal(err)
	}
	open := ClosedOpen{NextNodeID: [32]byte{7}, NextDutyGeneration: 1, Purpose: ardp.PurposeForwarding, Deadline: now.Add(time.Minute)}
	body, err := EncodeClosedOpen(open)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = channel.Accept(ardp.Frame{Kind: ardp.KindOpen, Lane: 1, Body: body}); err != nil {
		t.Fatalf("stale release prevented valid owner child admission: %v", err)
	}
	if event, ok := channel.NextAvailable(nil); !ok || event.Kind != ardp.KindOpen || event.Open != open {
		t.Fatalf("valid owner did not receive its accepted child: %+v / %t", event, ok)
	}
}

func TestClosedAdmissionStaleCopyCannotReleaseHostReservation(t *testing.T) {
	_, lease, now := closedAdmissionClaimFixture(t)
	released := 0
	lease.claim.release = func() error { released++; return nil }
	stale := lease
	channel, err := newClosedForwardingChannel(&lease, func(ClosedOpen) error { return nil }, nil, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if err := stale.Release(); err != nil || released != 0 {
		t.Fatalf("stale copy released live host reservation: %v / %d", err, released)
	}
	if err := channel.Cancel(); err != nil || released != 1 {
		t.Fatalf("forwarding cleanup host release = %v / %d", err, released)
	}
}

func TestClosedAdmissionReleaseBeforeTransferReturnsCapacityOnce(t *testing.T) {
	limits, lease, now := closedAdmissionClaimFixture(t)
	var released atomic.Int32
	lease.claim.release = func() error { released.Add(1); return nil }
	if err := lease.Release(); err != nil {
		t.Fatal(err)
	}
	if released.Load() != 1 {
		t.Fatalf("release callback count = %d", released.Load())
	}
	if _, err := newClosedForwardingChannel(&lease, func(ClosedOpen) error { return nil }, nil, func() time.Time { return now }); err == nil {
		t.Fatal("released admission transferred")
	}
	reservation, err := limits.reserveChannel()
	if err != nil {
		t.Fatalf("released channel capacity did not return: %v", err)
	}
	reservation.release()
}

func TestClosedAdmissionCopiedHandleCannotTransferTwiceWhileOwnerLives(t *testing.T) {
	_, lease, now := closedAdmissionClaimFixture(t)
	stale := lease
	channel, err := newClosedForwardingChannel(&lease, func(ClosedOpen) error { return nil }, nil, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = channel.Cancel() }()
	if _, err := newClosedForwardingChannel(&stale, func(ClosedOpen) error { return nil }, nil, func() time.Time { return now }); err == nil {
		t.Fatal("copied handle created a second forwarding owner")
	}
	open := ClosedOpen{NextNodeID: [32]byte{8}, NextDutyGeneration: 1, Purpose: ardp.PurposeForwarding, Deadline: now.Add(time.Minute)}
	body, err := EncodeClosedOpen(open)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = channel.Accept(ardp.Frame{Kind: ardp.KindOpen, Lane: 1, Body: body}); err != nil {
		t.Fatalf("first owner lost child capacity: %v", err)
	}
}

func TestClosedAdmissionCopiesHaveOneTransferOrReleaseWinner(t *testing.T) {
	limits, lease, now := closedAdmissionClaimFixture(t)
	var released atomic.Int32
	lease.claim.release = func() error { released.Add(1); return nil }
	first, second := lease, lease
	start := make(chan struct{})
	var group sync.WaitGroup
	group.Add(2)
	channels := make(chan *ClosedForwardingChannel, 1)
	go func() {
		defer group.Done()
		<-start
		channel, err := newClosedForwardingChannel(&first, func(ClosedOpen) error { return nil }, nil, func() time.Time { return now })
		if err == nil {
			channels <- channel
		}
	}()
	go func() {
		defer group.Done()
		<-start
		if err := second.Release(); err != nil {
			t.Error(err)
		}
	}()
	close(start)
	group.Wait()
	close(channels)
	var owner *ClosedForwardingChannel
	for channel := range channels {
		owner = channel
	}
	if owner != nil {
		if released.Load() != 0 {
			t.Fatalf("transfer winner released host reservation: %d", released.Load())
		}
		open := ClosedOpen{NextNodeID: [32]byte{9}, NextDutyGeneration: 1, Purpose: ardp.PurposeForwarding, Deadline: now.Add(time.Minute)}
		body, err := EncodeClosedOpen(open)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = owner.Accept(ardp.Frame{Kind: ardp.KindOpen, Lane: 1, Body: body}); err != nil {
			t.Fatalf("transfer winner cannot admit child work: %v", err)
		}
		if channels, children := closedAdmissionClaimCounts(limits); channels != 1 || children != 1 {
			t.Fatalf("live owner counters = channels %d, children %d", channels, children)
		}
		if err := owner.Cancel(); err != nil {
			t.Fatal(err)
		}
	} else {
		if released.Load() != 1 {
			t.Fatalf("release winner count = %d", released.Load())
		}
		if channels, children := closedAdmissionClaimCounts(limits); channels != 0 || children != 0 {
			t.Fatalf("release winner counters = channels %d, children %d", channels, children)
		}
	}
	if _, err := newClosedForwardingChannel(&lease, func(ClosedOpen) error { return nil }, nil, func() time.Time { return now }); err == nil {
		t.Fatal("a second copied handle transferred the reservation")
	}
	if released.Load() != 1 {
		t.Fatalf("host reservation was not released exactly once: %d", released.Load())
	}
	if channels, children := closedAdmissionClaimCounts(limits); channels != 0 || children != 0 {
		t.Fatalf("terminal counters = channels %d, children %d", channels, children)
	}
}
