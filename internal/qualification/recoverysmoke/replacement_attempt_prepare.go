package recoverysmoke

import (
	"context"
	"errors"
	"time"
)

func (attempt *replacementAttempt) Prepare(ctx context.Context) error {
	attempt.hostStartedAt = max(int64(1), time.Since(attempt.hostClock).Nanoseconds())
	if err := attempt.observer.resetRecoveryTopology(ctx, time.Minute); err != nil {
		return err
	}
	if err := resetSequentialGates(attempt.gateRoot(), attempt.offsets); err != nil {
		return err
	}
	if err := resetWorkloadStartGates(attempt.gateRoot()); err != nil {
		return err
	}
	if err := configureRecoveryDirection(attempt.observer.generation, attempt.direction); err != nil {
		return err
	}
	plan, err := configureReplacementPlans(attempt.observer.input.FixtureRoot, attempt.fixture,
		attempt.lifetime, attempt.proposalCount)
	if err != nil {
		return err
	}
	attempt.plan = plan
	written, err := prepareReplacementManifest(attempt.observer.input.FixtureRoot, attempt.direction,
		attempt.mode, attempt.seed, attempt.failures, attempt.offsets, attempt.lifetime, attempt.delay)
	if err != nil || written.Digest != attempt.manifest.Digest {
		return errors.Join(err, errors.New("replacement attempt manifest changed before prepare"))
	}
	if _, err := attempt.observer.compose(ctx, time.Minute, "--profile", "setup", "run", "--no-deps",
		"--rm", "volume-init"); err != nil {
		return err
	}
	attempt.processObserver = newDockerProcessAdapter(attempt.observer, attempt.hostScope, attempt.hostClock)
	attempt.identities, err = attempt.observer.startReplacementServices(ctx, attempt.fixture, attempt.plan)
	if err != nil {
		return err
	}
	attempt.proposalRoutes = make([]routeGeneration, attempt.proposalCount)
	return nil
}

func (attempt *replacementAttempt) Arm(ctx context.Context) error {
	if err := waitWorkloadStartReady(ctx, attempt.gateRoot()); err != nil {
		return err
	}
	attempt.sampler = attempt.observer.startStats(ctx, attempt.identities, attempt.hostClock)
	if err := attempt.sampler.waitReady(ctx); err != nil {
		return err
	}
	var err error
	attempt.hostProcesses, err = observeReplacementEndpointProcesses(ctx, attempt.processObserver, attempt.identities)
	if err != nil {
		return err
	}
	routeProcesses, err := observeReplacementRouteProcesses(ctx, attempt.processObserver, attempt.identities)
	if err != nil {
		return err
	}
	for role, process := range routeProcesses {
		attempt.hostProcesses[role] = process
	}
	for index := range attempt.proposalRoutes {
		attempt.proposalRoutes[index], err = observeRouteGeneration(ctx, attempt.processObserver, attempt.fixture,
			uint64(index+1), attempt.plan.selections[index])
		if err != nil {
			return err
		}
	}
	attempt.initializeCell()
	if attempt.overlap {
		return attempt.startOverlapController(ctx)
	}
	return nil
}

func (attempt *replacementAttempt) initializeCell() {
	attempt.failed = make(map[string]candidateProcess, 3)
	attempt.faultReceipts = make(map[string]processFaultEvidence, 3)
	attempt.receiver, attempt.senderRole = "publisher-app", "client"
	if attempt.direction == "publisher-to-client" {
		attempt.receiver, attempt.senderRole = "client-app", "publisher"
	}
	attempt.cell = replacementCell{Direction: attempt.direction, Mode: attempt.mode, Bytes: 4 << 20,
		Seed: attempt.seed, CellManifestDigest: attempt.manifest.Digest, FaultFamily: attempt.manifest.FaultFamily,
		FailureRoles: attempt.manifest.FailureRoles, FaultOffsets: attempt.manifest.FaultOffsets,
		ChunkBytes: attempt.manifest.ChunkBytes, CanaryBytes: attempt.manifest.CanaryBytes,
		ChunkDelayNanos: attempt.manifest.ChunkDelayNanos, SetupDeadlineNanos: attempt.manifest.SetupDeadlineNanos,
		LifetimeNanos: attempt.manifest.LifetimeNanos, HostStartedAtNanos: attempt.hostStartedAt,
		ExpectedDigest: workloadDigest(attempt.seed, 4<<20),
		Routes:         []routeGeneration{attempt.proposalRoutes[0]}, HostProcesses: attempt.hostProcesses,
		BaselineFinalTraffic:  attempt.overlapBaseline.finalTraffic,
		BaselineTerminalNanos: attempt.overlapBaseline.terminalNanos}
}
