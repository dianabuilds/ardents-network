package route

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

var retiredFrameBenchmarkResult bool

// BenchmarkClosedOuterRetiredLookup isolates the fixed retired-ID lookup from
// parsing, channel accounting, and the consumer's byte copy.
func BenchmarkClosedOuterRetiredLookup(b *testing.B) {
	for _, retired := range []int{0, 128, ClosedForwardChildren} {
		bridge := &ClosedOuterBridge{}
		for index := range retired {
			bridge.rememberRetired(uint32(index*2 + 1))
		}
		b.Run(fmt.Sprintf("retired=%d/miss", retired), func(b *testing.B) {
			frame := ardp.Frame{Kind: ardp.KindBytes, Lane: 2*ClosedForwardChildren + 1, Body: []byte{1}}
			b.ReportAllocs()
			for b.Loop() {
				retiredFrameBenchmarkResult = bridge.retiredFrame(frame)
			}
			if retiredFrameBenchmarkResult {
				b.Fatal("live lane matched retired history")
			}
		})
		if retired != 0 {
			b.Run(fmt.Sprintf("retired=%d/last-hit", retired), func(b *testing.B) {
				frame := ardp.Frame{Kind: ardp.KindBytes, Lane: uint32(retired*2 - 1), Body: []byte{1}}
				b.ReportAllocs()
				for b.Loop() {
					retiredFrameBenchmarkResult = bridge.retiredFrame(frame)
				}
				if !retiredFrameBenchmarkResult {
					b.Fatal("retired lane was not found")
				}
			})
		}
	}
}

// BenchmarkClosedOuterFrame includes admission accounting, opaque byte copies,
// the lane consumer, and returned credit. The parsed variant also includes the
// 16-byte ARDP header and body reader used by Node's outer Serve loop.
func BenchmarkClosedOuterFrame(b *testing.B) {
	for _, active := range []int{1, 64, ClosedForwardChildren} {
		for _, retired := range []int{0, 128, ClosedForwardChildren} {
			for _, bodySize := range []int{1, 1024, ardp.MaximumBodySize} {
				for _, parsed := range []bool{false, true} {
					mode := "decoded"
					if parsed {
						mode = "parsed"
					}
					name := fmt.Sprintf("%s/active=%d/retired=%d/body=%d", mode, active, retired, bodySize)
					b.Run(name, func(b *testing.B) {
						bridge, handshake, lanes, ids := outerFrameBenchmarkFixture(b, active, retired)
						defer func() { bridge.Close(); handshake.Close() }()
						body := bytes.Repeat([]byte{1}, bodySize)
						buffer := make([]byte, bodySize)
						frames := make([]ardp.Frame, active)
						raw := make([][]byte, active)
						for index, id := range ids {
							frames[index] = ardp.Frame{Kind: ardp.KindBytes, Lane: id, Body: body}
							if parsed {
								var err error
								raw[index], err = ardp.EncodeFrame(frames[index])
								if err != nil {
									b.Fatal(err)
								}
							}
						}
						var reader bytes.Reader
						index := 0
						b.SetBytes(int64(bodySize))
						b.ReportAllocs()
						b.ResetTimer()
						for b.Loop() {
							frame := frames[index]
							if parsed {
								reader.Reset(raw[index])
								var err error
								frame, err = ardp.ReadFrame(&reader)
								if err != nil {
									b.Fatal(err)
								}
							}
							if _, err := bridge.Accept(frame); err != nil {
								b.Fatal(err)
							}
							if count, err := lanes[index].Read(buffer); err != nil || count != bodySize {
								b.Fatalf("lane read = %d, %v", count, err)
							}
							index++
							if index == active {
								index = 0
							}
						}
						b.StopTimer()
					})
				}
			}
		}
	}
}

func outerFrameBenchmarkFixture(b *testing.B, active, retired int) (*ClosedOuterBridge, *ClosedOuterHandshake, []*ClosedOuterBridgeLane, []uint32) {
	b.Helper()
	now := time.Unix(1_800_300_000, 0).UTC()
	clock := func() time.Time { return now }
	receiver := ClosedOuterReceiver{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3},
		ProfileDigest: [32]byte{4}, NodeID: [32]byte{5}, RecordDigest: [32]byte{6}, DutyGeneration: 6,
		RoleDomain: closedRoleDomainRendezvous, Subrole: closedDutyIssuance, Deadline: now.Add(time.Hour)}
	limits, err := NewClosedDutyLimits(clock)
	if err != nil {
		b.Fatal(err)
	}
	handshake, err := NewClosedOuterHandshake(receiver, limits, clock)
	if err != nil {
		b.Fatal(err)
	}
	bridge, err := NewClosedOuterBridge(handshake, func(uint32, time.Time) error { return nil },
		func(ardp.Frame, func() time.Time, bool, bool) error { return nil })
	if err != nil {
		b.Fatal(err)
	}
	hello := ardp.Hello{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest,
		ProfileDigest: receiver.ProfileDigest, RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration,
		Purpose: ardp.PurposeForwarding, ChannelNonce: [32]byte{19}, Deadline: receiver.Deadline}
	helloBody, err := ardp.EncodeHello(hello)
	if err != nil {
		b.Fatal(err)
	}
	if _, err := bridge.Accept(ardp.Frame{Kind: ardp.KindHello, Body: helloBody}); err != nil {
		b.Fatal(err)
	}
	for index := range retired {
		bridge.rememberRetired(uint32(index*2 + 1))
	}
	openBody, err := EncodeClosedNodeOpen(ClosedOpen{NextNodeID: receiver.NodeID, NextDutyGeneration: receiver.DutyGeneration,
		Purpose: ardp.PurposeIssuer, Deadline: receiver.Deadline}, ClosedChildOrdinary)
	if err != nil {
		b.Fatal(err)
	}
	lanes := make([]*ClosedOuterBridgeLane, active)
	ids := make([]uint32, active)
	for index := range active {
		id := uint32((retired+index)*2 + 1)
		lane, err := bridge.Accept(ardp.Frame{Kind: ardp.KindOpen, Lane: id, Body: openBody})
		if err != nil || lane == nil {
			b.Fatalf("open lane %d = %v, %v", id, lane, err)
		}
		if _, err := bridge.Accept(ardp.Frame{Kind: ardp.KindBytes, Lane: id, Body: []byte{0}}); err != nil {
			b.Fatal(err)
		}
		if err := lane.BeginInnerHello(); err != nil {
			b.Fatal(err)
		}
		inner := hello
		inner.Purpose, inner.ChannelNonce = ardp.PurposeIssuer, [32]byte{20}
		if err := lane.Activate(inner); err != nil {
			b.Fatal(err)
		}
		if count, err := lane.Read(make([]byte, 1)); err != nil || count != 1 {
			b.Fatalf("initial lane read = %d, %v", count, err)
		}
		lanes[index], ids[index] = lane, id
	}
	return bridge, handshake, lanes, ids
}

// BenchmarkClosedForwardingUnavailable measures bounded unsuccessful scans.
// Rejecting every OPEN is a synthetic control-budget bound: Node normally
// accepts OPEN immediately. Rejected ready BYTES represent busy child writers.
func BenchmarkClosedForwardingUnavailable(b *testing.B) {
	reject := func(ClosedForwardingEvent) bool { return false }
	for _, controls := range []int{1, 124, 248} {
		b.Run(fmt.Sprintf("pending-open=%d", controls), func(b *testing.B) {
			channel := forwardingBenchmarkFixture(b, controls, 0)
			defer channel.Cancel()
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if _, available := channel.NextAvailable(reject); available {
					b.Fatal("rejected OPEN became available")
				}
			}
			b.StopTimer()
		})
	}
	for _, active := range []int{1, 64, ClosedForwardChildren} {
		for _, bodySize := range []int{1, 1024, ardp.MaximumBodySize} {
			b.Run(fmt.Sprintf("ready=%d/body=%d", active, bodySize), func(b *testing.B) {
				channel := forwardingBenchmarkFixture(b, active, bodySize)
				defer channel.Cancel()
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					if _, available := channel.NextAvailable(reject); available {
						b.Fatal("busy child became available")
					}
				}
				b.StopTimer()
			})
		}
	}
}

// BenchmarkClosedForwardingControlHead measures the available OPEN case with
// an admitted control queue. Requeueing outside the timer maintains occupancy
// without repeatedly charging the parent's finite lease.
func BenchmarkClosedForwardingControlHead(b *testing.B) {
	for _, controls := range []int{1, 124, 248} {
		b.Run(fmt.Sprintf("pending-open=%d", controls), func(b *testing.B) {
			channel := forwardingBenchmarkFixture(b, controls, 0)
			defer channel.Cancel()
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				event, available := channel.NextAvailable(nil)
				if !available || event.Kind != ardp.KindOpen {
					b.Fatalf("OPEN not selected: %+v, %t", event, available)
				}
				b.StopTimer()
				channel.mu.Lock()
				queued := channel.queueControl(event)
				channel.mu.Unlock()
				if !queued {
					b.Fatal("OPEN could not be requeued")
				}
				b.StartTimer()
			}
			b.StopTimer()
		})
	}
}

func forwardingBenchmarkFixture(b *testing.B, children, bodySize int) *ClosedForwardingChannel {
	b.Helper()
	now := time.Unix(1_800_000_000, 0).UTC()
	clock := func() time.Time { return now }
	limits, err := NewClosedDutyLimits(clock)
	if err != nil {
		b.Fatal(err)
	}
	reservation, err := limits.reserveChannel()
	if err != nil {
		b.Fatal(err)
	}
	lease := ClosedAdmission{Class: 2, Bytes: 32 << 20, Deadline: now.Add(time.Hour), claim: newClosedAdmissionClaim(reservation, nil)}
	channel, err := NewReplenishableClosedForwardingChannel(&lease, func(ClosedOpen) error { return nil },
		func(ClosedAdmissionVerification) (func() error, error) { return nil, nil }, clock)
	if err != nil {
		b.Fatal(err)
	}
	openBody, err := EncodeClosedOpen(ClosedOpen{NextNodeID: [32]byte{1}, NextDutyGeneration: 2,
		Purpose: ardp.PurposeForwarding, Deadline: now.Add(time.Hour)})
	if err != nil {
		b.Fatal(err)
	}
	body := bytes.Repeat([]byte{1}, bodySize)
	for index := range children {
		id := uint32(index*2 + 1)
		if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindOpen, Lane: id, Body: openBody}); err != nil {
			b.Fatal(err)
		}
		if bodySize == 0 {
			continue
		}
		if event, available := channel.NextAvailable(nil); !available || event.Kind != ardp.KindOpen {
			b.Fatalf("OPEN not scheduled: %+v, %t", event, available)
		}
		if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindBytes, Lane: id, Body: body}); err != nil {
			b.Fatal(err)
		}
	}
	return channel
}
