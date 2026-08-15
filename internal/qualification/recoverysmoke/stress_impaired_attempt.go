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

type impairedAttempt struct {
	observer                  dockerObserver
	fixture                   prepared
	direction, manifestDigest string
	hostScope                 hostScopeEvidence
	hostClock                 time.Time
	seed                      [32]byte
	directBefore              directBaseline
	shapers                   stressShapers
	identities                map[string]string
	hostProcesses             map[string]processObservationEvidence
	sampler                   *statsSampler
	releasedAt                time.Time
	activeHostNanos           int64
	cell                      impairedCell
}

func newImpairedAttempt(observer dockerObserver, fixture prepared, direction string,
	hostScope hostScopeEvidence, hostClock time.Time) (*impairedAttempt, campaign.CellInput,
	*campaign.CellReceipt, error) {
	seed, err := recoveryDirectionSeed(observer.generation, direction)
	if err != nil {
		return nil, campaign.CellInput{}, nil, err
	}
	manifest := stressImpairedCell(direction, seed)
	value := &impairedAttempt{observer: observer, fixture: fixture, direction: direction,
		manifestDigest: manifest.ManifestDigest, hostScope: hostScope, hostClock: hostClock, seed: seed}
	attemptID, retained, err := campaign.NextAttempt(observer.input.EvidenceRoot, manifest.CellID)
	if err != nil || retained != nil {
		return value, campaign.CellInput{}, retained, err
	}
	return value, campaign.CellInput{CellID: manifest.CellID, AttemptID: attemptID,
		ManifestDigest: manifest.ManifestDigest, ReceiptRoot: observer.input.EvidenceRoot}, nil, nil
}

func runImpairedAttempt(ctx context.Context, observer dockerObserver, fixture prepared, direction string,
	hostScope hostScopeEvidence, hostClock time.Time) (campaign.CellReceipt, error) {
	attempt, input, retained, err := newImpairedAttempt(observer, fixture, direction, hostScope, hostClock)
	if err != nil {
		return campaign.CellReceipt{}, err
	}
	if retained != nil {
		return *retained, nil
	}
	return campaign.RunCell(ctx, input, attempt)
}

func (attempt *impairedAttempt) Prepare(ctx context.Context) error {
	phase := map[string]string{"client-to-publisher": "direct-before-c2p",
		"publisher-to-client": "direct-before-p2c"}[attempt.direction]
	baseline, err := attempt.observer.runStressDirect(ctx, attempt.direction, phase, attempt.hostScope)
	if err != nil {
		return err
	}
	attempt.directBefore = baseline
	if err := configureImpairedFixture(attempt.observer.input.FixtureRoot); err != nil {
		return err
	}
	if err := resetWorkloadStartGates(filepath.Join(attempt.observer.input.FixtureRoot, "gate")); err != nil {
		return err
	}
	if err := configureRecoveryDirection(attempt.observer.generation, attempt.direction); err != nil {
		return err
	}
	attempt.observer.direction, attempt.observer.startGate = attempt.direction, true
	attempt.observer.streamLifetime = "15m"
	attempt.observer.input.Bytes, attempt.observer.input.ChunkDelay = 192<<20, "50ms"
	if _, err := attempt.observer.compose(ctx, time.Minute, "--profile", "setup", "run", "--no-deps", "--rm",
		"volume-init"); err != nil {
		return err
	}
	if err := attempt.observer.startRecoveryServices(ctx); err != nil {
		return err
	}
	attempt.identities, err = attempt.observer.recoveryIdentities(ctx)
	return err
}

func (attempt *impairedAttempt) Arm(ctx context.Context) error {
	gateRoot := filepath.Join(attempt.observer.input.FixtureRoot, "gate")
	if err := waitWorkloadStartReady(ctx, gateRoot); err != nil {
		return err
	}
	processObserver := newDockerProcessAdapter(attempt.observer, attempt.hostScope, attempt.hostClock)
	processes, err := observeReplacementEndpointProcesses(ctx, processObserver, attempt.identities)
	if err != nil {
		return err
	}
	for _, role := range []string{"client", "publisher"} {
		value, observeErr := observeProcessEvidence(ctx, processObserver, role, role)
		if observeErr != nil {
			return observeErr
		}
		processes[role] = value
	}
	attempt.hostProcesses = processes
	attempt.sampler = attempt.observer.startStats(ctx, attempt.identities, attempt.hostClock)
	if err := attempt.sampler.waitReady(ctx); err != nil {
		return err
	}
	phase := map[string]string{"client-to-publisher": "candidate-c2p",
		"publisher-to-client": "candidate-p2c"}[attempt.direction]
	shapers, err := prepareStressShapers(attempt.observer, phase, false,
		map[string]string{"client": attempt.identities["client"], "publisher": attempt.identities["publisher"]},
		attempt.hostClock)
	if err != nil {
		return err
	}
	attempt.shapers = shapers
	attempt.observer = shapers.observer
	return attempt.shapers.start(ctx)
}

func (attempt *impairedAttempt) Release(context.Context) (time.Time, error) {
	role := "client"
	if attempt.direction == "publisher-to-client" {
		role = "publisher"
	}
	released, err := releaseWorkloadStart(filepath.Join(attempt.observer.input.FixtureRoot, "gate"), role)
	if err != nil {
		return time.Time{}, err
	}
	attempt.releasedAt = released
	attempt.activeHostNanos = released.Sub(attempt.hostClock).Nanoseconds()
	return released, nil
}

func (attempt *impairedAttempt) Observe(ctx context.Context) (campaign.CellObservation, error) {
	receiver := "publisher-app"
	if attempt.direction == "publisher-to-client" {
		receiver = "client-app"
	}
	progress, completed, err := attempt.observer.measureProgress(ctx, receiver, attempt.releasedAt, 10*time.Minute)
	if err != nil {
		return campaign.CellObservation{}, err
	}
	samples, sampleErr := attempt.sampler.stopAfter(attempt.activeHostNanos)
	attempt.sampler = nil
	if sampleErr != nil || len(samples) < 3 {
		return campaign.CellObservation{}, errors.Join(sampleErr, errors.New("S4.3 resource observations are incomplete"))
	}
	shapers, err := attempt.shapers.finish(ctx)
	if err != nil {
		return campaign.CellObservation{}, err
	}
	measurementNanos := completed.Sub(attempt.releasedAt).Nanoseconds()
	for index := range shapers {
		shapers[index].ReadyObservedAtNanos = 1
		shapers[index].CompletedAtNanos = max(measurementNanos+1,
			shapers[index].CompletedAtNanos-attempt.activeHostNanos)
	}
	for _, service := range recoveryServiceNames() {
		if err := attempt.observer.waitContainerFor(ctx, attempt.identities[service], true, 4*time.Minute); err != nil {
			return campaign.CellObservation{}, fmt.Errorf("impaired %s: %w", service, err)
		}
	}
	terminalAt := time.Now()
	cell, err := attempt.finishCell(ctx, receiver, progress, samples, shapers,
		measurementNanos, terminalAt.Sub(attempt.releasedAt).Nanoseconds())
	if err != nil {
		return campaign.CellObservation{}, err
	}
	attempt.cell = cell
	return campaign.CellObservation{TerminalAt: terminalAt}, nil
}

func (attempt *impairedAttempt) Freeze(ctx context.Context) (campaign.FrozenCell, error) {
	phase := map[string]string{"client-to-publisher": "direct-after-c2p",
		"publisher-to-client": "direct-after-p2c"}[attempt.direction]
	after, err := attempt.observer.runStressDirect(ctx, attempt.direction, phase, attempt.hostScope)
	if err != nil {
		return campaign.FrozenCell{}, err
	}
	attempt.cell.DirectAfter = after
	evidence, err := json.Marshal(attempt.cell)
	if err != nil {
		return campaign.FrozenCell{}, err
	}
	return campaign.FrozenCell{Candidate: "pass", Evidence: evidence}, nil
}

func (attempt *impairedAttempt) Cleanup(ctx context.Context) (json.RawMessage, error) {
	if attempt.sampler != nil {
		_, _ = attempt.sampler.stop()
	}
	cleanupErr := attempt.observer.resetRecoveryTopology(ctx, time.Minute)
	cleanup, observeErr := attempt.observer.observeDockerCleanup(ctx, attempt.hostScope, attempt.hostClock)
	if err := errors.Join(cleanupErr, observeErr); err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		Adapter            string
		Scope, Observation [32]byte
		ObservedAtNanos    int64
		OwnedResources     uint32
		AdapterProjection  json.RawMessage
	}{Adapter: cleanup.adapter, Scope: cleanup.scope, Observation: cleanup.commitment,
		ObservedAtNanos: cleanup.observedAt, OwnedResources: cleanup.owned,
		AdapterProjection: cleanup.adapterProjection})
}
