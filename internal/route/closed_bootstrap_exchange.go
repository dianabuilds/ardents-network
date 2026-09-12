//go:build linux

package route

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

// ClosedIssuanceExchangeResult is the exact encrypted issuer response and its fresh
// operation nonce. Credential verifies the padded result and unblinds tokens;
// Route neither owns holder keys nor interprets a permission or token batch.
type ClosedIssuanceExchangeResult struct {
	Nonce [32]byte
	Body  []byte
}

// ExchangeClosedBootstrap executes one target-free issuance exchange through
// the retained Entry and Interior members to the sole current issuer. All
// addresses and keys come from live State. It performs no retry or resampling,
// and always retires bootstrap channels before returning to its context owner.
func ExchangeClosedBootstrap(ctx context.Context, source ClosedBootstrapState, selection ClosedBootstrapSelection, batch []byte) (result ClosedIssuanceExchangeResult, outcome error) {
	if ctx == nil || ctx.Err() != nil {
		return result, errors.New("closed bootstrap context is unavailable")
	}
	if _, err := rand.Read(result.Nonce[:]); err != nil {
		return ClosedIssuanceExchangeResult{}, err
	}
	operation, err := EncodeClosedIssuanceRequest(result.Nonce, batch)
	if err != nil {
		return ClosedIssuanceExchangeResult{}, err
	}
	defer clear(operation)
	plan, err := prepareClosedBootstrap(source, selection, time.Now().UTC())
	if err != nil {
		return ClosedIssuanceExchangeResult{}, err
	}
	attempt, cancel := context.WithDeadline(ctx, plan.deadline)
	defer cancel()
	entry := plan.peers[0]
	connection, err := OpenClosedRoleCarrier(attempt, ClosedRoleCarrierRequest{CarrierProfile: entry.carrier, Endpoint: entry.endpoint, ExpectedServer: entry.key, Deadline: plan.deadline})
	if err != nil {
		return ClosedIssuanceExchangeResult{}, err
	}
	retirement := &closedRoleRetirement{transport: connection}
	if secured, ok := connection.(*tls.Conn); ok {
		retirement.transport = secured.NetConn()
	}
	interrupted := make(chan struct{})
	stop := context.AfterFunc(attempt, func() {
		defer close(interrupted)
		_ = retirement.close()
	})
	var owned []*closedRoleChildStream
	defer func() {
		// Interrupt the transport before joining nested readers or TLS Close.
		cleanupErr := retirement.close()
		for index := len(owned) - 1; index >= 0; index-- {
			if err := owned[index].Close(); err != nil && cleanupErr == nil {
				cleanupErr = err
			}
		}
		if cleanupErr != nil {
			outcome = errors.Join(outcome, ErrClosedBootstrapCleanup, cleanupErr)
		}
		if !stop() {
			<-interrupted
		}
		if outcome != nil {
			clear(result.Body)
			result = ClosedIssuanceExchangeResult{}
		}
	}()
	if err := connection.SetDeadline(plan.deadline); err != nil {
		return result, err
	}
	var current net.Conn = connection
	var transport *closedRoleChildStream
	for index := 0; index < 3; index++ {
		if err := plan.current(source, selection); err != nil {
			return result, err
		}
		if err := beginClosedBootstrap(current, plan, index); err != nil {
			return result, fmt.Errorf("closed bootstrap hello %d: %w", index, err)
		}
		if transport != nil {
			if err := transport.activate(); err != nil {
				return result, err
			}
		}
		if index == 2 {
			break
		}
		peer := plan.peers[index+1]
		purpose := ClosedPurposeForwarding
		if index == 1 {
			purpose = ClosedPurposeIssuer
		}
		body, err := EncodeClosedOpen(ClosedOpen{NextNodeID: peer.node, NextDutyGeneration: peer.generation, Purpose: purpose, Deadline: plan.deadline})
		if err != nil {
			return result, err
		}
		if err := WriteClosedLaneFrame(current, ClosedLaneFrame{Kind: closedFrameOpen, Lane: 1, Body: body}); err != nil {
			return result, err
		}
		child := newClosedRoleChildStream(current, plan.deadline, retirement.close, nil)
		owned = append(owned, child)
		inner, err := OpenClosedRoleTLS(attempt, child, peer.key, plan.deadline)
		if err != nil {
			return result, fmt.Errorf("closed bootstrap TLS %d: %w", index+1, err)
		}
		current = inner
		transport = child
	}
	if err := plan.current(source, selection); err != nil {
		return result, err
	}
	if err := WriteClosedLaneFrame(current, ClosedLaneFrame{Kind: closedFrameOperation, Body: operation}); err != nil {
		return result, err
	}
	frame, err := ReadClosedLaneFrame(current)
	if err != nil {
		return result, err
	}
	if frame.Kind != closedFrameResult || frame.Lane != 0 {
		return result, errors.New("closed bootstrap issuer result is invalid")
	}
	if _, err := DecodeClosedIssuanceResult(frame.Body, result.Nonce); err != nil {
		return result, err
	}
	if err := attempt.Err(); err != nil {
		return result, err
	}
	if err := plan.current(source, selection); err != nil {
		return result, err
	}
	result.Body = frame.Body
	return result, nil
}

func beginClosedBootstrap(connection net.Conn, plan closedBootstrapPlan, index int) error {
	peer := plan.peers[index]
	hello := ClosedHello{NetworkID: plan.profile.NetworkID, StateGeneration: plan.profile.StateGeneration, StateDigest: plan.profile.StateDigest,
		ProfileDigest: plan.profile.Digest, RecipientNodeID: peer.node, RecipientDutyGeneration: peer.generation, Purpose: ClosedPurposeForwarding, Deadline: plan.deadline}
	if index == 2 {
		hello.Purpose = ClosedPurposeIssuer
	}
	if _, err := rand.Read(hello.ChannelNonce[:]); err != nil {
		return err
	}
	body, err := EncodeClosedHello(hello)
	if err != nil {
		return err
	}
	if err := WriteClosedLaneFrame(connection, ClosedLaneFrame{Kind: closedFrameHello, Body: body}); err != nil {
		return err
	}
	bootstrap := ClosedLaneFrame{Kind: closedFrameBootstrap, Body: EncodeClosedBootstrap(true)}
	if index != 2 {
		if err := WriteClosedLaneFrame(connection, bootstrap); err != nil {
			return err
		}
	}
	accepted, err := ReadClosedLaneFrame(connection)
	if err != nil {
		return err
	}
	status, credit, err := DecodeClosedAcceptFrame(accepted)
	if err != nil || status != 0 || credit != 64<<10 {
		return errors.New("closed bootstrap admission is unavailable")
	}
	if index == 2 {
		return WriteClosedLaneFrame(connection, bootstrap)
	}
	return nil
}

// ErrClosedBootstrapCleanup identifies failure to release the physical owner.
// A context must retain it, not turn it into a retryable remote refusal.
var ErrClosedBootstrapCleanup = errors.New("closed bootstrap cleanup failed")

type closedRoleRetirement struct {
	transport net.Conn
	once      sync.Once
	err       error
}

func (owner *closedRoleRetirement) close() error {
	owner.once.Do(func() {
		_ = owner.transport.SetDeadline(time.Now())
		owner.err = owner.transport.Close()
	})
	return owner.err
}
