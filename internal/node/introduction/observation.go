package introduction

import (
	"crypto/tls"
	"errors"
	"sort"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/terminal"
)

// PendingObservation describes one admitted delivery awaiting its result.
type PendingObservation struct {
	Lane          uint32
	Nonce         [32]byte
	End           time.Time
	Acknowledged  bool
	QueuedResults int
}

// SlotObservation is a copy of one retained registration's bounded state.
type SlotObservation struct {
	Request                      terminal.RegistrationRequest
	Active, WriterReserved, Done bool
	Next                         uint32
	Used, Maximum                uint64
	InFlight                     int
	Openings, Dispatched         [4]time.Time
	LocalAddress, RemoteAddress  string
	TLSVersion, CipherSuite      uint16
	TLSHandshakeComplete         bool
	Pending                      []PendingObservation
}

// Observation is a read-only view of the receiving role for qualification.
// It does not claim a complete TLS heap or transient admission-channel state.
type Observation struct {
	Phase               string
	Receiver            route.ClosedRoleReceiver
	Active              uint32
	ReservedConnections int
	Slots               []SlotObservation
}

// Observe copies slot-owned mutable fields while holding their actual lock.
func (server *Server) Observe(phase string) (Observation, error) {
	server.slotsMu.Lock()
	defer server.slotsMu.Unlock()
	snapshot := Observation{Phase: phase, Receiver: server.receiver, Active: server.active.Load(), ReservedConnections: len(server.capacity)}
	for _, slot := range server.slots {
		item := SlotObservation{Request: slot.request, Active: slot.active, WriterReserved: len(slot.writer) > 0,
			Next: slot.next, Used: slot.used, Maximum: slot.maximum, InFlight: slot.inFlight, Openings: slot.openings, Dispatched: slot.dispatched,
			LocalAddress: slot.connection.LocalAddr().String(), RemoteAddress: slot.connection.RemoteAddr().String()}
		select {
		case <-slot.done:
			item.Done = true
		default:
		}
		secured, ok := slot.connection.(*tls.Conn)
		if !ok {
			return Observation{}, errors.New("receiving registration lost actual role TLS")
		}
		state := secured.ConnectionState()
		item.TLSVersion, item.CipherSuite, item.TLSHandshakeComplete = state.Version, state.CipherSuite, state.HandshakeComplete
		for lane, pending := range slot.pending {
			item.Pending = append(item.Pending, PendingObservation{lane, pending.nonce, pending.end, pending.acknowledged, len(pending.result)})
		}
		sort.Slice(item.Pending, func(i, j int) bool { return item.Pending[i].Lane < item.Pending[j].Lane })
		snapshot.Slots = append(snapshot.Slots, item)
	}
	sort.Slice(snapshot.Slots, func(i, j int) bool {
		return string(snapshot.Slots[i].Request.Slot[:]) < string(snapshot.Slots[j].Request.Slot[:])
	})
	return snapshot, nil
}
