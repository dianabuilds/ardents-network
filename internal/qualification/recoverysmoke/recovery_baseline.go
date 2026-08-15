package recoverysmoke

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/recovery"
	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

type trafficBaseline struct {
	client, publisher uint64
	terminalNanos     int64
	finalTraffic      recovery.ResourceSample
}

func (observer dockerObserver) runNoFailureBaseline(ctx context.Context, direction string) (
	result trafficBaseline, returnErr error) {
	baselineClock := time.Now()
	if err := observer.resetRecoveryTopology(ctx, time.Minute); err != nil {
		return trafficBaseline{}, err
	}
	if !observer.fixedWorkload {
		if err := refreshWorkload(observer.generation); err != nil {
			return trafficBaseline{}, err
		}
	}
	observer = observer.forRecoveryOperation(direction)
	seed, err := recoveryDirectionSeed(observer.generation, direction)
	if err != nil {
		return trafficBaseline{}, err
	}
	observer.gateOffset = recoveryFaultOffset(seed)
	gateRoot := filepath.Join(observer.input.FixtureRoot, "gate")
	if err := resetRecoveryGates(gateRoot); err != nil {
		return trafficBaseline{}, err
	}
	if err := setRouteAttachments(observer.input.FixtureRoot, 1); err != nil {
		return trafficBaseline{}, err
	}
	if err := configureRecoveryDirection(observer.generation, direction); err != nil {
		return trafficBaseline{}, err
	}
	if _, err := observer.compose(ctx, time.Minute, "--profile", "setup", "run", "--no-deps", "--rm", "volume-init"); err != nil {
		return trafficBaseline{}, err
	}
	if err := observer.startRecoveryServices(ctx); err != nil {
		return trafficBaseline{}, err
	}
	identities, err := observer.recoveryIdentities(ctx)
	if err != nil {
		return trafficBaseline{}, err
	}
	sampler := observer.startStats(ctx, identities, baselineClock)
	defer func() {
		if sampler != nil {
			_, sampleErr := sampler.stop()
			returnErr = errors.Join(returnErr, sampleErr)
		}
	}()
	if err := sampler.waitReady(ctx); err != nil {
		return trafficBaseline{}, err
	}
	sender, receiver := "client", "publisher-app"
	if direction == "publisher-to-client" {
		sender, receiver = "publisher", "client-app"
	}
	gateWait, err := pacedGateWait(0, observer.gateOffset, observer.input.ChunkDelay)
	if err != nil {
		return trafficBaseline{}, err
	}
	if _, err := observer.waitGate(ctx, gateRoot, sender, observer.gateOffset, gateWait); err != nil {
		return trafficBaseline{}, err
	}
	if delivered, err := observer.waitProgress(ctx, receiver, observer.gateOffset); err != nil || delivered != observer.gateOffset {
		return trafficBaseline{}, errors.Join(err, errors.New("baseline did not reach its exact paired gate"))
	}
	if err := writeRelease(gateRoot, sender); err != nil {
		return trafficBaseline{}, err
	}
	waitOrder, err := recoveryTerminalWaitOrder(receiver)
	if err != nil {
		return trafficBaseline{}, err
	}
	if err := observer.waitContainer(ctx, identities[waitOrder[0]], true); err != nil {
		return trafficBaseline{}, fmt.Errorf("baseline %s: %w", waitOrder[0], err)
	}
	terminalNanos := time.Since(baselineClock).Nanoseconds()
	samples, sampleErr := sampler.stop()
	sampler = nil
	if sampleErr != nil {
		return trafficBaseline{}, sampleErr
	}
	for _, service := range waitOrder[1:] {
		if err := observer.waitContainer(ctx, identities[service], true); err != nil {
			return trafficBaseline{}, fmt.Errorf("baseline %s: %w", service, err)
		}
	}
	finalTraffic, sampleErr := finalResourceSample(samples, terminalNanos)
	if sampleErr != nil {
		return trafficBaseline{}, sampleErr
	}
	client, publisher, terminalErr := observer.baselineTerminals(ctx)
	if terminalErr != nil || client.Class != "clean service connection close" || publisher.Class != "clean service connection close" {
		return trafficBaseline{}, errors.Join(terminalErr, errors.New("paired no-failure baseline was not clean"))
	}
	_ = samples
	clientTraffic := finalTraffic.ClientReceived + finalTraffic.ClientSent
	publisherTraffic := finalTraffic.PublisherReceived + finalTraffic.PublisherSent
	if err := observer.resetRecoveryTopology(ctx, time.Minute); err != nil {
		return trafficBaseline{}, err
	}
	return trafficBaseline{client: clientTraffic, publisher: publisherTraffic,
		terminalNanos: terminalNanos, finalTraffic: finalTraffic}, nil
}

func (observer dockerObserver) baselineTerminals(ctx context.Context) (serviceconn.Result, serviceconn.Result, error) {
	clientRaw, clientErr := observer.compose(ctx, time.Minute, "logs", "--no-color", "--no-log-prefix", "client-endpoint")
	publisherRaw, publisherErr := observer.compose(ctx, time.Minute, "logs", "--no-color", "--no-log-prefix", "publisher-endpoint")
	if clientErr != nil || publisherErr != nil {
		return serviceconn.Result{}, serviceconn.Result{}, errors.Join(clientErr, publisherErr)
	}
	client, clientErr := terminalEndpoint(clientRaw)
	publisher, publisherErr := terminalEndpoint(publisherRaw)
	return client, publisher, errors.Join(clientErr, publisherErr)
}
