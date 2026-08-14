package recoverysmoke

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
	"github.com/dianabuilds/ardents-network/internal/qualification/recovery"
)

const recoveryBytes = uint32(4 << 20)

func (observer dockerObserver) runPositiveRecovery(ctx context.Context, direction string,
	baseline trafficBaseline) (recovery.Cell, error) {
	observer = observer.forRecoveryOperation(direction)
	cellClock := time.Now()
	if err := observer.resetRecoveryTopology(ctx, time.Minute); err != nil {
		return recovery.Cell{}, err
	}
	if err := setRouteAttachments(observer.input.FixtureRoot, 2); err != nil {
		return recovery.Cell{}, err
	}
	seed, err := recoveryDirectionSeed(observer.generation, direction)
	if err != nil {
		return recovery.Cell{}, err
	}
	faultThreshold := (uint32(17) + uint32(seed[0]%8)) * 16_381
	observer.gateOffset = faultThreshold
	gateRoot := filepath.Join(observer.input.FixtureRoot, "gate")
	for _, name := range []string{"client.ready", "client.release", "publisher.ready", "publisher.release"} {
		_ = os.Remove(filepath.Join(gateRoot, name))
	}
	manifestDigest := recoveryCellManifest(direction, seed, faultThreshold)
	if err := byteio.WriteJSON(filepath.Join(observer.input.FixtureRoot, "cell-manifest.json"), map[string]any{
		"schema": "ardents-h3-recovery-cell-manifest-v1", "direction": direction, "seed": seed,
		"bytes": recoveryBytes, "fault_family": "carrier-channel", "planned_fault_offset": faultThreshold,
		"canary_bytes": 32, "carrier_attachment_deadline": "10.5s", "chunk_delay": "20ms",
		"digest": manifestDigest}, 64<<10); err != nil {
		return recovery.Cell{}, err
	}
	if _, err := observer.compose(ctx, time.Minute, "--profile", "setup", "run", "--no-deps", "--rm", "volume-init"); err != nil {
		return recovery.Cell{}, err
	}
	if err := observer.startRecoveryServices(ctx); err != nil {
		return recovery.Cell{}, err
	}
	identities, err := observer.recoveryIdentities(ctx)
	if err != nil {
		return recovery.Cell{}, err
	}
	initialRouteContainers, initialRoutePIDs, err := observer.routeProcessIdentities(ctx, identities)
	if err != nil {
		return recovery.Cell{}, err
	}
	initialRouteIncarnations, err := observer.routeProcessIncarnations(ctx, identities)
	if err != nil {
		return recovery.Cell{}, err
	}
	traffic, err := observer.startTrafficObservers(ctx, identities)
	if err != nil {
		return recovery.Cell{}, err
	}
	defer func() { _ = traffic.remove(context.Background(), observer) }()
	faultController, err := observer.serviceID(ctx, "carrier-fault")
	if err != nil {
		return recovery.Cell{}, err
	}
	sampler := observer.startStats(ctx, identities, cellClock)
	defer func() { _, _ = sampler.stop() }()
	receiver := "publisher-app"
	if direction == "publisher-to-client" {
		receiver = "client-app"
	}
	senderRole := "client"
	if direction == "publisher-to-client" {
		senderRole = "publisher"
	}
	gateWait, err := pacedGateWait(0, faultThreshold, observer.input.ChunkDelay)
	if err != nil {
		return recovery.Cell{}, err
	}
	_, err = observer.waitGate(ctx, gateRoot, senderRole, faultThreshold, gateWait)
	if err != nil {
		return recovery.Cell{}, err
	}
	delivered, err := observer.waitProgress(ctx, receiver, faultThreshold)
	if err != nil || delivered != faultThreshold {
		return recovery.Cell{}, errors.Join(err, errors.New("receiver did not drain to the exact gated offset"))
	}
	lastDeliveryAt := time.Since(cellClock).Nanoseconds()
	network := observer.project + "_carrier_net"
	initialCarrier, err := observer.observeCarrier(ctx, faultController)
	if err != nil {
		return recovery.Cell{}, err
	}
	carrierObservedAt := time.Since(cellClock).Nanoseconds()
	fault, err := observer.destroyCarrier(ctx, faultController, identities["rendezvous"], network, initialCarrier, cellClock)
	if err != nil || !fault.resourceAbsent {
		return recovery.Cell{}, errors.Join(err, errors.New("faulted Carrier resource remained available"))
	}
	if err := os.WriteFile(filepath.Join(gateRoot, senderRole+".release"), []byte("release\n"), 0o600); err != nil {
		return recovery.Cell{}, err
	}
	canaryOffset := delivered + 32
	canaryDelivered, err := observer.waitProgress(ctx, receiver, canaryOffset+32)
	if err != nil || canaryDelivered < canaryOffset+32 {
		return recovery.Cell{}, errors.Join(err, errors.New("recovery canary was not externally observed"))
	}
	canaryAt := time.Since(cellClock).Nanoseconds()
	replacementCarrier, replacementObserver, err := observer.observeCarrierInNamespace(ctx, identities["rendezvous"])
	if err != nil || replacementCarrier.SocketIDSHA256 == initialCarrier.SocketIDSHA256 {
		return recovery.Cell{}, errors.Join(err, errors.New("recovery reused the failed Carrier socket"))
	}
	replacementObservedAt := time.Since(cellClock).Nanoseconds()
	recoveredRouteIncarnations, err := observer.routeProcessIncarnations(ctx, identities)
	if err != nil {
		return recovery.Cell{}, err
	}
	recoveredRouteContainers := make(map[string]string, len(initialRouteContainers))
	for role, identity := range initialRouteContainers {
		recoveredRouteContainers[role] = identity
	}
	for _, service := range recoveryServiceNames() {
		if err := observer.waitContainer(ctx, identities[service], true); err != nil {
			return recovery.Cell{}, fmt.Errorf("%s: %w", service, err)
		}
	}
	terminalAt := time.Since(cellClock).Nanoseconds()
	finalTraffic, trafficErr := traffic.snapshotAndRemove(ctx, observer, cellClock)
	samples, err := sampler.stop()
	if len(samples) < 3 {
		err = errors.Join(err, errors.New("fewer than three one-second host resource samples"))
	}
	if err != nil || trafficErr != nil {
		return recovery.Cell{}, errors.Join(err, trafficErr)
	}
	clientEndpoint, publisherEndpoint, application, routes, err := observer.recoveryTerminals(ctx, receiver)
	if err != nil {
		return recovery.Cell{}, err
	}
	expected := workloadDigest(seed, recoveryBytes)
	oldCarrierReused := initialCarrier.SocketIDSHA256 == replacementCarrier.SocketIDSHA256
	_ = routes
	memoryHighWater, externalCPU := resourceHighWater(samples)
	forwardBytes := max(finalTraffic.ClientSent, finalTraffic.PublisherSent)
	reverseBytes := max(finalTraffic.ClientReceived, finalTraffic.PublisherReceived)
	cell := recovery.Cell{Direction: direction, ClientProcess: identities["client-endpoint"],
		PublisherProcess: identities["publisher-endpoint"], ClientApplicationProcess: identities["client-app"],
		PublisherApplicationProcess: identities["publisher-app"], InitialCarrier: initialCarrier.SocketIDSHA256,
		ReplacementCarrier:  replacementCarrier.SocketIDSHA256,
		InitialCarrierLocal: initialCarrier.LocalAddress, InitialCarrierRemote: initialCarrier.RemoteAddress,
		ReplacementCarrierLocal: replacementCarrier.LocalAddress, ReplacementCarrierRemote: replacementCarrier.RemoteAddress,
		FaultedCarrier: fault.commitment, RetiredCarrier: fault.retiredCommitment,
		InitialCarrierInode: initialCarrier.Inode, ReplacementCarrierInode: replacementCarrier.Inode,
		InitialCarrierInterface: initialCarrier.InterfaceName, ReplacementCarrierInterface: replacementCarrier.InterfaceName,
		InitialCarrierInterfaceIndex: initialCarrier.InterfaceIndex, ReplacementCarrierInterfaceIndex: replacementCarrier.InterfaceIndex,
		Seed: seed, ExpectedDigest: expected, ObservedDigest: application.ReceivedDigest,
		CellManifestDigest: manifestDigest,
		FaultService:       "rendezvous-responder-carrier", FaultContainer: identities["rendezvous"], FaultNetwork: network,
		FaultController: faultController, FaultControllerRemoved: fault.controllerRemoved, ReplacementObserver: replacementObserver,
		InitialRouteContainers: initialRouteContainers, RecoveredRouteContainers: recoveredRouteContainers,
		InitialRoutePIDs: initialRoutePIDs, InitialRouteIncarnations: initialRouteIncarnations,
		RecoveredRouteIncarnations: recoveredRouteIncarnations,
		Canary:                     workloadCanary(seed, canaryOffset), Bytes: recoveryBytes, PlannedFaultOffset: faultThreshold,
		FaultOffset:          delivered,
		DeliveredBeforeFault: delivered, CanaryOffset: canaryOffset, LastDeliveryNanos: lastDeliveryAt,
		CarrierObservedNanos: carrierObservedAt, FaultAtNanos: fault.faultAt, FaultCompletedNanos: fault.completedAt,
		CarrierCutAfterNanos: fault.cutAfter, AbsenceAfterNanos: fault.absenceAfter,
		CarrierAttachmentDeadlineNanos: int64(10*time.Second + 500*time.Millisecond), ChunkDelayNanos: int64(20 * time.Millisecond),
		OldCarrierRetiredNanos: fault.socketRetiredAt,
		CanaryAtNanos:          canaryAt, ReplacementObservedNanos: replacementObservedAt, TerminalAtNanos: terminalAt,
		ClientRouteGeneration: clientEndpoint.RouteGeneration, PublisherRouteGeneration: publisherEndpoint.RouteGeneration,
		ClientRecoveryCount: clientEndpoint.RecoveryCount, PublisherRecoveryCount: publisherEndpoint.RecoveryCount,
		ClientApplicationAccepts:    clientEndpoint.ApplicationIPCAccepts,
		PublisherApplicationAccepts: publisherEndpoint.ApplicationIPCAccepts,
		ClientRouteAccepts:          clientEndpoint.RouteAttachmentsAccepted,
		PublisherRouteAccepts:       publisherEndpoint.RouteAttachmentsAccepted,
		ClientContinuity:            clientEndpoint.ContinuityCommitment, PublisherContinuity: publisherEndpoint.ContinuityCommitment,
		Ordered: application.Terminal == "success", Unique: application.ReceivedBytes == recoveryBytes,
		SameConnection:         clientEndpoint.ServiceConnectionsHighWater == 1 && publisherEndpoint.ServiceConnectionsHighWater == 1,
		ApplicationReconnected: clientEndpoint.ApplicationIPCAccepts != 1 || publisherEndpoint.ApplicationIPCAccepts != 1,
		OldCarrierReused:       oldCarrierReused, OldCarrierRetired: fault.socketRetired,
		FaultResourceAbsent: fault.resourceAbsent, FailedResourceUnavailable: fault.resourceAbsent && fault.socketRetired && !oldCarrierReused,
		TerminalClean: clientEndpoint.Class == "clean service connection close" &&
			publisherEndpoint.Class == "clean service connection close", QueueHighWater: max(clientEndpoint.QueueHighWater, publisherEndpoint.QueueHighWater),
		MemoryHighWater:    memoryHighWater,
		CPUSeconds:         max(clientEndpoint.CPUSeconds, publisherEndpoint.CPUSeconds),
		ExternalCPUPercent: externalCPU, ExternalStatsObserved: true,
		OpenFilesHighWater:  max(clientEndpoint.OpenFilesHighWater, publisherEndpoint.OpenFilesHighWater),
		GoroutinesHighWater: max(clientEndpoint.GoroutinesHighWater, publisherEndpoint.GoroutinesHighWater),
		TimerHighWater:      max(clientEndpoint.TimerHighWater, publisherEndpoint.TimerHighWater),
		CarrierForwardBytes: forwardBytes, CarrierReverseBytes: reverseBytes, ResourceSamples: samples,
		FinalTraffic: finalTraffic, BaselineClientTraffic: baseline.client, BaselinePublisherTraffic: baseline.publisher,
		BaselineFinalTraffic: baseline.finalTraffic, BaselineTerminalNanos: baseline.terminalNanos,
		BaselineClientRoute: baseline.routes[0], BaselinePublisherRoute: baseline.routes[1],
		BaselineClientTrafficObserver: baseline.observers[0], BaselinePublisherTrafficObserver: baseline.observers[1],
		ClientTrafficObserver: traffic.projections[0], PublisherTrafficObserver: traffic.projections[1]}
	if err := observer.resetRecoveryTopology(ctx, time.Minute); err != nil {
		return recovery.Cell{}, err
	}
	return cell, nil
}
