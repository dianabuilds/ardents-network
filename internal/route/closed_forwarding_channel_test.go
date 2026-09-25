package route

import (
	"bytes"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

func newForwardingTestChannel(lease *ClosedAdmission, authorize ClosedForwardingAuthorizer, clock func() time.Time) (*ClosedForwardingChannel, error) {
	return NewReplenishableClosedForwardingChannel(lease, authorize, func(ClosedAdmissionVerification) (func() error, error) { return nil, nil }, clock)
}
func TestClosedForwardingChannelBoundsAuthorizedOddChild(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	limits, err := NewClosedDutyLimits(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	reservation, err := limits.reserveChannel()
	if err != nil {
		t.Fatal(err)
	}
	lease := ClosedAdmission{Class: 2, Bytes: 32 << 20, Deadline: now.Add(time.Minute), claim: newClosedAdmissionClaim(reservation, nil)}
	allowed := ClosedOpen{NextNodeID: [32]byte{1}, NextDutyGeneration: 2, Purpose: ardp.PurposeForwarding, Deadline: now.Add(30 * time.Second)}
	channel, err := newForwardingTestChannel(&lease, func(open ClosedOpen) error {
		if open != allowed {
			return errUnexpectedForwardOpen
		}
		return nil
	}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	lease.Release()
	body, err := EncodeClosedOpen(allowed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindOpen, Lane: 1, Body: body}); err != nil {
		t.Fatal(err)
	}
	if event, available := channel.NextAvailable(nil); !available || event.Kind != ardp.KindOpen || event.Open != allowed || event.Restriction != ClosedChildOrdinary {
		t.Fatalf("scheduled open = %+v / %t", event, available)
	}
	bytesFrame := ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: bytes.Repeat([]byte{3}, int(closedLaneCredit))}
	if _, err := channel.Accept(bytesFrame); err != nil {
		t.Fatal(err)
	}
	if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: []byte{1}}); err == nil {
		t.Fatal("accepted bytes beyond fixed credit")
	}
	if event, available := channel.NextAvailable(nil); !available || event.Kind != ardp.KindBytes || len(event.Bytes) != int(closedLaneCredit) {
		t.Fatalf("scheduled bytes = %+v / %t", event, available)
	}
	credit, err := channel.Credit(1, 1)
	if err != nil || credit.Kind != ardp.KindCredit {
		t.Fatalf("credit = %+v / %v", credit, err)
	}
	if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: []byte{1}}); err != nil {
		t.Fatal(err)
	}
	if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindOpen, Lane: 1, Body: body}); err == nil {
		t.Fatal("reused child lane")
	}
	if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindOpen, Lane: 2, Body: body}); err == nil {
		t.Fatal("accepted endpoint even lane")
	}
	if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindEOF, Lane: 1}); err != nil {
		t.Fatal(err)
	}
	if event, available := channel.NextAvailable(nil); !available || event.Kind != ardp.KindBytes || len(event.Bytes) != 1 {
		t.Fatalf("scheduled trailing bytes = %+v / %t", event, available)
	}
	if event, available := channel.NextAvailable(nil); !available || event.Kind != ardp.KindEOF {
		t.Fatalf("scheduled EOF = %+v / %t", event, available)
	}
	if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: []byte{1}}); err == nil {
		t.Fatal("accepted bytes after EOF")
	}
}

func TestClosedForwardingChannelSerializesConcurrentLaneReuse(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	limits, err := NewClosedDutyLimits(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	reservation, err := limits.reserveChannel()
	if err != nil {
		t.Fatal(err)
	}
	allowed := ClosedOpen{NextNodeID: [32]byte{11}, NextDutyGeneration: 12, Purpose: ardp.PurposeForwarding, Deadline: now.Add(time.Minute)}
	lease := ClosedAdmission{Class: 2, Bytes: 32 << 20, Deadline: now.Add(time.Minute), claim: newClosedAdmissionClaim(reservation, nil)}
	channel, err := newForwardingTestChannel(&lease,
		func(open ClosedOpen) error {
			if open != allowed {
				return errUnexpectedForwardOpen
			}
			return nil
		}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	body, err := EncodeClosedOpen(allowed)
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			_, err := channel.Accept(ardp.Frame{Kind: ardp.KindOpen, Lane: 1, Body: body})
			errs <- err
		}()
	}
	group.Wait()
	close(errs)
	succeeded := 0
	for err := range errs {
		if err == nil {
			succeeded++
		}
	}
	if succeeded != 1 {
		t.Fatalf("concurrent lane reuse successes = %d", succeeded)
	}
	channel.Cancel()
}

func TestClosedForwardingChannelBoundsOnePrefixQueueBeforeDutyQueue(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	limits, err := NewClosedDutyLimits(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	reservation, err := limits.reserveChannel()
	if err != nil {
		t.Fatal(err)
	}
	lease := ClosedAdmission{Class: 2, Bytes: 32 << 20, Deadline: now.Add(time.Minute), claim: newClosedAdmissionClaim(reservation, nil)}
	allowed := ClosedOpen{NextNodeID: [32]byte{21}, NextDutyGeneration: 22, Purpose: ardp.PurposeForwarding, Deadline: now.Add(time.Minute)}
	channel, err := newForwardingTestChannel(&lease, func(open ClosedOpen) error {
		if open != allowed {
			return errUnexpectedForwardOpen
		}
		return nil
	}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	defer channel.Cancel()
	openBody, err := EncodeClosedOpen(allowed)
	if err != nil {
		t.Fatal(err)
	}
	frame := ardp.Frame{Kind: ardp.KindBytes, Body: bytes.Repeat([]byte{4}, int(closedLaneCredit))}
	for index := uint32(0); index < uint32(closedPrefixQueueBytes/closedLaneCredit); index++ {
		lane := index*2 + 1
		if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindOpen, Lane: lane, Body: openBody}); err != nil {
			t.Fatalf("open lane %d: %v", lane, err)
		}
		frame.Lane = lane
		if _, err := channel.Accept(frame); err != nil {
			t.Fatalf("queue lane %d: %v", lane, err)
		}
	}
	lane := uint32(closedPrefixQueueBytes/closedLaneCredit)*2 + 1
	if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindOpen, Lane: lane, Body: openBody}); err != nil {
		t.Fatal(err)
	}
	frame.Lane = lane
	if _, err := channel.Accept(frame); err == nil {
		t.Fatal("accepted bytes over the forwarding prefix queue")
	}
}

func TestClosedForwardingChannelSchedulesControlThenRoundRobinData(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	limits, err := NewClosedDutyLimits(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	reservation, err := limits.reserveChannel()
	if err != nil {
		t.Fatal(err)
	}
	lease := ClosedAdmission{Class: 2, Bytes: 32 << 20, Deadline: now.Add(time.Minute), claim: newClosedAdmissionClaim(reservation, nil)}
	allowed := ClosedOpen{NextNodeID: [32]byte{31}, NextDutyGeneration: 32, Purpose: ardp.PurposeForwarding, Deadline: now.Add(time.Minute)}
	channel, err := newForwardingTestChannel(&lease, func(open ClosedOpen) error {
		if open != allowed {
			return errUnexpectedForwardOpen
		}
		return nil
	}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	defer channel.Cancel()
	openBody, err := EncodeClosedOpen(allowed)
	if err != nil {
		t.Fatal(err)
	}
	for _, lane := range []uint32{1, 3} {
		if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindOpen, Lane: lane, Body: openBody}); err != nil {
			t.Fatal(err)
		}
	}
	for _, lane := range []uint32{1, 3} {
		event, available := channel.NextAvailable(nil)
		if !available || event.Kind != ardp.KindOpen || event.Lane != lane {
			t.Fatalf("control schedule = %+v / %t", event, available)
		}
	}
	for _, frame := range []ardp.Frame{
		{Kind: ardp.KindBytes, Lane: 1, Body: []byte{1}},
		{Kind: ardp.KindBytes, Lane: 3, Body: []byte{3}},
		{Kind: ardp.KindBytes, Lane: 1, Body: []byte{2}},
	} {
		if _, err := channel.Accept(frame); err != nil {
			t.Fatal(err)
		}
	}
	for _, want := range []struct {
		lane  uint32
		value byte
	}{{1, 1}, {3, 3}, {1, 2}} {
		event, available := channel.NextAvailable(nil)
		if !available || event.Kind != ardp.KindBytes || event.Lane != want.lane || len(event.Bytes) != 1 || event.Bytes[0] != want.value {
			t.Fatalf("round-robin schedule = %+v / %t", event, available)
		}
	}
	if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{5}}); err != nil {
		t.Fatal(err)
	}
	if event, available := channel.NextAvailable(nil); !available || event.Kind != ardp.KindClose || event.Lane != 1 || len(event.Bytes) != 1 || event.Bytes[0] != 5 {
		t.Fatalf("scheduled close = %+v / %t", event, available)
	}
}

func TestClosedForwardingChannelKeepsSkippedLaneAccounted(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	limits, err := NewClosedDutyLimits(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	reservation, err := limits.reserveChannel()
	if err != nil {
		t.Fatal(err)
	}
	lease := ClosedAdmission{Class: 2, Bytes: 32 << 20, Deadline: now.Add(time.Minute), claim: newClosedAdmissionClaim(reservation, nil)}
	open := ClosedOpen{NextNodeID: [32]byte{51}, NextDutyGeneration: 52, Purpose: ardp.PurposeForwarding, Deadline: now.Add(time.Minute)}
	channel, err := newForwardingTestChannel(&lease, func(value ClosedOpen) error {
		if value != open {
			return errUnexpectedForwardOpen
		}
		return nil
	}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	defer channel.Cancel()
	body, err := EncodeClosedOpen(open)
	if err != nil {
		t.Fatal(err)
	}
	for _, lane := range []uint32{1, 3} {
		if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindOpen, Lane: lane, Body: body}); err != nil {
			t.Fatal(err)
		}
		channel.NextAvailable(nil)
	}
	if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: []byte{1}}); err != nil {
		t.Fatal(err)
	}
	if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindBytes, Lane: 3, Body: []byte{3}}); err != nil {
		t.Fatal(err)
	}

	if event, ok := channel.NextAvailable(func(event ClosedForwardingEvent) bool { return event.Lane != 1 }); !ok || event.Kind != ardp.KindBytes || event.Lane != 3 || event.Bytes[0] != 3 {
		t.Fatalf("allowed B bytes = %+v / %t", event, ok)
	}
	if _, err := channel.Credit(3, 1); err != nil {
		t.Fatal(err)
	}
	if channel.queued != 1 {
		t.Fatalf("skipped A bytes lost accounting: %d", channel.queued)
	}
	if event, ok := channel.NextAvailable(nil); !ok || event.Kind != ardp.KindBytes || event.Lane != 1 || event.Bytes[0] != 1 {
		t.Fatalf("skipped A bytes = %+v / %t", event, ok)
	}
}

func TestClosedForwardingChannelRetains256ReadyLanesInRoundRobin(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	limits, err := NewClosedDutyLimits(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	reservation, err := limits.reserveChannel()
	if err != nil {
		t.Fatal(err)
	}
	lease := ClosedAdmission{Class: 2, Bytes: 32 << 20, Deadline: now.Add(time.Minute), claim: newClosedAdmissionClaim(reservation, nil)}
	allowed := ClosedOpen{NextNodeID: [32]byte{41}, NextDutyGeneration: 42, Purpose: ardp.PurposeForwarding, Deadline: now.Add(time.Minute)}
	channel, err := newForwardingTestChannel(&lease, func(open ClosedOpen) error {
		if open != allowed {
			return errUnexpectedForwardOpen
		}
		return nil
	}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	defer channel.Cancel()
	open, err := EncodeClosedOpen(allowed)
	if err != nil {
		t.Fatal(err)
	}
	for lane := uint32(1); lane <= 2*closedForwardChildren-1; lane += 2 {
		if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindOpen, Lane: lane, Body: open}); err != nil {
			t.Fatalf("open lane %d: %v", lane, err)
		}
		event, available := channel.NextAvailable(nil)
		if !available || event.Kind != ardp.KindOpen || event.Lane != lane {
			t.Fatalf("open schedule for lane %d = %+v / %t", lane, event, available)
		}
	}
	if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindOpen, Lane: 2*closedForwardChildren + 1, Body: open}); err == nil {
		t.Fatal("accepted a 257th forwarding lane")
	}
	for lane := uint32(1); lane <= 2*closedForwardChildren-1; lane += 2 {
		if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindBytes, Lane: lane, Body: []byte{byte(lane)}}); err != nil {
			t.Fatalf("queue lane %d: %v", lane, err)
		}
	}
	for lane := uint32(1); lane <= 2*closedForwardChildren-1; lane += 2 {
		event, available := channel.NextAvailable(nil)
		if !available || event.Kind != ardp.KindBytes || event.Lane != lane || len(event.Bytes) != 1 || event.Bytes[0] != byte(lane) {
			t.Fatalf("data schedule for lane %d = %+v / %t", lane, event, available)
		}
	}
}

func TestClosedForwardingChannelRefillDebitsOldReserveAndRetainsHostRelease(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	limits, err := NewClosedDutyLimits(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	reservation, err := limits.reserveChannel()
	if err != nil {
		t.Fatal(err)
	}
	lease := ClosedAdmission{Class: 2, Bytes: 32 << 20, Deadline: now.Add(time.Minute), hello: ardp.Hello{ChannelNonce: [32]byte{9}}, exporter: [32]byte{8}, claim: newClosedAdmissionClaim(reservation, nil)}
	called, released := 0, 0
	channel, err := NewReplenishableClosedForwardingChannel(&lease, func(ClosedOpen) error { return nil }, func(input ClosedAdmissionVerification) (func() error, error) {
		called++
		if input.Class != 2 || input.Deadline != now.Add(time.Minute) || input.Hello.ChannelNonce != [32]byte{9} || input.Exporter != [32]byte{8} {
			t.Fatalf("refill binding differs: %+v", input)
		}
		return func() error { released++; return nil }, nil
	}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	initialLimit := channel.byteLimit
	admit := ardp.Frame{Kind: ardp.KindAdmit, Body: append([]byte{2}, bytes.Repeat([]byte{7}, 354)...)}
	if _, err := channel.Accept(admit); err != nil {
		t.Fatal(err)
	}
	if called != 1 || channel.byteLimit-channel.usedBytes != 32<<20 || channel.byteLimit <= initialLimit {
		t.Fatalf("refill did not set one exact remaining reserve: called=%d limit=%d used=%d", called, channel.byteLimit, channel.usedBytes)
	}
	if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindAdmit, Lane: 1, Body: admit.Body}); err == nil {
		t.Fatal("child lane replenishment was accepted")
	}
	if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindAdmit, Body: append([]byte{1}, bytes.Repeat([]byte{7}, 354)...)}); err == nil {
		t.Fatal("non-forward token replenished parent")
	}
	if err := channel.Cancel(); err != nil || released != 1 {
		t.Fatalf("parent release = %v / %d", err, released)
	}
}

var errUnexpectedForwardOpen = &unexpectedForwardOpen{}

type unexpectedForwardOpen struct{}

func (*unexpectedForwardOpen) Error() string { return "unexpected forwarding OPEN" }
