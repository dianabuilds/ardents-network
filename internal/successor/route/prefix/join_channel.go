package prefix

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	roletls "github.com/dianabuilds/ardents-network/internal/successor/route/transport/tls"
)

// JoinChannel retains the exact original parent's physical setup and
// shared framing budget. JOIN owns operation bytes, RESULT and its acquisition;
// Prefix owns opening the authenticated terminal and preparing its framing.
// A retained acquisition keeps this parent's generation alive until cleanup.
type JoinChannel struct {
	ctx, lifetime context.Context
	owner         *framing.Session
	end           time.Time
	conn          net.Conn
	parent        *framing.Lane
	release       func()
	interruption  prefixSetupInterruption
}

// Even a failed opening returns its exact partial setup for joined retirement.
// Its caller retains that physical result before completing the acquisition's
// opening, so concurrent Close cannot lose a late setup failure.
func (p *Prefix) openJoinChannel(ctx, caller context.Context, duty network.RetainedDuty, end, setup time.Time) (*JoinChannel, error) {
	t := &JoinChannel{ctx: ctx, lifetime: p.ctx, owner: p.interior, end: end}
	var err error
	t.release, err = p.interior.HoldControl()
	if err != nil {
		return t, err
	}
	authority := role.Authority{Current: p.observeOriginal, Duty: duty, Profile: p.config.Leg.Profile}
	member, err := authority.Member()
	if err != nil {
		return t, err
	}
	t.parent, err = p.interior.Open(ctx, caller, ardp.EncodeOpen(ardp.Open{RecipientNodeID: member.NodeID, RecipientDutyGeneration: member.DutyGeneration, Purpose: uint8(ardp.PurposeDataJoin), Deadline: end}, false))
	if err != nil {
		return t, err
	}
	done := make(chan struct{})
	t.interruption.done = done
	t.interruption.stop = context.AfterFunc(ctx, func() { defer close(done); _ = t.parent.Close() })
	secured, err := roletls.OpenRole(ctx, t.parent, member.PublicKey, minDeadline(setup, time.Now().Add(10*time.Second)))
	if err != nil {
		return t, err
	}
	t.conn = transport.Retain(secured)
	if err := t.conn.SetDeadline(setup); err != nil {
		return t, err
	}
	hello, err := authority.FreshHello(end, ardp.PurposeDataJoin, false)
	if err == nil {
		err = role.Present(ctx, caller, t.conn, authority, hello, p.config.Present)
	}
	return t, err
}

func (t *JoinChannel) JoinSetupInterruption() bool {
	return t.interruption.join()
}

func (t *JoinChannel) CloseSetup() error {
	t.JoinSetupInterruption()
	var result error
	if t.conn != nil {
		result = errors.Join(result, t.conn.Close())
	}
	return errors.Join(result, t.CloseParent())
}

// CloseParent returns only this terminal's original lower lane and control
// hold. Its operation owner calls it after the inner reader, writer and
// authenticated terminal have joined, or after failed setup has joined.
// JOIN never receives a raw parent or a separate capacity return callback.
func (t *JoinChannel) CloseParent() error {
	var result error
	if t.parent != nil {
		result = t.parent.Close()
		t.parent.Finish()
	}
	if t.release != nil {
		t.release()
	}
	return result
}

// Prepare only after JOIN has verified RESULT. Stop and join setup interruption
// before transferring framing; local seal may still emit bounded termination,
// with the original caller and both generations checked at every frame effect.
func (t *JoinChannel) PrepareJoined(check func() error) (*framing.Session, *framing.Lane, error) {
	if err := t.conn.SetDeadline(t.end); err != nil {
		return nil, nil, err
	}
	if t.JoinSetupInterruption() {
		return nil, nil, t.ctx.Err()
	}
	return t.owner.PrepareJoined(t.lifetime, t.conn, t.end,
		admission.ForwardClass.ByteLimit()-role.AdmissionWireBytes-2*ardp.HeaderSize-4096-16384, check)
}

// Stream carries only this admitted terminal's ordered role bytes. Its lower
// parent/control hold remain private and return only after joined retirement.
func (t *JoinChannel) Stream() net.Conn { return t.conn }
