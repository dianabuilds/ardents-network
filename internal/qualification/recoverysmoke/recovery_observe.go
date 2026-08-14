package recoverysmoke

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

func (observer dockerObserver) startRecoveryServices(ctx context.Context) error {
	if _, err := observer.compose(ctx, time.Minute, "up", "-d", "introduction", "publisher-endpoint"); err != nil {
		return err
	}
	for _, service := range []string{"introduction", "publisher-endpoint"} {
		if err := observer.waitReady(ctx, service); err != nil {
			return err
		}
	}
	operatorID, err := observer.runDetached(ctx, "publication-operator")
	if err != nil {
		return err
	}
	if err := observer.waitContainer(ctx, operatorID, true); err != nil {
		return errors.New("publication operator failed")
	}
	servers := []string{"publisher-app", "publisher", "responder", "rendezvous", "initiator"}
	if _, err := observer.compose(ctx, time.Minute, append([]string{"up", "-d"}, servers...)...); err != nil {
		return err
	}
	for _, service := range []string{"publisher", "responder", "rendezvous", "initiator"} {
		if err := observer.waitReady(ctx, service); err != nil {
			return err
		}
	}
	if _, err := observer.compose(ctx, time.Minute, "up", "-d", "carrier-fault"); err != nil {
		return err
	}
	if err := observer.waitReady(ctx, "carrier-fault"); err != nil {
		return err
	}
	if _, err := observer.compose(ctx, time.Minute, "up", "-d", "client-endpoint"); err != nil {
		return err
	}
	if err := observer.waitReady(ctx, "client-endpoint"); err != nil {
		return err
	}
	_, err = observer.compose(ctx, time.Minute, "up", "-d", "client-app", "client")
	return err
}

func recoveryServiceNames() []string {
	return []string{"client", "initiator", "introduction", "rendezvous", "responder", "publisher",
		"client-endpoint", "publisher-endpoint", "client-app", "publisher-app"}
}

func (observer dockerObserver) recoveryIdentities(ctx context.Context) (map[string]string, error) {
	identities := make(map[string]string, len(recoveryServiceNames()))
	for _, service := range recoveryServiceNames() {
		identity, err := observer.serviceID(ctx, service)
		if err != nil {
			return nil, err
		}
		identities[service] = identity
	}
	return identities, nil
}

func (observer dockerObserver) recoveryTerminals(ctx context.Context, receiver string) (
	serviceconn.Result, serviceconn.Result, applicationEvidence, []route.Evidence, error) {
	logs := make(map[string][]byte, 5)
	for _, service := range []string{"client-endpoint", "publisher-endpoint", receiver, "client", "publisher"} {
		raw, err := observer.compose(ctx, time.Minute, "logs", "--no-color", "--no-log-prefix", service)
		if err != nil {
			return serviceconn.Result{}, serviceconn.Result{}, applicationEvidence{}, nil, err
		}
		logs[service] = raw
	}
	client, err := terminalEndpoint(logs["client-endpoint"])
	if err != nil {
		return client, serviceconn.Result{}, applicationEvidence{}, nil, err
	}
	publisher, err := terminalEndpoint(logs["publisher-endpoint"])
	if err != nil {
		return client, publisher, applicationEvidence{}, nil, err
	}
	application, err := terminalApplication(logs[receiver])
	if err != nil {
		return client, publisher, application, nil, err
	}
	var routes []route.Evidence
	for _, service := range []string{"client", "publisher"} {
		for _, line := range splitLines(logs[service]) {
			var value route.Evidence
			if json.Unmarshal(line, &value) == nil && value.Kind == "complete" {
				routes = append(routes, value)
			}
		}
	}
	if len(routes) < 4 {
		return client, publisher, application, routes, fmt.Errorf("four endpoint Route Attachment observations required, got %d", len(routes))
	}
	return client, publisher, application, routes, nil
}
