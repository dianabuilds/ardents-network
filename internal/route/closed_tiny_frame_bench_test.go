package route

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

// BenchmarkClosedForwardingQueuedFrames measures one finite admitted parent's
// queue build. Each iteration starts with open lanes and ends with their
// cancellation; no consumer removes a BYTES frame during the timed interval.
// The 1-byte cases expose per-frame storage, while the 16-KiB cases show the
// same owner under its ordinary credit and prefix queue bounds.
func BenchmarkClosedForwardingQueuedFrames(b *testing.B) {
	for _, lanes := range []int{1, 64, ClosedForwardChildren} {
		for _, bodySize := range []int{1, ardp.MaximumBodySize} {
			b.Run(fmt.Sprintf("lanes=%d/body=%d", lanes, bodySize), func(b *testing.B) {
				payload := min(uint64(lanes)*closedLaneCredit, 1<<20)
				if bodySize == ardp.MaximumBodySize && lanes == ClosedForwardChildren {
					// One full frame per ready lane reaches the 4-MiB prefix cap.
					payload = closedPrefixQueueBytes
				}
				frames := int(payload) / bodySize
				if frames == 0 || int(payload)%bodySize != 0 {
					b.Fatal("invalid finite queue workload")
				}
				body := make([]byte, bodySize)
				frame := ardp.Frame{Kind: ardp.KindBytes, Body: body}
				var liveMiB float64
				b.SetBytes(int64(frames * (ardp.HeaderSize + bodySize)))
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					b.StopTimer()
					runtime.GC()
					var before runtime.MemStats
					runtime.ReadMemStats(&before)
					channel := tinyFrameQueueFixture(b, lanes)
					b.StartTimer()
					for index := range frames {
						frame.Lane = uint32((index%lanes)*2 + 1)
						if _, err := channel.Accept(frame); err != nil {
							b.Fatalf("frame %d/%d refused: %v", index+1, frames, err)
						}
					}
					b.StopTimer()
					if channel.queued != payload || len(channel.ready) != lanes {
						b.Fatalf("queue state = %d bytes, %d ready lanes", channel.queued, len(channel.ready))
					}
					runtime.GC()
					var after runtime.MemStats
					runtime.ReadMemStats(&after)
					liveMiB += float64(int64(after.HeapAlloc)-int64(before.HeapAlloc)) / (1 << 20)
					if err := channel.Cancel(); err != nil {
						b.Fatal(err)
					}
				}
				b.ReportMetric(float64(frames), "frames/parent")
				b.ReportMetric(float64(payload)/(1<<10), "queuedKiB/parent")
				b.ReportMetric(liveMiB/float64(b.N), "liveMiB/parent")
			})
		}
	}
}

// BenchmarkClosedForwardingFirstRefusal keeps sending one admitted frame at a
// time until the first real owner refusal. The two-million-frame stop is above
// the largest one-byte schedule permitted by one 32-MiB parent allowance, and
// catches a benchmark fixture that accidentally removes that finite bound.
func BenchmarkClosedForwardingFirstRefusal(b *testing.B) {
	const maximumFrames = 2_000_000
	for _, lanes := range []int{1, 64, ClosedForwardChildren} {
		for _, bodySize := range []int{1, ardp.MaximumBodySize} {
			b.Run(fmt.Sprintf("lanes=%d/body=%d", lanes, bodySize), func(b *testing.B) {
				frame := ardp.Frame{Kind: ardp.KindBytes, Body: make([]byte, bodySize)}
				var accepted, queued, used uint64
				var liveMiB float64
				var refusal string
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					b.StopTimer()
					runtime.GC()
					var before runtime.MemStats
					runtime.ReadMemStats(&before)
					channel := tinyFrameQueueFixture(b, lanes)
					b.StartTimer()
					found := false
					for index := range maximumFrames {
						frame.Lane = uint32((index%lanes)*2 + 1)
						if _, err := channel.Accept(frame); err != nil {
							accepted += uint64(index)
							refusal = err.Error()
							found = true
							break
						}
					}
					b.StopTimer()
					if !found {
						b.Fatal("finite parent accepted two million frames without refusal")
					}
					queued += channel.queued
					used += channel.usedBytes
					runtime.GC()
					var after runtime.MemStats
					runtime.ReadMemStats(&after)
					liveMiB += float64(int64(after.HeapAlloc)-int64(before.HeapAlloc)) / (1 << 20)
					if err := channel.Cancel(); err != nil {
						b.Fatal(err)
					}
				}
				b.ReportMetric(float64(accepted)/float64(b.N), "frames/parent")
				b.ReportMetric(float64(queued)/float64(b.N), "queuedBytes/parent")
				b.ReportMetric(float64(used)/float64(b.N), "usedBytes/parent")
				b.ReportMetric(liveMiB/float64(b.N), "liveMiB/parent")
				b.Logf("first refusal: %s", refusal)
			})
		}
	}
}

func tinyFrameQueueFixture(b *testing.B, lanes int) *ClosedForwardingChannel {
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
	for index := range lanes {
		lane := uint32(index*2 + 1)
		if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindOpen, Lane: lane, Body: openBody}); err != nil {
			b.Fatal(err)
		}
		if event, ok := channel.NextAvailable(nil); !ok || event.Kind != ardp.KindOpen || event.Lane != lane {
			b.Fatalf("OPEN %d was not scheduled: %+v, %t", lane, event, ok)
		}
	}
	return channel
}
