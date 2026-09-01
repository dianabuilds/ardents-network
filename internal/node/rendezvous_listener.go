package node

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"io"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// Rendezvous owns one running State-selected Carrier listener and every
// accepted work item. It is a Node duty handle, never a transport selector.
type rendezvous struct {
	plan       rendezvousPlan
	listener   route.CarrierListener
	handshakes chan struct{}
	waitingCap chan struct{}
	pairs      chan struct{}

	mu        sync.Mutex
	draining  bool
	protected bool
	pre       map[route.PendingCarrier]struct{}
	waiting   map[[32]byte]*rendezvousLeg
	active    map[route.Carrier]struct{}
	usage     rendezvousUsage
	stopOnce  sync.Once
	cleanup   terminalCleanup
	work      sync.WaitGroup
	terminal  chan error
}

type rendezvousLeg struct {
	pending    route.PendingCarrier
	connection route.Carrier
	binding    route.LegBinding
	done       chan struct{}
	doneOnce   sync.Once
}

// startRendezvous binds one exact State-authorized Carrier and literal
// endpoint. It does not negotiate or fall back to another Carrier.
func startRendezvous(input rendezvousConfig) (*rendezvous, error) {
	plan, err := newRendezvousPlan(input)
	if err != nil {
		return nil, err
	}
	listener, err := route.ListenNodeCarrier(plan.CarrierProfile, plan.ListenAddress, plan.Certificate)
	if err != nil {
		return nil, err
	}
	return startRendezvousWithListener(plan, listener), nil
}

func startRendezvousWithListener(plan rendezvousPlan, listener route.CarrierListener) *rendezvous {
	running := &rendezvous{plan: plan, listener: listener, handshakes: make(chan struct{}, plan.HandshakeLimit),
		waitingCap: make(chan struct{}, plan.WaitingLimit), pairs: make(chan struct{}, plan.PairLimit),
		pre: make(map[route.PendingCarrier]struct{}), waiting: make(map[[32]byte]*rendezvousLeg), active: make(map[route.Carrier]struct{}),
		terminal: make(chan error, 1)}
	running.work.Add(1)
	go running.accept()
	return running
}

// Done yields the sole terminal listener outcome. A nil value means the duty
// was stopped deliberately; a non-nil value requires Node withdrawal.
func (running *rendezvous) Done() <-chan error {
	if running == nil {
		return nil
	}
	return running.terminal
}

// Protect cancels pre-admission work and refuses new work while preserving
// active paired legs. It is the Node resource-pressure transition, not an
// alternate duty state or peer-selection mechanism.
func (running *rendezvous) Protect(value bool) {
	if running == nil {
		return
	}
	running.mu.Lock()
	if running.draining || running.protected == value {
		running.mu.Unlock()
		return
	}
	running.protected = value
	var connections []io.Closer
	if value {
		connections = running.closePreAdmissionLocked()
	}
	running.mu.Unlock()
	for _, connection := range connections {
		running.cleanup.record(connection.Close())
	}
}

// Stop closes only Rendezvous admission. Call Drain afterwards to join every
// existing worker inside the duty's declared lease.
func (running *rendezvous) Stop() {
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

// Usage returns only aggregate local duty state.
func (running *rendezvous) Usage() rendezvousUsage {
	if running == nil {
		return rendezvousUsage{}
	}
	running.mu.Lock()
	defer running.mu.Unlock()
	return running.snapshotLocked()
}

func (running *rendezvous) accept() {
	defer running.work.Done()
	for {
		pending, err := running.listener.Accept(context.Background())
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
		if !running.admitHandshake(pending) {
			running.cleanup.record(pending.Close())
			continue
		}
		running.work.Add(1)
		go running.handle(pending)
	}
}

func (running *rendezvous) admitHandshake(pending route.PendingCarrier) bool {
	running.mu.Lock()
	defer running.mu.Unlock()
	if running.draining || running.protected || len(running.pairs) == cap(running.pairs) {
		running.usage.RefusedBeforeTLS++
		return false
	}
	select {
	case running.handshakes <- struct{}{}:
		running.pre[pending] = struct{}{}
		return true
	default:
		running.usage.RefusedBeforeTLS++
		return false
	}
}

func (running *rendezvous) handle(pending route.PendingCarrier) {
	defer running.work.Done()
	handshakeHeld, owned := true, true
	defer func() {
		if handshakeHeld {
			<-running.handshakes
		}
		if owned {
			running.mu.Lock()
			delete(running.pre, pending)
			running.mu.Unlock()
			running.cleanup.record(pending.Close())
		}
	}()
	deadline := boundedAdmissionDeadline(running.plan.now(), running.plan.AdmissionTimeout, running.plan.NotAfter)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	connection, state, err := pending.Authenticate(ctx, deadline)
	if err != nil || state.NegotiatedProtocol != route.Profile {
		return
	}
	binding, err := route.ReadNodeLegBinding(connection)
	if err != nil || running.validateIncoming(binding, state) != nil {
		return
	}
	if err := connection.SetDeadline(binding.NotAfter); err != nil {
		return
	}
	select {
	case running.waitingCap <- struct{}{}:
	default:
		running.mu.Lock()
		running.usage.WaitingRefused++
		running.mu.Unlock()
		return
	}
	if err := route.WriteNodeLegBinding(connection, running.reciprocal(binding)); err != nil {
		<-running.waitingCap
		return
	}
	<-running.handshakes
	handshakeHeld = false
	leg := &rendezvousLeg{pending: pending, connection: connection, binding: binding, done: make(chan struct{})}
	if !running.register(leg) {
		<-running.waitingCap
		return
	}
	owned = false
}

func (running *rendezvous) validateIncoming(binding route.LegBinding, state tls.ConnectionState) error {
	if binding.NetworkID != running.plan.NetworkID || binding.Epoch != running.plan.Epoch || binding.Digest != running.plan.EpochDigest ||
		binding.PeerRole != route.RendezvousRole || binding.PeerNodeID != running.plan.NodeID ||
		binding.NotAfter.After(running.plan.NotAfter.UTC()) || !running.plan.now().Before(binding.NotAfter) {
		return errors.New("Rendezvous LegBinding context is unauthorized")
	}
	peer, found := running.plan.peersByNode[binding.SenderNodeID]
	if !found || peer.Role != binding.SenderRole {
		return errors.New("Rendezvous LegBinding sender is unauthorized")
	}
	if len(state.PeerCertificates) != 1 {
		return errors.New("Rendezvous TLS client certificate is missing")
	}
	public, ok := state.PeerCertificates[0].PublicKey.(ed25519.PublicKey)
	if !ok || string(public) != string(peer.PublicKey[:]) {
		return errors.New("Rendezvous LegBinding and TLS identity differ")
	}
	return nil
}

func (running *rendezvous) reciprocal(binding route.LegBinding) route.LegBinding {
	return route.LegBinding{NetworkID: binding.NetworkID, Epoch: binding.Epoch, Digest: binding.Digest,
		AttachmentID: binding.AttachmentID, SenderRole: route.RendezvousRole, PeerRole: binding.SenderRole,
		SenderNodeID: running.plan.NodeID, PeerNodeID: binding.SenderNodeID, NotAfter: binding.NotAfter}
}

func (running *rendezvous) snapshotLocked() rendezvousUsage {
	result := running.usage
	result.Handshakes, result.WaitingLegs, result.ActivePairs = uint16(len(running.handshakes)), uint16(len(running.waitingCap)), uint16(len(running.pairs))
	result.Connections = uint16(len(running.pre) + len(running.waiting) + len(running.active))
	return result
}
