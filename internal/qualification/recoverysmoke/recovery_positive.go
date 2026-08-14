package recoverysmoke

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
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
	baselineClient, baselinePublisher uint64) (recovery.Cell, error) {
	cellClock := time.Now()
	_, _ = observer.compose(ctx, time.Minute, "down", "-v", "--remove-orphans")
	if err := setRouteAttachments(observer.input.FixtureRoot, 2); err != nil {
		return recovery.Cell{}, err
	}
	seed, err := recoveryDirectionSeed(observer.generation, direction)
	if err != nil {
		return recovery.Cell{}, err
	}
	faultThreshold := (uint32(184) + uint32(seed[0]%8)) * 16_381
	observer.gateOffset = faultThreshold
	gateRoot := filepath.Join(observer.input.FixtureRoot, "gate")
	for _, name := range []string{"client.ready", "client.release", "publisher.ready", "publisher.release"} {
		_ = os.Remove(filepath.Join(gateRoot, name))
	}
	manifestDigest := recoveryCellManifest(direction, seed, faultThreshold)
	if err := byteio.WriteJSON(filepath.Join(observer.input.FixtureRoot, "cell-manifest.json"), map[string]any{
		"schema": "ardents-h3-recovery-cell-manifest-v1", "direction": direction, "seed": seed,
		"bytes": recoveryBytes, "fault_family": "carrier-channel", "planned_fault_offset": faultThreshold,
		"canary_bytes": 32, "carrier_attachment_deadline": "13s", "chunk_delay": "30ms",
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
	_, err = observer.waitGate(ctx, gateRoot, senderRole, faultThreshold)
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
	recoveredRouteContainers, recoveredRoutePIDs, err := observer.routeProcessIdentities(ctx, identities)
	if err != nil {
		return recovery.Cell{}, err
	}
	for _, service := range recoveryServiceNames() {
		if err := observer.waitContainer(ctx, identities[service], true); err != nil {
			return recovery.Cell{}, fmt.Errorf("%s: %w", service, err)
		}
	}
	terminalAt := time.Since(cellClock).Nanoseconds()
	samples, err := sampler.stop()
	if err != nil {
		return recovery.Cell{}, err
	}
	clientEndpoint, publisherEndpoint, application, routes, err := observer.recoveryTerminals(ctx, receiver)
	if err != nil {
		return recovery.Cell{}, err
	}
	expected := workloadDigest(seed, recoveryBytes)
	oldCarrierReused := initialCarrier.SocketIDSHA256 == replacementCarrier.SocketIDSHA256
	_ = routes
	lastSample := samples[len(samples)-1]
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
		InitialRoutePIDs: initialRoutePIDs, RecoveredRoutePIDs: recoveredRoutePIDs,
		Canary: workloadCanary(seed, canaryOffset), Bytes: recoveryBytes, PlannedFaultOffset: faultThreshold,
		FaultOffset:          delivered,
		DeliveredBeforeFault: delivered, CanaryOffset: canaryOffset, LastDeliveryNanos: lastDeliveryAt,
		CarrierObservedNanos: carrierObservedAt, FaultAtNanos: fault.faultAt, FaultCompletedNanos: fault.completedAt,
		CarrierCutAfterNanos: fault.cutAfter, AbsenceAfterNanos: fault.absenceAfter,
		CarrierAttachmentDeadlineNanos: int64(13 * time.Second), ChunkDelayNanos: int64(30 * time.Millisecond),
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
		MemoryHighWater:    max(lastSample.ClientRSS, lastSample.PublisherRSS),
		CPUSeconds:         max(clientEndpoint.CPUSeconds, publisherEndpoint.CPUSeconds),
		ExternalCPUPercent: max(lastSample.ClientCPUPercent, lastSample.PublisherCPUPercent), ExternalStatsObserved: true,
		OpenFilesHighWater:  max(clientEndpoint.OpenFilesHighWater, publisherEndpoint.OpenFilesHighWater),
		GoroutinesHighWater: max(clientEndpoint.GoroutinesHighWater, publisherEndpoint.GoroutinesHighWater),
		TimerHighWater:      max(clientEndpoint.TimerHighWater, publisherEndpoint.TimerHighWater),
		CarrierForwardBytes: max(lastSample.ClientSent, lastSample.PublisherSent),
		CarrierReverseBytes: max(lastSample.ClientReceived, lastSample.PublisherReceived), ResourceSamples: samples,
		BaselineClientTraffic: baselineClient, BaselinePublisherTraffic: baselinePublisher}
	if _, err := observer.compose(ctx, time.Minute, "down", "-v", "--remove-orphans"); err != nil {
		return recovery.Cell{}, err
	}
	return cell, nil
}

func recoveryDirectionSeed(root, direction string) ([32]byte, error) {
	name := "client-seed.hex"
	if direction == "publisher-to-client" {
		name = "publisher-seed.hex"
	}
	var seed [32]byte
	raw, err := byteio.ReadFile(filepath.Join(root, name), 64)
	if err != nil || len(raw) != 64 {
		return seed, errors.Join(err, errors.New("recovery workload seed is invalid"))
	}
	_, err = hex.Decode(seed[:], raw)
	return seed, err
}

func workloadDigest(seed [32]byte, count uint32) [32]byte {
	hash := sha256.New()
	for offset := uint32(0); offset < count; offset += 32 {
		block := workloadBlock(seed, uint64(offset/32))
		length := min(uint32(32), count-offset)
		_, _ = hash.Write(block[:length])
	}
	var digest [32]byte
	copy(digest[:], hash.Sum(nil))
	return digest
}

func workloadCanary(seed [32]byte, offset uint32) [32]byte {
	var canary [32]byte
	for index := range canary {
		block := workloadBlock(seed, uint64((offset+uint32(index))/32))
		canary[index] = block[(offset+uint32(index))%32]
	}
	return canary
}

func workloadBlock(seed [32]byte, counter uint64) [32]byte {
	input := make([]byte, 40)
	copy(input, seed[:])
	binary.BigEndian.PutUint64(input[32:], counter)
	return sha256.Sum256(input)
}
