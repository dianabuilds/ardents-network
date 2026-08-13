package serviceconn_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

type serviceOutcome struct {
	result serviceconn.Result
	err    error
}

type endpointRunner interface {
	Do(context.Context, serviceconn.Request) (serviceconn.Result, error)
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
	outcomes := runConnections(ctx, fixture, client, publisher, publication,
		clientRoute, publisherRoute, clientEndpoint, publisherEndpoint)
	select {
	case outcome := <-outcomes:
		t.Fatalf("blocked Application completed without input: %+v %v", outcome.result, outcome.err)
	case <-time.After(50 * time.Millisecond):
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
	go func() {
		_, err := clientApplication.Write(seededBytes(4<<20, 33))
		writeDone <- err
	}()
	select {
	case err := <-writeDone:
		t.Fatalf("slow remote Application did not backpressure four MiB write: %v", err)
	case <-time.After(100 * time.Millisecond):
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
	result, _ := endpoint.Do(context.Background(), serviceconn.Request{Action: "admit",
		Surface: "connection", Principal: principal, At: at})
	return result.Session
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
