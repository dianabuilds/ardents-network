package prefix

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/reachability"
	"github.com/dianabuilds/ardents-network/internal/successor/route"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	roletls "github.com/dianabuilds/ardents-network/internal/successor/route/transport/tls"
)

// DescriptorRefusal is the matching fixed-size terminal refusal. It retains no
// Target, path or private proof and cannot authorize a replacement operation.
type DescriptorRefusal struct{ Status uint8 }

func (err DescriptorRefusal) Error() string {
	return fmt.Sprintf("route Descriptor refused (%d)", err.Status)
}

// DescriptorAcknowledgedFailure retains a failure after this original publish
// exchange decoded a matching successful Store RESULT. It is diagnostic evidence
// only: neither this result nor its remote ACK grants joined cleanup, current
// authority or Publisher readiness. Its cause is constructed solely by Source.
type DescriptorAcknowledgedFailure struct{ cause error }

func (err *DescriptorAcknowledgedFailure) Error() string {
	if err == nil || err.cause == nil {
		return "route Descriptor acknowledgement failure absent"
	}
	return "route Descriptor failed after matching Store ACK: " + err.cause.Error()
}

func (err *DescriptorAcknowledgedFailure) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.cause
}

// PublishDescriptor carries untrusted public signed input to a genuine receiving
// Store. Success is that operation's durable Store ACK, never Publisher readiness.
func (p *Prefix) PublishDescriptor(caller context.Context, end time.Time, raw []byte, excluded []route.Member) error {
	_, err := p.descriptor(caller, end, ardp.DescriptorRequest{Operation: ardp.DescriptorPublish, Proof: raw}, nil, excluded)
	return err
}

// LookupDescriptor verifies the independently selected Target through the
// original Source and separate private History after the physical exchange joins.
func (p *Prefix) LookupDescriptor(caller context.Context, end time.Time, target [32]byte, history *reachability.History, excluded []route.Member) (reachability.Descriptor, error) {
	if history == nil {
		return reachability.Descriptor{}, errors.New("route private lookup history absent")
	}
	return p.descriptor(caller, end, ardp.DescriptorRequest{Operation: ardp.DescriptorLookup, Target: target}, history, excluded)
}

func (p *Prefix) descriptor(caller context.Context, end time.Time, request ardp.DescriptorRequest, history *reachability.History, excluded []route.Member) (_ reachability.Descriptor, result error) {
	if caller != nil {
		if bound, ok := caller.Deadline(); ok && bound.Before(end) {
			end = bound.UTC().Truncate(time.Second)
		}
	}
	if p == nil || p.bootstrap || p.config.Leg.EntryMember.RoleDomain != 1 || caller == nil || end != end.UTC().Truncate(time.Second) || !time.Now().Before(end) || end.After(p.config.Deadline) || end.After(time.Now().Add(admission.ControlClass.Lifetime())) {
		return reachability.Descriptor{}, errors.New("route Descriptor original Source or bounds invalid")
	}
	ctx, cancel, opening, err := p.beginTerminalOpening(caller, end)
	if err != nil {
		return reachability.Descriptor{}, err
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
		return reachability.Descriptor{}, err
	}
	excluded = append([]route.Member(nil), excluded...)
	duty, err := p.config.Leg.ResolutionDuty(view, excluded)
	if err != nil {
		return reachability.Descriptor{}, err
	}
	authority := role.Authority{Current: p.observeOriginal, Duty: duty, Profile: p.config.Leg.Profile}
	check := func() error {
		if err := checkLocal(); err != nil {
			return err
		}
		current, err := p.observeOriginal()
		if err != nil {
			return err
		}
		original, err := p.config.Leg.ResolutionDuty(current, excluded)
		if err != nil || original != duty {
			return errors.Join(errors.New("route original resolution duty changed"), err)
		}
		return checkLocal()
	}
	if err := check(); err != nil {
		return reachability.Descriptor{}, err
	}
	var lookup *reachability.Lookup
	if request.Operation == ardp.DescriptorLookup {
		lookup, err = history.Begin(ctx, request.Target, p.config.Leg.Profile.Network, p.config.Leg.Profile.Digest, check)
		if err != nil {
			return reachability.Descriptor{}, err
		}
		defer lookup.Finish()
		ctx = lookup.Context()
	}
	if p.queues == nil {
		return reachability.Descriptor{}, errors.New("route Descriptor queue owner absent")
	}
	release, err := p.queues.HoldChild(6 * (ardp.HeaderSize + ardp.DescriptorResultBodySize))
	if err != nil {
		return reachability.Descriptor{}, err
	}
	defer release()
	if _, err := rand.Read(request.Nonce[:]); err != nil {
		return reachability.Descriptor{}, err
	}
	body, err := ardp.EncodeDescriptorRequest(request)
	if err != nil {
		return reachability.Descriptor{}, err
	}
	defer clear(body)
	payload, err := p.exchangeDescriptor(ctx, caller, end, authority, request, body, check)
	defer clear(payload)
	if err = errors.Join(err, check()); err != nil {
		return reachability.Descriptor{}, err
	}
	if lookup != nil {
		proof, err := lookup.Complete(payload, time.Now())
		if err = errors.Join(err, check()); err != nil {
			return reachability.Descriptor{}, err
		}
		if err := proof.Current(time.Now()); err != nil {
			return reachability.Descriptor{}, err
		}
		return proof, nil
	}
	return reachability.Descriptor{}, check()
}

// exchangeDescriptor joins every role/lower resource before history completion.
// Its caller retains the original opening, context and copy reservation throughout.
func (p *Prefix) exchangeDescriptor(ctx, caller context.Context, end time.Time, authority role.Authority, request ardp.DescriptorRequest, body []byte, check func() error) (payload []byte, result error) {
	acknowledged := false
	defer func() {
		// This runs after all original physical Close results have joined.
		// Preserve their exact causes even when the remote Store already ACKed.
		if acknowledged && result != nil {
			result = &DescriptorAcknowledgedFailure{cause: result}
		}
	}()
	releaseControl, err := p.interior.HoldControl()
	if err != nil {
		return nil, err
	}
	defer releaseControl()
	lane, err := p.interior.Open(ctx, caller, ardp.EncodeOpen(ardp.Open{RecipientNodeID: authority.Duty.NodeID, RecipientDutyGeneration: authority.Duty.RecordGeneration, Purpose: uint8(ardp.PurposeReachability), Deadline: end}, false))
	if err != nil {
		return nil, err
	}
	defer lane.Finish()
	defer func() { result = errors.Join(result, lane.Close()) }()
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(interrupted); _ = lane.Close() })
	interruption := prefixSetupInterruption{stop: stop, done: interrupted}
	defer interruption.join()
	member, err := authority.Member()
	if err != nil {
		return nil, err
	}
	secured, err := roletls.OpenRole(ctx, lane, member.PublicKey, minDeadline(end, time.Now().Add(10*time.Second)))
	if err != nil {
		return nil, err
	}
	conn := transport.Retain(secured)
	defer func() { result = errors.Join(result, conn.Close()) }()
	if err := conn.SetDeadline(end); err != nil {
		return nil, err
	}
	hello, err := authority.FreshHello(end, ardp.PurposeReachability, false)
	if err != nil {
		return nil, err
	}
	if err := presentPrefixRole(ctx, caller, conn, authority, hello, p.config.Present, false); err != nil {
		return nil, err
	}
	if err := check(); err != nil {
		return nil, err
	}
	if err := ardp.WriteFrame(conn, ardp.Frame{Kind: ardp.KindOperation, Body: body}); err != nil {
		return nil, err
	}
	if err := check(); err != nil {
		return nil, err
	}
	frame, err := ardp.ReadFrame(conn)
	defer clear(frame.Body)
	if err != nil {
		return nil, err
	}
	if frame.Kind != ardp.KindResult || frame.Lane != 0 {
		return nil, errors.New("route Descriptor lane-zero RESULT required")
	}
	status, borrowed, err := ardp.DecodeDescriptorResult(frame.Body, request.Operation, request.Nonce)
	if err != nil {
		return nil, err
	}
	acknowledged = request.Operation == ardp.DescriptorPublish && status == 0
	return finishDescriptorResponse(conn, lane, borrowed, status, check)
}

// finishDescriptorResponse retains the decoded original RESULT through final
// currentness and ordered role/lower termination. Its sole caller owns the
// authenticated connection, original lane and every physical Close obligation.
func finishDescriptorResponse(conn net.Conn, lower io.Reader, borrowed []byte, status uint8, check func() error) (payload []byte, result error) {
	if err := check(); err != nil {
		return nil, err
	}
	payload = append([]byte(nil), borrowed...)
	writer, ok := conn.(interface{ CloseWrite() error })
	if !ok {
		return payload, errors.New("route Descriptor TLS half-close unavailable")
	}
	if err := writer.CloseWrite(); err != nil {
		return payload, err
	}
	var extra [1]byte
	if n, err := conn.Read(extra[:]); n != 0 || !errors.Is(err, io.EOF) {
		return payload, errors.Join(errors.New("route Descriptor TLS peer termination required"), err)
	}
	if n, err := lower.Read(extra[:]); n != 0 || !errors.Is(err, io.EOF) {
		return payload, errors.Join(errors.New("route Descriptor lower peer terminal required"), err)
	}
	if err := check(); err != nil {
		return payload, err
	}
	if status != 0 {
		return nil, DescriptorRefusal{Status: status}
	}
	return payload, nil
}
