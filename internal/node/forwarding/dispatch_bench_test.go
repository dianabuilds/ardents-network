package forwarding

import (
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
