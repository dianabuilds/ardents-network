package endpoint_test

import (
	"bytes"
	"context"
	"io"
	"net"
	"testing"
	"time"

	endpointapi "github.com/dianabuilds/ardents-network/internal/endpoint"
)

func TestThreeSequentialFailuresKeepOneApplicationConnection(t *testing.T) {
	const transferSize = 2 << 20
	fixture := newFixture(t)
	binding := testRecoveryBinding(fixture)
	client, publisher, publication := connectedEndpoints(t, fixture)
	clientRoutes, publisherRoutes := sequentialRouteAttachments(3, 320<<10)
	clientEndpoint, clientApplication := net.Pipe()
	publisherEndpoint, publisherApplication := net.Pipe()
	defer clientApplication.Close()
	defer publisherApplication.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	outcomes := make(chan serviceOutcome, 2)
	go func() {
		request := recoveryOutbound(endpointapi.OutboundConnectionRequest{Principal: fixture.clientPrincipal,
			Capability: session(client, fixture.clientPrincipal, fixture.now), Target: fixture.first.Target,
			Publication: publication, Route: clientRoutes[0], OpenAttachment: attachmentQueue(clientRoutes[1:]...),
			Application: clientEndpoint, SendBytes: transferSize, At: fixture.now}, binding)
		result, err := client.Connect(ctx, request)
		outcomes <- serviceOutcome{result, err}
	}()
	go func() {
		request := recoveryInbound(endpointapi.InboundConnectionRequest{Principal: fixture.publisherPrincipal,
			Capability: session(publisher, fixture.publisherPrincipal, fixture.now), Route: publisherRoutes[0],
			OpenAttachment: attachmentQueue(publisherRoutes[1:]...), Application: publisherEndpoint,
			ReceiveBytes: transferSize, At: fixture.now}, binding)
		result, err := publisher.Accept(ctx, request)
		outcomes <- serviceOutcome{result, err}
	}()

	expected := seededBytes(transferSize, 73)
	writeDone := make(chan error, 1)
	go func() { _, err := clientApplication.Write(expected); writeDone <- err }()
	received := make([]byte, transferSize)
	if _, err := io.ReadFull(publisherApplication, received); err != nil || !bytes.Equal(received, expected) {
		t.Fatalf("sequential recovery bytes differ: err=%v", err)
	}
	if err := <-writeDone; err != nil {
		t.Fatal(err)
	}
	for range 2 {
		outcome := <-outcomes
		if outcome.err != nil || outcome.result.Class != "clean service connection close" ||
			outcome.result.RouteGeneration != 4 || outcome.result.RecoveryCount != 3 ||
			outcome.result.ApplicationIPCAccepts > 1 ||
			outcome.result.AcceptedBytes != outcome.result.AcknowledgedBytes {
			t.Fatalf("three recoveries changed the logical connection: result=%+v err=%v", outcome.result, outcome.err)
		}
	}
}

func sequentialRouteAttachments(failures int, bytesBeforeFailure int) ([]net.Conn, []net.Conn) {
	clients := make([]net.Conn, 0, failures+1)
	publishers := make([]net.Conn, 0, failures+1)
	for index := 0; index <= failures; index++ {
		client, publisher := net.Pipe()
		if index < failures {
			client = failAfter(client, bytesBeforeFailure)
		}
		clients = append(clients, client)
		publishers = append(publishers, publisher)
	}
	return clients, publishers
}
