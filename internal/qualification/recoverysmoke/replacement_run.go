package recoverysmoke

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"
)

func (observer dockerObserver) runReplacementRecovery(ctx context.Context, fixture prepared, direction string,
	failures []string, baseline trafficBaseline, sequential bool,
	hostScope hostScopeEvidence, hostClock time.Time) (result replacementCell, returnErr error) {
	if err := observer.resetRecoveryTopology(ctx, time.Minute); err != nil {
		return replacementCell{}, fmt.Errorf("reset replacement topology: %w", err)
	}
	if err := refreshWorkload(observer.generation); err != nil {
		return replacementCell{}, err
	}
	offsets, lifetime, delay, mode := isolatedReplacementSchedule(failures)
	if sequential {
		offsets, lifetime, delay, mode = sequentialReplacementSchedule()
	}
	if err := resetSequentialGates(filepathGateRoot(observer), offsets); err != nil {
		return replacementCell{}, err
	}
	observer.direction, observer.gateOffset, observer.gateOffsets = direction, 0, offsets
	observer.streamLifetime, observer.input.ChunkDelay = lifetime, delay
	if err := configureRecoveryDirection(observer.generation, direction); err != nil {
		return replacementCell{}, err
	}
	plan, err := configureReplacementPlans(observer.input.FixtureRoot, fixture, lifetime)
	if err != nil {
		return replacementCell{}, err
	}
	seed, err := recoveryDirectionSeed(observer.generation, direction)
	if err != nil {
		return replacementCell{}, err
	}
	manifest, err := prepareReplacementManifest(observer.input.FixtureRoot, direction, mode, seed,
		failures, offsets, lifetime, delay)
	if err != nil {
		return replacementCell{}, err
	}
	if _, err := observer.compose(ctx, time.Minute, "--profile", "setup", "run", "--no-deps", "--rm", "volume-init"); err != nil {
		return replacementCell{}, err
	}
	hostStartedAt := max(int64(1), time.Since(hostClock).Nanoseconds())
	cellClock := hostClock.Add(time.Duration(hostStartedAt))
	processObserver := newDockerProcessAdapter(observer, hostScope, hostClock)
	identities, err := observer.startReplacementServices(ctx, fixture, plan)
	if err != nil {
		return replacementCell{}, err
	}
	proposalCount, err := replacementProposalCount(mode)
	if err != nil {
		return replacementCell{}, err
	}
	proposalRoutes := make([]routeGeneration, proposalCount)
	var hostProcesses map[string]processObservationEvidence
	var traffic trafficObservers
	var sampler *statsSampler
	var resourceStartedAt int64
	defer func() {
		trafficErr := traffic.remove(context.Background(), observer)
		var sampleErr error
		if sampler != nil {
			_, sampleErr = sampler.stop()
		}
		returnErr = replacementObservationError(returnErr, trafficErr, sampleErr)
	}()
	err = orderedReplacementObservation(func() error {
		traffic, err = observer.startTrafficObservers(ctx, identities)
		if err == nil {
			resourceStartedAt = max(int64(1), time.Since(cellClock).Nanoseconds())
			sampler = observer.startStats(ctx, identities, cellClock)
		}
		return err
	}, func() error {
		hostProcesses, err = observeReplacementEndpointProcesses(ctx, processObserver, identities)
		if err != nil {
			return err
		}
		routeProcesses, routeErr := observeReplacementRouteProcesses(ctx, processObserver, identities)
		if routeErr != nil {
			return routeErr
		}
		for role, process := range routeProcesses {
			hostProcesses[role] = process
		}
		for index := range proposalRoutes {
			proposalRoutes[index], err = observeRouteGeneration(ctx, processObserver, fixture,
				uint64(index+1), plan.selections[index])
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return replacementCell{}, err
	}
	initialRoute := proposalRoutes[0]
	failed := make(map[string]candidateProcess, 3)
	faultReceipts := make(map[string]processFaultEvidence, 3)
	cell := replacementCell{Direction: direction, Mode: mode, Bytes: 4 << 20, Seed: seed,
		CellManifestDigest: manifest.Digest, FaultFamily: manifest.FaultFamily,
		FailureRoles: manifest.FailureRoles, FaultOffsets: manifest.FaultOffsets,
		ChunkBytes: manifest.ChunkBytes, CanaryBytes: manifest.CanaryBytes,
		ChunkDelayNanos: manifest.ChunkDelayNanos, SetupDeadlineNanos: manifest.SetupDeadlineNanos,
		LifetimeNanos:      manifest.LifetimeNanos,
		HostStartedAtNanos: hostStartedAt, ResourceStartedAtNanos: resourceStartedAt,
		ExpectedDigest: workloadDigest(seed, 4<<20), Routes: []routeGeneration{initialRoute},
		HostProcesses:        hostProcesses,
		BaselineFinalTraffic: baseline.finalTraffic, BaselineClientTraffic: baseline.client,
		BaselinePublisherTraffic: baseline.publisher, BaselineClientRoute: baseline.routes[0],
		BaselinePublisherRoute: baseline.routes[1], BaselineClientTrafficObserver: baseline.observers[0],
		BaselinePublisherTrafficObserver: baseline.observers[1], ClientTrafficObserver: traffic.projections[0],
		PublisherTrafficObserver: traffic.projections[1], ClientRoute: identities["client"],
		PublisherRoute: identities["publisher"], BaselineTerminalNanos: baseline.terminalNanos}
	receiver, senderRole := "publisher-app", "client"
	if direction == "publisher-to-client" {
		receiver, senderRole = "client-app", "publisher"
	}
	var previousOffset uint32
	for eventIndex, role := range failures {
		offset := offsets[eventIndex]
		gateWait, err := pacedGateWait(previousOffset, offset, delay)
		if err != nil {
			return replacementCell{}, err
		}
		if _, err := observer.waitSequentialGate(ctx, filepathGateRoot(observer), senderRole, offset, gateWait); err != nil {
			return replacementCell{}, err
		}
		delivered, err := observer.waitProgress(ctx, receiver, offset)
		if err != nil || delivered != offset {
			return replacementCell{}, errors.Join(err, errors.New("receiver did not drain to the exact replacement gate"))
		}
		lastDelivery := time.Since(cellClock).Nanoseconds()
		before := cell.Routes[len(cell.Routes)-1]
		fault := before.Processes[role]
		faultReceipt, err := stopCandidate(ctx, processObserver, fault)
		if err != nil {
			return replacementCell{}, err
		}
		failed[fault.ContainerID] = fault
		faultReceipts[fault.ContainerID] = faultReceipt
		faultAt := faultReceipt.InvocationStartedNanos - hostStartedAt
		nextProposal := eventIndex + 1
		if mode == "isolated-rendezvous" {
			nextProposal = 2
		}
		nextSelection := plan.selections[nextProposal]
		if err := writeSequentialRelease(filepathGateRoot(observer), senderRole, offset); err != nil {
			return replacementCell{}, err
		}
		canaryOffset := offset
		if delivered, err := observer.waitProgress(ctx, receiver, canaryOffset+32); err != nil || delivered < canaryOffset+32 {
			return replacementCell{}, errors.Join(err, errors.New("replacement recovery canary was not observed"))
		}
		canaryAt := time.Since(cellClock).Nanoseconds()
		if canaryAt-lastDelivery > int64(5*time.Second) {
			return replacementCell{}, errors.New("replacement recovery missed five seconds")
		}
		after, err := observeRouteGeneration(ctx, processObserver, fixture, uint64(eventIndex+2), nextSelection)
		if err != nil {
			return replacementCell{}, err
		}
		cell.Routes = append(cell.Routes, after)
		event := replacementEvent{Role: role, Layer: "leg", GenerationBefore: before.Generation,
			GenerationAfter: after.Generation, Failed: fault, Replacement: after.Processes[role],
			RendezvousBefore: before.Processes["rendezvous"], RendezvousAfter: after.Processes["rendezvous"],
			Introduction: after.Processes["introduction"], FaultOffset: offset, CanaryOffset: canaryOffset,
			Canary: workloadCanary(seed, canaryOffset), LastDeliveryNanos: lastDelivery, FaultAtNanos: faultAt,
			CanaryNanos: canaryAt}
		if role == "rendezvous" {
			event.Layer = "rendezvous"
		}
		cell.Events = append(cell.Events, event)
		previousOffset = offset
	}
	if err := observer.waitReplacementTerminal(ctx, processObserver, identities, failed, sequential,
		hostStartedAt); err != nil {
		return replacementCell{}, err
	}
	cell.TerminalNanos = time.Since(cellClock).Nanoseconds()
	return finalizeReplacementObservation(&traffic, &sampler,
		func(ownedTraffic *trafficObservers, ownedSampler *statsSampler) (replacementCell, error) {
			return observer.finishReplacementCell(ctx, processObserver, cell, receiver, ownedTraffic, ownedSampler,
				failed, faultReceipts, proposalRoutes, cellClock, hostStartedAt)
		})
}

func isolatedReplacementSchedule(failures []string) ([]uint32, string, string, string) {
	return []uint32{17 * 16_381}, "1m", "20ms", "isolated-" + failures[0]
}

func sequentialReplacementSchedule() ([]uint32, string, string, string) {
	return []uint32{64 * 16_381, 128 * 16_381, 192 * 16_381}, "12m", "2350ms", "sequential-three"
}

func filepathGateRoot(observer dockerObserver) string {
	return filepath.Join(observer.input.FixtureRoot, "gate")
}
