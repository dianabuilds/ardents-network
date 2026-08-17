package blockedverify

import (
	"fmt"
	"strings"
	"testing"
)

func TestFinalCampaignAcceptsExactFrozenEvidence(t *testing.T) {
	spec := validFinalSpec()
	summary := validFinalSummary()
	invalid, failures := verifyFinalCampaign(&spec, &summary)
	if len(invalid) != 0 || len(failures) != 0 {
		t.Fatalf("valid final campaign invalid=%v failures=%v", invalid, failures)
	}
}

func TestFinalCampaignDistinguishesIncompleteEvidenceFromGateFailure(t *testing.T) {
	spec := validFinalSpec()
	summary := validFinalSummary()
	summary.Sustained[0].Runs[0].WindowsMbit = summary.Sustained[0].Runs[0].WindowsMbit[:9]
	invalid, failures := verifyFinalCampaign(&spec, &summary)
	if len(invalid) == 0 || len(failures) != 0 {
		t.Fatalf("missing window invalid=%v failures=%v", invalid, failures)
	}

	summary = validFinalSummary()
	for index := range 3 {
		summary.Sustained[0].Runs[0].WindowsMbit[index] = 1
	}
	invalid, failures = verifyFinalCampaign(&spec, &summary)
	if len(invalid) != 0 || len(failures) == 0 {
		t.Fatalf("low goodput invalid=%v failures=%v", invalid, failures)
	}
}

func TestFinalCampaignRejectsChangedProfileAndPressureSchedule(t *testing.T) {
	spec := validFinalSpec()
	summary := validFinalSummary()
	spec.StrongerBridge.VCPU = 7
	invalid, _ := verifyFinalCampaign(&spec, &summary)
	if len(invalid) == 0 {
		t.Fatal("changed stronger host class was accepted")
	}

	spec = validFinalSpec()
	summary.Pressure[2].Injected = 19
	invalid, failures := verifyFinalCampaign(&spec, &summary)
	if len(invalid) == 0 || len(failures) != 0 {
		t.Fatalf("changed pressure corpus invalid=%v failures=%v", invalid, failures)
	}
}

func TestFinalCampaignRejectsCapacityAndResourceFailures(t *testing.T) {
	spec := validFinalSpec()
	summary := validFinalSummary()
	summary.Capacity[0].MaximumRefusalMillis = 1_001
	invalid, failures := verifyFinalCampaign(&spec, &summary)
	if len(invalid) != 0 || len(failures) == 0 {
		t.Fatalf("late refusal invalid=%v failures=%v", invalid, failures)
	}

	summary = validFinalSummary()
	summary.Capacity[5].Resources.BridgeCPUP95 = 5.13
	invalid, failures = verifyFinalCampaign(&spec, &summary)
	if len(invalid) != 0 || len(failures) == 0 {
		t.Fatalf("resource breach invalid=%v failures=%v", invalid, failures)
	}
}

func TestFinalCampaignRejectsMissingResourceDimensionAndOrdinaryEntry(t *testing.T) {
	spec := validFinalSpec()
	summary := validFinalSummary()
	summary.Capacity[0].Resources.Collected = summary.Capacity[0].Resources.Collected[:20]
	invalid, failures := verifyFinalCampaign(&spec, &summary)
	if len(invalid) == 0 || len(failures) != 0 {
		t.Fatalf("missing resource dimension invalid=%v failures=%v", invalid, failures)
	}

	summary = validFinalSummary()
	for index, allocation := range summary.Hosts[0].Allocations {
		if allocation.ID == "ordinary-entry" {
			summary.Hosts[0].Allocations = append(summary.Hosts[0].Allocations[:index],
				summary.Hosts[0].Allocations[index+1:]...)
			break
		}
	}
	summary.Hosts[0].AllocatedVCPU -= 2
	summary.Hosts[0].AllocatedMemoryMiB -= 2_048
	invalid, failures = verifyFinalCampaign(&spec, &summary)
	if len(invalid) == 0 || len(failures) != 0 {
		t.Fatalf("missing ordinary-entry invalid=%v failures=%v", invalid, failures)
	}
}

func TestFinalCampaignRejectsImpossibleNegativeResourceMeasurements(t *testing.T) {
	for name, mutate := range map[string]func(*finalResourceObservation){
		"adapter RSS": func(value *finalResourceObservation) { value.AdapterRSSP95MiB = -1 },
		"evidence projection": func(value *finalResourceObservation) {
			value.EvidenceProjectedPC = -1
		},
	} {
		t.Run(name, func(t *testing.T) {
			spec := validFinalSpec()
			summary := validFinalSummary()
			mutate(&summary.Capacity[0].Resources)
			invalid, failures := verifyFinalCampaign(&spec, &summary)
			if len(invalid) == 0 || len(failures) != 0 {
				t.Fatalf("negative resource invalid=%v failures=%v", invalid, failures)
			}
		})
	}
}

func validFinalSpec() finalSpec {
	hash := strings.Repeat("a", 64)
	spec := finalSpec{Schema: "ardents-h3-s5-final-spec-v1", RepositoryCommit: strings.Repeat("b", 40),
		SourceSHA256: hash, LinuxImage: acceptedFinalLinuxImage, ImageSHA256: acceptedFinalImageHash, Kernel: "6.8.0",
		ClientSHA256: acceptedFinalClientHash, ServerSHA256: acceptedFinalServerHash,
		Endpoint: finalHostClass{ID: "endpoint-reference", OperatingSystem: "ubuntu-lts",
			Architecture: "x86-64", StorageClass: "ssd", Dedicated: true, VCPU: 4, MemoryMiB: 8_192,
			LinkDownMbit: 100, LinkUpMbit: 20, CPUMeanCores: .5, CPUP95Cores: 1, MemoryP95MiB: 512},
		ReferenceBridge: finalHostClass{ID: "h3-s5-b1-v1", OperatingSystem: "ubuntu-lts",
			Architecture: "x86-64", StorageClass: "ssd", Dedicated: true, VCPU: 2, MemoryMiB: 2_048,
			LinkDownMbit: 100, LinkUpMbit: 100, CPUMaxCores: 1.6, CPUMeanCores: 1.12,
			CPUP95Cores: 1.28, MemoryMaxMiB: 1_280, MemoryP95MiB: 896, HelperRSSP95MiB: 128,
			HelperFDs: 64, HelperSockets: 32, MinimumReservePC: 20},
		StrongerBridge: finalHostClass{ID: "h3-s5-b1-v1-strong", OperatingSystem: "ubuntu-lts",
			Architecture: "x86-64", StorageClass: "ssd", Dedicated: true, VCPU: 8, MemoryMiB: 8_192,
			LinkDownMbit: 400, LinkUpMbit: 400, CPUMaxCores: 6.4, CPUMeanCores: 4.48,
			CPUP95Cores: 5.12, MemoryMaxMiB: 5_120, MemoryP95MiB: 3_584, HelperRSSP95MiB: 512,
			HelperFDs: 256, HelperSockets: 128, MinimumReservePC: 20},
		Collector: finalHostClass{ID: "h3-s5-collector-v1", OperatingSystem: "ubuntu-lts",
			Architecture: "x86-64", StorageClass: "ssd", Dedicated: true, VCPU: 16,
			MemoryMiB: 32_768, LinkDownMbit: 1_000, LinkUpMbit: 1_000},
		Network: finalNetwork{BaseRTTMillis: 80, LossPPM: 1_000, JitterP95Millis: 10},
		Clocks: finalClocks{OrdinaryBlockedMillis: 3_000, TransitionMillis: 2_000,
			AttemptMillis: 64_000, ContactMillis: 15_000, StartupMillis: 5_000,
			InterContactMillis: 1_000, AdapterCleanupMillis: 6_000, CellCleanupMillis: 15_000},
		CellOrder: requiredFinalCellOrder()}
	for index := range spec.CellOrder {
		spec.Seeds = append(spec.Seeds, fmt.Sprintf("%064x", index+1))
	}
	for _, path := range requiredFinalConfigurations {
		configurationHash := hash
		if expected := acceptedFinalConfigurationHashes[path]; expected != "" {
			configurationHash = expected
		}
		spec.Configurations = append(spec.Configurations,
			artifactCommitment{Path: path, SHA256: configurationHash, Bytes: 1})
	}
	return spec
}

func validFinalSummary() finalSummary {
	hash := strings.Repeat("a", 64)
	result := finalSummary{Schema: "ardents-h3-s5-final-summary-v1", ImageHash: acceptedFinalImageHash,
		ClientHash: acceptedFinalClientHash, ServerHash: acceptedFinalServerHash,
		Hosts: []finalObservedHost{validObservedHost()},
		Recovery: finalRecovery{Attempts: 5, ConnectionLoss: 5, AttemptIdentityStable: true,
			DeadlineStable: true}}
	spec := validFinalSpec()
	for index, identity := range spec.CellOrder {
		started := uint64(index * 20_000)
		terminal, _ := expectedFinalTerminal(identity)
		result.Cells = append(result.Cells, finalCellObservation{ID: identity, Seed: spec.Seeds[index],
			Terminal:            terminal,
			StartedOffsetMillis: started, TerminalOffsetMillis: started + 1_000,
			CleanupOffsetMillis: started + 2_000})
	}
	result.Profiles = append(result.Profiles, []finalProfileResult{{"C0", "success", 20, 20}, {"C1", "success", 20, 20},
		{"C2", "success", 20, 20}, {"C3", "bridge-attempt-exhausted", 5, 5},
		{"C4", "bridge-attempt-exhausted", 5, 5}, {"C5", "probe-contained", 20, 20},
		{"C6", "limitation-recorded", 20, 20}}...)
	for batch := range uint16(5) {
		result.Capacity = append(result.Capacity, validCapacity("h3-s5-b1-v1", batch, 5, 4, 1, 32))
	}
	for batch := range uint16(5) {
		result.Capacity = append(result.Capacity,
			validCapacity("h3-s5-b1-v1-strong", batch, 17, 16, 1, 128))
	}
	for directionIndex, direction := range []string{"endpoint-to-publisher", "publisher-to-endpoint"} {
		base := uint64(directionIndex) * 3_200_000
		pairID := "pair-" + direction
		cell := finalSustainedCell{Direction: direction, DirectBeforeMbit: 18, DirectAfterMbit: 18,
			DirectBeforeValid: true, DirectAfterValid: true, EndpointCarrierRatio: 1.2,
			PublisherCarrierRatio: 1.2, DirectPairID: pairID,
			DirectBefore: finalDirectRun{StartedOffsetMillis: base, FinishedOffsetMillis: base + 60_000,
				DurationMillis: 60_000, DeliveredBytes: 135_000_000, Digest: hash, PairID: pairID, Complete: true},
			DirectAfter: finalDirectRun{StartedOffsetMillis: base + 3_100_000, FinishedOffsetMillis: base + 3_160_000,
				DurationMillis: 60_000, DeliveredBytes: 135_000_000, Digest: hash, PairID: pairID, Complete: true},
			EndpointCarrierBytes: 4_800_000_000, PublisherCarrierBytes: 4_800_000_000,
			DeliveredBytes: 4_000_000_000}
		for range 5 {
			started := base + 60_000 + uint64(len(cell.Runs))*600_000
			ends := make([]uint64, 10)
			for index := range ends {
				ends[index] = started + uint64(index+1)*60_000
			}
			cell.Runs = append(cell.Runs, finalSustainedRun{StartedOffsetMillis: started,
				FinishedOffsetMillis: started + 600_000, WindowEndsMillis: ends,
				WindowsMbit: []float64{12, 12, 12, 12, 12,
					12, 12, 12, 12, 12}, Resources: validResources(32), Complete: true,
				DeliveredBytes: 800_000_000, Digest: hash})
		}
		result.Sustained = append(result.Sustained, cell)
	}
	result.Pressure = validPressure()
	return result
}

func validObservedHost() finalObservedHost {
	host := finalObservedHost{ID: "stand-0", LogicalCPUs: 128, MemoryMiB: 196_608,
		AllocatedVCPU: 102, AllocatedMemoryMiB: 190_464, DedicatedThreads: true, CgroupV2: true}
	wanted := expectedFinalAllocations()
	for index, id := range requiredAllocationOrder() {
		value := wanted[id]
		value.ProcessNamespace = fmt.Sprintf("pid-%02d", index)
		value.NetworkNamespace = fmt.Sprintf("net-%02d", index)
		host.Allocations = append(host.Allocations, value)
	}
	return host
}

func requiredAllocationOrder() []string {
	result := make([]string, 0, 24)
	for index := range 16 {
		result = append(result, fmt.Sprintf("endpoint-%02d", index))
	}
	return append(result, "publisher", "bridge", "harness", "ordinary-entry", "initiator", "introduction",
		"rendezvous", "responder")
}

func validCapacity(profile string, batch, offered, accepted, refused, socketPeak uint16) finalCapacityBatch {
	return finalCapacityBatch{Profile: profile, Terminal: "complete", Batch: batch, Offered: offered,
		Accepted: accepted, Refused: refused, MaximumRefusalMillis: 900, EstablishedProgress: true,
		Cleanup: true, SecurityExact: true, ReservePercent: 25, ResponseP95Millis: 7_000,
		Resources: validResources(socketPeak)}
}

func validResources(socketPeak uint16) finalResourceObservation {
	return finalResourceObservation{EndpointCPUMean: .4, EndpointCPUP95: .8, EndpointRSSP95MiB: 400,
		BridgeCPUMean: 1, BridgeCPUP95: 1.2, BridgeMemoryP95MiB: 800, HelperRSSP95MiB: 100,
		HelperFDPeak: 60, HelperSocketPeak: socketPeak, ReservePercent: 25, Samples: 10,
		SamplesComplete: true, AdapterRSSP95MiB: 50, AdapterFDPeak: 12, AdapterSocketPeak: 4,
		AdapterStateBytes: 1_000, AdapterStateEntries: 2, ThreadsPeak: 12, GoroutinesPeak: 20,
		TimersPeak: 4, QueueItemsPeak: 4, QueueBytesPeak: 64 << 10, DurableMembers: 2,
		DurableContacts: 4, DurableAttempts: 1, DurableRegimes: 1, DurableStateBytes: 4_000,
		EvidenceBytes: 100_000, EvidenceProjectedPC: 1, Collected: requiredResourceObservations()}
}

func validPressure() []finalPressureCell {
	return []finalPressureCell{
		{ID: "P0", Terminal: "normal", Units: 4, StreamMbit: 10, DurationMillis: 30_000,
			Progress: true, Cleanup: true},
		{ID: "P1", Terminal: "normal", Offers: 100, Refused: 100, MaximumRefusalMillis: 900,
			CadenceMillis: 100, DurationMillis: 10_000, Progress: true, Cleanup: true},
		{ID: "P2", Terminal: "normal", BaselineSockets: 6, Injected: 20, PeakSockets: 26,
			HighSamples: 3, LowSamples: 120, PartialBytes: 128, RatePerSecond: 2,
			Progress: true, Protect: true, Normal: true, Cleanup: true},
		{ID: "P3", Terminal: "drain", BaselineSockets: 6, Injected: 23, PeakSockets: 29,
			PartialBytes: 128, RatePerSecond: 2, Progress: true, Drain: true, Cleanup: true,
			ExitMillis: 59_000},
		{ID: "P4", Terminal: "normal", Offers: 1_000, Refused: 1_000, Batches: 10,
			CadenceMillis: 100, DurationMillis: 100_000, MaximumRefusalMillis: 900,
			Progress: true, Cleanup: true, Reconciliations: validReconciliations()},
	}
}

func validReconciliations() []finalReconciliation {
	result := make([]finalReconciliation, 10)
	for index := range result {
		result[index].Batch = uint16(index)
	}
	return result
}
