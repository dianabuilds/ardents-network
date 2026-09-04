package endpoint

import (
	"bytes"
	"context"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
)

type finalEOFApplication struct{ *bytes.Reader }

type observedApplication struct {
	net.Conn
	entered chan struct{}
	once    sync.Once
}

type connectionTrace struct {
	Operation string
	Deadline  time.Time
	At        time.Time
	Err       error
}

type tracedConnection struct {
	net.Conn
	mu     sync.Mutex
	traces []connectionTrace
}

func (connection *tracedConnection) SetDeadline(deadline time.Time) error {
	err := connection.Conn.SetDeadline(deadline)
	connection.record("deadline", deadline, err)
	return err
}

func (connection *tracedConnection) SetReadDeadline(deadline time.Time) error {
	err := connection.Conn.SetReadDeadline(deadline)
	connection.record("read deadline", deadline, err)
	return err
}

func (connection *tracedConnection) SetWriteDeadline(deadline time.Time) error {
	err := connection.Conn.SetWriteDeadline(deadline)
	connection.record("write deadline", deadline, err)
	return err
}

func (connection *tracedConnection) Read(value []byte) (int, error) {
	read, err := connection.Conn.Read(value)
	if err != nil {
		connection.record("read", time.Time{}, err)
	}
	return read, err
}

func (connection *tracedConnection) Write(value []byte) (int, error) {
	written, err := connection.Conn.Write(value)
	if err != nil {
		connection.record("write", time.Time{}, err)
	}
	return written, err
}

func (connection *tracedConnection) Close() error {
	err := connection.Conn.Close()
	connection.record("close", time.Time{}, err)
	return err
}

func (connection *tracedConnection) record(operation string, deadline time.Time, err error) {
	connection.mu.Lock()
	defer connection.mu.Unlock()
	connection.traces = append(connection.traces, connectionTrace{Operation: operation, Deadline: deadline, At: time.Now(), Err: err})
}

func (connection *tracedConnection) trace() []connectionTrace {
	connection.mu.Lock()
	defer connection.mu.Unlock()
	return append([]connectionTrace(nil), connection.traces...)
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
		result, err := client.connectForHarness(ctx, outboundConnectionRequest{Principal: fixture.clientPrincipal,
			Capability: session(client, fixture.clientPrincipal, fixture.now), Target: fixture.first.Target,
			Publication: publication, Route: clientRoute, Application: clientEndpoint,
			SendBytes: partial * 2, At: fixture.now})

		outcomes <- serviceOutcome{result, err}
	}()
	go func() {
		result, err := publisher.acceptForHarness(ctx, inboundConnectionRequest{Principal: fixture.publisherPrincipal,
			Capability: session(publisher, fixture.publisherPrincipal, fixture.now), Route: publisherRoute,
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
		result, err := client.connectForHarness(context.Background(), outboundConnectionRequest{
			Principal: fixture.clientPrincipal, Capability: session(client, fixture.clientPrincipal, fixture.now),
			Target: fixture.first.Target, Publication: publication, Route: clientRoute,
			Application: clientApplication, SendBytes: uint32(len(payload)), At: fixture.now})

		outcomes <- serviceOutcome{result, err}
	}()
	go func() {
		result, err := publisher.acceptForHarness(context.Background(), inboundConnectionRequest{
			Principal: fixture.publisherPrincipal, Capability: session(publisher, fixture.publisherPrincipal, fixture.now),
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
	result runtimeResult
	err    error
}

func TestSlowConsumersApplyBackpressureUntilLocalCancellation(t *testing.T) {
	started := time.Now()
	fixture := newFixture(t)
	client, publisher, publication := connectedEndpoints(t, fixture)
	clientRoute, publisherRoute := tcpPair(t)
	clientRouteTrace := &tracedConnection{Conn: clientRoute}
	publisherRouteTrace := &tracedConnection{Conn: publisherRoute}
	clientEndpoint, clientApplication := net.Pipe()
	publisherEndpoint, publisherApplication := net.Pipe()
	clientEndpointTrace := &tracedConnection{Conn: clientEndpoint}
	publisherEndpointTrace := &tracedConnection{Conn: publisherEndpoint}
	defer clientApplication.Close()
	defer publisherApplication.Close()
	t.Cleanup(func() {
		for name, connection := range map[string]*tracedConnection{
			"client route": clientRouteTrace, "publisher route": publisherRouteTrace,
			"client application": clientEndpointTrace, "publisher application": publisherEndpointTrace,
		} {
			for _, trace := range connection.trace() {
				t.Logf("[DEBUG-c005] %s trace=%+v", name, trace)
			}
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	clientEntered, publisherEntered := make(chan struct{}), make(chan struct{})
	outcomes := runConnections(ctx, fixture, client, publisher, publication,
		clientRouteTrace, publisherRouteTrace,
		&observedApplication{Conn: clientEndpointTrace, entered: clientEntered},
		&observedApplication{Conn: publisherEndpointTrace, entered: publisherEntered})
	for _, entered := range []chan struct{}{clientEntered, publisherEntered} {
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("Service Connection did not reach Application backpressure")
		}
	}
	t.Logf("[DEBUG-c005] applications entered after %s", time.Since(started))
	select {
	case outcome := <-outcomes:
		t.Fatalf("blocked Application completed without input: %+v %v", outcome.result, outcome.err)
	default:
	}
	cancelled := time.Now()
	cancel()
	var wrong []serviceOutcome
	for index := range 2 {
		outcome := <-outcomes
		t.Logf("[DEBUG-c005] cancellation outcome=%d after=%s class=%q accepted=%d received=%d err=%v", index,
			time.Since(cancelled), outcome.result.Class, outcome.result.AcceptedBytes, outcome.result.ReceivedBytes, outcome.err)
		if outcome.err == nil || outcome.result.Class != "local timeout or cancellation" ||
			outcome.result.AcceptedBytes != 0 || outcome.result.ReceivedBytes != 0 {
			wrong = append(wrong, outcome)
		}
	}
	for _, outcome := range wrong {
		t.Errorf("dishonest cancellation result: %+v err=%v", outcome.result, outcome.err)
	}
	if len(wrong) > 0 {
		t.FailNow()
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
		result, err := client.connectForHarness(ctx, outboundConnectionRequest{Principal: fixture.clientPrincipal,
			Capability: session(client, fixture.clientPrincipal, fixture.now), Target: fixture.first.Target,
			Publication: publication, Route: clientRoute, Application: clientEndpoint,
			BytesEachDirection: 4 << 20, At: fixture.now})

		outcomes <- serviceOutcome{result, err}
	}()
	go func() {
		result, err := publisher.acceptForHarness(ctx, inboundConnectionRequest{Principal: fixture.publisherPrincipal,
			Capability: session(publisher, fixture.publisherPrincipal, fixture.now), Route: publisherRoute,
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

func TestOrderlyHalfCloseReportsObservedPartialCounts(t *testing.T) {
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
		if outcome.err != nil || outcome.result.Class != "clean service connection close" ||
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
		client, _ := newEndpoint(setup{NetworkID: fixture.networkID, BrokerID: [32]byte{8},
			AuthorityPublic: fixture.authorityPublic, IntroductionPublic: fixture.introductionPublic,
			ConnectionPrincipal: fixture.clientPrincipal})

		session := admit(t, client, "connection", fixture.clientPrincipal, fixture.now)
		result, err := client.connectForHarness(context.Background(), outboundConnectionRequest{
			Principal: fixture.clientPrincipal, Capability: session, Target: fixture.first.Target,
			Publication: publication, At: fixture.now})

		if err == nil || result.Class != "service target authentication failure" {
			t.Fatalf("malformed publication returned %+v err=%v", result, err)
		}
	}
}

func connectedEndpoints(t *testing.T, fixture fixture) (endpointRunner, endpointRunner, []byte) {
	t.Helper()
	publisher, publication := startPublishedEndpoint(t, fixture)
	client, err := newEndpoint(setup{NetworkID: fixture.networkID, BrokerID: [32]byte{8},
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
		result, err := client.connectForHarness(ctx, outboundConnectionRequest{Principal: fixture.clientPrincipal,
			Capability: clientSession, Target: fixture.first.Target, Publication: publication, Route: clientRoute,
			Application: clientApplication, BytesEachDirection: 64 << 10, At: fixture.now})

		outcomes <- serviceOutcome{result, err}
	}()
	go func() {
		result, err := publisher.acceptForHarness(ctx, inboundConnectionRequest{Principal: fixture.publisherPrincipal,
			Capability: publisherSession, Route: publisherRoute, Application: publisherApplication,
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
