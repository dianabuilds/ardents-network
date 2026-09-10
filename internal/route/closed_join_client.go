//go:build linux

package route

import (
	"context"
	"crypto/rand"
	"errors"
	"sync"
	"time"
)

// ClosedJoinIntent is the Endpoint-owned pairing commitment and two distinct
// lifetime bounds. It contains no address, Node key, recipient or side override.
type ClosedJoinIntent struct {
	Secret, Context             [32]byte
	SetupDeadline, WorkDeadline time.Time
}

// Join opens one dedicated class-2 terminal role channel through this already
// admitted Source or Responder prefix. Present must durably mark its token
// before returning it. The caller starts capsule submission independently and
// concurrently; waiting for this result first would prevent Publisher pairing.
func (prefix *ClosedSourcePrefix) Join(ctx context.Context, present ClosedTokenPresenter, intent ClosedJoinIntent) (_ *ClosedJoinedStream, outcome error) {
	if prefix == nil || prefix.channels == nil || ctx == nil || ctx.Err() != nil || present == nil || intent.Secret == [32]byte{} || intent.Context == [32]byte{} {
		return nil, errors.New("closed JOIN client unavailable")
	}
	peer, end, err := prefix.joinPeer()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if !now.Before(intent.SetupDeadline) || !now.Before(intent.WorkDeadline) || !intent.SetupDeadline.Equal(intent.SetupDeadline.UTC().Truncate(time.Second)) || !intent.WorkDeadline.Equal(intent.WorkDeadline.UTC().Truncate(time.Second)) {
		return nil, errors.New("closed JOIN client lifetime invalid")
	}
	if intent.WorkDeadline.Before(end) {
		end = intent.WorkDeadline
	}
	if bound, ok := ctx.Deadline(); ok && bound.Before(end) {
		end = bound.UTC().Truncate(time.Second)
	}
	if bound := now.Add(30 * time.Minute).Truncate(time.Second); bound.Before(end) {
		end = bound
	}
	pending := now.Add(10 * time.Second).Truncate(time.Second)
	if intent.SetupDeadline.Before(pending) {
		pending = intent.SetupDeadline
	}
	if end.Before(pending) {
		pending = end
	}
	if !now.Before(pending) {
		return nil, errors.New("closed JOIN setup expired")
	}
	prefix.channels.mu.Lock()
	if prefix.channels.terminal != nil || prefix.channels.queued+closedJoinedQueue > 4<<20 {
		prefix.channels.mu.Unlock()
		return nil, errors.New("closed JOIN prefix queue unavailable")
	}
	prefix.channels.queued += closedJoinedQueue
	prefix.channels.mu.Unlock()
	var released sync.Once
	release := func() {
		released.Do(func() {
			prefix.channels.mu.Lock()
			prefix.channels.queued -= closedJoinedQueue
			prefix.channels.mu.Unlock()
		})
	}
	transferred := false
	defer func() {
		if !transferred {
			release()
		}
	}()
	lane, err := prefix.channels.open(ctx, ClosedOpen{NextNodeID: peer.node, NextDutyGeneration: peer.generation, Purpose: ClosedPurposeDataJoin, Deadline: end}, pending)
	if err != nil {
		return nil, err
	}
	interrupted := make(chan struct{})
	handshakeStopped := false
	stop := context.AfterFunc(ctx, func() { defer close(interrupted); _ = lane.Close() })
	defer func() {
		if !handshakeStopped && !stop() {
			<-interrupted
		}
		if !transferred {
			outcome = errors.Join(outcome, lane.Close())
		}
		if errors.Is(outcome, ErrClosedSourceCleanup) {
			outcome = errors.Join(outcome, prefix.Close())
		}
	}()
	secured, err := OpenClosedRoleTLS(ctx, lane, peer.key, pending)
	if err != nil {
		return nil, err
	}
	// Role TLS clears its handshake deadline; keep the complete JOIN setup bounded.
	if err := secured.SetDeadline(pending); err != nil {
		return nil, err
	}
	hello := ClosedHello{NetworkID: prefix.plan.profile.NetworkID, StateGeneration: prefix.plan.profile.StateGeneration, StateDigest: prefix.plan.profile.StateDigest, ProfileDigest: prefix.plan.profile.Digest, RecipientNodeID: peer.node, RecipientDutyGeneration: peer.generation, Purpose: ClosedPurposeDataJoin, Deadline: end}
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
	if err := prefix.currentJoin(peer); err != nil {
		return nil, err
	}
	token, err := present(hello, 2)
	if err != nil {
		clear(token)
		return nil, err
	}
	defer clear(token)
	if len(token) != 354 {
		return nil, errors.New("closed JOIN token invalid")
	}
	admit := append([]byte{2}, token...)
	defer clear(admit)
	if err := WriteClosedLaneFrame(secured, ClosedLaneFrame{Kind: closedFrameAdmit, Body: admit}); err != nil {
		return nil, err
	}
	accepted, err := ReadClosedLaneFrame(secured)
	if err != nil {
		return nil, err
	}
	if status, credit, err := DecodeClosedAcceptFrame(accepted); err != nil || status != 0 || credit != 64<<10 {
		return nil, errors.New("closed JOIN admission refused")
	}
	if err := lane.activate(); err != nil {
		return nil, err
	}
	if err := prefix.currentJoin(peer); err != nil {
		return nil, err
	}
	side := uint8(1)
	if prefix.plan.domain == 3 {
		side = 2
	}
	request := ClosedJoinRequest{Secret: intent.Secret, Context: intent.Context, Side: side, Deadline: pending}
	if _, err := rand.Read(request.Nonce[:]); err != nil {
		return nil, err
	}
	operation, err := EncodeClosedJoinRequest(request)
	if err != nil {
		return nil, err
	}
	defer clear(operation)
	if err := WriteClosedLaneFrame(secured, ClosedLaneFrame{Kind: closedFrameOperation, Lane: 1, Body: operation}); err != nil {
		return nil, err
	}
	result, err := ReadClosedLaneFrame(secured)
	if err != nil {
		return nil, err
	}
	defer clear(result.Body)
	status, err := DecodeClosedJoinResult(result.Body, request.Nonce)
	if err != nil || status != 0 || result.Kind != closedFrameResult || result.Lane != 1 {
		return nil, errors.New("closed JOIN result refused or mismatched")
	}
	if err := prefix.currentJoin(peer); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !time.Now().Before(pending) {
		return nil, errors.New("closed JOIN setup expired before stream transfer")
	}
	if err := secured.SetDeadline(end); err != nil {
		return nil, err
	}
	if !stop() {
		<-interrupted
		return nil, errors.New("closed JOIN canceled before stream transfer")
	}
	handshakeStopped = true
	stream := newClosedJoinedStream(ctx, secured, lane, release)
	transferred = true
	return stream, nil
}

func (prefix *ClosedSourcePrefix) joinPeer() (closedBootstrapPeer, time.Time, error) {
	node, generation, end, err := prefix.DataJoinRecipient()
	if err != nil {
		return closedBootstrapPeer{}, time.Time{}, err
	}
	peer, err := prefix.terminalPeer(ClosedPurposeDataJoin)
	if err != nil || peer.node != node || peer.generation != generation {
		return closedBootstrapPeer{}, time.Time{}, errors.New("closed JOIN recipient changed")
	}
	return peer, end, nil
}
func (prefix *ClosedSourcePrefix) currentJoin(expected closedBootstrapPeer) error {
	peer, _, err := prefix.joinPeer()
	if err != nil || peer != expected {
		return errors.New("closed JOIN current recipient unavailable")
	}
	return nil
}
