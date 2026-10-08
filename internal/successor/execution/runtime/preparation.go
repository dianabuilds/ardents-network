package runtime

import (
	"context"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/execution"
)

// Permission is the read-only effect check implemented only by this owner’s
// opaque preparation provenance and exact live operation. Neither conveys
// Network, Admission or Service authority.
type Permission interface {
	Check() error
	localPermission()
}

// Preparation is private successful joined launch provenance under one live
// local session. It exposes no worker attachment, Grant, Principal or binding.
type Preparation struct {
	session *execution.Session
	job     *execution.Job
}

func (*Preparation) localPermission() {}

func (preparation *Preparation) Check() error {
	if preparation == nil || preparation.job == nil || !preparation.job.CompletedCurrent() {
		return errors.New("installed preparation is no longer current")
	}
	return nil
}
func (preparation *Preparation) Context() context.Context { return preparation.session.Context() }
func (preparation *Preparation) Close() error {
	if preparation == nil {
		return nil
	}
	return preparation.session.Close()
}
