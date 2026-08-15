package recoverysmoke

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/campaign"
	"github.com/dianabuilds/ardents-network/internal/qualification/recovery"
)

const overlapRemote = "172.31.20.16:4606"

func (attempt *replacementAttempt) startOverlapController(ctx context.Context) error {
	target := attempt.proposalRoutes[1].Processes["responder"]
	rawID, err := attempt.observer.docker(ctx, 10*time.Second, "create", "--network", "container:"+target.ContainerID,
		"--ipc", "private", "--read-only", "--cap-drop", "ALL", "--cap-add", "NET_ADMIN",
		"--security-opt", "no-new-privileges", "--user", "0:0", "--pids-limit", "16", "--memory", "32m",
		"--cpus", "0.25", "--label", "com.docker.compose.project="+attempt.observer.project, attempt.observer.imageID,
		"/usr/local/bin/ardents-qualify", "overlap-fault", overlapRemote)
	if err != nil {
		return err
	}
	identity := strings.TrimSpace(string(rawID))
	if !validContainerID(identity) {
		return errors.New("overlap controller identity is invalid")
	}
	projection, err := attempt.observer.inspectReplacementObserver(ctx, identity)
	if err != nil {
		return err
	}
	if _, err := attempt.observer.docker(ctx, 10*time.Second, "start", identity); err != nil {
		return err
	}
	attempt.overlapController = identity
	attempt.cell.Overlap.Observer = projection
	return nil
}

func (attempt *replacementAttempt) observeOverlap(ctx context.Context) (campaign.CellObservation, error) {
	offset := attempt.offsets[0]
	gateWait, err := pacedGateWait(0, offset, attempt.delay)
	if err != nil {
		return campaign.CellObservation{}, err
	}
	if _, err := attempt.observer.waitSequentialGate(ctx, attempt.gateRoot(), attempt.senderRole, offset, gateWait); err != nil {
		return campaign.CellObservation{}, err
	}
	delivered, reached, err := attempt.observer.observeProgress(ctx, attempt.receiver, offset, 15*time.Second)
	if err != nil || !reached || delivered != offset {
		return campaign.CellObservation{}, errors.Join(err, errors.New("overlap receiver did not reach the exact fault gate"))
	}
	lastDelivery := time.Since(attempt.cellClock).Nanoseconds()
	before := attempt.cell.Routes[0]
	fault := before.Processes["initiator"]
	first, err := stopCandidate(ctx, attempt.processObserver, fault)
	if err != nil {
		return campaign.CellObservation{}, err
	}
	attempt.failed[fault.ContainerID], attempt.faultReceipts[fault.ContainerID] = fault, first
	firstAt := first.InvocationStartedNanos - attempt.activeStartedAt
	if err := writeSequentialRelease(attempt.gateRoot(), attempt.senderRole, offset); err != nil {
		return campaign.CellObservation{}, err
	}
	if err := attempt.finishOverlapController(ctx, firstAt); err != nil {
		return campaign.CellObservation{}, err
	}
	remaining := 8*time.Second - time.Duration(time.Since(attempt.cellClock).Nanoseconds()-lastDelivery)
	if remaining <= 0 {
		return attempt.failCandidate("overlap recovery missed eight seconds",
			replacementFailureObservation{Kind: "canary-late", ExpectedOffset: offset + 32,
				ObservedOffset: offset, LastDeliveryNanos: lastDelivery}), nil
	}
	delivered, reached, err = attempt.observer.observeProgress(ctx, attempt.receiver, offset+32, remaining)
	if err != nil {
		return campaign.CellObservation{}, err
	}
	if !reached || delivered < offset+32 {
		return attempt.failCandidate("overlap recovery canary was not observed within eight seconds",
			replacementFailureObservation{Kind: "canary-missing", ExpectedOffset: offset + 32,
				ObservedOffset: delivered, LastDeliveryNanos: lastDelivery}), nil
	}
	canaryAt := time.Since(attempt.cellClock).Nanoseconds()
	after, err := observeRouteGeneration(ctx, attempt.processObserver, attempt.fixture, 2, attempt.plan.selections[2])
	if err != nil {
		return campaign.CellObservation{}, err
	}
	attempt.cell.Routes = append(attempt.cell.Routes, after)
	attempt.cell.Events = append(attempt.cell.Events, replacementEvent{Role: "initiator", Layer: "overlap",
		GenerationBefore: 1, GenerationAfter: 2, Failed: fault, Replacement: after.Processes["initiator"],
		RendezvousBefore: before.Processes["rendezvous"], RendezvousAfter: after.Processes["rendezvous"],
		Introduction: after.Processes["introduction"], FaultOffset: offset, CanaryOffset: offset,
		Canary: workloadCanary(attempt.seed, offset), LastDeliveryNanos: lastDelivery,
		FaultAtNanos: firstAt, CanaryNanos: canaryAt})
	terminalAt, err := attempt.observer.waitReplacementTerminal(ctx, attempt.identities, attempt.receiver, false)
	if err != nil {
		return campaign.CellObservation{}, fmt.Errorf("overlap terminal: %w", err)
	}
	attempt.cell.TerminalNanos = terminalAt.Sub(attempt.cellClock).Nanoseconds()
	return campaign.CellObservation{TerminalAt: terminalAt}, nil
}

func (attempt *replacementAttempt) finishOverlapController(ctx context.Context, firstFaultAt int64) error {
	remaining := time.Duration(firstFaultAt + int64(time.Second) - time.Since(attempt.cellClock).Nanoseconds())
	if remaining <= 0 {
		return errors.New("overlap controller missed the one-second injection bound")
	}
	if _, err := attempt.observer.docker(ctx, remaining, "wait", attempt.overlapController); err != nil {
		return fmt.Errorf("wait for overlap controller: %w", err)
	}
	observedAt := time.Since(attempt.cellClock).Nanoseconds()
	raw, err := attempt.observer.docker(ctx, 10*time.Second, "logs", attempt.overlapController)
	if err != nil {
		return err
	}
	var receipt overlapFaultReceipt
	if json.Unmarshal(raw, &receipt) != nil || receipt.Kind != "overlap-faulted" || !receipt.Absent ||
		receipt.Observation.RemoteAddress != overlapRemote || receipt.Observation.SocketIDSHA256 == "" ||
		receipt.Observation.Inode == 0 || receipt.Observation.InterfaceName == "" ||
		receipt.Observation.InterfaceIndex <= 0 || receipt.ObservedAfterNanos <= 0 ||
		receipt.FaultStartedAfterNanos < receipt.ObservedAfterNanos ||
		receipt.FaultCompletedAfterNanos < receipt.FaultStartedAfterNanos {
		return errors.New("overlap controller receipt is invalid")
	}
	if _, err := attempt.observer.docker(ctx, 10*time.Second, "rm", attempt.overlapController); err != nil {
		return err
	}
	attempt.cell.Overlap = overlapEvidence{Observer: attempt.cell.Overlap.Observer,
		SocketDigest: receipt.Observation.SocketIDSHA256, LocalAddress: receipt.Observation.LocalAddress,
		RemoteAddress: receipt.Observation.RemoteAddress, InterfaceName: receipt.Observation.InterfaceName,
		Inode: receipt.Observation.Inode, InterfaceIndex: receipt.Observation.InterfaceIndex,
		ObservedAtNanos: observedAt, FaultAtNanos: observedAt, FaultCompletedNanos: observedAt,
		CarrierCutAfterNanos: receipt.CarrierCutAfterNanos, AbsenceAfterNanos: receipt.AbsenceAfterNanos,
		Absent: true, ObserverRemoved: true}
	attempt.cell.Overlap.Observer.Removed = true
	return nil
}

func overlapObserverValid(value recovery.ObserverProcess) bool {
	return value.Removed && value.ReadOnly && !value.Privileged && value.User == "0:0"
}
