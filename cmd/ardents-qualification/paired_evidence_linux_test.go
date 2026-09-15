//go:build linux

package main

import (
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
)

func TestPairedOwnerNetworkRequiresExactUsefulAndWireReconciliation(t *testing.T) {
	var reports streamqualification.PairedReports
	for reader := range reports.Readers {
		id := uint32(1 + reader*32)
		reports.Readers[reader].Streams = []streamqualification.StreamMeasurement{{ID: id, Tx: 100}}
		reports.Publisher.Streams = append(reports.Publisher.Streams, streamqualification.StreamMeasurement{ID: id, Rx: 100})
	}
	started, stopped := time.Unix(1000, 0).UTC(), time.Unix(1600, 0).UTC()
	policy := ownerNetworkVerdict{Started: started, Stopped: stopped, HostingProvider: "fixture-provider", HostingPeriodStart: started.Add(-time.Hour), HostingPeriodEnd: stopped.Add(time.Hour), HostingUnit: "GiB", HostingDirections: "tx+rx", HostingQuantity: 1, HostingLowWatermark: 1 << 20}
	owners := map[streamqualification.Role]ownerNetworkVerdict{
		streamqualification.ReaderRole: {
			Started: policy.Started, Stopped: policy.Stopped, HostingProvider: policy.HostingProvider, HostingPeriodStart: policy.HostingPeriodStart, HostingPeriodEnd: policy.HostingPeriodEnd, HostingUnit: policy.HostingUnit, HostingDirections: policy.HostingDirections, HostingQuantity: policy.HostingQuantity, HostingLowWatermark: policy.HostingLowWatermark,
			InterfaceTx: 500, InterfaceRx: 100, CountedBytes: 600, HostingLedgerDelta: 600, UsefulTx: 400,
			DirectionalWire: 600, DirectionalUseful: 400, DirectionalOverhead: 200, DirectionalCarrierRatio: 1.5,
			TxP95BitsPerSecond: 16, RxP95BitsPerSecond: 4, OneSecondSampleCount: 600,
		},
		streamqualification.PublisherRole: {
			Started: policy.Started, Stopped: policy.Stopped, HostingProvider: policy.HostingProvider, HostingPeriodStart: policy.HostingPeriodStart, HostingPeriodEnd: policy.HostingPeriodEnd, HostingUnit: policy.HostingUnit, HostingDirections: policy.HostingDirections, HostingQuantity: policy.HostingQuantity, HostingLowWatermark: policy.HostingLowWatermark,
			InterfaceTx: 100, InterfaceRx: 500, CountedBytes: 600, HostingLedgerDelta: 600, UsefulRx: 400,
			DirectionalWire: 600, DirectionalUseful: 400, DirectionalOverhead: 200, DirectionalCarrierRatio: 1.5,
			TxP95BitsPerSecond: 4, RxP95BitsPerSecond: 16, OneSecondSampleCount: 600,
		},
	}
	if criteria := evaluatePairedOwnerNetwork(reports, streamqualification.ClientToPublisher, streamqualification.NormalNetwork, owners, relayTrafficVerdict{Endpoints: []relayNodeTraffic{{ID: userEndpoint, Tx: 500, Rx: 100}, {ID: publisherEndpoint, Tx: 100, Rx: 500}}}); !streamqualification.CriteriaPassed(criteria) {
		t.Fatalf("reconciled owner pair refused: %+v", criteria)
	}
	reports.Publisher.Streams[0].Rx--
	if criteria := evaluatePairedOwnerNetwork(reports, streamqualification.ClientToPublisher, streamqualification.NormalNetwork, owners, relayTrafficVerdict{Endpoints: []relayNodeTraffic{{ID: userEndpoint, Tx: 500, Rx: 100}, {ID: publisherEndpoint, Tx: 100, Rx: 500}}}); streamqualification.CriteriaPassed(criteria) {
		t.Fatal("peer useful-byte disagreement accepted")
	}
}
