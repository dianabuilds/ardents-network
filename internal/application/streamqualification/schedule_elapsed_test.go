package streamqualification

import (
	"io"
	"testing"
	"time"
)

func TestDelayedTickerPreservesOfferedLoadAndTenMinutePublisherArithmetic(t *testing.T) {
	schedule, _ := ClientToPublisher.Definition(PublisherRole)
	streams := map[uint32]*workerStream{}
	var order []uint32
	for index := 0; index < int(schedule.ActiveConnections); index++ {
		id := uint32(index*2 + 1)
		order = append(order, id)
		streams[id] = &workerStream{id: id, offset: 47_343_750 - 10_000, sendCredit: frameCreditWindow}
	}
	if err := sendScheduledElapsed(io.Discard, streams, order, [32]byte{1}, schedule, 10*time.Minute); err != nil {
		t.Fatal(err)
	}
	for _, stream := range streams {
		if stream.offset != 47_343_750 {
			t.Fatalf("ten-minute offset=%d", stream.offset)
		}
	}
	// A repeated delayed tick cannot duplicate bytes that already left the stream.
	if err := sendScheduledElapsed(io.Discard, streams, order, [32]byte{1}, schedule, 11*time.Minute); err != nil {
		t.Fatal(err)
	}
	for _, stream := range streams {
		if stream.offset != 47_343_750 {
			t.Fatal("duplicate scheduled bytes")
		}
	}
}
