package endpoint

import (
	"bytes"
	"context"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

func TestFailedReplacementFallsThroughWithoutResettingConnection(t *testing.T) {
	const transferSize = 2 << 20
	fixture := newFixture(t)
	binding := testRecoveryBinding(fixture)
	client, publisher, publication := connectedEndpoints(t, fixture)
	initialClient, initialPublisher := net.Pipe()
	failedClientRaw, failedPublisherRaw := net.Pipe()
	failedClient := &closeObservedConnection{Conn: failedClientRaw}
	failedPublisher := &closeObservedConnection{Conn: failedPublisherRaw}
	validClient, validPublisher := net.Pipe()
	requests := make(chan observedAttachmentRequest, 8)
	clientAttachments := recordingAttachmentQueue(requests, failAfter(failedClient, 0), validClient)
	publisherAttachments := recordingAttachmentQueue(requests, failedPublisher, validPublisher)
	clientEndpoint, clientApplication := net.Pipe()
	publisherEndpoint, publisherApplication := net.Pipe()
	defer clientApplication.Close()
	defer publisherApplication.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	outcomes := make(chan serviceOutcome, 2)
	go func() {
		request := recoveryOutbound(outboundConnectionRequest{Principal: fixture.clientPrincipal,
			Capability: session(client, fixture.clientPrincipal, fixture.now), Target: fixture.first.Target,
			Publication: publication, Route: failAfter(initialClient, 320<<10), OpenAttachment: clientAttachments,
			Application: clientEndpoint, SendBytes: transferSize, At: fixture.now}, binding)
		result, err := client.connectForHarness(ctx, request)
		outcomes <- serviceOutcome{result, err}
	}()
	go func() {
		request := recoveryInbound(inboundConnectionRequest{Principal: fixture.publisherPrincipal,
			Capability: session(publisher, fixture.publisherPrincipal, fixture.now), Route: initialPublisher,
			OpenAttachment: publisherAttachments, Application: publisherEndpoint,
			ReceiveBytes: transferSize, At: fixture.now}, binding)
		result, err := publisher.acceptForHarness(ctx, request)
		outcomes <- serviceOutcome{result, err}
	}()

	expected := seededBytes(transferSize, 113)
	writeDone := make(chan error, 1)
	go func() { _, err := clientApplication.Write(expected); writeDone <- err }()
	received := make([]byte, transferSize)
	if _, err := io.ReadFull(publisherApplication, received); err != nil || !bytes.Equal(received, expected) {
		t.Fatalf("bytes changed across failed replacement: err=%v", err)
	}
	if err := <-writeDone; err != nil {
		t.Fatal(err)
	}
	for range 2 {
		outcome := <-outcomes
		if outcome.err != nil || outcome.result.Class != "clean service connection close" ||
			outcome.result.RouteGeneration != 2 || outcome.result.RecoveryCount != 1 ||
			outcome.result.AcceptedBytes != outcome.result.AcknowledgedBytes {
			t.Fatalf("failed proposal changed the logical connection: result=%+v err=%v", outcome.result, outcome.err)
		}
	}
	if !failedClient.closedValue() || !failedPublisher.closedValue() {
		t.Fatal("failed replacement attachment was not closed")
	}

	deadlines := map[string][]time.Time{}
	for range 4 {
		request := <-requests
		deadlines[request.Role] = append(deadlines[request.Role], request.Deadline)
	}
	for _, role := range []string{"client", "publisher"} {
		values := deadlines[role]
		if len(values) != 2 || values[0].IsZero() || !values[0].Equal(values[1]) {
			t.Fatalf("%s replacement attempts reset their deadline: %v", role, values)
		}
	}
}

type closeObservedConnection struct {
	net.Conn
	mu     sync.Mutex
	closed bool
}

func (connection *closeObservedConnection) Close() error {
	connection.mu.Lock()
	connection.closed = true
	connection.mu.Unlock()
	return connection.Conn.Close()
}

func (connection *closeObservedConnection) closedValue() bool {
	connection.mu.Lock()
	defer connection.mu.Unlock()
	return connection.closed
}
