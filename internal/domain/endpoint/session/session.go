package session

import (
	"context"
	"errors"
	"net"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/publisher"
	executionruntime "github.com/dianabuilds/ardents-network/internal/domain/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/reachability"
)

// ErrPermission means the original Execution lifetime is unavailable. No result
// may be delivered and the caller must end this conversation and join Close.
var ErrPermission = errors.New("original holder permission unavailable")

// ErrPurpose refuses an operation outside the original preparation/live purpose.
var ErrPurpose = errors.New("holder operation outside original purpose")

type Config struct {
	Route       *funding.Plan
	Publication *publisher.Plan
}

// Session takes ownership of Stock only after successful construction. Its
// authority and Execution permission remain borrowed until Close joins all users.
type Session struct {
	config       Config
	authority    funding.Authority
	holder       *stock.Owner
	permission   executionruntime.Permission
	attempt      stock.Attempt
	intent       stock.IssuanceIntent
	prefix       Prefix
	routeContext JoinContext
	registration Registration
	publication  publisher.Publication
	joined       net.Conn
	history      reachability.History
	closed       bool
	closeOnce    sync.Once
	closeErr     error
}

func OpenLocal(holder *stock.Owner, authority funding.Authority, config Config) (*Session, error) {
	return open(holder, authority, config, nil)
}
func OpenPreparation(holder *stock.Owner, authority funding.Authority, config Config, permission *executionruntime.Preparation) (*Session, error) {
	if permission == nil {
		return nil, ErrPermission
	}
	return open(holder, authority, config, permission)
}
func OpenOperation(holder *stock.Owner, authority funding.Authority, config Config, permission *executionruntime.Operation) (*Session, error) {
	if permission == nil {
		return nil, ErrPermission
	}
	return open(holder, authority, config, permission)
}
func open(holder *stock.Owner, authority funding.Authority, config Config, permission executionruntime.Permission) (*Session, error) {
	if holder == nil {
		return nil, errors.New("holder absent")
	}
	if permission != nil {
		if err := permission.Check(); err != nil {
			return nil, errors.Join(ErrPermission, err)
		}
	}
	if config.Publication != nil {
		operation, ok := permission.(*executionruntime.Operation)
		if !ok || operation.CheckPublisher() != nil || config.Route == nil || config.Route.Domain != 4 {
			return nil, errors.New("qualified snapshot Publisher operation absent")
		}
		copied := *config.Publication
		config.Publication = &copied
	}
	if config.Route != nil {
		copied := *config.Route
		copied.Exclusions = append(copied.Exclusions[:0:0], copied.Exclusions...)
		config.Route = &copied
	}
	return &Session{holder: holder, authority: authority, config: config, permission: permission}, nil
}

func (s *Session) retain(err error) error {
	if operation, ok := s.permission.(*executionruntime.Operation); ok {
		operation.RetainCleanup(err)
	}
	return err
}

// Close joins private and physical children before returning Stock. Repeated
// calls return the original result. Authority and Execution remain caller-owned.
func (s *Session) Close() error {
	if s == nil {
		return nil
	}
	s.closeOnce.Do(func() {
		s.closed = true
		s.closeErr = s.retain(s.publication.Close())
		if s.joined != nil {
			s.closeErr = errors.Join(s.closeErr, s.retain(s.joined.Close()))
		}
		s.closeErr = errors.Join(s.closeErr, s.retain(s.prefix.Close()))
		if s.registration.Owner() != nil {
			s.closeErr = errors.Join(s.closeErr, s.retain(s.registration.Owner().Close()))
		}
		s.closeErr = errors.Join(s.closeErr, s.retain(s.routeContext.Close()), s.retain(s.history.Close()), s.retain(s.holder.Close()))
	})
	return s.closeErr
}

// Finish implements explicit conversation completion, including the original
// permission recheck after joined cleanup. It never acquires another operation.
func (s *Session) Finish(ctx context.Context) error {
	if err := s.before(ctx, actionClose); err != nil {
		return err
	}
	if err := s.Close(); err != nil {
		return err
	}
	return s.after(nil)
}

func (s *Session) before(ctx context.Context, action action) error {
	if s == nil || s.closed || ctx == nil {
		return errors.New("holder session closed")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.permission == nil {
		return nil
	}
	if err := s.permission.Check(); err != nil {
		return errors.Join(ErrPermission, err)
	}
	switch s.permission.(type) {
	case *executionruntime.Preparation:
		if !preparationAllows(action) {
			return errors.Join(ErrPurpose, errors.New("joined preparation grants only permission bootstrap"))
		}
	case *executionruntime.Operation:
		if !operationAllows(action) {
			return errors.Join(ErrPurpose, errors.New("execution holder grants only bounded Route preparation and Control"))
		}
	}
	return nil
}
func (s *Session) after(err error) error {
	if err != nil || s.permission == nil {
		return err
	}
	if err := s.permission.Check(); err != nil {
		return errors.Join(ErrPermission, err)
	}
	return nil
}

type action uint8

const (
	actionRequest action = iota
	actionImport
	actionBootstrapOpen
	actionBootstrapClose
	actionIssue
	actionStatus
	actionClose
	actionPrefixOpen
	actionPrefixClose
	actionReplenish
	actionLookup
	actionSubmit
	actionBegin
	actionComplete
	actionDiscard
	actionTake
	actionRefillPlan
	actionPublicationOpen
	actionPublish
	actionRefresh
	actionLink
	actionWithdraw
	actionServe
	actionJoinPrefixOpen
	actionJoinOpen
	actionJoinClose
	actionRecipient
	actionRegistrationOpen
	actionRegistrationWithdraw
	actionRegistrationClose
	actionDescriptorPublish
)

func preparationAllows(a action) bool {
	switch a {
	case actionRequest, actionImport, actionBootstrapOpen, actionBootstrapClose, actionIssue, actionStatus, actionClose:
		return true
	}
	return false
}
func operationAllows(a action) bool {
	switch a {
	case actionRequest, actionImport, actionBootstrapOpen, actionBootstrapClose, actionIssue, actionStatus, actionClose, actionPrefixOpen, actionPrefixClose, actionReplenish, actionLookup, actionSubmit, actionBegin, actionComplete, actionDiscard, actionTake, actionRefillPlan, actionPublicationOpen, actionPublish, actionRefresh, actionLink, actionWithdraw, actionServe:
		return true
	}
	return false
}

// Check reobserves the original permission before command input is interpreted.
func (s *Session) Check() error {
	if s == nil || s.closed {
		return errors.New("holder session closed")
	}
	return s.after(nil)
}
