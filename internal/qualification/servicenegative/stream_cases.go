package servicenegative

import (
	"context"
	"io"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

type streamOutcome struct {
	result serviceconn.Result
	err    error
}

func (value fixture) streamObservations(ctx context.Context) (map[string]bool, map[string]string, map[string]uint32) {
	operations := map[string]bool{}
	classes := map[string]string{}
	counts := map[string]uint32{}
	operations["backpressure"], operations["cancellation"], classes["cancellation"],
		counts["cancellation-accepted"], counts["cancellation-received"] = value.observeCancellation(ctx)
	operations["partial-write"], classes["partial-write"], counts["partial-low"], counts["partial-high"] =
		value.observePartial(ctx)
	operations["recovery-queue-full"], counts["recovery-queue-high"] = value.observeRecoveryQueue(ctx)
	return operations, classes, counts
}

func (value fixture) observeRecoveryQueue(parent context.Context) (bool, uint32) {
	client, publisher, publication, ok := value.connected(parent)
	if !ok {
		return false, 0
	}
	clientRoute, publisherRoute, ok := connectedTCP()
	if !ok {
		return false, 0
	}
	clientEndpoint, clientPeer := net.Pipe()
	publisherEndpoint, publisherPeer := net.Pipe()
	defer clientPeer.Close()
	defer publisherPeer.Close()
	ctx, cancel := context.WithCancel(parent)
	queueReached := make(chan struct{})
	observedClient := &observedConnection{Conn: clientEndpoint, threshold: 256 << 10, reached: queueReached}
	gatedRoute := &gatedWriter{Conn: publisherRoute, ctx: ctx}
	observedPublisher := &observedConnection{Conn: publisherEndpoint, gate: gatedRoute}
	go func() { _, _ = io.Copy(io.Discard, publisherPeer) }()
	outcomes := value.runRecoveryQueueConnections(ctx, client, publisher, publication, clientRoute, gatedRoute,
		observedClient, observedPublisher, 4<<20)
	written := make(chan error, 1)
	go func() { _, err := clientPeer.Write(make([]byte, 4<<20)); written <- err }()
	select {
	case <-written:
		cancel()
		return false, 0
	case <-queueReached:
	case <-time.After(2 * time.Second):
		cancel()
		return false, 0
	}
	cancel()
	var high uint32
	for range 2 {
		outcome := <-outcomes
		high = max(high, outcome.result.QueueHighWater)
		if outcome.result.AcceptedBytes > 256<<10 || outcome.result.QueueHighWater > 256<<10 {
			return false, high
		}
	}
	return high > 0, high
}

func (value fixture) runRecoveryQueueConnections(ctx context.Context, client, publisher endpointRunner, publication []byte,
	clientRoute, publisherRoute, clientApplication, publisherApplication net.Conn, count uint32) <-chan streamOutcome {
	outcomes := make(chan streamOutcome, 2)
	binding := serviceconn.Recovery{CandidateView: [32]byte{41}, IsolationContext: [32]byte{42},
		DestinationBinding: [32]byte{43}, RouteProfile: "h3-recovery-queue-v1", WorkSafetyNotAfter: value.first.NotAfter,
		WorkSafetyMaximum: value.first.NotAfter, NoNewRecoveryAfter: value.first.NotAfter}
	go func() {
		result, err := client.Do(ctx, serviceconn.Request{Action: "connect", Principal: value.connection,
			Session: admit(ctx, client, value.connection, "connection", value.now), Target: value.first.Target,
			Publication: publication, Route: clientRoute, Application: clientApplication, OpenAttachment: unavailableRecovery,
			RecoveryBinding: binding, SendBytes: count, At: value.now})
		outcomes <- streamOutcome{result, err}
	}()
	go func() {
		result, err := publisher.Do(ctx, serviceconn.Request{Action: "accept", Principal: value.connection,
			Session: admit(ctx, publisher, value.connection, "connection", value.now), Route: publisherRoute,
			Application: publisherApplication, OpenAttachment: unavailableRecovery,
			RecoveryBinding: binding, ReceiveBytes: count, At: value.now})
		outcomes <- streamOutcome{result, err}
	}()
	return outcomes
}

func (value fixture) observeCancellation(parent context.Context) (bool, bool, string, uint32, uint32) {
	client, publisher, publication, ok := value.connected(parent)
	if !ok {
		return false, false, "", 0, 0
	}
	clientRoute, publisherRoute := net.Pipe()
	clientEndpoint, clientPeer := net.Pipe()
	publisherEndpoint, publisherPeer := net.Pipe()
	defer clientPeer.Close()
	defer publisherPeer.Close()
	ctx, cancel := context.WithCancel(parent)
	clientEntered, publisherEntered := make(chan struct{}), make(chan struct{})
	outcomes := value.runConnections(ctx, client, publisher, publication, clientRoute, publisherRoute,
		&observedConnection{Conn: clientEndpoint, entered: clientEntered},
		&observedConnection{Conn: publisherEndpoint, entered: publisherEntered})
	entered := true
	for _, signal := range []<-chan struct{}{clientEntered, publisherEntered} {
		select {
		case <-signal:
		case <-time.After(time.Second):
			entered = false
		}
	}
	blocked := entered
	completed := make([]streamOutcome, 0, 2)
	select {
	case outcome := <-outcomes:
		blocked = false
		completed = append(completed, outcome)
	default:
	}
	cancel()
	classified := true
	var accepted, received uint32
	for len(completed) < 2 {
		completed = append(completed, <-outcomes)
	}
	for _, outcome := range completed {
		classified = classified && outcome.err != nil && outcome.result.Class == "local timeout or cancellation" &&
			outcome.result.AcceptedBytes == 0 && outcome.result.ReceivedBytes == 0
		accepted += outcome.result.AcceptedBytes
		received += outcome.result.ReceivedBytes
	}
	return blocked, classified, "local timeout or cancellation", accepted, received
}

func (value fixture) observePartial(ctx context.Context) (bool, string, uint32, uint32) {
	client, publisher, publication, ok := value.connected(ctx)
	if !ok {
		return false, "", 0, 0
	}
	clientRoute, publisherRoute, ok := connectedTCP()
	if !ok {
		return false, "", 0, 0
	}
	clientEndpoint, clientPeer, ok := connectedTCP()
	if !ok {
		return false, "", 0, 0
	}
	publisherEndpoint, publisherPeer, ok := connectedTCP()
	if !ok {
		return false, "", 0, 0
	}
	defer clientPeer.Close()
	defer publisherPeer.Close()
	outcomes := value.runConnections(ctx, client, publisher, publication, clientRoute, publisherRoute,
		clientEndpoint, publisherEndpoint)
	if _, err := clientPeer.Write(make([]byte, 1024)); err != nil {
		return false, "", 0, 0
	}
	if _, err := publisherPeer.Write(make([]byte, 2048)); err != nil {
		return false, "", 0, 0
	}
	_ = clientPeer.(*net.TCPConn).CloseWrite()
	_ = publisherPeer.(*net.TCPConn).CloseWrite()
	seen := map[uint32]bool{}
	valid := true
	for range 2 {
		outcome := <-outcomes
		valid = valid && outcome.err != nil && outcome.result.Class == "abrupt connection loss" &&
			outcome.result.AcceptedBytes > 0 && outcome.result.ReceivedBytes > 0
		seen[outcome.result.AcceptedBytes] = true
	}
	return valid && seen[1024] && seen[2048], "abrupt connection loss", 1024, 2048
}

func (value fixture) connected(ctx context.Context) (endpointRunner, endpointRunner, []byte, bool) {
	publisher := value.endpoint()
	published, err := publish(ctx, publisher, value, value.first, value.firstPrivate)
	if err != nil {
		return nil, nil, nil, false
	}
	return value.endpointWithBroker([32]byte{8}), publisher, published.Publication, true
}

func (value fixture) runConnections(ctx context.Context, client, publisher endpointRunner, publication []byte,
	clientRoute, publisherRoute, clientApplication, publisherApplication net.Conn) <-chan streamOutcome {
	return value.runConnectionsWithCount(ctx, client, publisher, publication, clientRoute, publisherRoute,
		clientApplication, publisherApplication, 64<<10)
}

func (value fixture) runConnectionsWithCount(ctx context.Context, client, publisher endpointRunner, publication []byte,
	clientRoute, publisherRoute, clientApplication, publisherApplication net.Conn, count uint32) <-chan streamOutcome {
	outcomes := make(chan streamOutcome, 2)
	clientSession := admit(ctx, client, value.connection, "connection", value.now)
	publisherSession := admit(ctx, publisher, value.connection, "connection", value.now)
	go func() {
		result, err := client.Do(ctx, serviceconn.Request{Action: "connect", Principal: value.connection,
			Session: clientSession, Target: value.first.Target, Publication: publication, Route: clientRoute,
			Application: clientApplication, BytesEachDirection: count, At: value.now})
		outcomes <- streamOutcome{result, err}
	}()
	go func() {
		result, err := publisher.Do(ctx, serviceconn.Request{Action: "accept", Principal: value.connection,
			Session: publisherSession, Route: publisherRoute, Application: publisherApplication,
			BytesEachDirection: count, At: value.now})
		outcomes <- streamOutcome{result, err}
	}()
	return outcomes
}

func connectedTCP() (net.Conn, net.Conn, bool) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, false
	}
	accepted := make(chan net.Conn, 1)
	go func() { connection, _ := listener.Accept(); accepted <- connection }()
	peer, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		_ = listener.Close()
		return nil, nil, false
	}
	endpoint := <-accepted
	_ = listener.Close()
	return endpoint, peer, true
}
