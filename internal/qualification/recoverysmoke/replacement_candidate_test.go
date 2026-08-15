package recoverysmoke

import (
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/recovery"
)

func TestReplacementCandidateResultUsesCompleteCandidateRules(t *testing.T) {
	valid := candidateResultFixture()
	if verdict, reason := replacementCandidateResult(valid); verdict != "pass" {
		t.Fatalf("valid candidate classified %s: %s", verdict, reason)
	}
	for name, mutate := range map[string]func(*replacementCell){
		"acknowledgement": func(cell *replacementCell) { cell.ClientAcknowledgedBytes-- },
		"queue":           func(cell *replacementCell) { cell.ClientQueueHighWater = 256<<10 + 1 },
		"canary clock":    func(cell *replacementCell) { cell.Events[0].CanaryNanos += int64(6 * time.Second) },
		"resource": func(cell *replacementCell) {
			for index := range cell.ResourceSamples {
				cell.ResourceSamples[index].ClientRSS = 513 << 20
			}
		},
		"missing traffic": func(cell *replacementCell) { cell.FinalTraffic = recovery.ResourceSample{} },
	} {
		t.Run(name, func(t *testing.T) {
			cell := valid
			cell.ResourceSamples = append([]recovery.ResourceSample(nil), valid.ResourceSamples...)
			mutate(&cell)
			if verdict, _ := replacementCandidateResult(cell); verdict != "fail" {
				t.Fatalf("%s violation classified %s", name, verdict)
			}
		})
	}
}

func TestReplacementCandidateAcceptsRoundedTerminalTraffic(t *testing.T) {
	cell := candidateResultFixture()
	cell.FinalTraffic.ClientSent = uint64(cell.Bytes) - 128<<10
	cell.FinalTraffic.PublisherReceived = uint64(cell.Bytes) - 128<<10
	if verdict, reason := replacementCandidateResult(cell); verdict != "pass" {
		t.Fatalf("rounded terminal traffic classified %s: %s", verdict, reason)
	}
}

func candidateResultFixture() replacementCell {
	seed := [32]byte{1}
	digest := workloadDigest(seed, 4<<20)
	offset := uint32(17 * 16_381)
	samples := []recovery.ResourceSample{
		{AtNanos: int64(time.Second), ClientRSS: 1, PublisherRSS: 1, ClientCPUPercent: 1, PublisherCPUPercent: 1,
			ClientSent: 1 << 20, PublisherReceived: 1 << 20},
		{AtNanos: int64(2 * time.Second), ClientRSS: 1, PublisherRSS: 1, ClientCPUPercent: 1, PublisherCPUPercent: 1,
			ClientSent: 2 << 20, PublisherReceived: 2 << 20},
		{AtNanos: int64(3 * time.Second), ClientRSS: 1, PublisherRSS: 1, ClientCPUPercent: 1, PublisherCPUPercent: 1,
			ClientSent: 4 << 20, PublisherReceived: 4 << 20},
	}
	return replacementCell{Direction: "client-to-publisher", Mode: "isolated-initiator", Bytes: 4 << 20,
		Seed: seed, ExpectedDigest: digest, ObservedDigest: digest, Ordered: true, Unique: true, SameConnection: true,
		TerminalClean: true, ClientRouteGeneration: 2, PublisherRouteGeneration: 2,
		ClientRecoveryCount: 1, PublisherRecoveryCount: 1, ClientApplicationAccepts: 1, PublisherApplicationAccepts: 1,
		ClientRouteAccepts: 2, PublisherRouteAccepts: 2, ClientAcceptedBytes: 4 << 20,
		ClientAcknowledgedBytes: 4 << 20, PublisherReceivedBytes: 4 << 20,
		ClientQueueHighWater: 1, FinalCanaryOffset: 4<<20 - 32,
		FinalCanary: workloadCanary(seed, 4<<20-32), TerminalNanos: int64(4 * time.Second),
		FinalCanaryObservedNanos: int64(5 * time.Second), FaultOffsets: []uint32{offset},
		Routes: []routeGeneration{{Generation: 1}, {Generation: 2}},
		Events: []replacementEvent{{Role: "initiator", Layer: "leg", FaultOffset: offset, CanaryOffset: offset,
			Canary: workloadCanary(seed, offset), LastDeliveryNanos: int64(time.Second),
			FaultAtNanos: int64(2 * time.Second), CanaryNanos: int64(3 * time.Second)}},
		Proposals:       []replacementProposal{{Committed: true, Terminal: "success"}, {Committed: true, Terminal: "success"}},
		ResourceSamples: samples,
		FinalTraffic:    recovery.ResourceSample{ClientSent: 4 << 20, PublisherReceived: 4 << 20}}
}
