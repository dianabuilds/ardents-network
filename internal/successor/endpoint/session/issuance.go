package session

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/endpoint/funding"
)

func (s *Session) Request(ctx context.Context, maxima [3]uint32) (raw []byte, digest [32]byte, err error) {
	if err = s.before(ctx, actionRequest); err != nil {
		return
	}
	defer func() { err = s.after(err) }()
	return s.holder.Request(maxima)
}
func (s *Session) Import(ctx context.Context, digest [32]byte, raw []byte) (err error) {
	if err = s.before(ctx, actionImport); err != nil {
		return
	}
	defer func() { err = s.after(err) }()
	return s.holder.Import(digest, raw)
}
func (s *Session) validateIntent(intent stock.IssuanceIntent) error {
	if !s.authority.Available() {
		return nil
	}
	view, err := s.authority.Current()
	if err != nil {
		return err
	}
	return funding.ValidateIntent(view, intent)
}

// Begin retains a detached intent for the completion-time Network recheck.
// The caller may reuse its challenge slice after this operation returns.
func (s *Session) Begin(ctx context.Context, intent stock.IssuanceIntent) (raw []byte, deadline time.Time, err error) {
	if err = s.before(ctx, actionBegin); err != nil {
		return
	}
	defer func() { err = s.after(err) }()
	intent.Challenges = append(intent.Challenges[:0:0], intent.Challenges...)
	if err = s.validateIntent(intent); err != nil {
		return
	}
	var next stock.Attempt
	next, err = s.holder.Begin(intent)
	if err != nil {
		return
	}
	s.attempt, s.intent = next, intent
	return s.attempt.Request()
}
func (s *Session) Complete(ctx context.Context, raw []byte, failed bool) (err error) {
	if err = s.before(ctx, actionComplete); err != nil {
		return
	}
	defer func() { err = s.after(err) }()
	if err = s.validateIntent(s.intent); err != nil {
		return
	}
	if failed {
		return s.attempt.Complete(raw, errors.New("exchange failed"))
	}
	return s.attempt.CompleteBound(raw, nil, ctx.Err)
}
func (s *Session) Discard(ctx context.Context) (err error) {
	if err = s.before(ctx, actionDiscard); err != nil {
		return
	}
	defer func() { err = s.after(err) }()
	s.attempt.Discard()
	return nil
}
func (s *Session) Take(ctx context.Context, presentation stock.Presentation, class uint8) (raw []byte, err error) {
	if err = s.before(ctx, actionTake); err != nil {
		return
	}
	defer func() {
		err = s.after(err)
		if err != nil {
			clear(raw)
			raw = nil
		}
	}()
	if s.authority.Available() {
		if err = s.authority.Presentation(presentation); err != nil {
			return
		}
	}
	raw, err = s.holder.Take(ctx, presentation, class)
	if err == nil && s.authority.Available() {
		err = s.authority.Presentation(presentation)
	}
	return
}
func (s *Session) RefillPlan(ctx context.Context, receivers [][32]byte, class uint8, token bool) (result [][32]byte, err error) {
	if err = s.before(ctx, actionRefillPlan); err != nil {
		return
	}
	defer func() { err = s.after(err) }()
	return s.holder.RefillPlan(receivers, class, token)
}
func (s *Session) Status(ctx context.Context) (status stock.Status, err error) {
	if err = s.before(ctx, actionStatus); err != nil {
		return
	}
	defer func() { err = s.after(err) }()
	return s.holder.Status(), nil
}
