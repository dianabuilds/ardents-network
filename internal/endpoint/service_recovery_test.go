package endpoint_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"io"
	"math/big"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	serviceconn "github.com/dianabuilds/ardents-network/internal/endpoint"
)

func TestForgedReplacementTerminatesInsteadOfTryingAnotherProposal(t *testing.T) {
	fixture := newFixture(t)
	binding := testRecoveryBinding(fixture)
	client, publisher, publication := connectedEndpoints(t, fixture)
	initialClient, initialPublisher := net.Pipe()
	forgedClient, forgedServer := net.Pipe()
	validClient, validPublisher := net.Pipe()
	clientAttachments := attachmentQueue(forgedClient, validClient)
	publisherAttachments := attachmentQueue(validPublisher)
	clientEndpoint, clientApplication := net.Pipe()
	publisherEndpoint, publisherApplication := net.Pipe()
	defer clientApplication.Close()
	defer publisherApplication.Close()
	go serveForgedTLS(t, forgedServer)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	outcomes := make(chan serviceOutcome, 2)
	go func() {
		request := recoveryOutbound(serviceconn.OutboundConnectionRequest{Principal: fixture.clientPrincipal,
			Capability: session(client, fixture.clientPrincipal, fixture.now), Target: fixture.first.Target,
			Publication: publication, Route: failAfter(initialClient, 96<<10), OpenAttachment: clientAttachments,
			Application: clientEndpoint, BytesEachDirection: 256 << 10, At: fixture.now}, binding)
		result, err := client.Connect(ctx, request)
		outcomes <- serviceOutcome{result, err}
	}()
	go func() {
		request := recoveryInbound(serviceconn.InboundConnectionRequest{Principal: fixture.publisherPrincipal,
			Capability: session(publisher, fixture.publisherPrincipal, fixture.now), Route: initialPublisher,
			OpenAttachment: publisherAttachments, Application: publisherEndpoint,
			BytesEachDirection: 256 << 10, At: fixture.now}, binding)
		result, err := publisher.Accept(ctx, request)
		outcomes <- serviceOutcome{result, err}
	}()
	go func() { _, _ = clientApplication.Write(seededBytes(256<<10, 17)) }()
	go func() { _, _ = publisherApplication.Write(seededBytes(256<<10, 91)) }()
	go func() { _, _ = io.Copy(io.Discard, clientApplication) }()
	go func() { _, _ = io.Copy(io.Discard, publisherApplication) }()

	select {
	case outcome := <-outcomes:
		cancel()
		if outcome.err == nil || outcome.result.Class != "abrupt connection loss" ||
			outcome.result.RouteGeneration != 1 || outcome.result.RecoveryCount != 0 {
			t.Fatalf("forged attachment did not fail closed: result=%+v err=%v", outcome.result, outcome.err)
		}
	case <-time.After(time.Second):
		cancel()
		t.Fatal("forged attachment was retried instead of terminating the Service Connection")
	}
}

func TestCarrierFailureRecoversSameApplicationStreams(t *testing.T) {
	const transferSize = 4 << 20
	fixture := newFixture(t)
	binding := testRecoveryBinding(fixture)
	client, publisher, publication := connectedEndpoints(t, fixture)
	initialClient, initialPublisher := net.Pipe()
	replacementClient, replacementPublisher := net.Pipe()
	requests := make(chan observedAttachmentRequest, 2)
	clientAttachments := recordingAttachmentQueue(requests, replacementClient)
	publisherAttachments := recordingAttachmentQueue(requests, replacementPublisher)
	clientEndpoint, clientApplication := net.Pipe()
	publisherEndpoint, publisherApplication := net.Pipe()
	defer clientApplication.Close()
	defer publisherApplication.Close()

	outcomes := make(chan serviceOutcome, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() {
		request := recoveryOutbound(serviceconn.OutboundConnectionRequest{
			Principal:  fixture.clientPrincipal,
			Capability: session(client, fixture.clientPrincipal, fixture.now),
			Target:     fixture.first.Target, Publication: publication,
			Route: failAfter(initialClient, 768<<10), OpenAttachment: clientAttachments,
			Application: clientEndpoint, BytesEachDirection: transferSize, At: fixture.now,
		}, binding)
		result, err := client.Connect(ctx, request)
		outcomes <- serviceOutcome{result, err}
	}()
	go func() {
		request := recoveryInbound(serviceconn.InboundConnectionRequest{
			Principal:  fixture.publisherPrincipal,
			Capability: session(publisher, fixture.publisherPrincipal, fixture.now),
			Route:      initialPublisher, OpenAttachment: publisherAttachments,
			Application: publisherEndpoint, BytesEachDirection: transferSize, At: fixture.now,
		}, binding)
		result, err := publisher.Accept(ctx, request)
		outcomes <- serviceOutcome{result, err}
	}()

	assertExchange(t, clientApplication, publisherApplication,
		seededBytes(transferSize, 17), seededBytes(transferSize, 91))
	commitments := make([][32]byte, 0, 2)
	for range 2 {
		outcome := <-outcomes
		if outcome.err != nil || outcome.result.Class != "clean service connection close" ||
			outcome.result.AcceptedBytes != transferSize || outcome.result.ReceivedBytes != transferSize ||
			outcome.result.RouteGeneration != 2 || outcome.result.RecoveryCount != 1 {
			t.Fatalf("same-connection recovery failed: result=%+v err=%v", outcome.result, outcome.err)
		}
		commitments = append(commitments, outcome.result.ContinuityCommitment)
	}
	if commitments[0] == [32]byte{} || commitments[0] != commitments[1] {
		t.Fatalf("endpoints did not retain one evidence-safe continuity commitment: %x %x",
			commitments[0], commitments[1])
	}
	roles := map[string]bool{}
	for range 2 {
		request := <-requests
		roles[request.Role] = true
		if request.Generation != 2 || request.Deadline.IsZero() || request.NetworkID != fixture.networkID ||
			request.CandidateView != binding.CandidateView || request.IsolationContext != binding.IsolationContext ||
			request.DestinationBinding != binding.DestinationBinding || request.RouteProfile != binding.RouteProfile {
			t.Fatalf("fresh attachment lost immutable constraints: %+v", request)
		}
	}
	if !roles["client"] || !roles["publisher"] {
		t.Fatalf("attachment roles are incomplete: %v", roles)
	}
}

func TestDirectionalCarrierFailureDoesNotRequireReverseApplicationBytes(t *testing.T) {
	const transferSize = 1 << 20
	fixture := newFixture(t)
	binding := testRecoveryBinding(fixture)
	client, publisher, publication := connectedEndpoints(t, fixture)
	initialClient, initialPublisher := net.Pipe()
	replacementClient, replacementPublisher := net.Pipe()
	clientEndpoint, clientApplication := net.Pipe()
	publisherEndpoint, publisherApplication := net.Pipe()
	defer clientApplication.Close()
	defer publisherApplication.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	outcomes := make(chan serviceOutcome, 2)
	go func() {
		request := recoveryOutbound(serviceconn.OutboundConnectionRequest{Principal: fixture.clientPrincipal,
			Capability: session(client, fixture.clientPrincipal, fixture.now), Target: fixture.first.Target,
			Publication: publication, Route: failAfter(initialClient, 300<<10), OpenAttachment: attachmentQueue(replacementClient),
			Application: clientEndpoint, SendBytes: transferSize, At: fixture.now}, binding)
		result, err := client.Connect(ctx, request)
		outcomes <- serviceOutcome{result, err}
	}()
	go func() {
		request := recoveryInbound(serviceconn.InboundConnectionRequest{Principal: fixture.publisherPrincipal,
			Capability: session(publisher, fixture.publisherPrincipal, fixture.now), Route: initialPublisher,
			OpenAttachment: attachmentQueue(replacementPublisher), Application: publisherEndpoint,
			ReceiveBytes: transferSize, At: fixture.now}, binding)
		result, err := publisher.Accept(ctx, request)
		outcomes <- serviceOutcome{result, err}
	}()
	expected := seededBytes(transferSize, 37)
	writeDone := make(chan error, 1)
	go func() { _, err := clientApplication.Write(expected); writeDone <- err }()
	received := make([]byte, transferSize)
	if _, err := io.ReadFull(publisherApplication, received); err != nil || !bytes.Equal(received, expected) {
		t.Fatalf("directional bytes differ: err=%v", err)
	}
	if err := <-writeDone; err != nil {
		t.Fatal(err)
	}
	for range 2 {
		outcome := <-outcomes
		if outcome.err != nil || outcome.result.RouteGeneration != 2 || outcome.result.RecoveryCount != 1 {
			t.Fatalf("directional recovery failed: result=%+v err=%v", outcome.result, outcome.err)
		}
	}
}

func TestExpiredWorkSafetyBlocksFreshAttachment(t *testing.T) {
	fixture := newFixture(t)
	binding := testRecoveryBinding(fixture)
	binding.NoNewRecoveryAfter = fixture.now.Add(time.Second).Unix()
	client, publisher, publication := connectedEndpoints(t, fixture)
	initialClient, initialPublisher := net.Pipe()
	clientEndpoint, clientApplication := net.Pipe()
	publisherEndpoint, publisherApplication := net.Pipe()
	defer clientApplication.Close()
	defer publisherApplication.Close()
	var proposals atomic.Uint32
	opener := func(context.Context, serviceconn.Recovery) (net.Conn, error) {
		proposals.Add(1)
		return nil, errors.New("unexpected attachment proposal")
	}
	authorizedAt := fixture.now
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	outcomes := make(chan serviceOutcome, 2)
	go func() {
		request := recoveryOutbound(serviceconn.OutboundConnectionRequest{Principal: fixture.clientPrincipal,
			Capability: session(client, fixture.clientPrincipal, fixture.now), Target: fixture.first.Target,
			Publication: publication, Route: failAfterWait(initialClient, 96<<10, 1100*time.Millisecond), OpenAttachment: opener,
			Application: clientEndpoint, BytesEachDirection: 256 << 10, At: authorizedAt}, binding)
		result, err := client.Connect(ctx, request)
		outcomes <- serviceOutcome{result, err}
	}()
	go func() {
		request := recoveryInbound(serviceconn.InboundConnectionRequest{Principal: fixture.publisherPrincipal,
			Capability: session(publisher, fixture.publisherPrincipal, fixture.now), Route: initialPublisher,
			OpenAttachment: opener, Application: publisherEndpoint, BytesEachDirection: 256 << 10, At: authorizedAt}, binding)
		result, err := publisher.Accept(ctx, request)
		outcomes <- serviceOutcome{result, err}
	}()
	go func() { _, _ = clientApplication.Write(seededBytes(256<<10, 17)) }()
	go func() { _, _ = publisherApplication.Write(seededBytes(256<<10, 91)) }()
	go func() { _, _ = io.Copy(io.Discard, clientApplication) }()
	go func() { _, _ = io.Copy(io.Discard, publisherApplication) }()

	foundSafetyFailure := false
	for range 2 {
		outcome := <-outcomes
		if outcome.err != nil && strings.Contains(outcome.err.Error(), "Work Safety expired") {
			foundSafetyFailure = true
		}
	}
	if !foundSafetyFailure || proposals.Load() != 0 {
		t.Fatalf("expired safety started recovery: safety_failure=%v proposals=%d", foundSafetyFailure, proposals.Load())
	}
}

func TestNoAlternateTerminatesConnectionPromptly(t *testing.T) {
	for episode := range 5 {
		t.Run(strconv.Itoa(episode), testNoAlternateTerminatesConnectionPromptly)
	}
}

func testNoAlternateTerminatesConnectionPromptly(t *testing.T) {
	fixture := newFixture(t)
	binding := testRecoveryBinding(fixture)
	client, publisher, publication := connectedEndpoints(t, fixture)
	initialClient, initialPublisher := net.Pipe()
	clientEndpoint, clientApplication := net.Pipe()
	publisherEndpoint, publisherApplication := net.Pipe()
	defer clientApplication.Close()
	defer publisherApplication.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	outcomes := make(chan serviceOutcome, 2)
	go func() {
		request := recoveryOutbound(serviceconn.OutboundConnectionRequest{Principal: fixture.clientPrincipal,
			Capability: session(client, fixture.clientPrincipal, fixture.now), Target: fixture.first.Target,
			Publication: publication, Route: failAfter(initialClient, 96<<10),
			OpenAttachment: unavailableAttachments, Application: clientEndpoint,
			BytesEachDirection: 256 << 10, At: fixture.now}, binding)
		result, err := client.Connect(ctx, request)
		outcomes <- serviceOutcome{result, err}
	}()
	go func() {
		request := recoveryInbound(serviceconn.InboundConnectionRequest{Principal: fixture.publisherPrincipal,
			Capability: session(publisher, fixture.publisherPrincipal, fixture.now), Route: initialPublisher,
			OpenAttachment: unavailableAttachments, Application: publisherEndpoint,
			BytesEachDirection: 256 << 10, At: fixture.now}, binding)
		result, err := publisher.Accept(ctx, request)
		outcomes <- serviceOutcome{result, err}
	}()
	go func() { _, _ = clientApplication.Write(seededBytes(256<<10, 17)) }()
	go func() { _, _ = publisherApplication.Write(seededBytes(256<<10, 91)) }()
	go func() { _, _ = io.Copy(io.Discard, clientApplication) }()
	go func() { _, _ = io.Copy(io.Discard, publisherApplication) }()
	for range 2 {
		select {
		case outcome := <-outcomes:
			if outcome.err == nil || outcome.result.Class != "abrupt connection loss" || outcome.result.RecoveryCount != 0 {
				t.Fatalf("missing alternate did not terminate honestly: result=%+v err=%v", outcome.result, outcome.err)
			}
		case <-time.After(time.Second):
			t.Fatal("missing alternate outlived the terminal bound")
		}
	}
}

func testRecoveryBinding(fixture fixture) serviceconn.Recovery {
	return serviceconn.Recovery{CandidateView: [32]byte{41}, IsolationContext: [32]byte{42},
		DestinationBinding: [32]byte{43}, RouteProfile: "ardents-interactive-route-v1",
		WorkSafetyNotAfter: fixture.first.NotAfter, WorkSafetyMaximum: fixture.first.NotAfter,
		NoNewRecoveryAfter: fixture.first.NotAfter}
}

func recoveryOutbound(request serviceconn.OutboundConnectionRequest, binding serviceconn.Recovery) serviceconn.OutboundConnectionRequest {
	request.RecoveryBinding = binding
	return request
}

func recoveryInbound(request serviceconn.InboundConnectionRequest, binding serviceconn.Recovery) serviceconn.InboundConnectionRequest {
	request.RecoveryBinding = binding
	return request
}

func unavailableAttachments(context.Context, serviceconn.Recovery) (net.Conn, error) {
	return nil, errors.New("no safe eligible Route Attachment remains")
}

func attachmentQueue(connections ...net.Conn) func(context.Context, serviceconn.Recovery) (net.Conn, error) {
	return recordingAttachmentQueue(nil, connections...)
}

type observedAttachmentRequest struct{ serviceconn.Recovery }

func recordingAttachmentQueue(requests chan<- observedAttachmentRequest,
	connections ...net.Conn) func(context.Context, serviceconn.Recovery) (net.Conn, error) {
	queue := make(chan net.Conn, len(connections))
	for _, connection := range connections {
		queue <- connection
	}
	return func(ctx context.Context, request serviceconn.Recovery) (net.Conn, error) {
		if requests != nil {
			requests <- observedAttachmentRequest{request}
		}
		select {
		case connection := <-queue:
			return connection, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

type failingConnection struct {
	net.Conn
	mu        sync.Mutex
	remaining int
	delay     time.Duration
	once      sync.Once
}

func failAfter(connection net.Conn, bytes int) net.Conn {
	return &failingConnection{Conn: connection, remaining: bytes}
}

func failAfterWait(connection net.Conn, bytes int, delay time.Duration) net.Conn {
	return &failingConnection{Conn: connection, remaining: bytes, delay: delay}
}

func (connection *failingConnection) Write(value []byte) (int, error) {
	connection.mu.Lock()
	defer connection.mu.Unlock()
	if connection.remaining <= 0 {
		if connection.delay > 0 {
			time.Sleep(connection.delay)
		}
		connection.once.Do(func() { _ = connection.Conn.Close() })
		return 0, net.ErrClosed
	}
	if len(value) <= connection.remaining {
		written, err := connection.Conn.Write(value)
		connection.remaining -= written
		return written, err
	}
	written, err := connection.Conn.Write(value[:connection.remaining])
	connection.remaining -= written
	if connection.delay > 0 {
		time.Sleep(connection.delay)
	}
	connection.once.Do(func() { _ = connection.Conn.Close() })
	return written, errors.Join(err, net.ErrClosed)
}

func serveForgedTLS(t *testing.T, connection net.Conn) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Error(err)
		return
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(99), Subject: pkix.Name{CommonName: "forged"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		t.Error(err)
		return
	}
	server := tls.Server(connection, &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: private}}})
	_ = server.Handshake()
	_ = connection.Close()
}
