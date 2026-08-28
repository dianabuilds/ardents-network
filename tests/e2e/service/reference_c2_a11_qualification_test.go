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

// TestH48A11MultiHostSustainedCycleRegression is excluded from the A11
// denominator. It prevents the fixture transit byte floor from truncating the
// selected path before the full soak can begin to measure its contract.
func TestH48A11MultiHostSustainedCycleRegression(t *testing.T) {
	runReferenceC2MultiHost(t, referenceC2Scenario{transparentApplication: true, productRendezvousRelay: true, dynamicWorkload: referenceC2DynamicWorkload{
		Cycles: 240, IntervalMilliseconds: 250, CycleDeadlineMilliseconds: 5_000, NoFallbackEvery: 60, BytesEachDirection: 4 << 20}})
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
