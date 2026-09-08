package route

import (
	"crypto/sha256"
	"errors"
	"sync"
	"time"
)

const closedChannelExporterLabel = "EXPORTER-ardents-channel-v3"

// ClosedRoleReceiver is the exact current public State projection for one
// role TLS receiver. It authorizes no peer-selected destination or profile.
type ClosedRoleReceiver struct {
	NetworkID, StateGeneration, StateDigest, ProfileDigest, NodeID, RecordDigest [32]byte
	DutyGeneration                                                               uint64
	RoleDomain, Subrole                                                          uint8
	ExpectedPurpose                                                              ClosedPurpose
	NotAfter                                                                     time.Time
}

// ClosedTLSExporter derives secret channel binding bytes after role TLS has
// already authenticated its exact server key. Route retains the result but
// never serializes or forwards it.
type ClosedTLSExporter func(string, []byte, int) ([]byte, error)

// ClosedAdmissionVerification is the narrow credential-owner callback. Route
// supplies only public receiver facts, token bytes and local exporter binding;
// the callback selects no destination and returns only the verified hour.
type ClosedAdmissionVerification struct {
	Hello    ClosedHello
	Class    uint8
	Token    []byte
	Exporter [32]byte
}

type ClosedAdmissionVerifier func(ClosedAdmissionVerification) (time.Time, error)

// ClosedAdmission is the finite result of initial receiver admission. It
// retains no token, holder, permission, Target or exporter bytes.
type ClosedAdmission struct {
	Class    uint8
	Deadline time.Time
	Bytes    uint64
	duty     *closedDutyChannel
}

// Release returns an admitted channel reservation when its owner performed no
// forwarding handoff. A forwarding channel takes the same reservation and
// releases it from Cancel after joining all children.
func (admission *ClosedAdmission) Release() {
	if admission == nil {
		return
	}
	admission.duty.release()
	admission.duty = nil
}

// ClosedAdmissionChannel owns lane-zero receiver admission on one fresh
// role TLS channel. It rejects every work frame until a token is verified,
// durably burned and bounded capacity has been reserved.
type ClosedAdmissionChannel struct {
	mu       sync.Mutex
	receiver ClosedRoleReceiver
	spends   *ClosedSpendLedger
	limits   *ClosedDutyLimits
	exporter ClosedTLSExporter
	verify   ClosedAdmissionVerifier
	clock    func() time.Time
	hello    ClosedHello
	binding  [32]byte
	admitted bool
}

// NewClosedAdmissionChannel creates one unauthenticated receiver state. The
// caller must bind it to the just-handshaken TLS exporter; a zero or missing
// exporter cannot fall back to an unauthenticated lane.
func NewClosedAdmissionChannel(receiver ClosedRoleReceiver, spends *ClosedSpendLedger, limits *ClosedDutyLimits, exporter ClosedTLSExporter, verify ClosedAdmissionVerifier, clock func() time.Time) (*ClosedAdmissionChannel, error) {
	if !validClosedRoleReceiver(receiver) || spends == nil || limits == nil || exporter == nil || verify == nil || clock == nil || clock().IsZero() {
		return nil, errors.New("closed admission channel is invalid")
	}
	return &ClosedAdmissionChannel{receiver: receiver, spends: spends, limits: limits, exporter: exporter, verify: verify, clock: clock}, nil
}

// Accept processes only HELLO then initial lane-zero ADMIT. It returns an
// empty admission after HELLO and a finite result after ADMIT. Any other
// frame, duplicate HELLO or prior error is unavailable before forwarding.
func (channel *ClosedAdmissionChannel) Accept(frame ClosedLaneFrame) (ClosedAdmission, error) {
	if channel == nil {
		return ClosedAdmission{}, errors.New("closed admission channel is unavailable")
	}
	channel.mu.Lock()
	defer channel.mu.Unlock()
	if !channel.clock().UTC().Before(channel.receiver.NotAfter) {
		return ClosedAdmission{}, errors.New("closed admission channel is unavailable")
	}
	if channel.hello == (ClosedHello{}) {
		return channel.acceptHello(frame)
	}
	if channel.admitted || frame.Kind != closedFrameAdmit || frame.Lane != 0 {
		return ClosedAdmission{}, errors.New("closed admission frame is unavailable")
	}
	return channel.acceptInitialAdmit(frame.Body)
}

func (channel *ClosedAdmissionChannel) acceptHello(frame ClosedLaneFrame) (ClosedAdmission, error) {
	if frame.Kind != closedFrameHello || frame.Lane != 0 {
		return ClosedAdmission{}, errors.New("closed admission HELLO is required")
	}
	hello, err := DecodeClosedHello(frame.Body)
	if err != nil || !channel.matchesHello(hello) {
		return ClosedAdmission{}, errors.New("closed admission HELLO is unavailable")
	}
	context := sha256.Sum256(frame.Body)
	raw, err := channel.exporter(closedChannelExporterLabel, context[:], 32)
	if err != nil || len(raw) != len(channel.binding) {
		return ClosedAdmission{}, errors.New("closed admission TLS exporter is unavailable")
	}
	copy(channel.binding[:], raw)
	if channel.binding == [32]byte{} {
		return ClosedAdmission{}, errors.New("closed admission TLS exporter is unavailable")
	}
	channel.hello = hello
	return ClosedAdmission{}, nil
}

func (channel *ClosedAdmissionChannel) acceptInitialAdmit(body []byte) (ClosedAdmission, error) {
	class, token, err := decodeClosedAdmit(body)
	if err != nil {
		return ClosedAdmission{}, err
	}
	releaseVerification, err := channel.limits.BeginVerification()
	if err != nil {
		return ClosedAdmission{}, errors.New("closed admission token is unavailable")
	}
	defer releaseVerification()
	window, err := channel.verify(ClosedAdmissionVerification{Hello: channel.hello, Class: class, Token: token, Exporter: channel.binding})
	if err != nil || !validClosedSpendWindow(window) {
		return ClosedAdmission{}, errors.New("closed admission token is unavailable")
	}
	now := channel.clock().UTC()
	reservation, err := channel.limits.reserveChannel()
	if err != nil {
		return ClosedAdmission{}, errors.New("closed admission capacity is unavailable")
	}
	if err := channel.spends.Spend(token, window, now); err != nil {
		reservation.release()
		return ClosedAdmission{}, errors.New("closed admission token is unavailable")
	}
	lease := ClosedAdmission{Class: class, Bytes: closedClassBytes(class), Deadline: now.Add(closedClassLifetime(class))}
	if channel.hello.Deadline.Before(lease.Deadline) {
		lease.Deadline = channel.hello.Deadline
	}
	if channel.receiver.NotAfter.Before(lease.Deadline) {
		lease.Deadline = channel.receiver.NotAfter
	}
	if !now.Before(lease.Deadline) {
		reservation.release()
		return ClosedAdmission{}, errors.New("closed admission lease is unavailable")
	}
	lease.duty = reservation
	channel.admitted = true
	return lease, nil
}

func decodeClosedAdmit(body []byte) (uint8, []byte, error) {
	if len(body) != 355 || body[0] < 1 || body[0] > 3 {
		return 0, nil, errors.New("closed admission token is invalid")
	}
	return body[0], append([]byte(nil), body[1:]...), nil
}

func (channel *ClosedAdmissionChannel) matchesHello(hello ClosedHello) bool {
	return hello.NetworkID == channel.receiver.NetworkID && hello.StateGeneration == channel.receiver.StateGeneration && hello.StateDigest == channel.receiver.StateDigest &&
		hello.ProfileDigest == channel.receiver.ProfileDigest && hello.RecipientNodeID == channel.receiver.NodeID && hello.RecipientDutyGeneration == channel.receiver.DutyGeneration &&
		hello.Purpose == channel.receiver.ExpectedPurpose && !hello.Deadline.After(channel.receiver.NotAfter) && channel.clock().UTC().Before(hello.Deadline)
}

func validClosedRoleReceiver(receiver ClosedRoleReceiver) bool {
	return receiver.NetworkID != [32]byte{} && receiver.StateGeneration != [32]byte{} && receiver.StateDigest != [32]byte{} && receiver.ProfileDigest != [32]byte{} &&
		receiver.NodeID != [32]byte{} && receiver.RecordDigest != [32]byte{} && receiver.DutyGeneration != 0 &&
		ClosedPurposePermitsDuty(receiver.ExpectedPurpose, receiver.RoleDomain, receiver.Subrole) && !receiver.NotAfter.IsZero() && receiver.NotAfter == receiver.NotAfter.UTC().Truncate(time.Second)
}

func closedClassBytes(class uint8) uint64 {
	switch class {
	case 1:
		return 64 << 10
	case 2:
		return 32 << 20
	case 3:
		return 1 << 20
	}
	return 0
}

func closedClassLifetime(class uint8) time.Duration {
	switch class {
	case 1:
		return 30 * time.Second
	case 2:
		return 1800 * time.Second
	case 3:
		return 600 * time.Second
	}
	return 0
}
