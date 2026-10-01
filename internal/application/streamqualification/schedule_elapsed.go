package streamqualification

import (
	"errors"
	"io"
	"time"
)

// Schedule from elapsed time, not received ticker events: a delayed tick must
// not silently reduce the declared offered workload. One turn remains bounded
// to one data frame per active stream and its exact available credit.
func sendScheduledElapsed(output io.Writer, streams map[uint32]*workerStream, order []uint32, seed [32]byte, schedule Schedule, elapsed time.Duration) error {
	if schedule.ActiveConnections == 0 || len(order) < int(schedule.ActiveConnections) {
		return errors.New("qualification active schedule unavailable")
	}
	elapsed = min(max(elapsed, 0), 10*time.Minute)
	total := uint64(elapsed/time.Second)*offeredWorkloadBits(schedule)/8 +
		uint64(elapsed%time.Second)*offeredWorkloadBits(schedule)/uint64(8*time.Second)
	active := uint64(schedule.ActiveConnections)
	for index, id := range order[:int(active)] {
		stream := streams[id]
		target := total / active
		if uint64(index) < total%active {
			target++
		}
		if stream == nil || stream.sentEOF || stream.offset >= target {
			continue
		}
		size := min(uint64(frameLimit), uint64(stream.sendCredit), target-stream.offset)
		if size == 0 {
			continue
		}
		body := scheduledBytes(seed, id, stream.offset, int(size))
		if err := writeWorkerFrame(output, workerFrame{kind: frameBytes, id: id, body: body}); err != nil {
			return err
		}
		stream.offset += size
		stream.sendCredit -= uint32(size)
	}
	return nil
}

// Offer a fixed one-percent margin above the minimum throughput criterion.
// This absorbs bounded start/EOF delivery skew without shortening the measured
// interval, rounding a failing bitrate upward, or changing acceptance limits.
func offeredWorkloadBits(schedule Schedule) uint64 {
	return uint64(schedule.AggregateBits) * 101 / 100
}
