package prefix

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
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
	ctx, lifetime               context.Context
	owner                       *framing.Session
	end                         time.Time
	conn                        net.Conn
	parent                      *framing.Lane
	release                     func()
	interruption                prefixSetupInterruption
	parentOnce                  sync.Once
	parentResult, parentCleanup error
	parentJoined                atomic.Bool
	setupOnce                   sync.Once
	setupResult, setupCleanup   error
	setupJoined                 atomic.Bool
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
	t.setupOnce.Do(func() {
		t.JoinSetupInterruption()
		var output error
		if t.conn != nil {
			output = t.conn.Close()
		}
		t.setupResult = errors.Join(output, t.CloseParent())
		t.setupCleanup = t.parentCleanup
		if t.parent == nil {
			// Before acquiring a lower lane, only this directly owned partial
			// connection can have a physical close result. With a lower lane,
			// its joined receipt retains actual output/close failures separately
			// from the complete failed setup outcome above.
			t.setupCleanup = errors.Join(t.setupCleanup, output)
		}
		t.setupJoined.Store(true)
	})
	return t.setupResult
}

// SetupCleanupResult retains only original physical retirement after the setup
// callback, partial connection, lower lane and control return have joined.
// A failed setup or work refusal remains in CloseSetup, not this receipt.
func (t *JoinChannel) SetupCleanupResult() (error, bool) {
	if t == nil || !t.setupJoined.Load() {
		return nil, false
	}
	return t.setupCleanup, true
}

// CloseParent returns only this terminal's original lower lane and control
// hold. Its operation owner calls it after the inner reader, writer and
// authenticated terminal have joined, or after failed setup has joined.
// JOIN never receives a raw parent or a separate capacity return callback.
func (t *JoinChannel) CloseParent() error {
	t.parentOnce.Do(func() {
		if t.parent != nil {
			t.parentResult = t.parent.Close()
			cleanup, complete := t.parent.CleanupResult()
			if !complete {
				cleanup = errors.Join(cleanup, errors.New("original JOIN parent lane cleanup did not join"))
			}
			t.parentCleanup = cleanup
			t.parent.Finish()
		}
		if t.release != nil {
			t.release()
		}
		t.parentJoined.Store(true)
	})
	return t.parentResult
}

// ParentCleanupResult is the exact joined lower lane/control return, not the
// parent Close authority/work outcome or permission for another JOIN.
func (t *JoinChannel) ParentCleanupResult() (error, bool) {
	if t == nil || !t.parentJoined.Load() {
		return nil, false
	}
	return t.parentCleanup, true
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
