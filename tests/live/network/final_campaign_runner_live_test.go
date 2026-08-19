//go:build live

package network_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

func runFinalCampaignRunner() int {
	decoder, encoder := json.NewDecoder(os.Stdin), json.NewEncoder(os.Stdout)
	decoder.DisallowUnknownFields()
	schedule, err := loadFinalRunnerSchedule(os.Getenv("ARDENTS_BLOCKED_CAMPAIGN_SPEC"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "final runner:", err)
		return 1
	}
	started, cached := time.Now(), make(map[string]finalWorkerResult)
	ordinal := 0
	for {
		var plan finalRunnerPlan
		if err := decoder.Decode(&plan); err == io.EOF {
			closed := finalRunnerClosed{Schema: "ardents-h3-blocked-campaign-closed-v1"}
			if len(cached) == len(schedule.CellOrder) {
				closed.FinalSummary = finalRunnerSummaryFromWorkers(schedule, cached)
			}
			if encoder.Encode(closed) != nil {
				return 1
			}
			return 0
		} else if err != nil || !validFinalRunnerPlan(plan) || !matchesFinalRunnerSchedule(schedule, ordinal, plan) {
			fmt.Fprintln(os.Stderr, "final runner: invalid or reordered cell plan")
			return 1
		}
		result, ok := cached[plan.CellID]
		if !ok {
			groupOrigin := time.Now()
			groupStarted := uint64(groupOrigin.Sub(started).Milliseconds())
			group, groupErr := executeFinalCellGroup(schedule, plan.CellID, groupOrigin)
			if groupErr != nil {
				fmt.Fprintln(os.Stderr, "final runner:", groupErr)
				return 1
			}
			for _, value := range group {
				value.StartedOffsetMillis += groupStarted
				value.TerminalOffsetMillis += groupStarted
				value.CleanupOffsetMillis += groupStarted
				if _, duplicate := cached[value.CellID]; duplicate {
					fmt.Fprintln(os.Stderr, "final runner: worker duplicated a cell")
					return 1
				}
				cached[value.CellID] = value
			}
			result, ok = cached[plan.CellID]
		}
		if !ok || !validFinalWorkerResult(result) {
			fmt.Fprintln(os.Stderr, "final runner: worker omitted the requested cell")
			return 1
		}
		observation := finalObservationFromWorker(plan, result)
		if err := encoder.Encode(observation); err != nil {
			return 1
		}
		ordinal++
	}
}

func loadFinalRunnerSchedule(path string) (finalRunnerSchedule, error) {
	input, err := os.Open(path)
	if err != nil {
		return finalRunnerSchedule{}, errors.Join(err, errors.New("frozen cell schedule is unavailable"))
	}
	defer input.Close()
	decoder := json.NewDecoder(io.LimitReader(input, 4<<20))
	var schedule finalRunnerSchedule
	if err := decoder.Decode(&schedule); err != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		!validFinalRunnerSupplyIdentity(schedule) || len(schedule.CellOrder) != 594 ||
		len(schedule.Seeds) != len(schedule.CellOrder) {
		return finalRunnerSchedule{}, errors.New("frozen cell schedule is invalid")
	}
	return schedule, nil
}

func matchesFinalRunnerSchedule(schedule finalRunnerSchedule, ordinal int, plan finalRunnerPlan) bool {
	return ordinal >= 0 && ordinal < len(schedule.CellOrder) && plan.CellID == schedule.CellOrder[ordinal] &&
		plan.Seed == schedule.Seeds[ordinal]
}

func finalObservationFromWorker(plan finalRunnerPlan, result finalWorkerResult) finalRunnerObservation {
	return finalRunnerObservation{Schema: "ardents-h3-blocked-cell-observation-v1", EventID: plan.EventID,
		CellID: plan.CellID, Seed: plan.Seed, ObservedTerminal: result.Terminal, ProductStarted: true,
		FaultInjected: true, FaultOwner: "none", Attribution: "exact",
		Diagnostic:          "frozen live worker completed the selected cell",
		StartedOffsetMillis: result.StartedOffsetMillis, TerminalOffsetMillis: result.TerminalOffsetMillis,
		CleanupOffsetMillis:  result.CleanupOffsetMillis,
		AdapterCleanupMillis: result.CleanupOffsetMillis - result.TerminalOffsetMillis,
		Observers:            result.Observers, RawObservers: result.RawObservers,
		RawTelemetry: result.RawTelemetry, Residuals: result.Residuals}
}

func validFinalWorkerResult(value finalWorkerResult) bool {
	expectedSets := uint16(1)
	if value.CellID == "pressure/P4" {
		expectedSets = 10
	}
	if !value.EvidenceComplete || value.CellID == "" || value.Terminal == "" ||
		value.ObserverSets != expectedSets ||
		value.TerminalOffsetMillis < value.StartedOffsetMillis ||
		value.CleanupOffsetMillis < value.TerminalOffsetMillis || len(value.Observers) != 9 ||
		len(value.RawObservers) != int(expectedSets) || len(value.Residuals) != 10 {
		return false
	}
	for _, observed := range value.Observers {
		if !observed.IPv4UDPControl || !observed.IPv6UDPControl || !observed.IPv4TCPControl ||
			!observed.ObservationCompleted || observed.Attribution != "exact" || observed.ForbiddenPackets != 0 ||
			observed.UnclassifiedPackets != 0 {
			return false
		}
	}
	for _, residual := range value.Residuals {
		if residual.Count != 0 || residual.Owner != "none" {
			return false
		}
	}
	return true
}
func validFinalRunnerPlan(plan finalRunnerPlan) bool {
	if plan.Schema != "ardents-h3-blocked-cell-plan-v1" || plan.CellID == "" || len(plan.Seed) != 64 {
		return false
	}
	for _, value := range plan.Seed {
		if !strings.ContainsRune("0123456789abcdef", value) {
			return false
		}
	}
	return true
}

func executeFinalCellGroup(schedule finalRunnerSchedule, cell string,
	clockOrigin time.Time,
) ([]finalWorkerResult, error) {
	test := finalWorkerTest(cell)
	if test == "" {
		return nil, errors.New("hostile final cell worker is not implemented: " + cell)
	}
	return runFinalCellWorker(schedule, cell, test, clockOrigin)
}

func finalWorkerTest(cell string) string {
	switch {
	case strings.HasPrefix(cell, "profile/C0/"), strings.HasPrefix(cell, "profile/C1/"),
		strings.HasPrefix(cell, "profile/C2/"), strings.HasPrefix(cell, "profile/C5/"),
		strings.HasPrefix(cell, "profile/C6/"):
		return "TestBlockedEntryCommandsAcrossNamespaces"
	case strings.HasPrefix(cell, "profile/C3/"), strings.HasPrefix(cell, "profile/C4/"):
		return "TestBlockedEntryNegativeCommandsAcrossNamespaces"
	case strings.HasPrefix(cell, "hostile/G5-adapter-fault/accept-then-stall/"):
		return "TestBlockedEntryNegativeCommandsAcrossNamespaces"
	case strings.HasPrefix(cell, "capacity/"):
		return "TestBlockedEntryFinalReferenceAndStrongCapacity"
	case strings.HasPrefix(cell, "sustained/"):
		return "TestBlockedEntryFinalSustainedEvidence"
	case cell == "pressure/P0" || cell == "pressure/P1" || cell == "pressure/P4":
		return "TestBlockedEntryFinalProjectedAdmissionAndChurn"
	case cell == "pressure/P2":
		return "TestBlockedEntryReturnsFromExactRecoverableSocketPressure"
	case cell == "pressure/P3":
		return "TestBlockedEntryDrainsAtExactEmergencySocketPressure"
	case strings.HasPrefix(cell, "recovery/"):
		return "TestBlockedEntryRecoveryParentCommandsAcrossNamespaces"
	case strings.HasPrefix(cell, "hostile/G8-lifecycle/cancellation/"):
		return "TestBlockedEntryRecoveryParentCommandsAcrossNamespaces"
	case strings.HasPrefix(cell, "hostile/G1-invite/"):
		return "TestBlockedEntryFinalHostileInviteValidation"
	case strings.HasPrefix(cell, "hostile/G2-domain-collision/"):
		return "TestBlockedEntryFinalHostileDomainCollision"
	case strings.HasPrefix(cell, "hostile/G3-replay-replacement/active-reimport/"),
		strings.HasPrefix(cell, "hostile/G3-replay-replacement/retired-replay/"),
		strings.HasPrefix(cell, "hostile/G3-replay-replacement/same-generation-different-bytes/"),
		strings.HasPrefix(cell, "hostile/G3-replay-replacement/wrong-replacement-id/"),
		strings.HasPrefix(cell, "hostile/G3-replay-replacement/skipped-generation/"),
		strings.HasPrefix(cell, "hostile/G3-replay-replacement/third-generation/"),
		strings.HasPrefix(cell, "hostile/G3-replay-replacement/full-set/"),
		strings.HasPrefix(cell, "hostile/G3-replay-replacement/cross-slot-replacement/"):
		return "TestBlockedEntryFinalHostileReplay"
	case strings.HasPrefix(cell, "hostile/G4-restart/after-regime-publication/"),
		strings.HasPrefix(cell, "hostile/G4-restart/after-exposure-0/"),
		strings.HasPrefix(cell, "hostile/G8-lifecycle/endpoint-restart/"):
		return "TestBlockedEntryFinalHostileRestart"
	case strings.HasPrefix(cell, "hostile/G6-substitution/network/"),
		strings.HasPrefix(cell, "hostile/G6-substitution/route-profile/"):
		return "TestBlockedEntryFinalHostileInviteValidation"
	case strings.HasPrefix(cell, "hostile/G9-ledger-leakage/unknown-invite-field/"):
		return "TestBlockedEntryFinalHostileInviteValidation"
	default:
		return ""
	}
}
