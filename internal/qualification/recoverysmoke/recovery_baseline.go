package recoverysmoke

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

func (observer dockerObserver) runNoFailureBaseline(ctx context.Context, direction string) (uint64, uint64, error) {
	_, _ = observer.compose(ctx, time.Minute, "down", "-v", "--remove-orphans")
	if err := refreshWorkload(observer.generation); err != nil {
		return 0, 0, err
	}
	observer.direction, observer.gateOffset = direction, 0
	if err := setRouteAttachments(observer.input.FixtureRoot, 1); err != nil {
		return 0, 0, err
	}
	if err := configureRecoveryDirection(observer.generation, direction); err != nil {
		return 0, 0, err
	}
	if _, err := observer.compose(ctx, time.Minute, "--profile", "setup", "run", "--no-deps", "--rm", "volume-init"); err != nil {
		return 0, 0, err
	}
	if err := observer.startRecoveryServices(ctx); err != nil {
		return 0, 0, err
	}
	identities, err := observer.recoveryIdentities(ctx)
	if err != nil {
		return 0, 0, err
	}
	sampler := observer.startStats(ctx, identities, time.Now())
	defer func() { _, _ = sampler.stop() }()
	for _, service := range recoveryServiceNames() {
		if err := observer.waitContainer(ctx, identities[service], true); err != nil {
			return 0, 0, fmt.Errorf("baseline %s: %w", service, err)
		}
	}
	samples, sampleErr := sampler.stop()
	if sampleErr != nil {
		return 0, 0, sampleErr
	}
	client, publisher, terminalErr := observer.baselineTerminals(ctx)
	if terminalErr != nil || client.Class != "clean service connection close" || publisher.Class != "clean service connection close" {
		return 0, 0, errors.Join(terminalErr, errors.New("paired no-failure baseline was not clean"))
	}
	last := samples[len(samples)-1]
	if _, err := observer.compose(ctx, time.Minute, "down", "-v", "--remove-orphans"); err != nil {
		return 0, 0, err
	}
	return last.ClientReceived + last.ClientSent, last.PublisherReceived + last.PublisherSent, nil
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
