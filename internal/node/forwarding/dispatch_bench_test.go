package forwarding

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

// BenchmarkDrainForwardingBusy measures one steady-state negative poll through
// Node: checking child writers, asking Route for available work, and rejecting
// queued BYTES while every admitted child writer remains busy. It excludes
// network I/O, connection setup, and completed forwarding throughput.
func BenchmarkDrainForwardingBusy(b *testing.B) {
	for _, children := range []int{1, 64, route.ClosedForwardChildren} {
		b.Run(fmt.Sprintf("ready=%d/body=256", children), func(b *testing.B) {
			channel, links := busyForwardingBenchmarkFixture(b, children)
			defer channel.Cancel()
			server := &forwardServer{}
			ctx := context.Background()
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if err := server.drainForwarding(ctx, channel, links, nil, nil, nil); err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()
			for range children {
				event, available := channel.NextAvailable(nil)
				if !available || event.Kind != ardp.KindBytes || len(event.Bytes) != 256 {
					b.Fatalf("busy poll consumed queued BYTES: %+v, %t", event, available)
				}
			}
			if event, available := channel.NextAvailable(nil); available {
				b.Fatalf("unexpected queued event after drain: %+v", event)
			}
		})
	}
}

func busyForwardingBenchmarkFixture(b *testing.B, children int) (*route.ClosedForwardingChannel, map[uint32]*forwardLink) {
	b.Helper()
	now := time.Unix(1_800_000_000, 0).UTC()
	clock := func() time.Time { return now }
	governor, err := route.NewClosedBootstrapController(clock)
	if err != nil {
		b.Fatal(err)
	}
	limits, err := route.NewClosedDutyLimits(clock)
	if err != nil {
		b.Fatal(err)
	}
	deadline := now.Add(8 * time.Second)
	reservation, err := governor.Admit([32]byte{1}, deadline)
	if err != nil {
		b.Fatal(err)
	}
	channel, err := route.NewClosedBootstrapForwardingChannel(reservation, limits, func(route.ClosedOpen) error { return nil }, clock)
	if err != nil {
		b.Fatal(err)
	}
	openBody, err := route.EncodeClosedOpen(route.ClosedOpen{NextNodeID: [32]byte{3}, NextDutyGeneration: 4,
		Purpose: ardp.PurposeIssuer, Deadline: deadline})
	if err != nil {
		b.Fatal(err)
	}
	links := make(map[uint32]*forwardLink, children)
	for index := range children {
		id := uint32(index*2 + 1)
		if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindOpen, Lane: id, Body: openBody}); err != nil {
			b.Fatal(err)
		}
		if event, available := channel.NextAvailable(nil); !available || event.Kind != ardp.KindOpen {
			b.Fatalf("OPEN not scheduled: %+v, %t", event, available)
		}
		if _, err := channel.Accept(ardp.Frame{Kind: ardp.KindBytes, Lane: id, Body: make([]byte, 256)}); err != nil {
			b.Fatal(err)
		}
		links[id] = &forwardLink{forwarding: true}
	}
	return channel, links
}

// BenchmarkAdmitTinyFramesWithBusyWriters includes one admitted ARDP input and
// its Node dispatch attempt per frame. The bootstrap fixture keeps 2,048
// one-byte frames inside its finite input budget; this is an in-memory
// component measurement, not a TCP/TLS or class-2 throughput result.
func BenchmarkAdmitTinyFramesWithBusyWriters(b *testing.B) {
	const framesPerParent = 2048
	ctx := context.Background()
	for _, children := range []int{1, 64, route.ClosedForwardChildren} {
		for _, parsed := range []bool{false, true} {
			mode := "decoded"
			if parsed {
				mode = "parsed"
			}
			b.Run(fmt.Sprintf("%s/ready=%d/body=1", mode, children), func(b *testing.B) {
				input := ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: []byte{1}}
				raw, err := ardp.EncodeFrame(input)
				if err != nil {
					b.Fatal(err)
				}
				var reader bytes.Reader
				server := &forwardServer{}
				b.SetBytes(framesPerParent * (ardp.HeaderSize + 1))
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					b.StopTimer()
					channel, links := busyForwardingBenchmarkFixture(b, children)
					b.StartTimer()
					for range framesPerParent {
						frame := input
						if parsed {
							reader.Reset(raw)
							frame, err = ardp.ReadFrame(&reader)
							if err != nil {
								b.Fatal(err)
							}
						}
						if _, err := channel.Accept(frame); err != nil {
							b.Fatal(err)
						}
						if err := server.drainForwarding(ctx, channel, links, nil, nil, nil); err != nil {
							b.Fatal(err)
						}
					}
					b.StopTimer()
					if err := channel.Cancel(); err != nil {
						b.Fatal(err)
					}
				}
				b.ReportMetric(framesPerParent, "frames/parent")
			})
		}
	}
}
