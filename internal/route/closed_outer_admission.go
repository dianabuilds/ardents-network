package route

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"net"
)

// Admit consumes no new authority. It binds the actual receiving admission
// to the inner HELLO before permitting I/O beyond the initial handshake bound.
// The caller then transfers that same lease to its forwarding owner.
func (lane *ClosedOuterBridgeLane) Admit(lease *ClosedAdmission, connection net.Conn) error {
	if lane == nil || lane.lane == nil || lease == nil {
		return errors.New("closed outer admission unavailable")
	}
	secured, ok := connection.(*tls.Conn)
	if !ok || secured == nil || secured.NetConn() != lane {
		return errors.New("closed outer admission belongs to another TLS channel")
	}
	body, err := EncodeClosedHello(lease.hello)
	if err != nil {
		return err
	}
	context := sha256.Sum256(body)
	exporter, err := ClosedRoleTLSExporter(secured)
	if err != nil {
		return err
	}
	binding, err := exporter(closedChannelExporterLabel, context[:], 32)
	if err != nil || lease.exporter == [32]byte{} || !bytes.Equal(binding, lease.exporter[:]) {
		return errors.New("closed outer admission TLS binding differs")
	}
	return lane.admitVerified(lease)
}

func (lane *ClosedOuterBridgeLane) admitVerified(lease *ClosedAdmission) error {
	inner := lane.lane
	if err := inner.bridge.handshake.admit(inner.id, lease); err != nil {
		return err
	}
	inner.deadlineMu.Lock()
	inner.authorizedUntil = lease.Deadline
	inner.deadlineMu.Unlock()
	return nil
}

func (handshake *ClosedOuterHandshake) admit(lane uint32, lease *ClosedAdmission) error {
	handshake.mu.Lock()
	defer handshake.mu.Unlock()
	child, exists := handshake.children[lane]
	now := handshake.clock().UTC()
	if handshake.duty == nil || !exists || !child.active || child.admitted || child.eof ||
		child.restriction != ClosedChildOrdinary || lease.duty == nil || lease.hello != child.hello ||
		!closedOuterAdmissionClass(lease) || lease.Bytes != closedClassBytes(lease.Class) || !now.Before(child.pendingDeadline) ||
		!now.Before(lease.Deadline) || lease.Deadline.After(child.deadline) || lease.Deadline.After(handshake.receiver.Deadline) {
		return errors.New("closed outer admission does not match live child")
	}
	child.admitted = true
	child.deadline = lease.Deadline
	handshake.children[lane] = child
	return nil
}

func closedOuterAdmissionClass(lease *ClosedAdmission) bool {
	return lease.Class == 2 && (lease.hello.Purpose == ClosedPurposeForwarding || lease.hello.Purpose == ClosedPurposeDataJoin) ||
		lease.Class == 1 && (lease.hello.Purpose == ClosedPurposeIssuer || lease.hello.Purpose == ClosedPurposeReachability || lease.hello.Purpose == ClosedPurposeSubmission) || lease.Class == 3 && lease.hello.Purpose == ClosedPurposeIntroduction
}
