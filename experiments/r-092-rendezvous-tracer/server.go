//go:build ignore

package main

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

const wireTranscriptBytes = frameHeaderBytes + transcriptBytes

type serverArguments struct {
	listen         string
	deadline       time.Time
	drainAfter     time.Duration
	handshakeLimit int
	waitingLimit   int
	pairLimit      int
	expectPairs    int
}

type serverResult struct {
	Schema                  string `json:"schema"`
	Role                    string `json:"role"`
	Outcome                 string `json:"outcome"`
	Passed                  bool   `json:"passed"`
	CompletedPairs          int    `json:"completed_pairs"`
	SuccessfulPairs         int    `json:"successful_pairs"`
	PeakHandshakes          int    `json:"peak_handshakes"`
	PeakWaitingLegs         int    `json:"peak_waiting_legs"`
	PeakActivePairs         int    `json:"peak_active_pairs"`
	PreTLSCapacityRejected  int    `json:"pre_tls_capacity_rejected"`
	DuplicateSideRejected   int    `json:"duplicate_side_rejected"`
	WaitingCapacityRejected int    `json:"waiting_capacity_rejected"`
	UnmatchedExpired        int    `json:"unmatched_expired"`
	FinalHandshakes         int    `json:"final_handshakes"`
	FinalWaitingLegs        int    `json:"final_waiting_legs"`
	FinalActivePairs        int    `json:"final_active_pairs"`
	FinalConnections        int    `json:"final_connections"`
	CleanupJoined           bool   `json:"cleanup_joined"`
	ElapsedMS               int64  `json:"elapsed_ms"`
}

type rendezvousLeg struct {
	connection *tls.Conn
	binding    route.LegBinding
	paired     chan struct{}
}

type rendezvousCounters struct {
	completedPairs, successfulPairs                    int
	peakHandshakes, peakWaiting, peakPairs             int
	preTLSRejected, duplicateRejected, waitingRejected int
	unmatchedExpired                                   int
}

type rendezvousServer struct {
	arguments   serverArguments
	material    identitySet
	emit        func(any)
	listener    net.Listener
	handshakes  chan struct{}
	waitSlots   chan struct{}
	pairSlots   chan struct{}
	pairDone    chan bool
	acceptDone  chan struct{}
	stopOnce    sync.Once
	work        sync.WaitGroup
	mu          sync.Mutex
	draining    bool
	waiting     map[[32]byte]*rendezvousLeg
	connections map[net.Conn]struct{}
	counters    rendezvousCounters
}

func runServer(ctx context.Context, arguments serverArguments, emit func(any)) (serverResult, error) {
	started := time.Now()
	material, err := deterministicIdentities()
	if err != nil {
		return serverResult{}, err
	}
	listener, err := net.Listen("tcp", arguments.listen)
	if err != nil {
		return serverResult{}, err
	}
	server := &rendezvousServer{arguments: arguments, material: material, emit: emit, listener: listener,
		handshakes: make(chan struct{}, arguments.handshakeLimit), waitSlots: make(chan struct{}, arguments.waitingLimit),
		pairSlots: make(chan struct{}, arguments.pairLimit), pairDone: make(chan bool, arguments.pairLimit+1),
		acceptDone: make(chan struct{}), waiting: make(map[[32]byte]*rendezvousLeg),
		connections: make(map[net.Conn]struct{})}
	emit(map[string]any{"schema": "ardents-r092-rendezvous-event-v1", "role": "server", "event": "ready",
		"endpoint": listener.Addr().String(), "handshake_limit": arguments.handshakeLimit,
		"waiting_limit": arguments.waitingLimit, "pair_limit": arguments.pairLimit})
	go server.accept(ctx)
	successes := 0
	if arguments.expectPairs > 0 {
		var drain <-chan time.Time
		if arguments.drainAfter > 0 {
			timer := time.NewTimer(arguments.drainAfter)
			defer timer.Stop()
			drain = timer.C
		}
		for successes < arguments.expectPairs {
			select {
			case successful := <-server.pairDone:
				if successful {
					successes++
				}
			case <-ctx.Done():
				successes = arguments.expectPairs
			case <-drain:
				successes = arguments.expectPairs
			}
		}
	} else {
		wait := time.Until(arguments.deadline.Add(250 * time.Millisecond))
		if arguments.drainAfter > 0 && arguments.drainAfter < wait {
			wait = arguments.drainAfter
		}
		if wait < 0 {
			wait = 0
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
		case <-timer.C:
		}
		timer.Stop()
	}
	joined := server.drain()
	result := server.result(joined, time.Since(started))
	if !result.Passed {
		return result, errors.New("rendezvous server did not satisfy its cleanup oracle")
	}
	return result, nil
}

func (server *rendezvousServer) accept(ctx context.Context) {
	defer close(server.acceptDone)
	for {
		connection, err := server.listener.Accept()
		if err != nil {
			return
		}
		if server.rejectBeforeTLS() {
			_ = connection.Close()
			continue
		}
		select {
		case server.handshakes <- struct{}{}:
			server.observeHandshakePeak()
		default:
			server.incrementPreTLSRejected()
			_ = connection.Close()
			continue
		}
		server.track(connection, true)
		server.work.Add(1)
		go server.handle(ctx, connection)
	}
}

func (server *rendezvousServer) handle(parent context.Context, raw net.Conn) {
	defer server.work.Done()
	defer func() { <-server.handshakes }()
	owned := true
	defer func() {
		if owned {
			_ = raw.Close()
			server.track(raw, false)
		}
	}()
	connection := tls.Server(raw, serverTLS(server.material))
	server.track(raw, false)
	server.track(connection, true)
	raw = connection
	if err := connection.SetDeadline(server.arguments.deadline); err != nil {
		return
	}
	ctx, cancel := context.WithDeadline(parent, server.arguments.deadline)
	defer cancel()
	if err := connection.HandshakeContext(ctx); err != nil {
		return
	}
	state := connection.ConnectionState()
	if state.NegotiatedProtocol != nativeALPN || len(state.PeerCertificates) == 0 {
		return
	}
	binding, err := readBinding(connection)
	if err != nil {
		return
	}
	legState := &tlsLegState{peerID: publicIdentity(state.PeerCertificates[0])}
	if err := validateIncomingBinding(binding, legState, server.material, server.arguments.deadline); err != nil {
		return
	}
	select {
	case server.waitSlots <- struct{}{}:
	default:
		server.incrementWaitingRejected()
		return
	}
	if err := writeBinding(connection, reciprocalBinding(binding, server.material.serverID)); err != nil {
		<-server.waitSlots
		return
	}
	leg := &rendezvousLeg{connection: connection, binding: binding, paired: make(chan struct{})}
	if !server.register(leg) {
		<-server.waitSlots
		return
	}
	owned = false
}

func (server *rendezvousServer) register(leg *rendezvousLeg) bool {
	server.mu.Lock()
	if server.draining {
		server.mu.Unlock()
		return false
	}
	attachment := leg.binding.AttachmentID
	existing := server.waiting[attachment]
	if existing == nil {
		server.waiting[attachment] = leg
		server.observeWaitingPeakLocked()
		server.work.Add(1)
		go server.expire(leg)
		server.mu.Unlock()
		return true
	}
	if existing.binding.SenderRole == leg.binding.SenderRole {
		server.counters.duplicateRejected++
		server.mu.Unlock()
		return false
	}
	select {
	case server.pairSlots <- struct{}{}:
	default:
		delete(server.waiting, attachment)
		close(existing.paired)
		server.counters.waitingRejected++
		server.mu.Unlock()
		<-server.waitSlots
		_ = existing.connection.Close()
		server.track(existing.connection, false)
		return false
	}
	delete(server.waiting, attachment)
	close(existing.paired)
	<-server.waitSlots
	<-server.waitSlots
	server.observePairPeakLocked()
	server.mu.Unlock()
	initiator, responder := existing, leg
	if initiator.binding.SenderRole != initiatorRole {
		initiator, responder = responder, initiator
	}
	server.emit(map[string]any{"schema": "ardents-r092-rendezvous-event-v1", "role": "server",
		"event": "pair-active", "attachment": attachment})
	server.work.Add(1)
	go server.pump(initiator, responder)
	return true
}

func (server *rendezvousServer) expire(leg *rendezvousLeg) {
	defer server.work.Done()
	timer := time.NewTimer(time.Until(leg.binding.NotAfter))
	defer timer.Stop()
	select {
	case <-leg.paired:
		return
	case <-timer.C:
	}
	expired := false
	server.mu.Lock()
	if server.waiting[leg.binding.AttachmentID] == leg {
		delete(server.waiting, leg.binding.AttachmentID)
		server.counters.unmatchedExpired++
		expired = true
	}
	server.mu.Unlock()
	if expired {
		<-server.waitSlots
		_ = leg.connection.Close()
		server.track(leg.connection, false)
	}
}

func (server *rendezvousServer) pump(initiator, responder *rendezvousLeg) {
	defer server.work.Done()
	type copyResult struct {
		bytes int64
		err   error
	}
	results := make(chan copyResult, 2)
	copyLane := func(destination, source net.Conn) {
		count, err := io.CopyBuffer(destination, source, make([]byte, 32<<10))
		results <- copyResult{bytes: count, err: err}
	}
	go copyLane(responder.connection, initiator.connection)
	go copyLane(initiator.connection, responder.connection)
	first := <-results
	_ = initiator.connection.Close()
	_ = responder.connection.Close()
	second := <-results
	server.track(initiator.connection, false)
	server.track(responder.connection, false)
	<-server.pairSlots
	successful := first.bytes == wireTranscriptBytes && second.bytes == wireTranscriptBytes
	server.mu.Lock()
	server.counters.completedPairs++
	if successful {
		server.counters.successfulPairs++
	}
	server.mu.Unlock()
	server.pairDone <- successful
}

func (server *rendezvousServer) rejectBeforeTLS() bool {
	server.mu.Lock()
	defer server.mu.Unlock()
	if server.draining || len(server.pairSlots) == cap(server.pairSlots) {
		server.counters.preTLSRejected++
		return true
	}
	return false
}

func (server *rendezvousServer) drain() bool {
	server.stopOnce.Do(func() {
		server.mu.Lock()
		server.draining = true
		server.mu.Unlock()
		_ = server.listener.Close()
	})
	<-server.acceptDone
	server.mu.Lock()
	connections := make([]net.Conn, 0, len(server.connections))
	for connection := range server.connections {
		connections = append(connections, connection)
	}
	server.mu.Unlock()
	for _, connection := range connections {
		_ = connection.Close()
	}
	done := make(chan struct{})
	go func() { server.work.Wait(); close(done) }()
	select {
	case <-done:
		return true
	case <-time.After(5 * time.Second):
		return false
	}
}

func (server *rendezvousServer) result(joined bool, elapsed time.Duration) serverResult {
	server.mu.Lock()
	defer server.mu.Unlock()
	result := serverResult{Schema: "ardents-r092-rendezvous-server-v1", Role: "server", Outcome: "complete",
		CompletedPairs: server.counters.completedPairs, SuccessfulPairs: server.counters.successfulPairs,
		PeakHandshakes: server.counters.peakHandshakes, PeakWaitingLegs: server.counters.peakWaiting,
		PeakActivePairs: server.counters.peakPairs, PreTLSCapacityRejected: server.counters.preTLSRejected,
		DuplicateSideRejected:   server.counters.duplicateRejected,
		WaitingCapacityRejected: server.counters.waitingRejected, UnmatchedExpired: server.counters.unmatchedExpired,
		FinalHandshakes: len(server.handshakes), FinalWaitingLegs: len(server.waitSlots),
		FinalActivePairs: len(server.pairSlots), FinalConnections: len(server.connections),
		CleanupJoined: joined, ElapsedMS: elapsed.Milliseconds()}
	result.Passed = joined && result.FinalHandshakes == 0 && result.FinalWaitingLegs == 0 &&
		result.FinalActivePairs == 0 && result.FinalConnections == 0 &&
		result.SuccessfulPairs >= server.arguments.expectPairs
	return result
}

func (server *rendezvousServer) track(connection net.Conn, add bool) {
	server.mu.Lock()
	defer server.mu.Unlock()
	if add {
		server.connections[connection] = struct{}{}
	} else {
		delete(server.connections, connection)
	}
}

func (server *rendezvousServer) observeHandshakePeak() {
	server.mu.Lock()
	defer server.mu.Unlock()
	if len(server.handshakes) > server.counters.peakHandshakes {
		server.counters.peakHandshakes = len(server.handshakes)
	}
}

func (server *rendezvousServer) observeWaitingPeakLocked() {
	if len(server.waitSlots) > server.counters.peakWaiting {
		server.counters.peakWaiting = len(server.waitSlots)
	}
}

func (server *rendezvousServer) observePairPeakLocked() {
	if len(server.pairSlots) > server.counters.peakPairs {
		server.counters.peakPairs = len(server.pairSlots)
	}
}

func (server *rendezvousServer) incrementPreTLSRejected() {
	server.mu.Lock()
	server.counters.preTLSRejected++
	server.mu.Unlock()
}

func (server *rendezvousServer) incrementWaitingRejected() {
	server.mu.Lock()
	server.counters.waitingRejected++
	server.mu.Unlock()
}
