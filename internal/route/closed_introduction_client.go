//go:build linux

package route

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

// ClosedIntroductionRegistration owns one channel-bound slot. Done means the
// registration ended, never that a Service is ready or a Descriptor published.
// Close joins its reader and transport cleanup; it cannot reopen the slot.
type ClosedIntroductionRegistration struct {
	deliveries       chan *ClosedIntroductionDelivery
	available        chan struct{}
	pending          map[uint32]*ClosedIntroductionDelivery
	writer           chan struct{}
	lastDelivery     uint32
	used             uint64
	openings         [4]time.Time
	receipt          [32]byte
	prefix           *ClosedSourcePrefix
	lane             *closedSourceLane
	connection       net.Conn
	request          ClosedRegistrationRequest
	stop             func() bool
	interrupted      chan struct{}
	done             chan struct{}
	mu               sync.Mutex
	withdraw         [32]byte
	outcome, cleanup error
}

// IntroductionRecipient returns current public duty facts for token issuance.
// The caller gets no unverified address, transport key or arbitrary peer choice.
func (prefix *ClosedSourcePrefix) IntroductionRecipient() ([32]byte, time.Time, error) {
	peer, err := prefix.introductionPeer()
	if err != nil {
		return [32]byte{}, time.Time{}, err
	}
	end := peer.notAfter
	if prefix.plan.deadline.Before(end) {
		end = prefix.plan.deadline
	}
	return peer.node, end, nil
}

func (prefix *ClosedSourcePrefix) introductionPeer() (closedBootstrapPeer, error) {
	if prefix == nil || prefix.plan.domain != closedRoleDomainIntroduction {
		return closedBootstrapPeer{}, errors.New("registration requires Publisher Introduction role")
	}
	return prefix.terminalPeer(ClosedPurposeIntroduction)
}

func (prefix *ClosedSourcePrefix) RegisterIntroduction(ctx context.Context, present ClosedTokenPresenter, request ClosedRegistrationRequest) (registration *ClosedIntroductionRegistration, outcome error) {
	if ctx == nil || ctx.Err() != nil || present == nil || request.Withdraw {
		return nil, errors.New("closed Introduction registration unavailable")
	}
	peer, err := prefix.introductionPeer()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if !now.Before(request.Expiry) || request.Expiry.After(now.Add(600*time.Second)) || request.Expiry.After(peer.notAfter) || request.Expiry.After(prefix.plan.deadline) {
		return nil, errors.New("closed Introduction expiry unavailable")
	}
	if _, err := rand.Read(request.Nonce[:]); err != nil {
		return nil, err
	}
	operation, err := EncodeClosedRegistrationRequest(request)
	if err != nil {
		return nil, err
	}
	pending := now.Add(10 * time.Second)
	if request.Expiry.Before(pending) {
		pending = request.Expiry
	}
	lane, err := prefix.channels.open(ctx, ClosedOpen{NextNodeID: peer.node, NextDutyGeneration: peer.generation, Purpose: ClosedPurposeIntroduction, Deadline: request.Expiry}, pending)
	if err != nil {
		return nil, err
	}
	owner := &ClosedIntroductionRegistration{deliveries: make(chan *ClosedIntroductionDelivery, 16), available: make(chan struct{}, 1), pending: make(map[uint32]*ClosedIntroductionDelivery), writer: make(chan struct{}, 1), prefix: prefix, lane: lane, request: request, interrupted: make(chan struct{}), done: make(chan struct{})}
	owner.stop = context.AfterFunc(ctx, func() { defer close(owner.interrupted); _ = lane.Close() })
	handedOver := false
	defer func() {
		if !handedOver {
			outcome = errors.Join(outcome, owner.closeTransport())
		}
	}()
	secured, err := OpenClosedRoleTLS(ctx, lane, peer.key, pending)
	if err != nil {
		return nil, err
	}
	owner.connection = secured
	hello := ClosedHello{NetworkID: prefix.plan.profile.NetworkID, StateGeneration: prefix.plan.profile.StateGeneration, StateDigest: prefix.plan.profile.StateDigest,
		ProfileDigest: prefix.plan.profile.Digest, RecipientNodeID: peer.node, RecipientDutyGeneration: peer.generation, Purpose: ClosedPurposeIntroduction, Deadline: request.Expiry}
	if _, err := rand.Read(hello.ChannelNonce[:]); err != nil {
		return nil, err
	}
	body, err := EncodeClosedHello(hello)
	if err != nil {
		return nil, err
	}
	if err := WriteClosedLaneFrame(secured, ClosedLaneFrame{Kind: closedFrameHello, Body: body}); err != nil {
		return nil, err
	}
	current, err := prefix.introductionPeer()
	if err != nil || current != peer {
		return nil, errors.New("closed Introduction duty changed before admission")
	}
	token, err := present(hello, 3)
	if err != nil {
		clear(token)
		return nil, err
	}
	defer clear(token)
	if len(token) != 354 {
		return nil, errors.New("closed Introduction Publication token invalid")
	}
	admit := append([]byte{3}, token...)
	defer clear(admit)
	if err := WriteClosedLaneFrame(secured, ClosedLaneFrame{Kind: closedFrameAdmit, Body: admit}); err != nil {
		return nil, err
	}
	accepted, err := ReadClosedLaneFrame(secured)
	if err != nil {
		return nil, err
	}
	if status, credit, err := DecodeClosedAcceptFrame(accepted); err != nil || status != 0 || credit != 64<<10 {
		return nil, errors.New("closed Introduction Publication admission refused")
	}
	if err := lane.activate(); err != nil {
		return nil, err
	}
	if err := WriteClosedLaneFrame(secured, ClosedLaneFrame{Kind: closedFrameOperation, Body: operation}); err != nil {
		return nil, err
	}
	frame, err := ReadClosedLaneFrame(secured)
	if err != nil || frame.Kind != closedFrameResult || frame.Lane != 0 {
		return nil, errors.Join(errors.New("closed Introduction registration result unavailable"), err)
	}
	if status, proof, err := DecodeClosedDescriptorResult(frame.Body, request.Nonce); err != nil || status != 0 || len(proof) != 0 {
		return nil, errors.New("closed Introduction registration refused")
	}
	current, err = prefix.introductionPeer()
	if err != nil || current != peer || ctx.Err() != nil {
		return nil, errors.New("closed Introduction duty changed before registration completed")
	}
	if err := secured.SetDeadline(request.Expiry); err != nil {
		return nil, err
	}
	owner.receipt = sha256.Sum256(frame.Body)
	owner.used = uint64(16+len(body)+16+len(admit)+16+len(accepted.Body)+16+len(operation)+16+len(frame.Body)) + closedIntroductionWithdrawCost
	handedOver = true
	go owner.read()
	return owner, nil
}

func (owner *ClosedIntroductionRegistration) read() {
	var err error
	withdrawn := false
	for err == nil && !withdrawn {
		var frame ClosedLaneFrame
		frame, err = ReadClosedLaneFrame(owner.connection)
		if err != nil {
			break
		}
		owner.mu.Lock()
		if frame.Lane != 0 {
			err = owner.receiveDelivery(frame)
		} else if owner.withdraw == [32]byte{} || frame.Kind != closedFrameResult {
			err = errors.New("closed Introduction registration received unexpected operation")
		} else {
			status, proof, decodeErr := DecodeClosedDescriptorResult(frame.Body, owner.withdraw)
			if decodeErr != nil || status != 0 || len(proof) != 0 {
				err = errors.New("closed Introduction withdrawal refused")
			} else {
				withdrawn = true
			}
		}
		owner.mu.Unlock()
	}
	if err == nil {
		var trailing [1]byte
		if count, terminalErr := owner.connection.Read(trailing[:]); count != 0 || !errors.Is(terminalErr, io.EOF) {
			err = errors.New("closed Introduction withdrawal did not terminate")
		}
	}
	cleanup := owner.closeTransport()
	owner.mu.Lock()
	owner.outcome, owner.cleanup = err, cleanup
	for lane, delivery := range owner.pending {
		clear(delivery.operation)
		delivery.operation = nil
		delivery.outcome = errors.Join(err, cleanup, errors.New("closed Introduction registration ended"))
		close(delivery.done)
		delete(owner.pending, lane)
	}
	owner.mu.Unlock()
	close(owner.done)
}
func (owner *ClosedIntroductionRegistration) closeTransport() error {
	err := owner.lane.Close()
	if !owner.stop() {
		<-owner.interrupted
	}
	if errors.Is(err, ErrClosedSourceCleanup) {
		err = errors.Join(err, owner.prefix.Close())
	}
	return err
}

// Receipt commits the exact accepted REGISTER result, not Service readiness.
func (owner *ClosedIntroductionRegistration) Receipt() [32]byte { return owner.receipt }

func (owner *ClosedIntroductionRegistration) Done() <-chan struct{} { return owner.done }

func (owner *ClosedIntroductionRegistration) Close() error {
	if owner == nil {
		return nil
	}
	err := owner.lane.Close()
	<-owner.done
	owner.mu.Lock()
	defer owner.mu.Unlock()
	return errors.Join(err, owner.cleanup)
}

func (owner *ClosedIntroductionRegistration) Withdraw(ctx context.Context) (outcome error) {
	if owner == nil || ctx == nil || ctx.Err() != nil {
		return errors.New("closed Introduction withdrawal unavailable")
	}
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	owner.mu.Lock()
	select {
	case <-owner.done:
		owner.mu.Unlock()
		return errors.New("closed Introduction registration ended")
	default:
	}
	if owner.withdraw != [32]byte{} {
		owner.mu.Unlock()
		return errors.New("closed Introduction withdrawal already pending")
	}
	owner.withdraw = nonce
	owner.mu.Unlock()
	operation, err := EncodeClosedRegistrationRequest(ClosedRegistrationRequest{Nonce: nonce, Slot: owner.request.Slot, Revision: owner.request.Revision, Withdraw: true})
	if err != nil {
		return err
	}
	end := time.Now().Add(10 * time.Second)
	if owner.request.Expiry.Before(end) {
		end = owner.request.Expiry
	}
	if deadline, ok := ctx.Deadline(); ok && deadline.Before(end) {
		end = deadline
	}
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(interrupted); _ = owner.lane.Close() })
	defer func() {
		if !stop() {
			<-interrupted
		}
		outcome = errors.Join(outcome, ctx.Err())
	}()
	if err := owner.connection.SetDeadline(end); err != nil {
		return errors.Join(err, owner.Close())
	}
	if err := owner.writeFrame(ctx, ClosedLaneFrame{Kind: closedFrameOperation, Body: operation}, end); err != nil {
		return errors.Join(err, owner.Close())
	}
	select {
	case <-owner.done:
	case <-ctx.Done():
		return errors.Join(ctx.Err(), owner.Close())
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	return errors.Join(owner.outcome, owner.cleanup)
}
