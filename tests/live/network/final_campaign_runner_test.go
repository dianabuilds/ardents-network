//go:build live

package network_test

import (
	"strings"
	"testing"
)

func TestFinalRunnerDispatchesMaintainedCellWorkers(t *testing.T) {
	cases := map[string]string{
		"profile/C1/00":                         "TestBlockedEntryCommandsAcrossNamespaces",
		"profile/C4/04":                         "TestBlockedEntryNegativeCommandsAcrossNamespaces",
		"capacity/h3-s5-b1-v1/0":                "TestBlockedEntryFinalReferenceAndStrongCapacity",
		"sustained/endpoint-to-publisher/run-0": "TestBlockedEntryFinalSustainedEvidence",
		"pressure/P0":                           "TestBlockedEntryFinalProjectedAdmissionAndChurn",
		"pressure/P2":                           "TestBlockedEntryReturnsFromExactRecoverableSocketPressure",
		"pressure/P3":                           "TestBlockedEntryDrainsAtExactEmergencySocketPressure",
		"recovery/0":                            "TestBlockedEntryRecoveryParentCommandsAcrossNamespaces",
	}
	for cell, want := range cases {
		if got := finalWorkerTest(cell); got != want {
			t.Fatalf("worker for %s=%q want %q", cell, got, want)
		}
	}
	if got := finalWorkerTest("hostile/G1-invite/malformed/0"); got != "" {
		t.Fatalf("unimplemented hostile worker was silently dispatched to %q", got)
	}
}

func TestFinalRunnerObservationPreservesWorkerEvidence(t *testing.T) {
	seed := strings.Repeat("a", 64)
	plan := finalRunnerPlan{Schema: "ardents-h3-blocked-cell-plan-v1", CellID: "profile/C1/00", Seed: seed}
	worker := finalWorkerResult{CellID: plan.CellID, Terminal: "success", EvidenceComplete: true, StartedOffsetMillis: 4,
		TerminalOffsetMillis: 8, CleanupOffsetMillis: 11,
		Observers: fixtureFinalRunnerObservers(), Residuals: fixtureFinalRunnerResiduals()}
	if !validFinalRunnerPlan(plan) {
		t.Fatal("valid final plan rejected")
	}
	schedule := finalRunnerSchedule{CellOrder: []string{plan.CellID}, Seeds: []string{seed}}
	if !matchesFinalRunnerSchedule(schedule, 0, plan) || matchesFinalRunnerSchedule(schedule, 1, plan) {
		t.Fatal("runner did not enforce the exact schedule ordinal")
	}
	observed := finalObservationFromWorker(plan, worker)
	if observed.CellID != plan.CellID || observed.Seed != seed || observed.ObservedTerminal != "success" ||
		observed.StartedOffsetMillis != 4 || observed.TerminalOffsetMillis != 8 ||
		observed.CleanupOffsetMillis != 11 || observed.AdapterCleanupMillis != 3 ||
		len(observed.Observers) != 9 || len(observed.Residuals) != 10 {
		t.Fatalf("runner observation=%+v", observed)
	}
	plan.Seed = strings.Repeat("A", 64)
	if validFinalRunnerPlan(plan) {
		t.Fatal("non-canonical seed accepted")
	}
}

func fixtureFinalRunnerObservers() []finalRunnerObserver {
	boundaries := []string{"endpoint-adapter", "tls-front", "webtunnel-server", "bridge-next-leg",
		"publisher-endpoint", "ordinary-initiator", "ordinary-introduction", "ordinary-rendezvous",
		"ordinary-responder"}
	result := make([]finalRunnerObserver, 0, len(boundaries))
	for _, boundary := range boundaries {
		result = append(result, finalRunnerObserver{Boundary: boundary, IPv4UDPControl: true,
			IPv6UDPControl: true, IPv4TCPControl: true, Attribution: "exact", ForbiddenOwner: "none",
			ObservationCompleted: true})
	}
	return result
}

func fixtureFinalRunnerResiduals() []finalRunnerResidual {
	kinds := []string{"process", "listener", "socket", "namespace", "mount", "file", "queue", "timer",
		"cgroup", "publishable-secret"}
	result := make([]finalRunnerResidual, 0, len(kinds))
	for _, kind := range kinds {
		result = append(result, finalRunnerResidual{Kind: kind, Owner: "none"})
	}
	return result
}
