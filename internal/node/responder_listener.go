package node

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// Responder owns the Publisher-side C-2 listener and exactly one State-pinned
// Rendezvous leg per accepted attachment. It never reads an Introduction
// plaintext or accepts a client-selected next hop.
type responder struct {
	plan       responderPlan
	listener   net.Listener
	handshakes chan struct{}
	relays     chan struct{}

	mu        sync.Mutex
	draining  bool
	protected bool
	pre       map[net.Conn]struct{}
	active    map[route.Carrier]struct{}
	usage     responderUsage
	stopOnce  sync.Once
	cleanup   terminalCleanup
	work      sync.WaitGroup
	terminal  chan error
}

func startResponder(input responderConfig) (*responder, error) {
	plan, err := newResponderPlan(input)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", plan.ListenAddress)
	if err != nil {
		return nil, err
	}
	running := &responder{plan: plan, listener: listener, handshakes: make(chan struct{}, plan.HandshakeLimit), relays: make(chan struct{}, plan.RelayLimit),
		pre: make(map[net.Conn]struct{}), active: make(map[route.Carrier]struct{}), terminal: make(chan error, 1)}
	running.work.Add(1)
	go running.accept()
	return running, nil
}

func (running *responder) Done() <-chan error {
	if running == nil {
		return nil
	}
	return running.terminal
}

func (running *responder) Protect(value bool) {
	if running == nil {
		return
	}
	running.mu.Lock()
	if running.draining || running.protected == value {
		running.mu.Unlock()
		return
	}
	running.protected = value
	connections := []net.Conn(nil)
	if value {
		for connection := range running.pre {
			connections = append(connections, connection)
		}
	}
	running.mu.Unlock()
	for _, connection := range connections {
		running.cleanup.record(connection.Close())
	}
}

func (running *responder) Stop() {
	if running == nil {
		return
	}
	running.stopOnce.Do(func() {
		running.mu.Lock()
		running.draining = true
		running.mu.Unlock()
		running.cleanup.record(running.listener.Close())
	})
}

func (running *responder) Usage() responderUsage {
	if running == nil {
		return responderUsage{}
	}
	running.mu.Lock()
	defer running.mu.Unlock()
	result := running.usage
	result.Handshakes, result.ActiveRelays = uint16(len(running.handshakes)), uint16(len(running.relays))
	result.Connections = uint16(len(running.pre) + len(running.active))
	return result
}

func (running *responder) accept() {
	defer running.work.Done()
	for {
		raw, err := running.listener.Accept()
		if err != nil {
			running.mu.Lock()
			draining := running.draining
			running.mu.Unlock()
			if draining {
				running.terminal <- nil
			} else {
				running.terminal <- err
			}
			return
		}
		if !running.reserveHandshake(raw) {
			running.cleanup.record(raw.Close())
			continue
		}
		running.work.Add(1)
		go running.handle(raw)
	}
}

func (running *responder) reserveHandshake(raw net.Conn) bool {
	running.mu.Lock()
	defer running.mu.Unlock()
	if running.draining || running.protected || len(running.relays) == cap(running.relays) {
		running.usage.RefusedBeforeTLS++
		return false
	}
	select {
	case running.handshakes <- struct{}{}:
		running.pre[raw] = struct{}{}
		return true
	default:
		running.usage.RefusedBeforeTLS++
		return false
	}
}

func (running *responder) handle(raw net.Conn) {
	defer running.work.Done()
	handshakeHeld, owned := true, true
	defer func() {
		if handshakeHeld {
			<-running.handshakes
		}
		if owned {
			running.mu.Lock()
			delete(running.pre, raw)
			running.mu.Unlock()
			running.cleanup.record(raw.Close())
		}
	}()
	deadline := boundedAdmissionDeadline(running.plan.now(), running.plan.AdmissionTimeout, running.plan.NotAfter)
	admissionCtx, admissionCancel := context.WithDeadline(context.Background(), deadline)
	accepted, err := route.AcceptEndpointTransitAttachment(admissionCtx, raw, route.EndpointTransitAttachmentAcceptance{NetworkID: running.plan.NetworkID,
		Digest: running.plan.EpochDigest, TransitNodeID: running.plan.NodeID, Epoch: running.plan.Epoch, TransitRole: route.ResponderRole,
		Deadline: running.plan.NotAfter, AdmissionDeadline: deadline, Certificate: running.plan.Certificate, Admit: running.plan.Admit})
	admissionCancel()
	if err != nil || accepted.Connection.SetDeadline(accepted.Binding.NotAfter) != nil || !running.reserveRelay(raw) {
		running.refuseRelay()
		return
	}
	workCtx, workCancel := context.WithDeadline(context.Background(), running.plan.NotAfter)
	defer workCancel()
	<-running.handshakes
	handshakeHeld = false
	owned = false
	next, err := route.OpenNodeLeg(workCtx, route.NodeLegRequest{CarrierProfile: running.plan.rendezvous.CarrierProfile, Endpoint: running.plan.rendezvous.Endpoint, Certificate: running.plan.Certificate,
		ExpectedPeerKey: running.plan.rendezvous.PublicKey, Deadline: accepted.Binding.NotAfter, Binding: route.LegBinding{NetworkID: accepted.Binding.NetworkID,
			Epoch: accepted.Binding.Epoch, Digest: accepted.Binding.Digest, AttachmentID: accepted.Binding.AttachmentID, SenderRole: route.ResponderRole,
			PeerRole: route.RendezvousRole, SenderNodeID: running.plan.NodeID, PeerNodeID: running.plan.rendezvous.NodeID, NotAfter: accepted.Binding.NotAfter}})
	if err != nil {
		running.releaseRelay(raw, nil)
		return
	}
	if err := next.SetDeadline(accepted.Binding.NotAfter); err != nil {
		running.cleanup.record(next.Close())
		running.releaseRelay(raw, nil)
		return
	}
	running.mu.Lock()
	running.active[next] = struct{}{}
	running.mu.Unlock()
	running.work.Add(1)
	go running.relay(raw, accepted.Connection, next)
}

func (running *responder) reserveRelay(raw net.Conn) bool {
	running.mu.Lock()
	defer running.mu.Unlock()
	if running.draining || running.protected {
		return false
	}
	select {
	case running.relays <- struct{}{}:
		delete(running.pre, raw)
		running.active[raw] = struct{}{}
		return true
	default:
		return false
	}
}

func (running *responder) refuseRelay() {
	running.mu.Lock()
	running.usage.RelayRefused++
	running.mu.Unlock()
}

func (running *responder) relay(raw, endpoint net.Conn, next route.Carrier) {
	defer running.work.Done()
	type lane struct{ bytes int64 }
	results := make(chan lane, 2)
	copyLane := func(destination, source route.Carrier) {
		limited := &io.LimitedReader{R: source, N: int64(running.plan.RelayByteLimit)}
		count, _ := io.CopyBuffer(destination, limited, make([]byte, 32<<10))
		results <- lane{bytes: count}
	}
	go copyLane(endpoint, next)
	go copyLane(next, endpoint)
	first := <-results
	running.cleanup.record(raw.Close())
	running.cleanup.record(next.Close())
	second := <-results
	running.mu.Lock()
	running.usage.RelayedBytes += uint64(first.bytes + second.bytes)
	running.usage.CompletedRelays++
	running.mu.Unlock()
	running.releaseRelay(raw, next)
}

func (running *responder) releaseRelay(raw net.Conn, next route.Carrier) {
	running.mu.Lock()
	delete(running.active, raw)
	if next != nil {
		delete(running.active, next)
	}
	<-running.relays
	running.mu.Unlock()
	running.cleanup.record(raw.Close())
	if next != nil {
		running.cleanup.record(next.Close())
	}
}

func (running *responder) Drain(ctx context.Context) error {
	if running == nil || ctx == nil {
		return errors.New("responder duty is unavailable")
	}
	running.Stop()
	running.mu.Lock()
	connections := make([]route.Carrier, 0, len(running.pre)+len(running.active))
	for connection := range running.pre {
		connections = append(connections, connection)
	}
	for connection := range running.active {
		connections = append(connections, connection)
	}
	running.mu.Unlock()
	for _, connection := range connections {
		running.cleanup.record(connection.Close())
	}
	done := make(chan struct{})
	go func() { running.work.Wait(); close(done) }()
	timer := time.NewTimer(running.plan.DrainTimeout)
	defer timer.Stop()
	select {
	case <-done:
		return running.cleanup.result()
	case <-ctx.Done():
		return errors.Join(running.cleanup.result(), ctx.Err())
	case <-timer.C:
		return errors.Join(running.cleanup.result(), errors.New("responder drain exceeded its Work Safety Lease"))
	}
}

func (running *responder) Close() error { return running.Drain(context.Background()) }
