package session

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/successor/endpoint/publisher"
	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
)

func (s *Session) OpenPublication(ctx context.Context) (err error) {
	if err = s.before(ctx, actionPublicationOpen); err != nil {
		return
	}
	defer func() { err = s.after(err) }()
	if s.publication.Owner() != nil || s.prefix.Retained() || s.routeContext.Retained() || s.config.Publication == nil {
		return errors.New("live Publisher original context unavailable")
	}
	operation, ok := s.permission.(*executionruntime.Operation)
	if !ok || operation.CheckPublisher() != nil {
		return errors.New("qualified snapshot Publisher operation absent")
	}
	s.publication, err = publisher.Open(operation.Context(), *s.config.Publication, *s.config.Route, s.authority, s.holder, operation)
	return
}
func (s *Session) Publish(ctx context.Context) (link string, err error) {
	if err = s.before(ctx, actionPublish); err != nil {
		return
	}
	defer func() { err = s.after(err) }()
	if s.publication.Owner() == nil {
		return "", errors.New("live Publisher absent")
	}
	return s.publication.Owner().Publish(ctx)
}
func (s *Session) Refresh(ctx context.Context) (link string, err error) {
	if err = s.before(ctx, actionRefresh); err != nil {
		return
	}
	defer func() { err = s.after(err) }()
	if s.publication.Owner() == nil {
		return "", errors.New("live Publisher absent")
	}
	return s.publication.Owner().Refresh(ctx)
}
func (s *Session) Link(ctx context.Context) (link string, err error) {
	if err = s.before(ctx, actionLink); err != nil {
		return
	}
	defer func() { err = s.after(err) }()
	if s.publication.Owner() == nil {
		return "", errors.New("live Publisher absent")
	}
	return s.publication.Owner().Link(ctx)
}
func (s *Session) Withdraw(ctx context.Context) (err error) {
	if err = s.before(ctx, actionWithdraw); err != nil {
		return
	}
	defer func() { err = s.after(err) }()
	if s.publication.Owner() == nil {
		return errors.New("live Publisher absent")
	}
	return s.retain(s.publication.Owner().Withdraw(ctx))
}
func (s *Session) Serve(ctx context.Context) (err error) {
	if err = s.before(ctx, actionServe); err != nil {
		return
	}
	defer func() { err = s.after(err) }()
	if s.publication.Owner() == nil {
		return errors.New("qualified Text Publisher is unavailable")
	}
	return s.publication.Serve(ctx)
}
