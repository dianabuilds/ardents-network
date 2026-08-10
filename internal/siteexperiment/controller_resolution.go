package siteexperiment

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"time"
)

type activeResolver struct {
	transport http.RoundTripper
	fixture   *authorityFixture
	gateway   *httptest.Server
	relay     *httptest.Server
}

func openActiveResolver(fixture *authorityFixture, now time.Time) (*activeResolver, error) {
	resolver, err := newResolutionGateway(fixture, func() time.Time { return now })
	if err != nil {
		return nil, err
	}
	key, gatewayHandler, err := newOHTTPGateway(resolver)
	if err != nil {
		return nil, err
	}
	gateway := httptest.NewServer(gatewayHandler)
	relay := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, readErr := io.ReadAll(io.LimitReader(request.Body, resolutionMessageSize*2))
		if readErr != nil {
			http.Error(writer, "relay read", http.StatusBadGateway)
			return
		}
		forward, requestErr := http.NewRequestWithContext(request.Context(), http.MethodPost, gateway.URL, bytes.NewReader(body))
		if requestErr != nil {
			http.Error(writer, "relay request", http.StatusBadGateway)
			return
		}
		forward.Header.Set("Content-Type", request.Header.Get("Content-Type"))
		response, forwardErr := http.DefaultClient.Do(forward)
		if forwardErr != nil {
			http.Error(writer, "relay forward", http.StatusBadGateway)
			return
		}
		defer response.Body.Close()
		writer.Header().Set("Content-Type", response.Header.Get("Content-Type"))
		writer.WriteHeader(response.StatusCode)
		_, _ = io.Copy(writer, io.LimitReader(response.Body, resolutionMessageSize*2))
	}))
	transport, err := newOHTTPTransport(key, relay.URL, &http.Client{Timeout: 15 * time.Second})
	if err != nil {
		relay.Close()
		gateway.Close()
		return nil, err
	}
	return &activeResolver{transport: transport, fixture: fixture, gateway: gateway, relay: relay}, nil
}

func (resolver *activeResolver) close() {
	resolver.relay.Close()
	resolver.gateway.Close()
}

func (resolver *activeResolver) resolve(ctx context.Context, now time.Time) (fixtureRecord, fixtureRecord, error) {
	nameData, nameNonce, err := resolveMessage(ctx, resolver.transport, "name", "site.reference", resolver.fixture.runID, resolver.fixture.networkID, now)
	if err != nil {
		return fixtureRecord{}, fixtureRecord{}, err
	}
	name, err := verifyNameRecord(nameData, resolver.fixture.namePublic, resolver.fixture.runID, resolver.fixture.networkID, nameNonce, now)
	if err != nil {
		return fixtureRecord{}, fixtureRecord{}, err
	}
	descriptorData, descriptorNonce, err := resolveMessage(ctx, resolver.transport, "reachability", name.Target, resolver.fixture.runID, resolver.fixture.networkID, now)
	if err != nil {
		return fixtureRecord{}, fixtureRecord{}, err
	}
	descriptor, err := verifyDescriptor(descriptorData, resolver.fixture.servicePublic, resolver.fixture.runID, resolver.fixture.networkID, descriptorNonce, name.Target, resolver.fixture.instanceGeneration, now)
	return name, descriptor, err
}
