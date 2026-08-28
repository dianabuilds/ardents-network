package endpoint_test

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	endpointapi "github.com/dianabuilds/ardents-network/internal/endpoint"
)

func TestAlphaTransparentConnectionClassifiesPublisherApplicationReset(t *testing.T) {
	fixture := newFixture(t)
	publisher := newPublisher(t, fixture)
	publication := publish(t, publisher, fixture, fixture.first, fixture.firstPrivate)
	client, err := endpointapi.New(endpointapi.Setup{NetworkID: fixture.networkID, BrokerID: [32]byte{8},
		AuthorityPublic: fixture.authorityPublic, IntroductionPublic: fixture.introductionPublic,
		ConnectionPrincipal: fixture.clientPrincipal})
	if err != nil {
		t.Fatal(err)
	}
	binding := issuedAlphaBinding(t, fixture.networkID, fixture.first.Target)
	clientRoute, publisherRoute := net.Pipe()
	publisherEndpoint, publisherApplication := tcpPair(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	publisherDone := make(chan serviceOutcome, 1)
	publisherSession := admit(t, publisher, "connection", fixture.publisherPrincipal, fixture.now)
	go func() {
		result, runErr := publisher.Accept(ctx, endpointapi.InboundConnectionRequest{
			Principal: fixture.publisherPrincipal, Capability: publisherSession, Route: publisherRoute,
			Application: publisherEndpoint, BytesEachDirection: 64 << 10, At: fixture.now,
		})
		publisherDone <- serviceOutcome{result, runErr}
	}()
	go resetPublisherApplicationAfterPartialResponse(publisherApplication)
	clientSession := admit(t, client, "connection", fixture.clientPrincipal, fixture.now)
	running, err := client.StartAlphaTransparentConnection(ctx, endpointapi.AlphaTransparentConnectionRequest{Binding: binding,
		Connection: endpointapi.OutboundConnectionRequest{Principal: fixture.clientPrincipal, Capability: clientSession,
			Target: fixture.first.Target, Publication: publication, Route: clientRoute, BytesEachDirection: 64 << 10, At: fixture.now}})
	if err != nil {
		t.Fatal(err)
	}
	ready, ok := <-running.Ready()
	if !ok || ready.URL != "http://blog.alice.ard/" || ready.AlphaProxyURL == "" {
		t.Fatalf("transparent reset origin readiness = %+v", ready)
	}
	proxyURL, err := url.Parse(ready.AlphaProxyURL)
	if err != nil {
		t.Fatal(err)
	}
	httpClient := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}, Timeout: time.Second}
	response, err := httpClient.Get(ready.URL)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK || string(body) != "first-" {
		t.Fatalf("partial Publisher response = %d %q", response.StatusCode, body)
	}
	userOutcome := <-running.Done()
	if userOutcome.Result.Class != "abrupt connection loss" || userOutcome.Err == nil ||
		userOutcome.Result.AuthenticatedTarget != fixture.first.Target || userOutcome.Result.Generation != 1 {
		t.Fatalf("User Publisher-reset outcome = %+v / %v", userOutcome.Result, userOutcome.Err)
	}
	publisherOutcome := <-publisherDone
	if publisherOutcome.result.Class != "abrupt connection loss" || publisherOutcome.err == nil ||
		publisherOutcome.result.AuthenticatedTarget != fixture.first.Target || publisherOutcome.result.Generation != 1 {
		t.Fatalf("Publisher Application-reset outcome = %+v / %v", publisherOutcome.result, publisherOutcome.err)
	}
	for _, candidate := range []string{ready.URL, "http://unregistered.ard/", "http://ordinary.invalid/"} {
		response, err := httpClient.Get(candidate)
		if err != nil {
			continue
		}
		_ = response.Body.Close()
		if response.StatusCode < http.StatusBadRequest {
			t.Fatalf("Publisher reset selected fallback %q with status %d", candidate, response.StatusCode)
		}
	}
}

func TestPublisherEndpointRouteLossRetainsAuthenticatedServiceIdentity(t *testing.T) {
	fixture := newFixture(t)
	client, publisher, publication := connectedEndpoints(t, fixture)
	clientRoute, publisherRoute := net.Pipe()
	clientEndpoint, clientApplication := net.Pipe()
	publisherEndpoint, publisherApplication := net.Pipe()
	defer clientApplication.Close()
	defer publisherApplication.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	outcomes := runConnections(ctx, fixture, client, publisher, publication, clientRoute, publisherRoute, clientEndpoint, publisherEndpoint)
	written := make(chan error, 1)
	go func() {
		_, err := clientApplication.Write([]byte("authenticated"))
		written <- err
	}()
	received := make([]byte, len("authenticated"))
	if _, err := io.ReadFull(publisherApplication, received); err != nil || string(received) != "authenticated" {
		t.Fatalf("Publisher Endpoint route was not authenticated before loss: %q / %v", received, err)
	}
	if err := <-written; err != nil {
		t.Fatal(err)
	}
	_ = clientRoute.Close()
	_ = publisherRoute.Close()
	for range 2 {
		outcome := <-outcomes
		if outcome.err == nil || outcome.result.Class != "abrupt connection loss" ||
			outcome.result.AuthenticatedTarget != fixture.first.Target || outcome.result.Generation != 1 ||
			outcome.result.RouteGeneration != 1 || outcome.result.RecoveryCount != 0 {
			t.Fatalf("Publisher Endpoint route-loss outcome = %+v / %v", outcome.result, outcome.err)
		}
	}
}

func resetPublisherApplicationAfterPartialResponse(connection net.Conn) {
	defer connection.Close()
	request, err := http.ReadRequest(bufio.NewReader(connection))
	if err != nil {
		return
	}
	_ = request.Body.Close()
	_, _ = io.WriteString(connection, "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: 12\r\n\r\nfirst-")
	if tcp, ok := connection.(*net.TCPConn); ok {
		_ = tcp.SetLinger(0)
	}
}
