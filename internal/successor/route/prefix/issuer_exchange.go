package prefix

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	issuertransport "github.com/dianabuilds/ardents-network/internal/successor/route/issuer"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	roletls "github.com/dianabuilds/ardents-network/internal/successor/route/transport/tls"
)

// IssuerBinding is private delivery provenance for the actual retained opening.
// It is not sent as a path identity and grants no Stock or Admission authority.
type IssuerBinding struct {
	ID, ProfileDigest [32]byte
	Deadline          time.Time
	Bootstrap         bool
}

// IssuerBatch transfers copied request bytes to this operation. Complete owns
// Stock finalization and must enforce check after verification, before deposit.
// The check performs only local original-lifetime checks, without I/O.
type IssuerBatch struct {
	Request  []byte
	Deadline time.Time
	Complete func(payload []byte, exchangeErr error, check func() error) error
}

// Issue retains preparation, transport, physical join and Stock completion
// under one original opening. Prefix retirement cancels and waits for it; a
// replacement generation cannot acquire its response or completion callback.
func (p *Prefix) Issue(caller context.Context, end time.Time, begin func(IssuerBinding) (IssuerBatch, error)) (result error) {
	if caller != nil {
		if bound, ok := caller.Deadline(); ok && bound.Before(end) {
			end = bound.UTC().Truncate(time.Second)
		}
	}
	if p == nil || begin == nil || end != end.UTC().Truncate(time.Second) || !time.Now().Before(end) || end.After(p.config.Deadline) || end.After(time.Now().Add(admission.ControlClass.Lifetime())) {
		return errors.New("route issuer original bounds invalid")
	}
	ctx, cancel, opening, err := p.beginTerminalOpening(caller, end)
	if err != nil {
		return err
	}
	defer opening.finish()
	defer cancel()
	callerDone := make(chan struct{})
	stopCaller := context.AfterFunc(caller, func() { defer close(callerDone); cancel() })
	defer func() {
		if !stopCaller() {
			<-callerDone
		}
	}()
	checkLocal := func() error {
		p.lifetimeMu.Lock()
		defer p.lifetimeMu.Unlock()
		if !time.Now().Before(end) {
			return context.DeadlineExceeded
		}
		return errors.Join(p.localCurrent(), caller.Err(), ctx.Err())
	}
	view, err := p.observeOriginal()
	if err = errors.Join(err, checkLocal()); err != nil {
		return err
	}
	duty, err := p.config.Leg.IssuerDuty(view)
	if err != nil {
		return err
	}
	authority := role.Authority{Current: p.observeOriginal, Duty: duty, Profile: p.config.Leg.Profile}
	check := func() error {
		if err := checkLocal(); err != nil {
			return err
		}
		v, err := p.observeOriginal()
		if err != nil {
			return err
		}
		current, err := p.config.Leg.IssuerDuty(v)
		if err != nil || current != duty {
			return errors.Join(errors.New("route original issuer changed"), err)
		}
		return checkLocal()
	}
	if err := check(); err != nil {
		return err
	}
	if p.queues == nil {
		return errors.New("route issuer queue owner absent")
	}
	releaseCopies, err := p.queues.HoldChild(4*(ardp.HeaderSize+ardp.IssuerBodySize) + admission.MaximumBatchTokens*admission.BlindSignatureSize)
	if err != nil {
		return err
	}
	defer releaseCopies()
	batch, err := begin(IssuerBinding{ID: p.entryHello.ChannelNonce, ProfileDigest: p.config.Leg.Profile.Digest, Deadline: end, Bootstrap: p.bootstrap})
	if err != nil {
		return err
	}
	defer clear(batch.Request)
	var payload []byte
	// Complete follows all physical cleanup, while the opening is still held.
	defer func() {
		// Interruption can make a physical read return EOF before its next
		// success-path check. Retain the original caller/generation cause even
		// on failed I/O, and deny Stock deposit before completion sees it.
		result = errors.Join(result, checkLocal())
		if batch.Complete == nil {
			result = errors.Join(result, errors.New("route issuer Stock completion absent"))
			return
		}
		result = errors.Join(result, batch.Complete(payload, result, checkLocal))
		clear(payload)
		if result == nil {
			result = check()
		}
	}()
	if batch.Complete == nil || batch.Deadline.IsZero() || batch.Deadline.After(end) || batch.Deadline != batch.Deadline.UTC().Truncate(time.Second) {
		return errors.New("route issuer batch original bounds invalid")
	}
	end = batch.Deadline
	if err := check(); err != nil {
		return err
	}
	release, err := p.interior.HoldControl()
	if err != nil {
		return err
	}
	defer release()
	lane, err := p.interior.Open(ctx, caller, ardp.EncodeOpen(ardp.Open{RecipientNodeID: duty.NodeID, RecipientDutyGeneration: duty.RecordGeneration, Purpose: uint8(ardp.PurposeIssuer), Deadline: end}, false))
	if err != nil {
		return err
	}
	defer lane.Finish()
	defer func() { result = errors.Join(result, lane.Close()) }()
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(interrupted); _ = lane.Close() })
	interruption := prefixSetupInterruption{stop: stop, done: interrupted}
	defer interruption.join()
	member, err := authority.Member()
	if err != nil {
		return err
	}
	secured, err := roletls.OpenRole(ctx, lane, member.PublicKey, minDeadline(end, time.Now().Add(10*time.Second)))
	if err != nil {
		return err
	}
	conn := transport.Retain(secured)
	defer func() { result = errors.Join(result, conn.Close()) }()
	if err := conn.SetDeadline(end); err != nil {
		return err
	}
	hello, err := authority.FreshHello(end, ardp.PurposeIssuer, false)
	if err != nil {
		return err
	}
	if err := presentPrefixRole(ctx, caller, conn, authority, hello, p.config.Present, p.bootstrap); err != nil {
		return err
	}
	if err := check(); err != nil {
		return err
	}
	payload, err = issuertransport.Exchange(conn, batch.Request, check)
	if err != nil {
		return err
	}
	var extra [1]byte
	if n, err := lane.Read(extra[:]); n != 0 || !errors.Is(err, io.EOF) {
		return errors.Join(errors.New("route issuer lower peer terminal required"), err)
	}
	return check()
}
