package node

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// Introduction owns one live-slot listener. It forwards exact sealed bytes;
// it cannot decrypt them, select a peer, or create a Publisher-side leg.
type Introduction struct {
	plan       introductionPlan
	listener   net.Listener
	handshakes chan struct{}
	slotsCap   chan struct{}
	deliveries chan struct{}

	mu        sync.Mutex
	draining  bool
	protected bool
	pre       map[net.Conn]struct{}
	slots     map[[32]byte]*introductionLiveSlot
	active    map[net.Conn]struct{}
	usage     IntroductionUsage
	stopOnce  sync.Once
	cleanup   terminalCleanup
	work      sync.WaitGroup
	terminal  chan error
}

type introductionLiveSlot struct {
	registration route.IntroductionSlotRegistration
	raw          net.Conn
	connection   net.Conn
	spent        bool
}

// StartIntroduction binds one State-authorized Introduction endpoint. It does
// not discover Publishers, retain offline messages, or expose a Service port.
func StartIntroduction(input IntroductionConfig) (*Introduction, error) {
	plan, err := newIntroductionPlan(input)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", plan.ListenAddress)
	if err != nil {
		return nil, err
	}
	running := &Introduction{plan: plan, listener: listener, handshakes: make(chan struct{}, plan.HandshakeLimit),
		slotsCap: make(chan struct{}, plan.SlotLimit), deliveries: make(chan struct{}, plan.DeliveryLimit),
		pre: make(map[net.Conn]struct{}), slots: make(map[[32]byte]*introductionLiveSlot), active: make(map[net.Conn]struct{}), terminal: make(chan error, 1)}
	running.work.Add(1)
	go running.accept()
	return running, nil
}

func (running *Introduction) Done() <-chan error {
	if running == nil {
		return nil
	}
	return running.terminal
}

// Protect refuses only new handshakes and closes pre-control work. Registered
// slots remain alive until their own deadline or deliberate drain.
func (running *Introduction) Protect(value bool) {
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

func (running *Introduction) Stop() {
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

func (running *Introduction) Usage() IntroductionUsage {
	if running == nil {
		return IntroductionUsage{}
	}
	running.mu.Lock()
	defer running.mu.Unlock()
	result := running.usage
	result.Handshakes, result.Slots, result.Deliveries = uint16(len(running.handshakes)), uint16(len(running.slots)), uint16(len(running.deliveries))
	result.Connections = uint16(len(running.pre) + len(running.active))
	return result
}

func (running *Introduction) accept() {
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

func (running *Introduction) reserveHandshake(raw net.Conn) bool {
	running.mu.Lock()
	defer running.mu.Unlock()
	if running.draining || running.protected {
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

func (running *Introduction) handle(raw net.Conn) {
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
		Digest: running.plan.EpochDigest, TransitNodeID: running.plan.NodeID, Epoch: running.plan.Epoch, TransitRole: route.IntroductionRole,
		Deadline: running.plan.NotAfter, AdmissionDeadline: deadline, Certificate: running.plan.Certificate, Admit: running.plan.Admit})
	admissionCancel()
	if err != nil || accepted.Connection.SetDeadline(running.plan.NotAfter) != nil {
		return
	}
	record, err := route.ReadIntroductionControlRecord(accepted.Connection)
	if err != nil {
		return
	}
	if record.Registration != nil {
		if !running.register(raw, accepted.Connection, accepted.Binding, *record.Registration) {
			return
		}
		<-running.handshakes
		handshakeHeld = false
		owned = false
		if err := route.WriteIntroductionSlotReady(accepted.Connection, route.IntroductionSlotReady{Reachability: record.Registration.Reachability, JoinHandle: record.Registration.JoinHandle, NotAfter: record.Registration.NotAfter}); err != nil {
			running.releaseSlot(record.Registration.Reachability)
			return
		}
		if err := accepted.Connection.SetDeadline(record.Registration.NotAfter); err != nil {
			running.releaseSlot(record.Registration.Reachability)
			return
		}
		running.work.Add(1)
		go running.watchSlot(record.Registration.Reachability, raw, accepted.Connection)
		return
	}
	if record.Sealed != nil {
		running.submit(accepted.Connection, accepted.Binding.AttachmentID, *record.Sealed, record.Raw)
	}
}

func (running *Introduction) register(raw, connection net.Conn, binding route.EndpointTransitBinding, registration route.IntroductionSlotRegistration) bool {
	running.mu.Lock()
	defer running.mu.Unlock()
	if running.draining || running.protected || registration.NotAfter.After(running.plan.NotAfter) || !running.plan.now().Before(registration.NotAfter) ||
		binding.NotAfter != registration.NotAfter || running.slots[registration.Reachability] != nil {
		return false
	}
	select {
	case running.slotsCap <- struct{}{}:
		delete(running.pre, raw)
		running.active[raw] = struct{}{}
		running.slots[registration.Reachability] = &introductionLiveSlot{registration: registration, raw: raw, connection: connection}
		running.usage.Registered++
		return true
	default:
		return false
	}
}

func (running *Introduction) submit(connection net.Conn, attachment [32]byte, sealed route.SealedIntroduction, raw []byte) {
	available := false
	running.mu.Lock()
	if !running.draining && !running.protected && sealed.NetworkID == running.plan.NetworkID && sealed.Digest == running.plan.EpochDigest && sealed.Epoch == running.plan.Epoch &&
		sealed.IntroductionNodeID == running.plan.NodeID {
		if slot := running.slots[sealed.Reachability]; slot != nil && !slot.spent && slot.registration.JoinHandle == sealed.JoinHandle && slot.registration.NotAfter.Equal(sealed.NotAfter) && running.plan.now().Before(sealed.NotAfter) {
			select {
			case running.deliveries <- struct{}{}:
				slot.spent = true
				available = true
				running.work.Add(1)
				go running.forward(slot, sealed.Reachability, raw)
			default:
			}
		}
	}
	if !available {
		running.usage.Unavailable++
	}
	running.mu.Unlock()
	outcome := route.IntroductionUnavailable
	if available {
		outcome = route.IntroductionDelivered
	}
	_ = route.WriteIntroductionDeliveryResult(connection, route.IntroductionDeliveryResult{AttachmentID: attachment, Outcome: outcome})
}

func (running *Introduction) forward(slot *introductionLiveSlot, reachability [32]byte, raw []byte) {
	defer running.work.Done()
	_ = slot.connection.SetWriteDeadline(slot.registration.NotAfter)
	err := writeRouteBytes(slot.connection, raw)
	running.cleanup.record(slot.connection.Close())
	running.mu.Lock()
	<-running.deliveries
	if err == nil {
		running.usage.Delivered++
	} else {
		running.usage.Unavailable++
	}
	running.mu.Unlock()
	running.releaseSlot(reachability)
}

func (running *Introduction) watchSlot(reachability [32]byte, raw, connection net.Conn) {
	defer running.work.Done()
	buffer := make([]byte, 1)
	_, _ = connection.Read(buffer)
	running.cleanup.record(raw.Close())
	running.releaseSlot(reachability)
}

func (running *Introduction) releaseSlot(reachability [32]byte) {
	running.mu.Lock()
	slot := running.slots[reachability]
	if slot != nil {
		delete(running.slots, reachability)
		<-running.slotsCap
		delete(running.active, slot.raw)
	}
	running.mu.Unlock()
}

func writeRouteBytes(writer net.Conn, value []byte) error {
	for len(value) > 0 {
		count, err := writer.Write(value)
		if err != nil {
			return err
		}
		if count <= 0 || count > len(value) {
			return errors.New("Introduction delivery write is short")
		}
		value = value[count:]
	}
	return nil
}

// Drain closes admission and every live slot, then joins all TLS/control work
// inside the duty's declared drain bound.
func (running *Introduction) Drain(ctx context.Context) error {
	if running == nil || ctx == nil {
		return errors.New("Introduction duty is unavailable")
	}
	running.Stop()
	running.mu.Lock()
	connections := make([]net.Conn, 0, len(running.pre)+len(running.active))
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
		return errors.Join(running.cleanup.result(), errors.New("Introduction drain exceeded its Work Safety Lease"))
	}
}

func (running *Introduction) Close() error { return running.Drain(context.Background()) }
