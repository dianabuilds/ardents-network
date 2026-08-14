package recoverysmoke

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

func (observer dockerObserver) runNoFailureBaseline(ctx context.Context, direction string) (trafficBaseline, error) {
	baselineClock := time.Now()
	_, _ = observer.compose(ctx, time.Minute, "down", "-v", "--remove-orphans")
	if err := refreshWorkload(observer.generation); err != nil {
		return trafficBaseline{}, err
	}
	observer.direction, observer.gateOffset = direction, 0
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
	traffic, err := observer.startTrafficObservers(ctx, identities)
	if err != nil {
		return trafficBaseline{}, err
	}
	defer func() { _ = traffic.remove(context.Background(), observer) }()
	sampler := observer.startStats(ctx, identities, time.Now())
	defer func() { _, _ = sampler.stop() }()
	for _, service := range recoveryServiceNames() {
		if err := observer.waitContainer(ctx, identities[service], true); err != nil {
			return trafficBaseline{}, fmt.Errorf("baseline %s: %w", service, err)
		}
	}
	terminalNanos := time.Since(baselineClock).Nanoseconds()
	finalTraffic, trafficErr := traffic.snapshotAndRemove(ctx, observer, baselineClock)
	samples, sampleErr := sampler.stop()
	if sampleErr != nil || trafficErr != nil {
		return trafficBaseline{}, errors.Join(sampleErr, trafficErr)
	}
	client, publisher, terminalErr := observer.baselineTerminals(ctx)
	if terminalErr != nil || client.Class != "clean service connection close" || publisher.Class != "clean service connection close" {
		return trafficBaseline{}, errors.Join(terminalErr, errors.New("paired no-failure baseline was not clean"))
	}
	_ = samples
	clientTraffic := finalTraffic.ClientReceived + finalTraffic.ClientSent
	publisherTraffic := finalTraffic.PublisherReceived + finalTraffic.PublisherSent
	if _, err := observer.compose(ctx, time.Minute, "down", "-v", "--remove-orphans"); err != nil {
		return trafficBaseline{}, err
	}
	return trafficBaseline{client: clientTraffic, publisher: publisherTraffic,
		terminalNanos: terminalNanos, finalTraffic: finalTraffic,
		routes: traffic.routes, observers: traffic.projections}, nil
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
