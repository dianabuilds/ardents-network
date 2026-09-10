//go:build linux

package route

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"net"
	"sync"
	"time"
)

// Endpoint returns a token only after verifying its challenge and durably
// marking this attempt. Route calls it inside the intended authenticated TLS.
type ClosedTokenPresenter func(ClosedHello, uint8) ([]byte, error)

// ClosedSourcePrefix owns a fresh admitted Entry/Interior tree. Its stream
// carries the Interior role protocol, never a direct Application Connection.
type ClosedSourcePrefix struct {
	interruptMu      sync.Mutex
	interruptedEarly bool
	source           ClosedBootstrapState
	selection        ClosedBootstrapSelection
	plan             closedBootstrapPlan
	channels         *closedSourceChannels
	connection       net.Conn
	child            *closedRoleChildStream
	retirement       *closedRoleRetirement
	stop             func() bool
	interrupted      chan struct{}
	done             chan struct{}
	once             sync.Once
	failure          error
}

var ErrClosedSourceCleanup = errors.New("closed source prefix cleanup failed")

func OpenClosedSourcePrefix(ctx context.Context, source ClosedBootstrapState, selection ClosedBootstrapSelection, present ClosedTokenPresenter) (*ClosedSourcePrefix, error) {
	return openClosedPrefix(ctx, source, selection, 1, present)
}

// OpenClosedIntroductionPrefix opens the Publisher's separate Introduction
// Entry/Interior ownership. Issuance and resolution still use its Source role.
// The selection contains only retained State Node identifiers, never addresses.
func OpenClosedIntroductionPrefix(ctx context.Context, source ClosedBootstrapState, selection ClosedBootstrapSelection, present ClosedTokenPresenter) (*ClosedSourcePrefix, error) {
	return openClosedPrefix(ctx, source, selection, closedRoleDomainIntroduction, present)
}

// OpenClosedResponderPrefix opens the independently retained Publisher data
// prefix. It consumes the same receiver-local class-2 admission as Source.
func OpenClosedResponderPrefix(ctx context.Context, source ClosedBootstrapState, selection ClosedBootstrapSelection, present ClosedTokenPresenter) (*ClosedSourcePrefix, error) {
	return openClosedPrefix(ctx, source, selection, 3, present)
}

func openClosedPrefix(ctx context.Context, source ClosedBootstrapState, selection ClosedBootstrapSelection, domain uint8, present ClosedTokenPresenter) (prefix *ClosedSourcePrefix, outcome error) {
	if ctx == nil || ctx.Err() != nil || present == nil {
		return nil, errors.New("closed source owner unavailable")
	}
	now := time.Now().UTC()
	plan, err := prepareClosedPrefix(source, selection, domain, now)
	if err != nil {
		return nil, err
	}
	handshakeEnd := plan.deadline
	snapshot, err := source.Current()
	if err != nil {
		return nil, err
	}
	end := now.Add(30 * time.Minute).Truncate(time.Second)
	for _, limit := range []time.Time{plan.profile.NotAfter, snapshot.ValidUntil, plan.peers[0].notAfter, plan.peers[1].notAfter} {
		if limit.Before(end) {
			end = limit
		}
	}
	plan.deadline = end
	entry := plan.peers[0]
	connection, err := OpenClosedRoleCarrier(ctx, ClosedRoleCarrierRequest{CarrierProfile: entry.carrier, Endpoint: entry.endpoint, ExpectedServer: entry.key, Deadline: handshakeEnd})
	if err != nil {
		return nil, err
	}
	owner := &ClosedSourcePrefix{source: source, selection: selection, plan: plan, connection: connection, retirement: &closedRoleRetirement{transport: connection}, interrupted: make(chan struct{})}
	if secured, ok := connection.(*tls.Conn); ok {
		owner.retirement.transport = secured.NetConn()
	}
	owner.stop = context.AfterFunc(ctx, func() { defer close(owner.interrupted); owner.interrupt() })
	defer func() {
		if outcome != nil {
			outcome = errors.Join(outcome, owner.Close())
		}
	}()
	for index := 0; index < 2; index++ {
		if err := plan.current(source, selection); err != nil {
			return nil, err
		}
		pending := time.Now().Add(10 * time.Second)
		if end.Before(pending) {
			pending = end
		}
		if err := owner.connection.SetDeadline(pending); err != nil {
			return nil, err
		}
		if err := admitClosedSource(owner.connection, plan, index, present); err != nil {
			return nil, err
		}
		if owner.child != nil {
			if err := owner.child.activate(); err != nil {
				return nil, err
			}
		}
		if index == 1 {
			if err := owner.connection.SetDeadline(end); err != nil {
				return nil, err
			}
			break
		}
		if err := plan.current(source, selection); err != nil {
			return nil, err
		}
		if err := owner.openChild(ctx, plan.peers[1], end, pending); err != nil {
			return nil, err
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := plan.current(source, selection); err != nil {
		return nil, err
	}
	owner.interruptMu.Lock()
	if owner.interruptedEarly {
		owner.interruptMu.Unlock()
		return nil, errors.New("closed prefix canceled before channel ownership")
	}
	owner.channels = newClosedSourceChannelOwner(owner.connection, end, owner.retirement.close)
	owner.channels.framing = owner.child
	owner.channels.start()
	owner.interruptMu.Unlock()
	owner.done = make(chan struct{})
	go owner.finishAfterChannels()
	return owner, nil
}

func admitClosedSource(connection net.Conn, plan closedBootstrapPlan, index int, present ClosedTokenPresenter) error {
	peer := plan.peers[index]
	hello := ClosedHello{NetworkID: plan.profile.NetworkID, StateGeneration: plan.profile.StateGeneration, StateDigest: plan.profile.StateDigest,
		ProfileDigest: plan.profile.Digest, RecipientNodeID: peer.node, RecipientDutyGeneration: peer.generation, Purpose: ClosedPurposeForwarding, Deadline: plan.deadline}
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
	token, err := present(hello, 2)
	if err != nil {
		clear(token)
		return err
	}
	defer clear(token)
	if len(token) != 354 {
		return errors.New("closed source token invalid")
	}
	admission := append([]byte{2}, token...)
	defer clear(admission)
	if err := WriteClosedLaneFrame(connection, ClosedLaneFrame{Kind: closedFrameAdmit, Body: admission}); err != nil {
		return err
	}
	accepted, err := ReadClosedLaneFrame(connection)
	if err != nil {
		return err
	}
	status, credit, err := DecodeClosedAcceptFrame(accepted)
	if err != nil || status != 0 || credit != 64<<10 {
		return errors.New("closed source admission refused")
	}
	return nil
}

func (prefix *ClosedSourcePrefix) Close() error {
	if prefix == nil {
		return nil
	}
	prefix.finish()
	if prefix.done != nil {
		<-prefix.done
	}
	return prefix.failure
}

func (prefix *ClosedSourcePrefix) finish() {
	prefix.once.Do(func() {
		if prefix.channels != nil {
			prefix.failure = prefix.channels.Close()
		} else {
			prefix.failure = prefix.retirement.close()
		}
		if prefix.child != nil {
			prefix.failure = errors.Join(prefix.failure, prefix.child.Close())
		}
		if !prefix.stop() {
			<-prefix.interrupted
		}
		if prefix.failure != nil {
			prefix.failure = errors.Join(ErrClosedSourceCleanup, errors.New("closed source physical retirement failed"), prefix.failure)
		}
	})
}
func (prefix *ClosedSourcePrefix) openChild(ctx context.Context, peer closedBootstrapPeer, end, pending time.Time) error {
	// OPEN emission and inner TLS share the pending interval. The longer wire
	// deadline is a maximum lease, not permission to block opening until then.
	if err := prefix.connection.SetDeadline(pending); err != nil {
		return err
	}
	body, err := EncodeClosedOpen(ClosedOpen{NextNodeID: peer.node, NextDutyGeneration: peer.generation, Purpose: ClosedPurposeForwarding, Deadline: end})
	if err != nil {
		return err
	}
	if err := WriteClosedLaneFrame(prefix.connection, ClosedLaneFrame{Kind: closedFrameOpen, Lane: 1, Body: body}); err != nil {
		return err
	}
	prefix.child = prefix.newChild(prefix.connection, end)
	inner, err := OpenClosedRoleTLS(ctx, prefix.child, peer.key, pending)
	if err != nil {
		return err
	}
	prefix.connection = inner
	return nil
}

// Mark the child owner terminal before closing its physical transport. Children
// then join that shutdown instead of trying to emit CLOSE on a retired socket.
// Publication of channels is serialized with cancellation during initial open.
func (prefix *ClosedSourcePrefix) interrupt() {
	prefix.interruptMu.Lock()
	prefix.interruptedEarly = true
	channels := prefix.channels
	prefix.interruptMu.Unlock()
	if channels != nil {
		channels.stop()
	} else {
		_ = prefix.retirement.close()
	}
}

func (prefix *ClosedSourcePrefix) newChild(parent net.Conn, end time.Time) *closedRoleChildStream {
	return newClosedRoleChildStream(parent, end, prefix.retirement.close, prefix.observeChildTerminal)
}

func (prefix *ClosedSourcePrefix) observeChildTerminal(cause error) {
	prefix.interruptMu.Lock()
	channels := prefix.channels
	if channels == nil {
		prefix.interruptedEarly = true
	}
	prefix.interruptMu.Unlock()
	if channels != nil {
		// Publish the original nested cause before physical retirement. The
		// upper reader may still be inside TLS or delivering buffered bytes.
		channels.fail(cause)
	}
}
