package node

import "testing"

func TestRendezvousPressureUsageDoesNotTreatCompletedBytesAsQueuedWork(t *testing.T) {
	usage := rendezvousUsage{Handshakes: 2, WaitingLegs: 1, ActivePairs: 1, Connections: 4,
		CompletedPairs: 7, RelayedBytes: 16 << 20}
	timers, queueItems, queueBytes := rendezvousPressureUsage(usage)
	if timers != 3 || queueItems != 0 || queueBytes != 0 {
		t.Fatalf("pressure usage = timers %d, queue items %d, queue bytes %d", timers, queueItems, queueBytes)
	}
}
