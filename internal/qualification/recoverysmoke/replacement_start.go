package recoverysmoke

import (
	"context"
	"fmt"
	"time"
)

func (observer dockerObserver) startReplacementServices(ctx context.Context, fixture prepared,
	plan replacementPlan) (map[string]string, error) {
	introduction, err := candidateService(fixture.candidates, plan.selections[0]["introduction"])
	if err != nil {
		return nil, err
	}
	if _, err := observer.compose(ctx, time.Minute, "up", "-d", introduction, "publisher-endpoint"); err != nil {
		return nil, fmt.Errorf("start initial Introduction and publisher Endpoint: %w", err)
	}
	for _, service := range []string{introduction, "publisher-endpoint"} {
		if err := observer.waitReady(ctx, service); err != nil {
			return nil, fmt.Errorf("wait for replacement %s readiness: %w", service, err)
		}
	}
	operatorID, err := observer.runDetached(ctx, "publication-operator")
	if err != nil {
		return nil, err
	}
	if err := observer.waitContainer(ctx, operatorID, true); err != nil {
		return nil, fmt.Errorf("publication operator failed: %w", err)
	}
	if _, err := observer.compose(ctx, time.Minute, "up", "-d", "publisher-app"); err != nil {
		return nil, fmt.Errorf("start publisher Application: %w", err)
	}
	if _, err := observer.compose(ctx, time.Minute, "up", "-d", "publisher"); err != nil {
		return nil, fmt.Errorf("start bounded publisher Attachments: %w", err)
	}
	if err := observer.waitReady(ctx, "publisher"); err != nil {
		return nil, fmt.Errorf("wait for bounded publisher Attachment readiness: %w", err)
	}
	if _, err := observer.compose(ctx, time.Minute, append([]string{"--profile", "s42", "up", "-d"}, plan.services...)...); err != nil {
		return nil, fmt.Errorf("start finite replacement candidates: %w", err)
	}
	for _, service := range plan.services {
		if err := observer.waitReady(ctx, service); err != nil {
			return nil, fmt.Errorf("wait for replacement candidate %s readiness: %w", service, err)
		}
	}
	if _, err := observer.compose(ctx, time.Minute, "up", "-d", "client-endpoint"); err != nil {
		return nil, fmt.Errorf("start client Endpoint: %w", err)
	}
	if err := observer.waitReady(ctx, "client-endpoint"); err != nil {
		return nil, fmt.Errorf("wait for client Endpoint readiness: %w", err)
	}
	if _, err := observer.compose(ctx, time.Minute, "up", "-d", "client-app", "client"); err != nil {
		return nil, fmt.Errorf("start client Application and bounded Route policy: %w", err)
	}
	return observer.replacementCoreIdentities(ctx, plan)
}

func (observer dockerObserver) replacementCoreIdentities(ctx context.Context, plan replacementPlan) (map[string]string, error) {
	services := []string{"client", "publisher",
		"client-endpoint", "publisher-endpoint", "client-app", "publisher-app"}
	services = append(services, plan.services...)
	result := make(map[string]string, len(services)+4)
	for _, service := range services {
		identity, err := observer.serviceID(ctx, service)
		if err != nil {
			return nil, fmt.Errorf("resolve replacement %s process identity: %w", service, err)
		}
		result[service] = identity
	}
	return result, nil
}
