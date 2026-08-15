package recoverysmoke

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/campaign"
)

type replacementAttempt struct {
	observer                       dockerObserver
	fixture                        prepared
	direction                      string
	failures                       []string
	sequential                     bool
	overlap                        bool
	hostScope                      hostScopeEvidence
	hostClock, cellClock           time.Time
	offsets                        []uint32
	lifetime, delay, mode          string
	proposalCount                  int
	seed                           [32]byte
	manifest                       replacementCellManifest
	plan                           replacementPlan
	identities                     map[string]string
	processObserver                hostProcessAdapter
	proposalRoutes                 []routeGeneration
	hostProcesses                  map[string]processObservationEvidence
	sampler                        *statsSampler
	hostStartedAt, activeStartedAt int64
	cell                           replacementCell
	failed                         map[string]candidateProcess
	faultReceipts                  map[string]processFaultEvidence
	receiver, senderRole           string
	candidateFailure               string
	overlapController              string
	overlapBaseline                trafficBaseline
	failureObservation             replacementFailureObservation
}

func newOverlapAttempt(observer dockerObserver, fixture prepared, direction string,
	hostScope hostScopeEvidence, hostClock time.Time, baseline trafficBaseline) (*replacementAttempt, campaign.CellInput, *campaign.CellReceipt, error) {
	failures := []string{"initiator"}
	offsets, lifetime, delay, mode := overlapReplacementSchedule()
	attempt, input, retained, err := newReplacementAttemptValues(observer, fixture, direction, failures, false, true,
		hostScope, hostClock, offsets, lifetime, delay, mode)
	if attempt != nil {
		attempt.overlapBaseline = baseline
	}
	return attempt, input, retained, err
}

type replacementFailureObservation struct {
	Kind                               string
	EventIndex                         int
	ExpectedOffset, ObservedOffset     uint32
	LastDeliveryNanos, ObservedAtNanos int64
}

type replacementFailureEvidence struct {
	Failure replacementFailureObservation
	Cell    replacementCell
	Faults  map[string]processFaultEvidence
}

func newReplacementAttempt(observer dockerObserver, fixture prepared, direction string, failures []string,
	sequential bool, hostScope hostScopeEvidence, hostClock time.Time) (
	*replacementAttempt, campaign.CellInput, *campaign.CellReceipt, error) {
	offsets, lifetime, delay, mode := isolatedReplacementSchedule(failures)
	if sequential {
		offsets, lifetime, delay, mode = sequentialReplacementSchedule()
	}
	return newReplacementAttemptValues(observer, fixture, direction, failures, sequential, false,
		hostScope, hostClock, offsets, lifetime, delay, mode)
}

func newReplacementAttemptValues(observer dockerObserver, fixture prepared, direction string, failures []string,
	sequential, overlap bool, hostScope hostScopeEvidence, hostClock time.Time,
	offsets []uint32, lifetime, delay, mode string) (
	*replacementAttempt, campaign.CellInput, *campaign.CellReceipt, error) {
	proposalCount, err := replacementProposalCount(mode)
	if err != nil {
		return nil, campaign.CellInput{}, nil, err
	}
	seed, err := recoveryDirectionSeed(observer.generation, direction)
	if err != nil {
		return nil, campaign.CellInput{}, nil, err
	}
	manifest, err := buildReplacementManifest(direction, mode, seed, failures, offsets, lifetime, delay)
	if err != nil {
		return nil, campaign.CellInput{}, nil, err
	}
	shortDirection := map[string]string{"client-to-publisher": "c2p", "publisher-to-client": "p2c"}[direction]
	if shortDirection == "" {
		return nil, campaign.CellInput{}, nil, errors.New("replacement attempt direction is invalid")
	}
	observer.direction, observer.gateOffset, observer.gateOffsets = direction, 0, offsets
	observer.streamLifetime, observer.input.ChunkDelay, observer.startGate = lifetime, delay, true
	value := &replacementAttempt{observer: observer, fixture: fixture, direction: direction,
		failures: append([]string(nil), failures...), sequential: sequential, overlap: overlap,
		hostScope: hostScope, hostClock: hostClock, offsets: offsets, lifetime: lifetime, delay: delay,
		mode: mode, proposalCount: proposalCount, seed: seed, manifest: manifest}
	cellID := shortDirection + "-" + mode
	attemptID, retained, err := campaign.NextAttempt(observer.input.EvidenceRoot, cellID)
	if err != nil {
		return nil, campaign.CellInput{}, nil, err
	}
	if retained != nil {
		return nil, campaign.CellInput{}, retained, nil
	}
	input := campaign.CellInput{CellID: cellID, AttemptID: attemptID,
		ManifestDigest: manifest.Digest, ReceiptRoot: observer.input.EvidenceRoot}
	return value, input, nil, nil
}

func runOverlapAttempt(ctx context.Context, observer dockerObserver, fixture prepared, direction string,
	hostScope hostScopeEvidence, hostClock time.Time, baseline trafficBaseline) (replacementCell, campaign.CellReceipt, error) {
	attempt, input, retained, err := newOverlapAttempt(observer, fixture, direction, hostScope, hostClock, baseline)
	if err != nil {
		return replacementCell{}, campaign.CellReceipt{}, err
	}
	if retained != nil {
		return replacementCell{}, *retained, nil
	}
	receipt, err := campaign.RunCell(ctx, input, attempt)
	return attempt.cell, receipt, err
}

func runReplacementAttempt(ctx context.Context, observer dockerObserver, fixture prepared, direction string,
	failures []string, sequential bool, hostScope hostScopeEvidence,
	hostClock time.Time) (replacementCell, campaign.CellReceipt, error) {
	attempt, input, retained, err := newReplacementAttempt(observer, fixture, direction, failures,
		sequential, hostScope, hostClock)
	if err != nil {
		return replacementCell{}, campaign.CellReceipt{}, err
	}
	if retained != nil {
		return replacementCell{}, *retained, nil
	}
	receipt, err := campaign.RunCell(ctx, input, attempt)
	return attempt.cell, receipt, err
}

func (attempt *replacementAttempt) Freeze(ctx context.Context) (campaign.FrozenCell, error) {
	if attempt.candidateFailure != "" {
		samples, err := attempt.sampler.stopAfter(attempt.activeStartedAt)
		attempt.sampler = nil
		if err != nil {
			return campaign.FrozenCell{}, err
		}
		attempt.cell.ResourceSamples = samples
		evidence, err := json.Marshal(replacementFailureEvidence{
			Failure: attempt.failureObservation, Cell: attempt.cell, Faults: attempt.faultReceipts})
		if err != nil {
			return campaign.FrozenCell{}, fmt.Errorf("encode failed replacement cell evidence: %w", err)
		}
		return campaign.FrozenCell{Candidate: "fail", Reason: attempt.candidateFailure, Evidence: evidence}, nil
	}
	cell, err := attempt.observer.finishReplacementCell(ctx, attempt.processObserver, attempt.cell,
		attempt.receiver, attempt.sampler, attempt.failed, attempt.faultReceipts, attempt.proposalRoutes,
		attempt.activeStartedAt)
	attempt.sampler = nil
	if err != nil {
		return campaign.FrozenCell{}, err
	}
	attempt.cell = cell
	verdict, reason := replacementCandidateResult(cell)
	evidence, err := json.Marshal(cell)
	if err != nil {
		return campaign.FrozenCell{}, fmt.Errorf("encode replacement cell evidence: %w", err)
	}
	return campaign.FrozenCell{Candidate: verdict, Reason: reason, Evidence: evidence}, nil
}

func (attempt *replacementAttempt) Cleanup(ctx context.Context) (json.RawMessage, error) {
	var sampleErr error
	if attempt.sampler != nil {
		_, sampleErr = attempt.sampler.stop()
		attempt.sampler = nil
	}
	cleanupErr := attempt.observer.resetRecoveryTopology(ctx, time.Minute)
	cleanup, observationErr := attempt.observer.observeDockerCleanup(ctx, attempt.hostScope, attempt.hostClock)
	if err := errors.Join(sampleErr, cleanupErr, observationErr); err != nil {
		return nil, err
	}
	evidence, err := json.Marshal(struct {
		Adapter            string
		Scope, Observation [32]byte
		ObservedAtNanos    int64
		OwnedResources     uint32
		AdapterProjection  json.RawMessage
	}{Adapter: cleanup.adapter, Scope: cleanup.scope, Observation: cleanup.commitment,
		ObservedAtNanos: cleanup.observedAt, OwnedResources: cleanup.owned,
		AdapterProjection: cleanup.adapterProjection})
	if err != nil {
		return nil, fmt.Errorf("encode replacement cleanup observation: %w", err)
	}
	return evidence, nil
}

func (attempt *replacementAttempt) gateRoot() string {
	return filepath.Join(attempt.observer.input.FixtureRoot, "gate")
}
