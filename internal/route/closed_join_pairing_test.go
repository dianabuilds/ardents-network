//go:build linux

package route

import (
	"bytes"
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type closedJoinFixture struct {
	now      time.Time
	clock    atomic.Int64
	receiver ClosedRoleReceiver
	limits   *ClosedDutyLimits
	spends   *ClosedSpendLedger
	pairs    *ClosedJoinPairs
	nonce    byte
}

// Real admission, spend, duty and pairing owners; crypto/exporter and time are
// explicit fixtures. These tests do not prove live Node/TLS/data integration.
func newClosedJoinFixture(t *testing.T) *closedJoinFixture {
	t.Helper()
	f := &closedJoinFixture{now: time.Now().UTC().Truncate(time.Second)}
	f.clock.Store(f.now.Unix())
	clock := func() time.Time { return time.Unix(f.clock.Load(), 0).UTC() }
	f.receiver = ClosedRoleReceiver{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3}, ProfileDigest: [32]byte{4}, NodeID: [32]byte{5}, RecordDigest: [32]byte{6}, DutyGeneration: 6,
		RoleDomain: 2, Subrole: 4, ExpectedPurpose: ClosedPurposeDataJoin, NotAfter: f.now.Add(time.Hour)}
	var err error
	f.spends, err = OpenClosedSpendLedger(t.TempDir(), ClosedSpendBinding{NetworkID: f.receiver.NetworkID, ProfileDigest: f.receiver.ProfileDigest, ReceiverNodeID: f.receiver.NodeID, ReceiverDutyGeneration: 6})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := f.spends.Close(); err != nil {
			t.Error(err)
		}
	})
	f.limits, err = NewClosedDutyLimits(clock)
	if err != nil {
		t.Fatal(err)
	}
	f.pairs, err = NewClosedJoinPairs(f.receiver, f.limits)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(f.pairs.Close)
	return f
}

func (f *closedJoinFixture) admission(t *testing.T, class uint8) *ClosedAdmission {
	t.Helper()
	f.nonce++
	clock := func() time.Time { return time.Unix(f.clock.Load(), 0).UTC() }
	channel, err := NewClosedAdmissionChannel(f.receiver, f.spends, f.limits,
		func(string, []byte, int) ([]byte, error) { return bytes.Repeat([]byte{9}, 32), nil },
		func(ClosedAdmissionVerification) (time.Time, error) { return clock().Truncate(time.Hour), nil }, clock)
	if err != nil {
		t.Fatal(err)
	}
	r := f.receiver
	hello := ClosedHello{NetworkID: r.NetworkID, StateGeneration: r.StateGeneration, StateDigest: r.StateDigest, ProfileDigest: r.ProfileDigest, RecipientNodeID: r.NodeID, RecipientDutyGeneration: r.DutyGeneration,
		Purpose: ClosedPurposeDataJoin, ChannelNonce: [32]byte{f.nonce, 7}, Deadline: f.now.Add(time.Minute)}
	body, err := EncodeClosedHello(hello)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := channel.Accept(ClosedLaneFrame{Kind: closedFrameHello, Body: body}); err != nil {
		t.Fatal(err)
	}
	lease, err := channel.Accept(ClosedLaneFrame{Kind: closedFrameAdmit, Body: append([]byte{class}, bytes.Repeat([]byte{f.nonce}, 354)...)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(lease.Release)
	return &lease
}

func (f *closedJoinFixture) join(t *testing.T, role uint8, contextByte byte, seconds int) (*ClosedJoinSide, error) {
	t.Helper()
	lease := f.admission(t, 2)
	defer lease.Release()
	raw, err := EncodeClosedJoinRequest(ClosedJoinRequest{Nonce: [32]byte{f.nonce, 8}, Secret: [32]byte{20}, Context: [32]byte{contextByte}, Side: role, Deadline: f.now.Add(time.Duration(seconds) * time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	side, err := f.pairs.Reserve(lease, ClosedLaneFrame{Kind: closedFrameOperation, Lane: 1, Body: raw})
	if err == nil {
		t.Cleanup(side.Close)
	}
	return side, err
}

func TestClosedJoinPairingKeepsLocalResultsAndOriginalAdmission(t *testing.T) {
	f := newClosedJoinFixture(t)
	first, err := f.join(t, 1, 21, 10)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.Result(); err == nil {
		t.Fatal("result before counterpart")
	}
	if err := first.ConfirmResult(); err == nil {
		t.Fatal("confirmation without result")
	}
	if _, err := f.join(t, 1, 21, 10); err == nil {
		t.Fatal("duplicate side admitted")
	}
	if _, err := f.join(t, 2, 22, 10); err == nil {
		t.Fatal("different handshake context paired")
	}
	select {
	case <-first.Done():
		t.Fatal("refused candidate canceled retained side")
	default:
	}
	second, err := f.join(t, 2, 21, 10)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.join(t, 2, 21, 10); err == nil {
		t.Fatal("third side admitted")
	}
	for _, side := range []*ClosedJoinSide{first, second} {
		if err := side.WaitPair(t.Context()); err != nil {
			t.Fatal(err)
		}
		raw, err := side.Result()
		if err != nil {
			t.Fatal(err)
		}
		if status, err := DecodeClosedJoinResult(raw, side.nonce); err != nil || status != 0 {
			t.Fatal("result lost local nonce")
		}
		if _, err := side.Result(); err == nil {
			t.Fatal("duplicate result produced")
		}
	}
	if first.nonce == second.nonce {
		t.Fatal("fixture did not separate local nonces")
	}
	if err := first.ConfirmResult(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-first.pair.dataReady:
		t.Fatal("one result activated data")
	default:
	}
	if err := second.ConfirmResult(); err != nil {
		t.Fatal(err)
	}
	if err := second.ConfirmResult(); err == nil {
		t.Fatal("confirmation reused")
	}
	f.clock.Store(f.now.Add(11 * time.Second).Unix())
	if err := first.WaitData(t.Context()); err != nil {
		t.Fatalf("setup deadline truncated data admission: %v", err)
	}
	if f.limits.channels != 2 || f.limits.children != 2 {
		t.Fatalf("reservation transferred incorrectly: %d/%d", f.limits.channels, f.limits.children)
	}
	f.clock.Store(f.now.Add(time.Minute).Unix())
	if err := second.WaitData(t.Context()); err == nil {
		t.Fatal("pair outlived original admission")
	}
	first.Close()
	if f.limits.channels != 1 {
		t.Fatal("closing one side released partner reservation")
	}
	second.Close()
	if f.limits.channels != 0 || f.limits.children != 0 || len(f.pairs.entries) != 0 {
		t.Fatal("joined close leaked reservations")
	}
}

func TestClosedJoinPairingExpiresWithoutReleasingLiveHandler(t *testing.T) {
	f := newClosedJoinFixture(t)
	side, err := f.join(t, 1, 21, 1)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-side.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("unpaired timer did not cancel")
	}
	if f.limits.channels != 1 {
		t.Fatal("timer released handler reservation before close")
	}
	if _, err := side.Result(); err == nil {
		t.Fatal("expired side obtained result")
	}
	side.Close()
	if f.limits.channels != 0 {
		t.Fatal("expired handler did not release")
	}
}

func TestClosedJoinPairingCloseWaitsForHandlers(t *testing.T) {
	f := newClosedJoinFixture(t)
	side, err := f.join(t, 1, 21, 10)
	if err != nil {
		t.Fatal(err)
	}
	closed := make(chan struct{})
	go func() { f.pairs.Close(); close(closed) }()
	select {
	case <-side.Done():
	case <-time.After(time.Second):
		t.Fatal("close did not cancel side")
	}
	select {
	case <-closed:
		t.Fatal("close returned before handler joined")
	default:
	}
	side.Close()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("close did not join returned handler")
	}
}

func TestClosedJoinPairingCancellationRefusesUnconfirmedData(t *testing.T) {
	f := newClosedJoinFixture(t)
	first, err := f.join(t, 1, 21, 10)
	if err != nil {
		t.Fatal(err)
	}
	second, err := f.join(t, 2, 21, 10)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := first.WaitData(ctx); err == nil {
		t.Fatal("unconfirmed pair returned data")
	}
	select {
	case <-second.Done():
	default:
		t.Fatal("cancellation left counterpart live")
	}
	if _, err := second.Result(); err == nil {
		t.Fatal("canceled pair produced result")
	}
}

func TestClosedJoinPairingCancellationAfterReadyStopsPartner(t *testing.T) {
	for attempt := 0; attempt < 32; attempt++ {
		f := newClosedJoinFixture(t)
		first, err := f.join(t, 1, 21, 10)
		if err != nil {
			t.Fatal(err)
		}
		second, err := f.join(t, 2, 21, 10)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if err := first.WaitPair(ctx); err == nil {
			t.Fatal("canceled ready wait succeeded")
		}
		select {
		case <-second.Done():
		default:
			t.Fatal("ready selection bypassed cancellation of partner")
		}
		first.Close()
		second.Close()
	}
}

func TestClosedJoinPairingCloseJoinsStartedTimer(t *testing.T) {
	f := newClosedJoinFixture(t)
	started, release := make(chan struct{}), make(chan struct{})
	f.pairs.mu.Lock()
	timer := f.pairs.schedule(0, func() { close(started); <-release })
	f.pairs.mu.Unlock()
	<-started
	timer.Stop()
	finished := make(chan struct{})
	go func() { f.pairs.Close(); close(finished) }()
	select {
	case <-finished:
		close(release)
		t.Fatal("close returned with a started callback outstanding")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("close failed to join completed callback")
	}
}

func TestClosedJoinPairingRefusalRetainsOriginalAdmission(t *testing.T) {
	for _, reason := range []string{"lane", "class", "profile", "capacity", "expired"} {
		t.Run(reason, func(t *testing.T) {
			f := newClosedJoinFixture(t)
			first, err := f.join(t, 1, 21, 10)
			if err != nil {
				t.Fatal(err)
			}
			class := uint8(2)
			if reason == "class" {
				class = 1
			}
			lease := f.admission(t, class)
			raw, err := EncodeClosedJoinRequest(ClosedJoinRequest{Nonce: [32]byte{90}, Secret: [32]byte{20}, Context: [32]byte{21}, Side: 2, Deadline: f.now.Add(10 * time.Second)})
			if err != nil {
				t.Fatal(err)
			}
			frame := ClosedLaneFrame{Kind: closedFrameOperation, Lane: 1, Body: raw}
			switch reason {
			case "lane":
				frame.Lane = 3
			case "profile":
				lease.hello.ProfileDigest = [32]byte{99}
			case "capacity":
				// Saturate the actual aggregate child owner using real held channels.
				for i := 0; i < 4; i++ {
					held, err := f.limits.reserveChannel()
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(held.release)
					count := 256
					if i == 3 {
						count = 255
					}
					for j := 0; j < count; j++ {
						if err := held.reserveChild(); err != nil {
							t.Fatal(err)
						}
					}
				}
			case "expired":
				f.clock.Store(f.now.Add(10 * time.Second).Unix())
			}
			original := lease.duty
			if side, err := f.pairs.Reserve(lease, frame); err == nil {
				side.Close()
				t.Fatal("invalid reservation accepted")
			}
			if lease.duty != original || original.released {
				t.Fatal("refusal consumed caller admission")
			}
			if reason != "expired" {
				select {
				case <-first.Done():
					t.Fatal("refusal canceled valid first side")
				default:
				}
			}
		})
	}
}
