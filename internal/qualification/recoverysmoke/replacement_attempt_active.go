package recoverysmoke

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/campaign"
)

func (attempt *replacementAttempt) Release(context.Context) (time.Time, error) {
	releasedAt, err := releaseWorkloadStart(attempt.gateRoot(), attempt.senderRole)
	if err != nil {
		return time.Time{}, err
	}
	attempt.activeStartedAt = max(attempt.hostStartedAt+1, releasedAt.Sub(attempt.hostClock).Nanoseconds())
	attempt.cellClock = releasedAt
	attempt.cell.ActiveStartedAtNanos = attempt.activeStartedAt
	return releasedAt, nil
}

func (attempt *replacementAttempt) Observe(ctx context.Context) (campaign.CellObservation, error) {
	var previousOffset uint32
	for eventIndex, role := range attempt.failures {
		offset := attempt.offsets[eventIndex]
		gateWait, err := pacedGateWait(previousOffset, offset, attempt.delay)
		if err != nil {
			return campaign.CellObservation{}, err
		}
		if _, err := attempt.observer.waitSequentialGate(ctx, attempt.gateRoot(), attempt.senderRole,
			offset, gateWait); err != nil {
			return campaign.CellObservation{}, err
		}
		delivered, reached, err := attempt.observer.observeProgress(ctx, attempt.receiver, offset, 15*time.Second)
		if err != nil {
			return campaign.CellObservation{}, err
		}
		if !reached || delivered != offset {
			return attempt.failCandidate("receiver did not drain to the exact replacement gate",
				replacementFailureObservation{Kind: "progress", EventIndex: eventIndex,
					ExpectedOffset: offset, ObservedOffset: delivered}), nil
		}
		lastDelivery := time.Since(attempt.cellClock).Nanoseconds()
		before := attempt.cell.Routes[len(attempt.cell.Routes)-1]
		fault := before.Processes[role]
		faultReceipt, err := stopCandidate(ctx, attempt.processObserver, fault)
		if err != nil {
			return campaign.CellObservation{}, err
		}
		attempt.failed[fault.ContainerID] = fault
		attempt.faultReceipts[fault.ContainerID] = faultReceipt
		faultAt := faultReceipt.InvocationStartedNanos - attempt.activeStartedAt
		nextProposal := eventIndex + 1
		if attempt.mode == "isolated-rendezvous" {
			nextProposal = 2
		}
		nextSelection := attempt.plan.selections[nextProposal]
		if err := writeSequentialRelease(attempt.gateRoot(), attempt.senderRole, offset); err != nil {
			return campaign.CellObservation{}, err
		}
		canaryOffset := offset
		remaining := 5*time.Second - time.Duration(time.Since(attempt.cellClock).Nanoseconds()-lastDelivery)
		if remaining <= 0 {
			return attempt.failCandidate("replacement recovery missed five seconds",
				replacementFailureObservation{Kind: "canary-late", EventIndex: eventIndex,
					ExpectedOffset: canaryOffset + 32, ObservedOffset: canaryOffset,
					LastDeliveryNanos: lastDelivery}), nil
		}
		delivered, reached, err = attempt.observer.observeProgress(ctx, attempt.receiver, canaryOffset+32, remaining)
		if err != nil {
			return campaign.CellObservation{}, err
		}
		if !reached || delivered < canaryOffset+32 {
			return attempt.failCandidate("replacement recovery canary was not observed within five seconds",
				replacementFailureObservation{Kind: "canary-missing", EventIndex: eventIndex,
					ExpectedOffset: canaryOffset + 32, ObservedOffset: delivered,
					LastDeliveryNanos: lastDelivery}), nil
		}
		canaryAt := time.Since(attempt.cellClock).Nanoseconds()
		if canaryAt-lastDelivery > int64(5*time.Second) {
			return attempt.failCandidate("replacement recovery missed five seconds",
				replacementFailureObservation{Kind: "canary-late", EventIndex: eventIndex,
					ExpectedOffset: canaryOffset + 32, ObservedOffset: delivered,
					LastDeliveryNanos: lastDelivery}), nil
		}
		after, err := observeRouteGeneration(ctx, attempt.processObserver, attempt.fixture,
			uint64(eventIndex+2), nextSelection)
		if err != nil {
			return campaign.CellObservation{}, err
		}
		attempt.cell.Routes = append(attempt.cell.Routes, after)
		event := replacementEvent{Role: role, Layer: "leg", GenerationBefore: before.Generation,
			GenerationAfter: after.Generation, Failed: fault, Replacement: after.Processes[role],
			RendezvousBefore: before.Processes["rendezvous"], RendezvousAfter: after.Processes["rendezvous"],
			Introduction: after.Processes["introduction"], FaultOffset: offset, CanaryOffset: canaryOffset,
			Canary: workloadCanary(attempt.seed, canaryOffset), LastDeliveryNanos: lastDelivery,
			FaultAtNanos: faultAt, CanaryNanos: canaryAt}
		if role == "rendezvous" {
			event.Layer = "rendezvous"
		}
		attempt.cell.Events = append(attempt.cell.Events, event)
		previousOffset = offset
	}
	if err := attempt.observer.waitReplacementTerminal(ctx, attempt.identities, attempt.sequential); err != nil {
		if errors.Is(err, errReplacementProcessStillRunning) {
			return attempt.failCandidate("replacement process exceeded its candidate lifetime",
				replacementFailureObservation{Kind: "lifetime", EventIndex: len(attempt.failures)}), nil
		}
		return campaign.CellObservation{}, err
	}
	terminalAt := time.Now()
	attempt.cell.TerminalNanos = terminalAt.Sub(attempt.cellClock).Nanoseconds()
	return campaign.CellObservation{TerminalAt: terminalAt}, nil
}

func (attempt *replacementAttempt) failCandidate(reason string,
	failure replacementFailureObservation) campaign.CellObservation {
	terminalAt := time.Now()
	attempt.candidateFailure = reason
	attempt.cell.TerminalNanos = terminalAt.Sub(attempt.cellClock).Nanoseconds()
	failure.ObservedAtNanos = attempt.cell.TerminalNanos
	attempt.failureObservation = failure
	return campaign.CellObservation{Candidate: "fail", Reason: reason, TerminalAt: terminalAt}
}

func isolatedReplacementSchedule(failures []string) ([]uint32, string, string, string) {
	return []uint32{17 * 16_381}, "1m", "20ms", "isolated-" + failures[0]
}

func sequentialReplacementSchedule() ([]uint32, string, string, string) {
	return []uint32{64 * 16_381, 128 * 16_381, 192 * 16_381}, "12m", "2350ms", "sequential-three"
}
