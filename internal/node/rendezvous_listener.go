package node

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"net"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// Rendezvous owns one running TCP/TLS listener and every accepted work item.
// It is a Node duty handle, never a general Carrier or endpoint adapter.
type Rendezvous struct {
	plan       rendezvousPlan
	listener   net.Listener
	handshakes chan struct{}
	waitingCap chan struct{}
	pairs      chan struct{}

	mu       sync.Mutex
	draining bool
	pre      map[net.Conn]struct{}
	waiting  map[[32]byte]*rendezvousLeg
	active   map[net.Conn]struct{}
	usage    RendezvousUsage
	stopOnce sync.Once
	work     sync.WaitGroup
}

type rendezvousLeg struct {
	raw        net.Conn
	connection *tls.Conn
	binding    route.LegBinding
	done       chan struct{}
	doneOnce   sync.Once
}

// StartRendezvous binds one exact State-authorized literal TCP endpoint. It
// does not acquire State, advertise itself, or choose a Node identity.
func StartRendezvous(input RendezvousConfig) (*Rendezvous, error) {
	plan, err := newRendezvousPlan(input)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", plan.ListenAddress)
	if err != nil {
		return nil, err
	}
	return startRendezvous(plan, listener), nil
}

func startRendezvous(plan rendezvousPlan, listener net.Listener) *Rendezvous {
	running := &Rendezvous{plan: plan, listener: listener, handshakes: make(chan struct{}, plan.HandshakeLimit),
		waitingCap: make(chan struct{}, plan.WaitingLimit), pairs: make(chan struct{}, plan.PairLimit),
		pre: make(map[net.Conn]struct{}), waiting: make(map[[32]byte]*rendezvousLeg), active: make(map[net.Conn]struct{})}
	running.work.Add(1)
	go running.accept()
	return running
}

// Usage returns only aggregate local duty state.
func (running *Rendezvous) Usage() RendezvousUsage {
	if running == nil {
		return RendezvousUsage{}
	}
	running.mu.Lock()
	defer running.mu.Unlock()
	return running.snapshotLocked()
}

func (running *Rendezvous) accept() {
	defer running.work.Done()
	for {
		raw, err := running.listener.Accept()
		if err != nil {
			return
		}
		if !running.admitHandshake(raw) {
			_ = raw.Close()
			continue
		}
		running.work.Add(1)
		go running.handle(raw)
	}
}

func (running *Rendezvous) admitHandshake(raw net.Conn) bool {
	running.mu.Lock()
	defer running.mu.Unlock()
	if running.draining || len(running.pairs) == cap(running.pairs) {
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

func (running *Rendezvous) handle(raw net.Conn) {
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
			_ = raw.Close()
		}
	}()
	connection := tls.Server(raw, running.serverTLS())
	if err := connection.SetDeadline(running.plan.NotAfter); err != nil {
		return
	}
	ctx, cancel := context.WithDeadline(context.Background(), running.plan.NotAfter)
	defer cancel()
	if err := connection.HandshakeContext(ctx); err != nil || connection.ConnectionState().NegotiatedProtocol != route.Profile {
		return
	}
	binding, err := route.ReadNodeLegBinding(connection)
	if err != nil || running.validateIncoming(binding, connection.ConnectionState()) != nil {
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
	leg := &rendezvousLeg{raw: raw, connection: connection, binding: binding, done: make(chan struct{})}
	if !running.register(leg) {
		<-running.waitingCap
		return
	}
	owned = false
}

func (running *Rendezvous) serverTLS() *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, Certificates: []tls.Certificate{running.plan.Certificate},
		ClientAuth: tls.RequireAnyClientCert, SessionTicketsDisabled: true, NextProtos: []string{route.Profile},
		VerifyConnection: func(state tls.ConnectionState) error {
			if len(state.PeerCertificates) != 1 {
				return errors.New("Rendezvous TLS client certificate is missing")
			}
			public, ok := state.PeerCertificates[0].PublicKey.(ed25519.PublicKey)
			if !ok || len(public) != ed25519.PublicKeySize {
				return errors.New("Rendezvous TLS client key is invalid")
			}
			return nil
		}}
}

func (running *Rendezvous) validateIncoming(binding route.LegBinding, state tls.ConnectionState) error {
	if binding.NetworkID != running.plan.NetworkID || binding.Epoch != running.plan.Epoch || binding.Digest != running.plan.EpochDigest ||
		binding.PeerRole != route.RendezvousRole || binding.PeerNodeID != running.plan.NodeID ||
		!binding.NotAfter.Equal(running.plan.NotAfter.UTC()) || !running.plan.now().Before(binding.NotAfter) {
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

func (running *Rendezvous) reciprocal(binding route.LegBinding) route.LegBinding {
	return route.LegBinding{NetworkID: binding.NetworkID, Epoch: binding.Epoch, Digest: binding.Digest,
		AttachmentID: binding.AttachmentID, SenderRole: route.RendezvousRole, PeerRole: binding.SenderRole,
		SenderNodeID: running.plan.NodeID, PeerNodeID: binding.SenderNodeID, NotAfter: binding.NotAfter}
}

func (running *Rendezvous) snapshotLocked() RendezvousUsage {
	result := running.usage
	result.Handshakes, result.WaitingLegs, result.ActivePairs = uint16(len(running.handshakes)), uint16(len(running.waitingCap)), uint16(len(running.pairs))
	result.Connections = uint16(len(running.pre) + len(running.waiting) + len(running.active))
	return result
}
