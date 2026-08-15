package recoverysmoke

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/qualification/recovery"
)

func (attempt *impairedAttempt) finishCell(ctx context.Context, receiver string, progress []progressSample,
	samples []recovery.ResourceSample, shapers []shaperEvidence, measurementNanos, terminalNanos int64) (impairedCell, error) {
	client, publisher, application, err := attempt.observer.connectionTerminals(ctx, receiver)
	if err != nil {
		return impairedCell{}, err
	}
	if len(progress) == 0 || len(samples) == 0 {
		return impairedCell{}, errors.New("S4.3 terminal progress or resource evidence is missing")
	}
	expected := workloadDigest(attempt.seed, 192<<20)
	cell := impairedCell{Direction: attempt.direction, Mode: "impaired", CellManifestDigest: attempt.manifestDigest,
		HostProcesses: attempt.hostProcesses, Seed: attempt.seed, ExpectedDigest: expected,
		ObservedDigest: application.ReceivedDigest, Bytes: 192 << 20,
		MeasurementDelivered: progress[len(progress)-1].Delivered,
		HostStartedAtNanos:   attempt.activeHostNanos, ActiveStartedAtNanos: 1,
		MeasurementCompletedAtNanos: measurementNanos + 1, TerminalNanos: terminalNanos,
		ClientRouteGeneration: client.RouteGeneration, PublisherRouteGeneration: publisher.RouteGeneration,
		ClientRecoveryCount: client.RecoveryCount, PublisherRecoveryCount: publisher.RecoveryCount,
		ClientApplicationAccepts:    client.ApplicationIPCAccepts,
		PublisherApplicationAccepts: publisher.ApplicationIPCAccepts,
		ClientRouteAccepts:          client.RouteAttachmentsAccepted, PublisherRouteAccepts: publisher.RouteAttachmentsAccepted,
		ClientAcceptedBytes: client.AcceptedBytes, ClientAcknowledgedBytes: client.AcknowledgedBytes,
		ClientReceivedBytes: client.ReceivedBytes, PublisherAcceptedBytes: publisher.AcceptedBytes,
		PublisherAcknowledgedBytes: publisher.AcknowledgedBytes, PublisherReceivedBytes: publisher.ReceivedBytes,
		ClientQueueHighWater: client.QueueHighWater, PublisherQueueHighWater: publisher.QueueHighWater,
		ClientContinuity: client.ContinuityCommitment, PublisherContinuity: publisher.ContinuityCommitment,
		Ordered: application.Terminal == "success" && application.ReceivedDigest == expected,
		Unique:  application.ReceivedBytes == 192<<20,
		SameConnection: client.ServiceConnectionsHighWater == 1 && publisher.ServiceConnectionsHighWater == 1 &&
			client.ContinuityCommitment != [32]byte{} && client.ContinuityCommitment == publisher.ContinuityCommitment,
		ApplicationReconnected: client.ApplicationIPCAccepts != 1 || publisher.ApplicationIPCAccepts != 1,
		TerminalClean: client.Class == "clean service connection close" && publisher.Class == client.Class &&
			application.ResultClass == client.Class,
		Progress: progress, ResourceSamples: samples, TrafficStart: samples[0], TrafficEnd: samples[len(samples)-1],
		DirectBefore: attempt.directBefore, Shapers: shapers}
	return cell, nil
}
