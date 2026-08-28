//go:build h4_8_a11

package service_test

import "testing"

func TestH48A11MultiHostPacedPublisherToUserSoak(t *testing.T) {
	runReferenceC2MultiHost(t, referenceC2Scenario{transparentApplication: true, productRendezvousRelay: true, dynamicWorkload: referenceC2DynamicWorkload{
		Cycles: 1800, IntervalMilliseconds: 1_000, CycleDeadlineMilliseconds: 5_000, NoFallbackEvery: 60, BytesEachDirection: 4 << 20}})
}

func TestH48A11MultiHostPublisherApplicationLossAfterWarmup(t *testing.T) {
	runReferenceC2MultiHost(t, referenceC2Scenario{transparentApplication: true, productRendezvousRelay: true, publisherTerminal: referenceC2PublisherApplicationReset,
		dynamicWorkload: referenceC2DynamicWorkload{Cycles: 60, IntervalMilliseconds: 1_000, CycleDeadlineMilliseconds: 5_000,
			NoFallbackEvery: 60, BytesEachDirection: 4 << 20}})
}

func TestH48A11MultiHostPublisherEndpointLossAfterWarmup(t *testing.T) {
	runReferenceC2MultiHost(t, referenceC2Scenario{transparentApplication: true, productRendezvousRelay: true, publisherTerminal: referenceC2PublisherEndpointStop,
		dynamicWorkload: referenceC2DynamicWorkload{Cycles: 60, IntervalMilliseconds: 1_000, CycleDeadlineMilliseconds: 5_000,
			NoFallbackEvery: 60, BytesEachDirection: 4 << 20}})
}

func TestH48A11MultiHostTenCycleCanary(t *testing.T) {
	runReferenceC2MultiHost(t, referenceC2Scenario{transparentApplication: true, productRendezvousRelay: true, dynamicWorkload: referenceC2DynamicWorkload{
		Cycles: 10, IntervalMilliseconds: 1_000, CycleDeadlineMilliseconds: 5_000, NoFallbackEvery: 10, BytesEachDirection: 4 << 20}})
}

// TestH48A11DiagnosticCycleCountProbe is excluded from the A11 denominator.
// It distinguishes a cycle/byte threshold from a wall-clock lifetime boundary
// using the same full multi-host topology before another soak is authorized.
func TestH48A11DiagnosticCycleCountProbe(t *testing.T) {
	runReferenceC2MultiHost(t, referenceC2Scenario{transparentApplication: true, productRendezvousRelay: true, dynamicWorkload: referenceC2DynamicWorkload{
		Cycles: 240, IntervalMilliseconds: 50, CycleDeadlineMilliseconds: 5_000, NoFallbackEvery: 60, BytesEachDirection: 4 << 20}})
}

func TestH48A11MultiHostCarrierLossAfterWarmup(t *testing.T) {
	runReferenceC2MultiHost(t, referenceC2Scenario{transparentApplication: true, productRendezvousRelay: true, transitFault: referenceC2TransitFaultCarrierLoss,
		dynamicWorkload: referenceC2DynamicWorkload{Cycles: 60, IntervalMilliseconds: 1_000, CycleDeadlineMilliseconds: 5_000,
			NoFallbackEvery: 60, BytesEachDirection: 4 << 20}})
}

func TestH48A11MultiHostProductNodeLossAfterWarmup(t *testing.T) {
	runReferenceC2MultiHost(t, referenceC2Scenario{transparentApplication: true, productRendezvousRelay: true, transitFault: referenceC2TransitFaultProductNodeLoss,
		dynamicWorkload: referenceC2DynamicWorkload{Cycles: 60, IntervalMilliseconds: 1_000, CycleDeadlineMilliseconds: 5_000,
			NoFallbackEvery: 60, BytesEachDirection: 4 << 20}})
}
