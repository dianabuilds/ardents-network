package route

import (
	"crypto/sha256"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/replay"
)

// ClosedIntroductionRegistrationByteLimit is the complete bidirectional
// protocol allowance of one admitted Publication registration. It covers the
// registration exchange, retained delivery frames and reserved withdrawal.
// Eight MiB admits the fixed 256-Connection closed-alpha Publisher workload
// while keeping every registration finite and independently accounted.
const ClosedIntroductionRegistrationByteLimit = uint64(8 << 20)

const closedChannelExporterLabel = "EXPORTER-ardents-channel-v3"

// HELLO and ADMIT have already arrived when their admission is transferred.
const closedAdmissionFrameBytes = 2*ardp.HeaderSize + 209 + 355

// ClosedRoleReceiver is the exact current public State projection for one
// role TLS receiver. It authorizes no peer-selected destination or profile.
type ClosedRoleReceiver struct {
	NetworkID, StateGeneration, StateDigest, ProfileDigest, NodeID, RecordDigest [32]byte
	DutyGeneration                                                               uint64
	RoleDomain, Subrole                                                          uint8
	ExpectedPurpose                                                              ardp.Purpose
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
	Hello    ardp.Hello
	Class    uint8
	Token    []byte
	Exporter [32]byte
	Deadline time.Time
}

// ClosedAdmissionApproval contains the verified token hour plus an optional
// release for capacity reserved before Route spends that token. Route calls
// the release when it cannot transfer the admission to its resulting owner.
type ClosedAdmissionApproval struct {
	Window  time.Time
	Release func() error
}

type ClosedAdmissionVerifier func(ClosedAdmissionVerification) (ClosedAdmissionApproval, error)

// ClosedAdmission is the finite result of initial receiver admission. It
// retains only private HELLO/exporter binding until its receiving owner transfers
// the reservation; no token, holder, permission or Target survives admission.
type ClosedAdmission struct {
	Class    uint8
	Deadline time.Time
	Bytes    uint64
	hello    ardp.Hello
	exporter [32]byte
	claim    *closedAdmissionClaim
}

// closedAdmissionClaim is the one-use private reservation shared by every
// copied admission handle. It belongs either to one Release caller or to one
// forwarding owner, never to both.
type closedAdmissionClaim struct {
	mu      sync.Mutex
	duty    *closedDutyChannel
	release func() error
}

func newClosedAdmissionClaim(duty *closedDutyChannel, release func() error) *closedAdmissionClaim {
	return &closedAdmissionClaim{duty: duty, release: release}
}

func (claim *closedAdmissionClaim) live() bool {
	if claim == nil {
		return false
	}
	claim.mu.Lock()
	defer claim.mu.Unlock()
	return claim.duty != nil
}

func (claim *closedAdmissionClaim) transfer() (*closedDutyChannel, func() error, bool) {
	if claim == nil {
		return nil, nil, false
	}
	claim.mu.Lock()
	defer claim.mu.Unlock()
	if claim.duty == nil {
		return nil, nil, false
	}
	duty, release := claim.duty, claim.release
	claim.duty, claim.release = nil, nil
	return duty, release, true
}

func (claim *closedAdmissionClaim) transferChildFor(limits *ClosedDutyLimits) (*closedDutyChannel, func() error, bool) {
	if claim == nil || limits == nil {
		return nil, nil, false
	}
	claim.mu.Lock()
	defer claim.mu.Unlock()
	if claim.duty == nil || claim.duty.limits != limits || claim.duty.reserveChild() != nil {
		return nil, nil, false
	}
	duty, release := claim.duty, claim.release
	claim.duty, claim.release = nil, nil
	return duty, release, true
}

func (claim *closedAdmissionClaim) releaseReservation() error {
	duty, release, transferred := claim.transfer()
	if !transferred {
		return nil
	}
	duty.release()
	if release == nil {
		return nil
	}
	return release()
}

// Release returns an admitted channel reservation when its owner performed no
// forwarding handoff. A forwarding channel takes the same reservation and
// releases it from Cancel after joining all children.
func (admission *ClosedAdmission) Release() error {
	if admission == nil {
		return nil
	}
	if admission.claim == nil {
		return nil
	}
	return admission.claim.releaseReservation()
}

// ClosedAdmissionChannel owns lane-zero receiver admission on one fresh
// role TLS channel. It rejects every work frame until a token is verified,
// durably burned and bounded capacity has been reserved.
type ClosedAdmissionChannel struct {
	mu       sync.Mutex
	receiver ClosedRoleReceiver
	spends   *replay.Ledger
	limits   *ClosedDutyLimits
	exporter ClosedTLSExporter
	verify   ClosedAdmissionVerifier
	clock    func() time.Time
	hello    ardp.Hello
	binding  [32]byte
	admitted bool
}

// NewClosedAdmissionChannel creates one unauthenticated receiver state. The
// caller must bind it to the just-handshaken TLS exporter; a zero or missing
// exporter cannot fall back to an unauthenticated lane.
func NewClosedAdmissionChannel(receiver ClosedRoleReceiver, spends *replay.Ledger, limits *ClosedDutyLimits, exporter ClosedTLSExporter, verify ClosedAdmissionVerifier, clock func() time.Time) (*ClosedAdmissionChannel, error) {
	if !validClosedRoleReceiver(receiver) || spends == nil || limits == nil || exporter == nil || verify == nil || clock == nil || clock().IsZero() {
		return nil, errors.New("closed admission channel is invalid")
	}
	return &ClosedAdmissionChannel{receiver: receiver, spends: spends, limits: limits, exporter: exporter, verify: verify, clock: clock}, nil
}

// Accept processes only HELLO then initial lane-zero ADMIT. It returns an
// empty admission after HELLO and a finite result after ADMIT. Any other
// frame, duplicate HELLO or prior error is unavailable before forwarding.
func (channel *ClosedAdmissionChannel) Accept(frame ardp.Frame) (ClosedAdmission, error) {
	if channel == nil {
		return ClosedAdmission{}, errors.New("closed admission channel is unavailable")
	}
	channel.mu.Lock()
	defer channel.mu.Unlock()
	if !channel.clock().UTC().Before(channel.receiver.NotAfter) {
		return ClosedAdmission{}, errors.New("closed admission channel is unavailable")
	}
	if channel.hello == (ardp.Hello{}) {
		return channel.acceptHello(frame)
	}
	if channel.admitted || frame.Kind != ardp.KindAdmit || frame.Lane != 0 {
		return ClosedAdmission{}, errors.New("closed admission frame is unavailable")
	}
	return channel.acceptInitialAdmit(frame.Body)
}

func (channel *ClosedAdmissionChannel) acceptHello(frame ardp.Frame) (ClosedAdmission, error) {
	if frame.Kind != ardp.KindHello || frame.Lane != 0 {
		return ClosedAdmission{}, errors.New("closed admission HELLO is required")
	}
	hello, err := ardp.DecodeHello(frame.Body)
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
	now := channel.clock().UTC()
	deadline := now.Add(closedClassLifetime(class))
	if channel.hello.Deadline.Before(deadline) {
		deadline = channel.hello.Deadline
	}
	if channel.receiver.NotAfter.Before(deadline) {
		deadline = channel.receiver.NotAfter
	}
	approval, err := channel.verify(ClosedAdmissionVerification{Hello: channel.hello, Class: class, Token: token, Exporter: channel.binding, Deadline: deadline})
	if err != nil || !replay.ValidWindow(approval.Window) {
		return ClosedAdmission{}, errors.New("closed admission token is unavailable")
	}
	releaseApproval := func() error {
		if approval.Release == nil {
			return nil
		}
		return approval.Release()
	}
	now = channel.clock().UTC()
	reservation, err := channel.limits.reserveChannel()
	if err != nil {
		return ClosedAdmission{}, errors.Join(errors.New("closed admission capacity is unavailable"), releaseApproval())
	}
	if err := channel.spends.Spend(token, approval.Window, now); err != nil {
		reservation.release()
		return ClosedAdmission{}, errors.Join(errors.New("closed admission token is unavailable"), releaseApproval())
	}
	lease := ClosedAdmission{hello: channel.hello, exporter: channel.binding, Class: class, Bytes: closedClassBytes(class), Deadline: deadline}
	if !now.Before(lease.Deadline) {
		reservation.release()
		return ClosedAdmission{}, errors.Join(errors.New("closed admission lease is unavailable"), releaseApproval())
	}
	lease.claim = newClosedAdmissionClaim(reservation, releaseApproval)
	channel.admitted = true
	return lease, nil
}

func decodeClosedAdmit(body []byte) (uint8, []byte, error) {
	if len(body) != 355 || body[0] < 1 || body[0] > 3 {
		return 0, nil, errors.New("closed admission token is invalid")
	}
	return body[0], append([]byte(nil), body[1:]...), nil
}

func (channel *ClosedAdmissionChannel) matchesHello(hello ardp.Hello) bool {
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
		return ClosedIntroductionRegistrationByteLimit
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
