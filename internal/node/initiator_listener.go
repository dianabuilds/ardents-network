package node

import (
	"context"
	"errors"
	"net"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

// Initiator owns one running native Entry listener, its State-pinned next-leg
// dials, and all accepted relay work. It never selects another Node or route.
type Initiator struct {
	plan       initiatorPlan
	listener   net.Listener
	handshakes chan struct{}
	relays     chan struct{}

	mu        sync.Mutex
	draining  bool
	protected bool
	pre       map[net.Conn]struct{}
	active    map[route.Carrier]struct{}
	usage     InitiatorUsage
	stopOnce  sync.Once
	cleanup   terminalCleanup
	work      sync.WaitGroup
	terminal  chan error
}

// startInitiator binds one exact State-authorized literal Entry endpoint. It
// does not acquire State, discover a Rendezvous Node, or create an Entry root.
func startInitiator(input initiatorConfig) (*Initiator, error) {
	plan, err := newInitiatorPlan(input)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", plan.ListenAddress)
	if err != nil {
		return nil, err
	}
	running := &Initiator{plan: plan, listener: listener, handshakes: make(chan struct{}, plan.HandshakeLimit),
		relays: make(chan struct{}, plan.RelayLimit), pre: make(map[net.Conn]struct{}), active: make(map[route.Carrier]struct{}),
		terminal: make(chan error, 1)}
	running.work.Add(1)
	go running.accept()
	return running, nil
}

// Done yields the sole terminal listener outcome. A nil value means the duty
// was stopped deliberately; a non-nil value requires Node withdrawal.
func (running *Initiator) Done() <-chan error {
	if running == nil {
		return nil
	}
	return running.terminal
}

// Protect cancels pre-relay work and refuses new Entry work while preserving
// relays already established through the accepted setup.
func (running *Initiator) Protect(value bool) {
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
		connections = running.closePreAdmissionLocked()
	}
	running.mu.Unlock()
	for _, connection := range connections {
		running.cleanup.record(connection.Close())
	}
}

// Stop closes only new Initiator admission. Call Drain to join every accepted
// handshake and relay within the duty's declared lease.
func (running *Initiator) Stop() {
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
func (running *Initiator) Usage() InitiatorUsage {
	if running == nil {
		return InitiatorUsage{}
	}
	running.mu.Lock()
	defer running.mu.Unlock()
	result := running.usage
	result.Handshakes, result.ActiveRelays = uint16(len(running.handshakes)), uint16(len(running.relays))
	result.Connections = uint16(len(running.pre) + len(running.active))
	return result
}

func (running *Initiator) accept() {
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
		if !running.admitHandshake(raw) {
			running.cleanup.record(raw.Close())
			continue
		}
		running.work.Add(1)
		go running.handle(raw)
	}
}

func (running *Initiator) admitHandshake(raw net.Conn) bool {
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

func (running *Initiator) handle(raw net.Conn) {
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
	entryConnection, err := route.AcceptEntryAttachment(admissionCtx, raw, route.EntryAttachmentAcceptance{NetworkID: running.plan.NetworkID,
		Digest: running.plan.EpochDigest, InitiatorNodeID: running.plan.NodeID, Epoch: running.plan.Epoch,
		Deadline: running.plan.NotAfter, AdmissionDeadline: deadline, Certificate: running.plan.Certificate, Admit: running.plan.Admit})
	admissionCancel()
	if err != nil || entryConnection.SetDeadline(running.plan.NotAfter) != nil {
		return
	}
	workCtx, workCancel := context.WithDeadline(context.Background(), running.plan.NotAfter)
	defer workCancel()
	operation, err := route.ReadEntryOperation(entryConnection)
	if err != nil || !running.validEntryOperation(operation) || !running.reserveRelay(raw) {
		running.mu.Lock()
		running.usage.SetupRefused++
		running.mu.Unlock()
		return
	}
	<-running.handshakes
	handshakeHeld = false
	owned = false
	if operation.Relay != nil {
		running.openRelay(workCtx, raw, entryConnection, *operation.Relay)
		return
	}
	if operation.ResolutionRelay != nil {
		running.openResolutionRelay(workCtx, raw, entryConnection, *operation.ResolutionRelay)
		return
	}
	running.openCredentialRelay(workCtx, raw, entryConnection, *operation.CredentialRelay)
}

func (running *Initiator) validEntryOperation(operation route.EntryOperation) bool {
	if operation.Relay != nil && operation.ResolutionRelay == nil {
		return running.validateSetup(*operation.Relay) == nil
	}
	if operation.Relay == nil && operation.ResolutionRelay != nil && operation.CredentialRelay == nil {
		return running.validateResolutionSetup(*operation.ResolutionRelay) == nil
	}
	if operation.Relay == nil && operation.ResolutionRelay == nil && operation.CredentialRelay != nil {
		return running.validateCredentialSetup(*operation.CredentialRelay) == nil
	}
	return false
}

func (running *Initiator) openCredentialRelay(ctx context.Context, raw, entryConnection net.Conn, setup route.CredentialRelaySetup) {
	if err := entryConnection.SetDeadline(setup.NotAfter); err != nil || route.WriteCredentialRelayReady(entryConnection, route.CredentialRelayReady{Setup: setup}) != nil {
		running.releaseRelay(raw, nil)
		return
	}
	envelope, err := route.ReadCredentialRelayEnvelope(entryConnection)
	if err != nil {
		running.releaseRelay(raw, nil)
		return
	}
	forwardCtx, cancel := context.WithDeadline(ctx, setup.NotAfter)
	defer cancel()
	client, err := credential.HTTPClient(setup.IssuerNodePublicKey, running.plan.Certificate)
	if err == nil {
		response, forwardErr := credential.ForwardOHTTP(forwardCtx, running.plan.CredentialIssuer.URL, client, envelope.OHTTP)
		client.CloseIdleConnections()
		err = forwardErr
		if err == nil {
			err = route.WriteCredentialRelayResponse(entryConnection, route.CredentialRelayResponse{OHTTP: response, Framing: route.CredentialOHTTPResponse})
		}
	}
	running.mu.Lock()
	if err == nil {
		running.usage.RelayedBytes += uint64(len(envelope.OHTTP))
		running.usage.CompletedRelays++
	}
	running.mu.Unlock()
	running.releaseRelay(raw, nil)
}

func (running *Initiator) openRelay(ctx context.Context, raw, entryConnection net.Conn, setup route.RelaySetup) {
	if err := entryConnection.SetDeadline(setup.NotAfter); err != nil {
		running.releaseRelay(raw, nil)
		return
	}
	next, err := route.OpenNodeLeg(ctx, route.NodeLegRequest{CarrierProfile: running.plan.Rendezvous.CarrierProfile, Endpoint: running.plan.Rendezvous.Endpoint,
		Certificate: running.plan.Certificate, ExpectedPeerKey: running.plan.Rendezvous.PublicKey, Deadline: setup.NotAfter,
		Binding: route.LegBinding{NetworkID: setup.NetworkID, Epoch: setup.Epoch, Digest: setup.Digest, AttachmentID: setup.AttachmentID,
			SenderRole: route.InitiatorRole, PeerRole: route.RendezvousRole, SenderNodeID: running.plan.NodeID,
			PeerNodeID: running.plan.Rendezvous.NodeID, NotAfter: setup.NotAfter}})
	if err != nil {
		running.releaseRelay(raw, nil)
		return
	}
	if err := next.SetDeadline(setup.NotAfter); err != nil || route.WriteRelayReady(entryConnection, route.RelayReady{Setup: setup}) != nil {
		running.cleanup.record(next.Close())
		running.releaseRelay(raw, nil)
		return
	}
	running.mu.Lock()
	running.active[next] = struct{}{}
	running.mu.Unlock()
	running.work.Add(1)
	go running.relay(raw, entryConnection, next)
}

func (running *Initiator) openResolutionRelay(ctx context.Context, raw, entryConnection net.Conn, setup route.ResolutionRelaySetup) {
	if err := entryConnection.SetDeadline(setup.NotAfter); err != nil || route.WriteResolutionRelayReady(entryConnection, route.ResolutionRelayReady{Setup: setup}) != nil {
		running.releaseRelay(raw, nil)
		return
	}
	envelope, err := route.ReadResolutionRelayEnvelope(entryConnection)
	if err != nil {
		running.releaseRelay(raw, nil)
		return
	}
	forwardCtx, cancel := context.WithDeadline(ctx, setup.NotAfter)
	defer cancel()
	client, err := reachability.GatewayHTTPClient(setup.GatewayNodePublicKey)
	if err == nil {
		response, forwardErr := reachability.ForwardOHTTP(forwardCtx, running.plan.ResolutionGateway.URL, client, envelope.OHTTP)
		client.CloseIdleConnections()
		err = forwardErr
		if err == nil {
			framing := route.ResolutionOHTTPResponse
			if response.Chunked {
				framing = route.ResolutionOHTTPChunkedResponse
			}
			err = route.WriteResolutionRelayResponse(entryConnection, route.ResolutionRelayResponse{OHTTP: response.Envelope, Framing: framing})
		}
	}
	running.mu.Lock()
	if err == nil {
		running.usage.RelayedBytes += uint64(len(envelope.OHTTP))
		running.usage.CompletedRelays++
	}
	running.mu.Unlock()
	running.releaseRelay(raw, nil)
}

func (running *Initiator) validateSetup(setup route.RelaySetup) error {
	if setup.NetworkID != running.plan.NetworkID || setup.Digest != running.plan.EpochDigest || setup.Epoch != running.plan.Epoch ||
		setup.TransitRole != route.InitiatorRole || setup.TransitNodeID != running.plan.NodeID ||
		setup.NextRole != route.RendezvousRole || setup.NextNodeID != running.plan.Rendezvous.NodeID ||
		setup.NextNodePublicKey != running.plan.Rendezvous.PublicKey || setup.NotAfter.After(running.plan.NotAfter) ||
		!running.plan.now().Before(setup.NotAfter) {
		return errors.New("RelaySetup is not authorized by the current Initiator duty")
	}
	return nil
}

func (running *Initiator) validateResolutionSetup(setup route.ResolutionRelaySetup) error {
	gateway := running.plan.ResolutionGateway
	if gateway.NodeID == [32]byte{} || setup.NetworkID != running.plan.NetworkID || setup.Digest != running.plan.EpochDigest ||
		setup.Epoch != running.plan.Epoch || setup.InitiatorNodeID != running.plan.NodeID ||
		setup.GatewayNodeID != gateway.NodeID || setup.GatewayNodePublicKey != gateway.PublicKey ||
		setup.NotAfter.After(running.plan.NotAfter) || !running.plan.now().Before(setup.NotAfter) {
		return errors.New("ResolutionRelaySetup is not authorized by the current Initiator duty")
	}
	return nil
}

func (running *Initiator) validateCredentialSetup(setup route.CredentialRelaySetup) error {
	issuer := running.plan.CredentialIssuer
	if issuer.NodeID == [32]byte{} || setup.NetworkID != running.plan.NetworkID || setup.Digest != running.plan.EpochDigest ||
		setup.Epoch != running.plan.Epoch || setup.InitiatorNodeID != running.plan.NodeID || setup.IssuerNodeID != issuer.NodeID ||
		setup.IssuerNodePublicKey != issuer.PublicKey || setup.IssuerProfileDigest != issuer.ProfileDigest ||
		setup.NotAfter.After(running.plan.NotAfter) || !running.plan.now().Before(setup.NotAfter) {
		return errors.New("CredentialRelaySetup is not authorized by the current Initiator duty")
	}
	return nil
}

func (running *Initiator) reserveRelay(raw net.Conn) bool {
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

func (running *Initiator) releaseRelay(raw net.Conn, next route.Carrier) {
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
