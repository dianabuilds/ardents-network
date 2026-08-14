package recoverysmoke

import (
	"context"
	"errors"
	"fmt"
	"time"
)

func (observer dockerObserver) waitReplacementTerminal(ctx context.Context, processObserver hostProcessAdapter,
	identities map[string]string, failed map[string]candidateProcess, sequential bool, hostStartedAt int64) error {
	limit := time.Minute
	if sequential {
		limit = 13 * time.Minute
	}
	clean := map[string]bool{"client": true, "publisher": true, "client-endpoint": true,
		"publisher-endpoint": true, "client-app": true, "publisher-app": true}
	for service, identity := range identities {
		var err error
		if clean[service] {
			err = observer.waitContainerFor(ctx, identity, true, limit)
		} else {
			err = observer.waitContainerStopped(ctx, identity, limit)
		}
		if err != nil {
			return fmt.Errorf("replacement %s: %w", service, err)
		}
	}
	for _, process := range failed {
		if _, err := candidateUnavailable(ctx, processObserver, process, hostStartedAt); err != nil {
			return err
		}
	}
	return nil
}

func (observer dockerObserver) finishReplacementCell(ctx context.Context, processObserver hostProcessAdapter,
	cell replacementCell, receiver string, traffic *trafficObservers, sampler *statsSampler,
	failed map[string]candidateProcess, faultReceipts map[string]processFaultEvidence,
	proposalRoutes []routeGeneration,
	cellClock time.Time, hostStartedAt int64) (replacementCell, error) {
	finalTraffic, trafficErr := traffic.snapshotAndRemove(ctx, observer, cellClock)
	samples, sampleErr := sampler.stop()
	if trafficErr != nil || sampleErr != nil || len(samples) < 3 {
		return replacementCell{}, errors.Join(trafficErr, sampleErr,
			errors.New("replacement resource observations are incomplete"))
	}
	client, publisher, application, terminalErr := observer.connectionTerminals(ctx, receiver)
	if terminalErr != nil {
		return replacementCell{}, terminalErr
	}
	cell.ObservedDigest = application.ReceivedDigest
	cell.ClientRouteGeneration, cell.PublisherRouteGeneration = client.RouteGeneration, publisher.RouteGeneration
	cell.ClientRecoveryCount, cell.PublisherRecoveryCount = client.RecoveryCount, publisher.RecoveryCount
	cell.ClientApplicationAccepts, cell.PublisherApplicationAccepts = client.ApplicationIPCAccepts, publisher.ApplicationIPCAccepts
	cell.ClientRouteAccepts, cell.PublisherRouteAccepts = client.RouteAttachmentsAccepted, publisher.RouteAttachmentsAccepted
	cell.ClientAcceptedBytes, cell.ClientAcknowledgedBytes, cell.ClientReceivedBytes =
		client.AcceptedBytes, client.AcknowledgedBytes, client.ReceivedBytes
	cell.PublisherAcceptedBytes, cell.PublisherAcknowledgedBytes, cell.PublisherReceivedBytes =
		publisher.AcceptedBytes, publisher.AcknowledgedBytes, publisher.ReceivedBytes
	cell.ClientQueueHighWater, cell.PublisherQueueHighWater = client.QueueHighWater, publisher.QueueHighWater
	cell.ClientContinuity, cell.PublisherContinuity = client.ContinuityCommitment, publisher.ContinuityCommitment
	cell.Ordered = application.ReceivedBytes == cell.Bytes && application.ReceivedDigest == cell.ExpectedDigest
	cell.Unique = cell.Ordered
	cell.SameConnection = client.ContinuityCommitment != [32]byte{} &&
		client.ContinuityCommitment == publisher.ContinuityCommitment
	cell.ApplicationReconnected = client.ApplicationIPCAccepts != 1 || publisher.ApplicationIPCAccepts != 1
	cell.TerminalClean = client.Class == "clean service connection close" && publisher.Class == client.Class &&
		application.Terminal == "success" && application.ResultClass == client.Class
	cell.ResourceSamples, cell.FinalTraffic = samples, finalTraffic
	cell.FinalCanaryOffset = cell.Bytes - uint32(len(application.ReceivedTail))
	cell.FinalCanary = application.ReceivedTail
	cell.FinalCanaryObservedNanos = time.Since(cellClock).Nanoseconds()
	clientRaw, err := observer.compose(ctx, time.Minute, "logs", "--no-color", "--no-log-prefix", "client")
	if err != nil {
		return replacementCell{}, err
	}
	cell.Proposals, err = replacementProposals(clientRaw, cell.Mode)
	if err != nil {
		return replacementCell{}, err
	}
	if len(cell.Proposals) != len(proposalRoutes) {
		return replacementCell{}, errors.New("replacement process proposal evidence is incomplete")
	}
	for proposalIndex := range cell.Proposals {
		for roleIndex, role := range replacementRoles {
			process := proposalRoutes[proposalIndex].Processes[role]
			receipt, receiptErr := candidateUnavailable(ctx, processObserver, process, hostStartedAt)
			if receiptErr != nil {
				return replacementCell{}, receiptErr
			}
			cell.Proposals[proposalIndex].Processes[roleIndex] = process
			cell.Proposals[proposalIndex].Stopped[roleIndex] = receipt
		}
	}
	if err := observer.bindReplacementPlanTimings(ctx, &cell, clientRaw); err != nil {
		return replacementCell{}, err
	}
	for index := range cell.Events {
		event := &cell.Events[index]
		raw, err := observer.compose(ctx, time.Minute, "logs", "--no-color", "--no-log-prefix", event.Introduction.Service)
		if err != nil {
			return replacementCell{}, err
		}
		attachment, err := parseAttachmentCount(raw, "introduction")
		if err != nil {
			return replacementCell{}, err
		}
		expected := uint32(0)
		for generation := 0; generation <= index+1; generation++ {
			if sameProcessIncarnation(cell.Routes[generation].Processes["introduction"], event.Introduction) {
				expected++
			}
		}
		if expected == 0 || attachment < expected {
			return replacementCell{}, errors.New("fresh authenticated Introduction attachment is missing")
		}
		event.IntroductionAttachment = expected
		if event.Role == "rendezvous" {
			serviceRaw, logsErr := observer.compose(ctx, time.Minute, "logs", "--no-color", "--no-log-prefix", "publisher")
			if logsErr != nil {
				return replacementCell{}, logsErr
			}
			receipt, setupAttachment, opaqueBytes, opaqueDigest, err := introductionSetupEvidence(clientRaw, raw, serviceRaw)
			if err != nil {
				return replacementCell{}, err
			}
			event.IntroductionSetupReceipt, event.IntroductionSetupAttachment = receipt, setupAttachment
			event.IntroductionOpaqueBytes, event.IntroductionOpaqueDigest = opaqueBytes, opaqueDigest
		}
		process, ok := failed[event.Failed.ContainerID]
		if !ok {
			return replacementCell{}, errors.New("failed Route candidate is absent from the stopped set")
		}
		receipt, err := candidateUnavailable(ctx, processObserver, process, hostStartedAt)
		if err != nil {
			return replacementCell{}, err
		}
		faultReceipt, ok := faultReceipts[process.ContainerID]
		if !ok {
			return replacementCell{}, errors.New("failed route candidate fault receipt is missing")
		}
		receipt.Fault = faultReceipt
		event.FailedResource = receipt
	}
	return cell, nil
}
