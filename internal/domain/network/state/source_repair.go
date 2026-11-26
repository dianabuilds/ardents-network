package state

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/domain/network/duty"
	"github.com/dianabuilds/ardents-network/internal/domain/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state/durable"
)

// ErrSourceRepairUnavailable denotes a history outside the bounded repair class.
var ErrSourceRepairUnavailable = errors.New("retained Source producer repair is unavailable")

// A repair holds both existing leases until the native adapter's final
// observation and the one participation commit. It never starts a State, joins
// an imaginary SourceCompletion, repairs floors or resumes an interrupted wave.
type sourceRepair struct {
	config config
	state  *durable.RetainedReader
	roles  interface {
		Remove([32]byte) error
		Close() error
	}
}

func openSourceRepair(ctx context.Context, resolved config) (_ *sourceRepair, result error) {
	if ctx == nil || ctx.Err() != nil || !resolved.sourceInfo.Serving || resolved.localRoles == "" {
		if ctx != nil {
			return nil, errors.Join(ErrSourceRepairUnavailable, ctx.Err())
		}
		return nil, ErrSourceRepairUnavailable
	}
	r := &sourceRepair{config: resolved}
	defer func() {
		if result != nil {
			result = errors.Join(result, r.close())
		}
	}()
	var err error
	r.state, err = durable.OpenRetainedReader(resolved.root, storageLimits())
	if err != nil {
		return nil, err
	}
	current, err := r.checkHistory()
	if err != nil {
		return nil, err
	}
	roles, err := duty.OpenRetainedOperation(ctx, duty.Config{Root: resolved.localRoles, Clock: resolved.clock})
	if err != nil {
		return nil, err
	}
	r.roles = roles
	if err = roles.CheckSourceProducer(sourceProducer("server", resolved.root), sourceServerDuty(current)); err != nil {
		return nil, errors.Join(ErrSourceRepairUnavailable, err)
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *sourceRepair) checkHistory() (*epoch.Decision, error) {
	current, journal, err := readRetainedHistory(r.config, r.state)
	if err != nil {
		return nil, err
	}
	now, err := trustedNow(r.config, journal)
	if err != nil || now.Before(current.Header.ValidFrom) || !now.Before(current.Snapshot.ValidUntil) {
		return nil, errors.Join(ErrSourceRepairUnavailable, err)
	}
	// Serving-only repair cannot resolve outstanding acquisition/exposure work.
	if journal.cycleActive || journal.pendingDigest != ([32]byte{}) || len(journal.history) != 0 {
		return nil, ErrSourceRepairUnavailable
	}
	return current, nil
}

func (r *sourceRepair) close() error {
	var err error
	if r.roles != nil {
		err = r.roles.Close()
	}
	if r.state != nil {
		err = errors.Join(err, r.state.Close())
	}
	return err
}
