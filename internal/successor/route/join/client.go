package join

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
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
	if a == nil || a.self != a || ctx == nil || config.Secret == [32]byte{} || config.Context == [32]byte{} || !time.Now().Before(config.Deadline) || config.Deadline != config.Deadline.UTC().Truncate(time.Second) || config.Deadline.After(time.Now().Add(admission.ForwardClass.Lifetime())) || !time.Now().Before(config.SetupDeadline) || config.SetupDeadline != config.SetupDeadline.UTC().Truncate(time.Second) || config.SetupDeadline.After(config.Deadline) {
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
		if err := errors.Join(ctx.Err(), a.caller.Err()); err != nil {
			return err
		}
		// Prefix validates each original leg and its exact recipient together
		// against a fresh view. Scoped claims additionally check their original
		// logical owners before that observation. Do not repeat the same full
		// owner checks immediately ahead of this read-only validation.
		if err := a.borrows.CheckRecipient(config.Duty, config.Deadline); err != nil {
			return err
		}
		return errors.Join(ctx.Err(), a.originalCurrent())
	}
	check := func() error {
		if err := a.checkLocal(); err != nil {
			return err
		}
		// physicalCheck already fully observes the original owners with and
		// after the recipient check. Fence it locally instead of observing those same
		// owners twice more without an intervening exchange or handoff.
		return errors.Join(physicalCheck(), a.checkLocal())
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
	joined = &Joined{acquisition: a, session: session, lane: lane, parent: terminal, retiring: make(chan struct{}), watcherDone: make(chan struct{}), end: config.Deadline,
		setupEnd: config.SetupDeadline, side: side, secretDigest: sha256.Sum256(config.Secret[:]), intent: config.Context}
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
func (a *JoinAcquisition) cleanupJoin(joined *Joined, terminal interface {
	CloseSetup() error
	SetupCleanupResult() (error, bool)
}) error {
	var retirement error
	var cleanup error
	if joined != nil {
		retirement = joined.closePhysical()
		cleanup = joined.cleanup
	} else {
		retirement = terminal.CloseSetup()
		var complete bool
		cleanup, complete = terminal.SetupCleanupResult()
		if !complete {
			cleanup = errors.Join(cleanup, errors.New("original JOIN setup cleanup did not join"))
			retirement = errors.Join(retirement, cleanup)
		}
	}
	// Close racing setup waits for opening, which closes only after this
	// original physical result has been retained. Ordinary setup refusal
	// contributes nothing here when its cleanup succeeds.
	a.mu.Lock()
	a.setupRetirement = errors.Join(a.setupRetirement, retirement)
	a.setupCleanup = errors.Join(a.setupCleanup, cleanup)
	a.mu.Unlock()
	return retirement
}
