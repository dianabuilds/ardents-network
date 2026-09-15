//go:build linux

package main

import (
	"testing"

	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
)

func TestPairedOwnerNetworkRequiresExactUsefulAndWireReconciliation(t *testing.T) {
	var reports streamqualification.PairedReports
	for reader := range reports.Readers {
		id := uint32(1 + reader*32)
		reports.Readers[reader].Streams = []streamqualification.StreamMeasurement{{ID: id, Tx: 100}}
		reports.Publisher.Streams = append(reports.Publisher.Streams, streamqualification.StreamMeasurement{ID: id, Rx: 100})
	}
	owners := map[streamqualification.Role]ownerNetworkVerdict{
		streamqualification.ReaderRole: {
			InterfaceTx: 500, InterfaceRx: 100, CountedBytes: 600, HostingLedgerDelta: 600, UsefulTx: 400,
			DirectionalWire: 600, DirectionalUseful: 400, DirectionalOverhead: 200, DirectionalCarrierRatio: 1.5,
			TxP95BitsPerSecond: 16, RxP95BitsPerSecond: 4, OneSecondSampleCount: 600,
		},
		streamqualification.PublisherRole: {
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
