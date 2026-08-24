package endpoint_test

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	endpointapi "github.com/dianabuilds/ardents-network/internal/endpoint"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

func TestReferenceConnectionServesDeclaredStaticRouteAfterTargetAuthentication(t *testing.T) {
	fixture := newFixture(t)
	publisher := newPublisher(t, fixture)
	publication := publish(t, publisher, fixture, fixture.first, fixture.firstPrivate)
	client, err := endpointapi.New(endpointapi.Setup{NetworkID: fixture.networkID, BrokerID: [32]byte{8},
		AuthorityPublic: fixture.authorityPublic, IntroductionPublic: fixture.introductionPublic,
		ConnectionPrincipal: fixture.clientPrincipal})
	if err != nil {
		t.Fatal(err)
	}
	link, err := targetlink.Encode(targetlink.Link{Network: fixture.networkID, Target: fixture.first.Target})
	if err != nil {
		t.Fatal(err)
	}
	clientRoute, publisherRoute := net.Pipe()
	publisherEndpoint, publisherApplication := net.Pipe()
	defer publisherApplication.Close()
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
	requestSeen := make(chan *http.Request, 1)
	go serveOneStaticReference(publisherApplication, requestSeen)
	clientSession := admit(t, client, "connection", fixture.clientPrincipal, fixture.now)
	running, err := client.StartReferenceConnection(ctx, endpointapi.ReferenceConnectionRequest{
		TargetLink: link,
		Routes:     map[string]string{"": "/"},
		Connection: endpointapi.OutboundConnectionRequest{Principal: fixture.clientPrincipal, Capability: clientSession,
			Target: fixture.first.Target, Publication: publication, Route: clientRoute, BytesEachDirection: 64 << 10, At: fixture.now},
	})
	if err != nil {
		t.Fatal(err)
	}
	ready, ok := <-running.Ready()
	if !ok || ready.URL == "" || ready.AuthenticatedTarget != fixture.first.Target {
		t.Fatalf("Reference origin was not published after exact Target authentication: %+v", ready)
	}
	httpClient := &http.Client{Transport: &http.Transport{Proxy: nil}}
	response, err := httpClient.Get(ready.URL)
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if readErr != nil || response.StatusCode != http.StatusOK || string(body) != "<h1>Reference</h1>" {
		t.Fatalf("browser response = %d %q %v", response.StatusCode, body, readErr)
	}
	remoteRequest := <-requestSeen
	if remoteRequest == nil || remoteRequest.Method != http.MethodGet || remoteRequest.URL.Path != "/" ||
		remoteRequest.Host != "reference" || remoteRequest.Header.Get("Cookie") != "" {
		t.Fatalf("remote static request was not the closed profile: %#v", remoteRequest)
	}
	if err := running.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-running.Done():
	case <-time.After(time.Second):
		t.Fatal("Reference Connection did not withdraw after local close")
	}
	select {
	case <-publisherDone:
	case <-time.After(time.Second):
		t.Fatal("publisher Service Connection did not terminate after client close")
	}
}

func serveOneStaticReference(connection net.Conn, seen chan<- *http.Request) {
	request, err := http.ReadRequest(bufio.NewReader(connection))
	if err != nil {
		seen <- nil
		return
	}
	seen <- request
	_, _ = io.WriteString(connection, "HTTP/1.1 200 OK\r\nContent-Type: text/html\r\nContent-Length: 18\r\nConnection: close\r\n\r\n<h1>Reference</h1>")
	_ = connection.Close()
}
