package session

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/domain/route/introduction"
)

func (s *Session) OpenPrefix(ctx context.Context) error { return s.openPrefix(ctx, actionPrefixOpen) }
func (s *Session) OpenBootstrap(ctx context.Context) error {
	return s.openPrefix(ctx, actionBootstrapOpen)
}
func (s *Session) OpenJoinPrefix(ctx context.Context) error {
	return s.openPrefix(ctx, actionJoinPrefixOpen)
}
func (s *Session) openPrefix(ctx context.Context, a action) (err error) {
	if err = s.before(ctx, a); err != nil {
		return
	}
	defer func() { err = s.after(err) }()
	if s.publication.Owner() != nil {
		return errors.New("live Publisher retains its original Route context")
	}
	if s.prefix.Retained() {
		select {
		case <-s.prefix.Done():
			return errors.New("route prefix retired; close to retrieve outcome")
		default:
			return errors.New("route prefix already open")
		}
	}
	if s.config.Route == nil || !s.authority.Available() {
		return errors.New("route prefix unavailable")
	}
	if a == actionJoinPrefixOpen {
		if !s.routeContext.Retained() {
			s.routeContext, err = OpenJoinContext(ctx, *s.config.Route, s.authority, s.holder)
		}
		if err == nil {
			s.prefix, err = s.routeContext.Open(ctx)
		}
	} else if s.routeContext.Retained() {
		return errors.New("holder console retains its JOIN context")
	} else if a == actionBootstrapOpen {
		s.prefix, err = OpenBootstrap(ctx, *s.config.Route, s.authority, s.holder)
	} else {
		s.prefix, err = OpenPrefix(ctx, *s.config.Route, s.authority, s.holder)
	}
	return
}
func (s *Session) ClosePrefix(ctx context.Context) error {
	return s.closePrefix(ctx, actionPrefixClose)
}
func (s *Session) CloseBootstrap(ctx context.Context) error {
	return s.closePrefix(ctx, actionBootstrapClose)
}
func (s *Session) closePrefix(ctx context.Context, a action) (err error) {
	if err = s.before(ctx, a); err != nil {
		return
	}
	defer func() { err = s.after(err) }()
	if a == actionBootstrapClose && !s.prefix.Bootstrap() {
		return errors.New("route bootstrap absent")
	}
	if !s.prefix.Retained() {
		return errors.New("route prefix absent")
	}
	err = s.retain(s.prefix.Close())
	if err == nil {
		s.prefix = Prefix{}
	}
	return
}
func (s *Session) Replenish(ctx context.Context) (err error) {
	if err = s.before(ctx, actionReplenish); err != nil {
		return
	}
	defer func() { err = s.after(err) }()
	return s.prefix.Replenish(ctx)
}
func (s *Session) Issue(ctx context.Context, class uint8, receivers [][32]byte) (err error) {
	if err = s.before(ctx, actionIssue); err != nil {
		return
	}
	defer func() { err = s.after(err) }()
	return s.prefix.Issue(ctx, class, receivers)
}
func (s *Session) LookupDescriptor(ctx context.Context, target [32]byte) (raw []byte, err error) {
	if err = s.before(ctx, actionLookup); err != nil {
		return
	}
	defer func() { err = s.after(err) }()
	return s.prefix.LookupDescriptor(ctx, target, &s.history)
}
func (s *Session) PublishDescriptor(ctx context.Context, raw []byte) (err error) {
	if err = s.before(ctx, actionDescriptorPublish); err != nil {
		return
	}
	defer func() { err = s.after(err) }()
	return s.prefix.PublishDescriptor(ctx, raw)
}
func (s *Session) SubmitCapsule(ctx context.Context, recipient Recipient, raw []byte) (err error) {
	if err = s.before(ctx, actionSubmit); err != nil {
		return
	}
	defer func() { err = s.after(err) }()
	return s.prefix.SubmitCapsule(ctx, recipient, raw)
}
func (s *Session) SealSubmitCapsule(ctx context.Context, recipient Recipient, intent CapsuleIntent) (err error) {
	defer clear(intent.Request)
	if err = s.before(ctx, actionSubmit); err != nil {
		return
	}
	defer func() { err = s.after(err) }()
	return s.prefix.SealSubmitCapsule(ctx, recipient, intent)
}
func (s *Session) Recipient(ctx context.Context, choice uint8) (recipient Recipient, err error) {
	if err = s.before(ctx, actionRecipient); err != nil {
		return
	}
	defer func() { err = s.after(err) }()
	return s.prefix.Recipient(choice)
}
func (s *Session) OpenJoin(ctx context.Context, intent JoinIntent) (err error) {
	if err = s.before(ctx, actionJoinOpen); err != nil {
		return
	}
	defer func() { err = s.after(err) }()
	if s.joined != nil {
		return errors.New("route JOIN unavailable")
	}
	s.joined, err = s.prefix.Join(ctx, intent)
	return
}
func (s *Session) CloseJoin(ctx context.Context) (err error) {
	if err = s.before(ctx, actionJoinClose); err != nil {
		return
	}
	defer func() { err = s.after(err) }()
	if s.joined == nil {
		return errors.New("route JOIN absent")
	}
	err = s.joined.Close()
	if err == nil {
		s.joined = nil
	}
	return
}
func (s *Session) OpenRegistration(ctx context.Context, revision uint64) (facts introduction.RegistrationFacts, err error) {
	if err = s.before(ctx, actionRegistrationOpen); err != nil {
		return
	}
	defer func() { err = s.after(err) }()
	if s.registration.Owner() != nil {
		select {
		case <-s.registration.Owner().Done():
			err = errors.New("route registration retired; close to retrieve outcome")
		default:
			err = errors.New("route registration already open")
		}
		return
	}
	s.registration, err = s.prefix.Register(ctx, revision)
	if err == nil {
		facts = s.registration.Facts()
	}
	return
}
func (s *Session) WithdrawRegistration(ctx context.Context) (err error) {
	if err = s.before(ctx, actionRegistrationWithdraw); err != nil {
		return
	}
	defer func() { err = s.after(err) }()
	if s.registration.Owner() == nil {
		return errors.New("route registration absent")
	}
	err = s.registration.Owner().Withdraw(ctx)
	if err == nil {
		s.registration = Registration{}
	}
	return
}
func (s *Session) CloseRegistration(ctx context.Context) (err error) {
	if err = s.before(ctx, actionRegistrationClose); err != nil {
		return
	}
	defer func() { err = s.after(err) }()
	if s.registration.Owner() == nil {
		return errors.New("route registration absent")
	}
	err = s.registration.Owner().Close()
	if err == nil {
		s.registration = Registration{}
	}
	return
}
