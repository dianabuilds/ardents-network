package servicesmoke

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

func (observer dockerObserver) runGeneration(ctx context.Context, fixture prepared, number int) (generationEvidence, bool, error) {
	observer.generation = filepath.Join(observer.input.FixtureRoot, "generations", strconv.Itoa(number))
	if err := refreshWorkload(observer.generation); err != nil {
		return generationEvidence{}, false, err
	}
	observer.evidenceFile = filepath.Join(observer.input.EvidenceRoot, "empty.json")
	down := []string{"down", "--remove-orphans"}
	if number == 1 {
		down = []string{"down", "-v", "--remove-orphans"}
	}
	_, _ = observer.compose(ctx, time.Minute, down...)
	if _, err := observer.compose(ctx, time.Minute, "--profile", "setup", "run", "--no-deps", "--rm", "volume-init"); err != nil {
		return generationEvidence{}, false, errors.Join(errors.New("stage 3 socket volumes could not be initialized"), err)
	}
	if _, err := observer.compose(ctx, time.Minute, "up", "-d", "introduction"); err != nil {
		return generationEvidence{}, false, err
	}
	if err := observer.waitReady(ctx, "introduction"); err != nil {
		return generationEvidence{}, false, err
	}
	if _, err := observer.compose(ctx, time.Minute, "up", "-d", "publisher-endpoint"); err != nil {
		return generationEvidence{}, false, err
	}
	if err := observer.waitReady(ctx, "publisher-endpoint"); err != nil {
		return generationEvidence{}, false, err
	}
	operatorID, err := observer.runDetached(ctx, "publication-operator")
	if err != nil || observer.waitContainer(ctx, operatorID, true) != nil {
		return generationEvidence{}, false, errors.New("publication operator failed")
	}
	routeServers := []string{"publisher-app", "publisher", "responder", "rendezvous", "initiator"}
	if _, err := observer.compose(ctx, time.Minute, append([]string{"up", "-d"}, routeServers...)...); err != nil {
		return generationEvidence{}, false, err
	}
	for _, role := range []string{"publisher", "responder", "rendezvous", "initiator"} {
		if err := observer.waitReady(ctx, role); err != nil {
			return generationEvidence{}, false, err
		}
	}
	if _, err := observer.compose(ctx, time.Minute, "up", "-d", "client-endpoint"); err != nil {
		return generationEvidence{}, false, err
	}
	if err := observer.waitReady(ctx, "client-endpoint"); err != nil {
		return generationEvidence{}, false, err
	}
	if _, err := observer.compose(ctx, time.Minute, "up", "-d", "client-app", "client"); err != nil {
		return generationEvidence{}, false, err
	}
	hostileID, err := observer.runDetached(ctx, "hostile-sibling")
	if err != nil {
		return generationEvidence{}, false, err
	}
	hostileRejected := observer.waitContainer(ctx, hostileID, false) == nil
	hostile, hostileErr := observer.hostileObservation(ctx, hostileID)
	if hostileErr != nil {
		return generationEvidence{}, false, hostileErr
	}
	services := []string{"client", "initiator", "introduction", "rendezvous", "responder", "publisher",
		"client-endpoint", "publisher-endpoint", "client-app", "publisher-app"}
	identities := make([]string, 0, 12)
	for _, service := range services {
		identity, serviceErr := observer.serviceID(ctx, service)
		if serviceErr != nil {
			return generationEvidence{}, hostileRejected, serviceErr
		}
		identities = append(identities, identity)
		if err := observer.waitContainer(ctx, identity, true); err != nil {
			return generationEvidence{}, hostileRejected, err
		}
	}
	identities = append(identities, operatorID, hostileID)
	value, err := observer.collectGeneration(ctx, fixture, number, services, identities)
	if err != nil {
		return generationEvidence{}, hostileRejected, err
	}
	value.HostileSibling = hostile
	cleanup := []string{"down", "--remove-orphans"}
	if number == 2 {
		cleanup = []string{"down", "-v", "--remove-orphans"}
	}
	if _, err := observer.compose(ctx, time.Minute, cleanup...); err != nil {
		return generationEvidence{}, hostileRejected, err
	}
	remaining, err := observer.compose(ctx, time.Minute, "ps", "-q")
	if err != nil || len(strings.TrimSpace(string(remaining))) != 0 {
		return generationEvidence{}, hostileRejected, errors.New("stage 3 containers remain after cleanup")
	}
	return value, hostileRejected, nil
}

func (observer dockerObserver) collectGeneration(ctx context.Context, fixture prepared, number int,
	services, identities []string) (generationEvidence, error) {
	logs := map[string][]byte{}
	for _, service := range services {
		raw, err := observer.compose(ctx, time.Minute, "logs", "--no-color", "--no-log-prefix", service)
		if err != nil {
			return generationEvidence{}, err
		}
		logs[service] = raw
	}
	clientEndpoint, err := terminalEndpoint(logs["client-endpoint"])
	if err != nil {
		return generationEvidence{}, err
	}
	publisherEndpoint, err := terminalEndpoint(logs["publisher-endpoint"])
	if err != nil {
		return generationEvidence{}, err
	}
	clientApp, err := terminalApplication(logs["client-app"])
	if err != nil {
		return generationEvidence{}, err
	}
	publisherApp, err := terminalApplication(logs["publisher-app"])
	if err != nil {
		return generationEvidence{}, err
	}
	value := generationEvidence{Generation: uint64(number), Credential: fixture.credentials[number-1],
		IntroductionAcknowledgement: publisherEndpoint.IntroductionAcknowledgement,
		PublicationReady:            len(publisherEndpoint.IntroductionAcknowledgement) != 0,
		ClientEndpoint:              endpointReceipt(clientEndpoint), PublisherEndpoint: endpointReceipt(publisherEndpoint),
		ClientApplication: clientApp, PublisherApplication: publisherApp, ContainerIDs: identities}
	value.ClientGrant = grantEvidence{Broker: fixture.bindings[number-1][0].Broker,
		Principal: fixture.bindings[number-1][0].Principal, Surface: fixture.bindings[number-1][0].Surface}
	value.PublisherGrant = grantEvidence{Broker: fixture.bindings[number-1][1].Broker,
		Principal: fixture.bindings[number-1][1].Principal, Surface: fixture.bindings[number-1][1].Surface}
	for _, role := range []string{"client", "initiator", "introduction", "rendezvous", "responder", "publisher"} {
		observation, routeErr := terminalRoute(logs[role])
		if routeErr != nil {
			return generationEvidence{}, routeErr
		}
		value.Roles = append(value.Roles, routeReceipt(observation))
	}
	return value, nil
}

func endpointReceipt(value serviceconn.Result) endpointEvidence {
	return endpointEvidence{Class: value.Class, AuthenticatedTarget: value.AuthenticatedTarget,
		Generation: value.Generation, AcceptedBytes: value.AcceptedBytes, ReceivedBytes: value.ReceivedBytes,
		ConnectionCanary: value.ConnectionCanary, PrincipalCommitment: value.PrincipalCommitment,
		SessionCommitment: value.SessionCommitment, GrantSurface: value.GrantSurface,
		SessionConsumed: value.SessionConsumed, BrokerCommitment: value.BrokerCommitment,
		GrantCommitment: value.GrantCommitment, SessionIssuedAt: value.SessionIssuedAt,
		SessionExpiresAt: value.SessionExpiresAt, MemoryHighWater: value.MemoryHighWater, CPUSeconds: value.CPUSeconds,
		OpenFilesHighWater: value.OpenFilesHighWater, GoroutinesHighWater: value.GoroutinesHighWater,
		ActiveSessions: value.ActiveSessions, TimerHighWater: value.TimerHighWater, QueueHighWater: value.QueueHighWater,
		AcceptedIPCHighWater:        value.AcceptedIPCHighWater,
		ServiceConnectionsHighWater: value.ServiceConnectionsHighWater,
		ControlFilesHighWater:       value.ControlFilesHighWater}
}

func routeReceipt(value route.Evidence) roleEvidence {
	receipt := roleEvidence{Role: value.Role, RuntimeID: value.RuntimeID, Terminal: value.Terminal,
		PID: value.PID, Cleanup: value.Cleanup, ManifestDigest: value.ManifestDigest,
		NetworkID: value.NetworkID, OpaqueBytes: value.OpaqueBytes, SourceID: value.SourceID,
		BuildDigest: value.BuildDigest, OpaqueDigest: value.OpaqueDigest,
		ReverseOpaqueBytes: value.ReverseOpaqueBytes, ReverseOpaqueDigest: value.ReverseOpaqueDigest,
		NodeID: value.NodeID, NextNodeID: value.NextNodeID}
	for _, position := range value.Positions {
		receipt.Positions = append(receipt.Positions, routePositionEvidence{Role: position.Role,
			NodeID: position.NodeID, Endpoint: position.Endpoint})
	}
	return receipt
}

func observedFixedRoute(generations []generationEvidence) bool {
	for _, generation := range generations {
		roles := make(map[string]roleEvidence, len(generation.Roles))
		for _, role := range generation.Roles {
			roles[role.Role] = role
		}
		client := roles["client"]
		chain := []string{"initiator", "introduction", "rendezvous", "responder"}
		if len(client.Positions) != len(chain) || roles["publisher"].NodeID == [32]byte{} {
			return false
		}
		for index, name := range chain {
			expected := roles["publisher"].NodeID
			if index+1 < len(chain) {
				expected = client.Positions[index+1].NodeID
			}
			if client.Positions[index].Role != name || client.Positions[index].NodeID != roles[name].NodeID ||
				roles[name].NextNodeID != expected {
				return false
			}
		}
	}
	return len(generations) != 0
}
