package runtime

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/successor/execution"
)

// AdministrationContext identifies the original admitted outer session of a
// qualified Publisher operation. Copies retain that same identity and joined
// retirement signal, never a Job, worker lease or replacement session. It
// grants no Publication, private recipient or Service Connection authority.
type AdministrationContext struct {
	session *execution.Session
}

func (operation *Operation) AdministrationContext() (AdministrationContext, error) {
	if err := operation.CheckPublisher(); err != nil {
		return AdministrationContext{}, err
	}
	if err := operation.administration.Check(operation); err != nil {
		return AdministrationContext{}, err
	}
	return operation.administration, nil
}

// Check requires this original still-live context and its exact qualified
// operation. Context identity alone never makes a retired worker usable.
func (original AdministrationContext) Check(operation *Operation) error {
	if original.session == nil || operation == nil || operation.administration != original {
		return errors.New("original Administration context is unavailable")
	}
	return errors.Join(original.session.Check(), operation.CheckPublisher())
}

// Context signals original session revocation. Worker retirement is separate.
func (original AdministrationContext) Context() context.Context {
	if original.session == nil {
		return nil
	}
	return original.session.Context()
}

// Done signals the original context's terminal cleanup report. Retained private
// history must also check Completion: a failed report is not successful join.
func (original AdministrationContext) Done() <-chan struct{} {
	if original.session == nil {
		return nil
	}
	return original.session.Done()
}

// Completion retains the original session's terminal cleanup result. Done
// alone cannot authorize erasure after an unverified or failed physical join.
func (original AdministrationContext) Completion() (error, bool) {
	return original.session.Completion()
}
