package endpoint_test

import (
	"bytes"
	"context"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	serviceconn "github.com/dianabuilds/ardents-network/internal/endpoint"
)

type finalEOFApplication struct{ *bytes.Reader }

type observedApplication struct {
	net.Conn
	entered chan struct{}
	once    sync.Once
}

func (application *observedApplication) Read(value []byte) (int, error) {
	application.once.Do(func() { close(application.entered) })
	return application.Conn.Read(value)
}

func (application *finalEOFApplication) Read(value []byte) (int, error) {
	read, _ := application.Reader.Read(value)
	return read, io.EOF
}

func (*finalEOFApplication) Write(value []byte) (int, error) { return len(value), nil }
func (*finalEOFApplication) Close() error                    { return nil }

func TestPartialApplicationChunkIsFramedWithoutWaitingForRecordBoundary(t *testing.T) {
	const partial = 16_381
	fixture := newFixture(t)
	client, publisher, publication := connectedEndpoints(t, fixture)
	clientRoute, publisherRoute := net.Pipe()
	clientEndpoint, clientApplication := net.Pipe()
	publisherEndpoint, publisherApplication := net.Pipe()
	defer clientApplication.Close()
	defer publisherApplication.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	outcomes := make(chan serviceOutcome, 2)
	go func() {
		result, err := client.Do(ctx, serviceconn.Request{Action: "connect", Principal: fixture.clientPrincipal,
			Session: session(client, fixture.clientPrincipal, fixture.now), Target: fixture.first.Target,
			Publication: publication, Route: clientRoute, Application: clientEndpoint,
			SendBytes: partial * 2, At: fixture.now})
		outcomes <- serviceOutcome{result, err}
	}()
	go func() {
		result, err := publisher.Do(ctx, serviceconn.Request{Action: "accept", Principal: fixture.publisherPrincipal,
			Session: session(publisher, fixture.publisherPrincipal, fixture.now), Route: publisherRoute,
			Application: publisherEndpoint, ReceiveBytes: partial * 2, At: fixture.now})
		outcomes <- serviceOutcome{result, err}
	}()
	payload := seededBytes(partial, 51)
	written := make(chan error, 1)
	go func() { _, err := clientApplication.Write(payload); written <- err }()
	if err := <-written; err != nil {
		t.Fatal(err)
	}
	_ = publisherApplication.SetReadDeadline(time.Now().Add(250 * time.Millisecond))
	received := make([]byte, partial)
	if _, err := io.ReadFull(publisherApplication, received); err != nil {
		t.Fatalf("partial chunk remained buffered at Endpoint: %v", err)
	}
	cancel()
	for range 2 {
		<-outcomes
	}
}

func TestFinalApplicationBytesReturnedWithEOFCompleteCleanly(t *testing.T) {
	payload := seededBytes(16_381, 73)
	fixture := newFixture(t)
	client, publisher, publication := connectedEndpoints(t, fixture)
	clientRoute, publisherRoute := net.Pipe()
	publisherEndpoint, publisherApplication := net.Pipe()
	defer publisherApplication.Close()
	clientApplication := &finalEOFApplication{Reader: bytes.NewReader(payload)}
	outcomes := make(chan serviceOutcome, 2)
	go func() {
		result, err := client.Do(context.Background(), serviceconn.Request{Action: "connect",
			Principal: fixture.clientPrincipal, Session: session(client, fixture.clientPrincipal, fixture.now),
			Target: fixture.first.Target, Publication: publication, Route: clientRoute,
			Application: clientApplication, SendBytes: uint32(len(payload)), At: fixture.now})
		outcomes <- serviceOutcome{result, err}
	}()
	go func() {
		result, err := publisher.Do(context.Background(), serviceconn.Request{Action: "accept",
			Principal: fixture.publisherPrincipal, Session: session(publisher, fixture.publisherPrincipal, fixture.now),
			Route: publisherRoute, Application: publisherEndpoint, ReceiveBytes: uint32(len(payload)), At: fixture.now})
		outcomes <- serviceOutcome{result, err}
	}()
	received := make([]byte, len(payload))
	if _, err := io.ReadFull(publisherApplication, received); err != nil || !bytes.Equal(received, payload) {
		t.Fatalf("final bytes accompanying EOF were not delivered: %v", err)
	}
	for range 2 {
		outcome := <-outcomes
		if outcome.err != nil || outcome.result.Class != "clean service connection close" {
			t.Fatalf("final bytes accompanying EOF failed: %+v err=%v", outcome.result, outcome.err)
		}
	}
}

type serviceOutcome struct {
	result serviceconn.RuntimeResult
	err    error
}

type endpointRunner interface {
	Do(context.Context, serviceconn.Request) (serviceconn.RuntimeResult, error)
	Admit([32]byte, broker.Surface) ([32]byte, error)
	Publish(context.Context, serviceconn.PublicationRequest) (serviceconn.PublicationResult, error)
	Withdraw(context.Context, serviceconn.WithdrawalRequest) (serviceconn.WithdrawalResult, error)
	Connect(context.Context, serviceconn.OutboundConnectionRequest) (serviceconn.RuntimeResult, error)
	Accept(context.Context, serviceconn.InboundConnectionRequest) (serviceconn.RuntimeResult, error)
}

func TestSlowConsumersApplyBackpressureUntilLocalCancellation(t *testing.T) {
	fixture := newFixture(t)
	client, publisher, publication := connectedEndpoints(t, fixture)
	clientRoute, publisherRoute := tcpPair(t)
	clientEndpoint, clientApplication := net.Pipe()
	publisherEndpoint, publisherApplication := net.Pipe()
	defer clientApplication.Close()
	defer publisherApplication.Close()
	ctx, cancel := context.WithCancel(context.Background())
	clientEntered, publisherEntered := make(chan struct{}), make(chan struct{})
	outcomes := runConnections(ctx, fixture, client, publisher, publication,
		clientRoute, publisherRoute,
		&observedApplication{Conn: clientEndpoint, entered: clientEntered},
		&observedApplication{Conn: publisherEndpoint, entered: publisherEntered})
	for _, entered := range []chan struct{}{clientEntered, publisherEntered} {
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("Service Connection did not reach Application backpressure")
		}
	}
	select {
	case outcome := <-outcomes:
		t.Fatalf("blocked Application completed without input: %+v %v", outcome.result, outcome.err)
	default:
	}
	cancel()
	for range 2 {
		outcome := <-outcomes
		if outcome.err == nil || outcome.result.Class != "local timeout or cancellation" ||
			outcome.result.AcceptedBytes != 0 || outcome.result.ReceivedBytes != 0 {
			t.Fatalf("dishonest cancellation result: %+v err=%v", outcome.result, outcome.err)
		}
	}
}

func TestLogicalQueueBackpressuresAtFrozenDirectionalCap(t *testing.T) {
	fixture := newFixture(t)
	client, publisher, publication := connectedEndpoints(t, fixture)
	clientRoute, publisherRoute := tcpPair(t)
	clientEndpoint, clientApplication := net.Pipe()
	publisherEndpoint, publisherApplication := net.Pipe()
	defer clientApplication.Close()
	defer publisherApplication.Close()
	ctx, cancel := context.WithCancel(context.Background())
	outcomes := make(chan serviceOutcome, 2)
	go func() {
		result, err := client.Do(ctx, serviceconn.Request{Action: "connect", Principal: fixture.clientPrincipal,
			Session: session(client, fixture.clientPrincipal, fixture.now), Target: fixture.first.Target,
			Publication: publication, Route: clientRoute, Application: clientEndpoint,
			BytesEachDirection: 4 << 20, At: fixture.now})
		outcomes <- serviceOutcome{result, err}
	}()
	go func() {
		result, err := publisher.Do(ctx, serviceconn.Request{Action: "accept", Principal: fixture.publisherPrincipal,
			Session: session(publisher, fixture.publisherPrincipal, fixture.now), Route: publisherRoute,
			Application: publisherEndpoint, BytesEachDirection: 4 << 20, At: fixture.now})
		outcomes <- serviceOutcome{result, err}
	}()
	writeDone := make(chan error, 1)
	writeProgress := make(chan struct{}, 256)
	go func() {
		payload := seededBytes(4<<20, 33)
		for offset := 0; offset < len(payload); offset += 16 << 10 {
			end := min(offset+(16<<10), len(payload))
			if _, err := clientApplication.Write(payload[offset:end]); err != nil {
				writeDone <- err
				return
			}
			writeProgress <- struct{}{}
		}
		writeDone <- nil
	}()
	for range 2 {
		select {
		case <-writeProgress:
		case <-time.After(2 * time.Second):
			t.Fatal("Application did not exercise the logical send queue")
		}
	}
	select {
	case err := <-writeDone:
		t.Fatalf("slow remote Application did not backpressure four MiB write: %v", err)
	default:
	}
	cancel()
	bounded := false
	for range 2 {
		outcome := <-outcomes
		if outcome.result.AcceptedBytes > 256<<10 || outcome.result.QueueHighWater > 256<<10 {
			t.Fatalf("logical queue exceeded frozen cap: result=%+v err=%v", outcome.result, outcome.err)
		}
		bounded = bounded || outcome.result.QueueHighWater > 0
	}
	if !bounded {
		t.Fatal("test did not exercise the logical queue")
	}
}

func TestAbruptCloseReportsObservedPartialCounts(t *testing.T) {
	fixture := newFixture(t)
	client, publisher, publication := connectedEndpoints(t, fixture)
	clientRoute, publisherRoute := tcpPair(t)
	clientEndpoint, clientApplication := tcpPair(t)
	publisherEndpoint, publisherApplication := tcpPair(t)
	outcomes := runConnections(context.Background(), fixture, client, publisher, publication,
		clientRoute, publisherRoute, clientEndpoint, publisherEndpoint)
	writePartial(t, clientApplication, 1024, 17)
	writePartial(t, publisherApplication, 2048, 91)
	closeWrite(t, clientApplication)
	closeWrite(t, publisherApplication)
	defer clientApplication.Close()
	defer publisherApplication.Close()
	seen := map[uint32]bool{}
	for range 2 {
		outcome := <-outcomes
		if outcome.err == nil || outcome.result.Class != "abrupt connection loss" ||
			outcome.result.AcceptedBytes == 0 || outcome.result.ReceivedBytes == 0 {
			t.Fatalf("partial close lost its observed counts: %+v err=%v", outcome.result, outcome.err)
		}
		seen[outcome.result.AcceptedBytes] = true
	}
	if !seen[1024] || !seen[2048] {
		t.Fatalf("directional accepted counts are wrong: %v", seen)
	}
}

func TestMalformedAndOversizedPublicationsAreTargetAuthenticationFailures(t *testing.T) {
	fixture := newFixture(t)
	for _, publication := range [][]byte{{1, 2, 3}, make([]byte, 9<<10)} {
		client, _ := serviceconn.New(serviceconn.Setup{NetworkID: fixture.networkID, BrokerID: [32]byte{8},
			AuthorityPublic: fixture.authorityPublic, IntroductionPublic: fixture.introductionPublic,
			ConnectionPrincipal: fixture.clientPrincipal})
		session := admit(t, client, "connection", fixture.clientPrincipal, fixture.now)
		result, err := client.Do(context.Background(), serviceconn.Request{Action: "connect",
			Principal: fixture.clientPrincipal, Session: session, Target: fixture.first.Target,
			Publication: publication, At: fixture.now})
		if err == nil || result.Class != "service target authentication failure" {
			t.Fatalf("malformed publication returned %+v err=%v", result, err)
		}
	}
}

func connectedEndpoints(t *testing.T, fixture fixture) (endpointRunner, endpointRunner, []byte) {
	t.Helper()
	publisher := newPublisher(t, fixture)
	publication := publish(t, publisher, fixture, fixture.first, fixture.firstPrivate)
	client, err := serviceconn.New(serviceconn.Setup{NetworkID: fixture.networkID, BrokerID: [32]byte{8},
		AuthorityPublic: fixture.authorityPublic, IntroductionPublic: fixture.introductionPublic,
		ConnectionPrincipal: fixture.clientPrincipal})
	if err != nil {
		t.Fatal(err)
	}
	return client, publisher, publication
}

func runConnections(ctx context.Context, fixture fixture, client, publisher endpointRunner, publication []byte,
	clientRoute, publisherRoute, clientApplication, publisherApplication net.Conn) <-chan serviceOutcome {
	outcomes := make(chan serviceOutcome, 2)
	clientSession := session(client, fixture.clientPrincipal, fixture.now)
	publisherSession := session(publisher, fixture.publisherPrincipal, fixture.now)
	go func() {
		result, err := client.Do(ctx, serviceconn.Request{Action: "connect", Principal: fixture.clientPrincipal,
			Session: clientSession, Target: fixture.first.Target, Publication: publication, Route: clientRoute,
			Application: clientApplication, BytesEachDirection: 64 << 10, At: fixture.now})
		outcomes <- serviceOutcome{result, err}
	}()
	go func() {
		result, err := publisher.Do(ctx, serviceconn.Request{Action: "accept", Principal: fixture.publisherPrincipal,
			Session: publisherSession, Route: publisherRoute, Application: publisherApplication,
			BytesEachDirection: 64 << 10, At: fixture.now})
		outcomes <- serviceOutcome{result, err}
	}()
	return outcomes
}

func session(endpoint endpointRunner, principal [32]byte, at time.Time) [32]byte {
	if at.IsZero() {
		return [32]byte{}
	}
	result, _ := endpoint.Admit(principal, broker.Connection)
	return result
}

func writePartial(t *testing.T, connection net.Conn, count, seed int) {
	t.Helper()
	if written, err := connection.Write(seededBytes(count, seed)); err != nil || written != count {
		t.Fatalf("partial write=%d err=%v", written, err)
	}
}

func tcpPair(t *testing.T) (net.Conn, net.Conn) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	accepted := make(chan net.Conn, 1)
	go func() {
		connection, _ := listener.Accept()
		accepted <- connection
	}()
	application, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	endpoint := <-accepted
	_ = listener.Close()
	return endpoint, application
}

func closeWrite(t *testing.T, connection net.Conn) {
	t.Helper()
	half, ok := connection.(interface{ CloseWrite() error })
	if !ok {
		t.Fatal("test connection does not support half-close")
	}
	if err := half.CloseWrite(); err != nil {
		t.Fatal(err)
	}
}
