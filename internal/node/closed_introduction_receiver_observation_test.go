//go:build linux

package node

import (
	"crypto/tls"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// Start the same receiver constructor used by the production Run adapter.
// Node supervision/placement and accepted State remain fixture boundaries.
func observedIntroductionStart(observed **closedIntroductionServer) func(Config) (func() error, error) {
	return func(config Config) (func() error, error) {
		resolved, err := resolveConfig(config)
		if err != nil {
			return nil, err
		}
		snapshot, err := currentFacts(resolved)
		if err != nil {
			return nil, err
		}
		server, err := newClosedIntroductionServer(resolved, snapshot)
		if err != nil {
			return nil, err
		}
		*observed = server
		return func() error {
			stopErr := server.stop()
			select {
			case <-server.drained:
				return errors.Join(stopErr, server.drainErr)
			case <-time.After(testLifecycleWait):
				return errors.New("observed Introduction receiver did not drain")
			}
		}, nil
	}
}

type introductionPendingObservation struct {
	Lane          uint32
	Nonce         [32]byte
	End           time.Time
	Acknowledged  bool
	QueuedResults int
}
type introductionSlotObservation struct {
	Request                      route.ClosedRegistrationRequest
	Active, WriterReserved, Done bool
	Next                         uint32
	Used, Maximum                uint64
	InFlight                     int
	Openings, Dispatched         [4]time.Time
	LocalAddress, RemoteAddress  string
	TLSVersion, CipherSuite      uint16
	TLSHandshakeComplete         bool
	Pending                      []introductionPendingObservation
}
type introductionReceiverObservation struct {
	Phase               string
	Receiver            route.ClosedRoleReceiver
	Active              uint32
	ReservedConnections int
	Slots               []introductionSlotObservation
}

// Snapshot slot-owned mutable fields under their actual lock. TLS exposes only
// its public connection state here; this does not claim a complete TLS heap,
// transient admission-channel state, or a whole-role P3 verdict.
func observeIntroductionReceiver(t *testing.T, server *closedIntroductionServer, phase string) introductionReceiverObservation {
	t.Helper()
	server.slotsMu.Lock()
	defer server.slotsMu.Unlock()
	snapshot := introductionReceiverObservation{Phase: phase, Receiver: server.receiver, Active: server.active.Load(), ReservedConnections: len(server.capacity)}
	for _, slot := range server.slots {
		item := introductionSlotObservation{Request: slot.request, Active: slot.active, WriterReserved: len(slot.writer) > 0,
			Next: slot.next, Used: slot.used, Maximum: slot.maximum, InFlight: slot.inFlight, Openings: slot.openings, Dispatched: slot.dispatched,
			LocalAddress: slot.connection.LocalAddr().String(), RemoteAddress: slot.connection.RemoteAddr().String()}
		select {
		case <-slot.done:
			item.Done = true
		default:
		}
		secured, ok := slot.connection.(*tls.Conn)
		if !ok {
			t.Fatal("receiving registration lost actual role TLS")
		}
		state := secured.ConnectionState()
		item.TLSVersion, item.CipherSuite, item.TLSHandshakeComplete = state.Version, state.CipherSuite, state.HandshakeComplete
		for lane, pending := range slot.pending {
			item.Pending = append(item.Pending, introductionPendingObservation{lane, pending.nonce, pending.end, pending.acknowledged, len(pending.result)})
		}
		sort.Slice(item.Pending, func(i, j int) bool { return item.Pending[i].Lane < item.Pending[j].Lane })
		snapshot.Slots = append(snapshot.Slots, item)
	}
	sort.Slice(snapshot.Slots, func(i, j int) bool {
		return string(snapshot.Slots[i].Request.Slot[:]) < string(snapshot.Slots[j].Request.Slot[:])
	})
	return snapshot
}
