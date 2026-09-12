//go:build linux

package route

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"time"
)

// exchangeControl owns one confidential terminal Control lane. Both issuance
// and Descriptor operations use this lifecycle; neither accepts an address,
// key or recipient supplied by an Application.
func (prefix *ClosedSourcePrefix) exchangeControl(ctx context.Context, purpose ClosedPurpose, peer closedBootstrapPeer, present ClosedTokenPresenter, operation []byte) (result []byte, outcome error) {
	if prefix == nil || prefix.channels == nil || ctx == nil || ctx.Err() != nil || present == nil {
		return nil, errors.New("closed source Control owner unavailable")
	}
	defer func() {
		if errors.Is(outcome, ErrClosedSourceCleanup) {
			outcome = errors.Join(outcome, prefix.Close())
		}
	}()
	if err := prefix.currentControl(purpose, peer); err != nil {
		return nil, err
	}

	end := time.Now().UTC().Add(30 * time.Second).Truncate(time.Second)
	for _, limit := range []time.Time{prefix.plan.deadline, peer.notAfter} {
		if limit.Before(end) {
			end = limit
		}
	}
	if callerEnd, ok := ctx.Deadline(); ok && callerEnd.Before(end) {
		end = callerEnd.UTC().Truncate(time.Second)
	}
	pending := time.Now().UTC().Add(10 * time.Second)
	if end.Before(pending) {
		pending = end
	}
	lane, err := prefix.channels.open(ctx, ClosedOpen{NextNodeID: peer.node, NextDutyGeneration: peer.generation,
		Purpose: purpose, Deadline: end}, pending)
	if err != nil {
		return nil, errors.Join(errors.New("closed source Control lane opening failed"), err)
	}
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(interrupted); _ = lane.Close() })
	defer func() {
		closeErr := lane.Close()
		if !stop() {
			<-interrupted
		}
		outcome = errors.Join(outcome, closeErr)
		if outcome != nil {
			clear(result)
			result = nil
		}
	}()
	secured, err := OpenClosedRoleTLS(ctx, lane, peer.key, pending)
	if err != nil {
		return nil, errors.Join(errors.New("closed source Control TLS unavailable"), err)
	}
	hello := ClosedHello{NetworkID: prefix.plan.profile.NetworkID, StateGeneration: prefix.plan.profile.StateGeneration,
		StateDigest: prefix.plan.profile.StateDigest, ProfileDigest: prefix.plan.profile.Digest,
		RecipientNodeID: peer.node, RecipientDutyGeneration: peer.generation,
		Purpose: purpose, Deadline: end}
	if _, err := rand.Read(hello.ChannelNonce[:]); err != nil {
		return nil, err
	}
	body, err := EncodeClosedHello(hello)
	if err != nil {
		return nil, err
	}
	if err := WriteClosedLaneFrame(secured, ClosedLaneFrame{Kind: closedFrameHello, Body: body}); err != nil {
		return nil, errors.Join(errors.New("closed source Control HELLO write failed"), err)
	}
	if err := prefix.currentControl(purpose, peer); err != nil {
		return nil, err
	}
	token, err := present(hello, 1)
	if err != nil {
		clear(token)
		return nil, err
	}
	defer clear(token)
	if len(token) != 354 {
		return nil, errors.New("closed source Control token invalid")
	}
	admit := append([]byte{1}, token...)
	defer clear(admit)
	if err := WriteClosedLaneFrame(secured, ClosedLaneFrame{Kind: closedFrameAdmit, Body: admit}); err != nil {
		return nil, errors.Join(errors.New("closed source Control ADMIT write failed"), err)
	}
	accepted, err := ReadClosedLaneFrame(secured)
	if err != nil {
		return nil, errors.Join(errors.New("closed source Control admission response unavailable"), err)
	}
	if status, credit, err := DecodeClosedAcceptFrame(accepted); err != nil || status != 0 || credit != 64<<10 {
		return nil, errors.New("closed source Control admission refused")
	}
	if err := lane.activate(); err != nil {
		return nil, errors.Join(errors.New("closed source Control activation failed"), err)
	}
	if err := secured.SetDeadline(end); err != nil {
		return nil, err
	}
	if err := prefix.currentControl(purpose, peer); err != nil {
		return nil, err
	}
	if err := WriteClosedLaneFrame(secured, ClosedLaneFrame{Kind: closedFrameOperation, Body: operation}); err != nil {
		return nil, errors.Join(errors.New("closed source Control operation write failed"), err)
	}
	frame, err := ReadClosedLaneFrame(secured)
	if err != nil || frame.Kind != closedFrameResult || frame.Lane != 0 {
		return nil, errors.Join(errors.New("closed source Control result unavailable"), err)
	}

	// The recipient owns terminal TLS close after one operation. Do not emit
	// another TLS record after its terminal CLOSE has retired that channel.
	var trailing [1]byte
	if count, err := secured.Read(trailing[:]); count != 0 || !errors.Is(err, io.EOF) {
		return nil, errors.Join(errors.New("closed source Control did not terminate its operation"), err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := prefix.currentControl(purpose, peer); err != nil {
		return nil, err
	}
	result = frame.Body
	return result, nil
}
