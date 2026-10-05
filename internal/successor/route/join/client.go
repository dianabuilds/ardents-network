package join

import (
	"context"
	"crypto/rand"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
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
	child, cancel := context.WithDeadline(a.ctx, config.Deadline)
	callerDone := make(chan struct{})
	stopCaller := context.AfterFunc(ctx, func() { defer close(callerDone); cancel() })
	defer func() {
		if !stopCaller() {
			<-callerDone
		}
		cancel()
	}()
	physicalCheck := func() error {
		if err := errors.Join(ctx.Err(), a.originalCurrent()); err != nil {
			return err
		}
		if err := a.borrows.CheckRecipient(config.Duty, config.Deadline); err != nil {
			return err
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
	terminal, side, err := a.borrows.OpenChannel(child, ctx, config.Duty, config.Deadline, config.SetupDeadline)
	var joined *Joined
	defer func() {
		terminal.JoinSetupInterruption()
		if result != nil {
			result = errors.Join(result, a.cleanupJoin(joined, terminal))
		}
	}()
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
	err = ardp.WriteFrame(terminal.Stream(), ardp.Frame{Kind: ardp.KindOperation, Lane: 1, Body: body})
	clear(body)
	if err != nil {
		return nil, err
	}
	frame, err := ardp.ReadFrame(terminal.Stream())
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
	session, lane, err := terminal.PrepareJoined(physicalCheck)
	if err != nil {
		return nil, err
	}
	joined = &Joined{acquisition: a, session: session, lane: lane, parent: terminal, retiring: make(chan struct{}), watcherDone: make(chan struct{})}
	go joined.watch(ctx)
	session.Start()
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
func (a *JoinAcquisition) cleanupJoin(joined *Joined, terminal interface{ CloseSetup() error }) error {
	var retirement error
	if joined != nil {
		retirement = joined.closePhysical()
	} else {
		retirement = terminal.CloseSetup()
	}
	// Close racing setup waits for opening, which closes only after this
	// original physical result has been retained. Ordinary setup refusal
	// contributes nothing here when its cleanup succeeds.
	a.mu.Lock()
	a.setupRetirement = errors.Join(a.setupRetirement, retirement)
	a.mu.Unlock()
	return retirement
}
