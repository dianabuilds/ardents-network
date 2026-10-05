//go:build linux

package transport

import (
	"context"
	"crypto/rand"
	"errors"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/carrier"
)

// JoinConfig names an already selected public recipient and one fresh opaque
// pair intent. It conveys no Publication, Instance or Application authority.
type JoinConfig struct {
	Duty                    network.RetainedDuty
	Secret, Context         [32]byte
	Deadline, SetupDeadline time.Time
}

// Join performs exactly one genuine terminal TLS/admission/JOIN exchange. A
// failed exchange consumes this acquisition's attempt; retry cannot reuse its
// channel or silently switch the original Source or Responder.
func (a *JoinAcquisition) Join(ctx context.Context, config JoinConfig) (_ *Joined, result error) {
	if a == nil || ctx == nil || config.Secret == [32]byte{} || config.Context == [32]byte{} || !time.Now().Before(config.Deadline) || config.Deadline != config.Deadline.UTC().Truncate(time.Second) || config.Deadline.After(time.Now().Add(admission.ForwardClass.Lifetime())) || !time.Now().Before(config.SetupDeadline) || config.SetupDeadline != config.SetupDeadline.UTC().Truncate(time.Second) || config.SetupDeadline.After(config.Deadline) {
		return nil, errors.New("route JOIN intent bounds invalid")
	}
	if end, exists := ctx.Deadline(); exists && config.Deadline.After(end) {
		return nil, errors.New("route JOIN exceeds caller bound")
	}
	a.mu.Lock()
	if a.closed || a.attempted {
		a.mu.Unlock()
		return nil, errors.New("route JOIN acquisition already used")
	}
	a.attempted = true
	a.opening = make(chan struct{})
	opening := a.opening
	a.mu.Unlock()
	defer close(opening)
	p, side := a.source, uint8(1)
	if a.responder != nil {
		p, side = a.responder, 2
	}
	child, cancel := context.WithDeadline(a.ctx, config.Deadline)
	callerDone := make(chan struct{})
	stopCaller := context.AfterFunc(ctx, func() { defer close(callerDone); cancel() })
	defer func() {
		if !stopCaller() {
			<-callerDone
		}
		cancel()
	}()
	authority := Authority{Current: p.config.Current, Duty: config.Duty, Profile: p.config.Leg.Profile}
	physicalCheck := func() error {
		if err := errors.Join(ctx.Err(), a.originalCurrent()); err != nil {
			return err
		}
		for _, original := range []*Prefix{a.source, a.responder} {
			if original == nil {
				continue
			}
			view, err := original.config.Current()
			if err != nil {
				return err
			}
			duty, err := original.config.Leg.RendezvousDuty(view, config.Duty.NodeID, config.Duty.RecordGeneration, nil)
			if err != nil || duty != config.Duty || config.Deadline.After(original.config.Deadline) || config.Deadline.After(config.Duty.RecordValidUntil) || config.Deadline.After(config.Duty.Epoch.ValidUntil) || config.Deadline.After(original.config.Leg.Profile.NotAfter) || !time.Now().Before(config.Deadline) {
				return errors.Join(errors.New("route JOIN recipient or horizon differs"), err)
			}
		}
		return errors.Join(ctx.Err(), a.originalCurrent())
	}
	check := func() error {
		if err := a.current(); err != nil {
			return err
		}
		return errors.Join(physicalCheck(), a.current())
	}
	if err := check(); err != nil {
		return nil, err
	}
	release, err := p.interior.queues.channel()
	if err != nil {
		return nil, err
	}
	var parent *lane
	var secured net.Conn
	var joined *Joined
	interrupted := make(chan struct{})
	stop := func() bool { return true }
	defer func() {
		if !stop() {
			<-interrupted
		}
		if result != nil {
			result = errors.Join(result, a.cleanupJoin(joined, secured, parent, release))
		}
	}()
	member, err := authority.member()
	if err != nil {
		return nil, err
	}
	parent, err = p.interior.openLane(child, ctx, encodeOpen(ardpHello{RecipientNodeID: member.NodeID, RecipientDutyGeneration: member.DutyGeneration, Purpose: uint8(ardp.PurposeDataJoin), Deadline: config.Deadline}, false))
	if err != nil {
		return nil, err
	}
	stop = context.AfterFunc(child, func() { defer close(interrupted); _ = parent.Close() })
	tls, err := carrier.OpenClosedRoleTLS(child, parent, member.PublicKey, minDeadline(config.SetupDeadline, time.Now().Add(10*time.Second)))
	if err != nil {
		return nil, err
	}
	secured = &retiredConn{Conn: tls}
	if err := secured.SetDeadline(config.SetupDeadline); err != nil {
		return nil, err
	}
	hello, err := freshPurposeHello(authority, config.Deadline, ardp.PurposeDataJoin, false)
	if err == nil {
		err = presentChannel(child, ctx, secured, authority, hello, p.config.Present)
	}
	if err != nil {
		return nil, err
	}
	if err := check(); err != nil {
		return nil, err
	}
	request := ardp.JoinRequest{Secret: config.Secret, Context: config.Context, Side: side, Deadline: config.SetupDeadline}
	if _, err := rand.Read(request.Nonce[:]); err != nil {
		return nil, err
	}
	body, err := ardp.EncodeJoinRequest(request)
	clear(request.Secret[:])
	clear(request.Context[:])
	if err != nil {
		return nil, err
	}
	err = ardp.WriteFrame(secured, ardp.Frame{Kind: ardp.KindOperation, Lane: 1, Body: body})
	clear(body)
	if err != nil {
		return nil, err
	}
	frame, err := ardp.ReadFrame(secured)
	if err != nil {
		return nil, err
	}
	status, err := ardp.DecodeJoinResult(frame.Body, request.Nonce)
	if err != nil || frame.Kind != ardp.KindResult || frame.Lane != 1 || status != 0 {
		return nil, errors.Join(errors.New("route JOIN refused"), err)
	}
	if err := check(); err != nil {
		return nil, err
	}
	if err := secured.SetDeadline(config.Deadline); err != nil {
		return nil, err
	}
	if !stop() {
		<-interrupted
		return nil, child.Err()
	}
	stop = func() bool { return true }
	// Local seal can still emit bounded terminal output; original caller and
	// both physical generations remain checked at every frame effect.
	session, lane, err := prepareJoinedSession(p.ctx, secured, config.Deadline, admission.ForwardClass.ByteLimit()-admissionWireBytes-2*ardp.HeaderSize-4096-16384, physicalCheck, p.interior.queues)
	if err != nil {
		return nil, err
	}
	joined = &Joined{acquisition: a, session: session, lane: lane, parent: parent, release: release, retiring: make(chan struct{}), watcherDone: make(chan struct{})}
	go joined.watch(ctx)
	session.startReading()
	if err := check(); err != nil {
		return nil, err
	}
	if err := a.publish(ctx, joined); err != nil {
		return nil, err
	}
	return joined, nil
}

// cleanupJoin joins setup resources that were never published to a.stream.
// Its result is physical retirement provenance, separate from setup refusal.
func (a *JoinAcquisition) cleanupJoin(joined *Joined, secured net.Conn, parent *lane, release func()) error {
	var retirement error
	if joined != nil {
		retirement = joined.closePhysical()
	} else {
		if secured != nil {
			retirement = errors.Join(retirement, secured.Close())
		}
		if parent != nil {
			retirement = errors.Join(retirement, parent.Close())
			parent.finish()
		}
		release()
	}
	// Close racing setup waits for opening, which closes only after this
	// original physical result has been retained. Ordinary setup refusal
	// contributes nothing here when its cleanup succeeds.
	a.mu.Lock()
	a.setupRetirement = errors.Join(a.setupRetirement, retirement)
	a.mu.Unlock()
	return retirement
}
