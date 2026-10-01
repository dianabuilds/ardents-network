//go:build linux

package streamqualification

import (
	"testing"
	"time"
)

func TestPairedVerdictRequiresReceiverDeliveryAndExactReaderOwnership(t *testing.T) {
	start := time.Unix(100, 0)
	base := Report{Started: start, Stopped: start.Add(600 * time.Second), Finished: start.Add(601 * time.Second)}
	var reports PairedReports
	reports.Publisher = base
	for reader := range reports.Readers {
		reports.Readers[reader] = base
		for position := 0; position < 64; position++ {
			id := uint32(reader*32 + position*2 + 1)
			count := uint64(46_875_000)
			tx, rx := count, uint64(0)
			if position >= 16 {
				id, tx, rx = uint32(129+reader*96+(position-16)*2), 32, 32
			}
			measurement := StreamMeasurement{ID: id, Tx: tx, Rx: rx, LastTx: base.Stopped, LastRx: base.Stopped}
			reports.Readers[reader].Streams = append(reports.Readers[reader].Streams, measurement)
			measurement.Tx, measurement.Rx = rx, tx
			reports.Publisher.Streams = append(reports.Publisher.Streams, measurement)
		}
	}
	if !CriteriaPassed(EvaluatePairedConditionWorkload(reports, ClientToPublisher, NormalNetwork)) {
		t.Fatal("matching delivered streams refused")
	}
	reports.Publisher.Streams[0].Rx--
	if CriteriaPassed(EvaluatePairedConditionWorkload(reports, ClientToPublisher, NormalNetwork)) {
		t.Fatal("accepted bytes without matching peer delivery")
	}
	reports.Publisher.Streams[0].Rx++
	reports.Readers[0], reports.Readers[1] = reports.Readers[1], reports.Readers[0]
	if CriteriaPassed(EvaluatePairedConditionWorkload(reports, ClientToPublisher, NormalNetwork)) {
		t.Fatal("Reader ownership substitution accepted")
	}
}
