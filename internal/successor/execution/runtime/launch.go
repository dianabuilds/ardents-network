package runtime

import (
	"context"
	"crypto/rand"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/execution"
	"github.com/dianabuilds/ardents-network/internal/successor/execution/worker"
)

// One process-wide inventory gate covers every local owner. Per-Job locks
// cannot prevent a sibling's activation from entering the wrong baseline.
var launchGate = make(chan struct{}, 1)

// Owner owns one local authority generation and separate cleanup supervision.
type Owner struct {
	authority  *execution.Authority
	supervisor *execution.Supervisor
}

func New(config execution.Config) (*Owner, error) {
	authority, err := execution.New(config)
	if err != nil {
		return nil, err
	}
	supervisor, err := execution.NewSupervisor(authority)
	if err != nil {
		authority.Close()
		return nil, err
	}
	return &Owner{authority: authority, supervisor: supervisor}, nil
}

func (owner *Owner) Close() error {
	if owner == nil {
		return nil
	}
	return owner.supervisor.Close()
}

// Prepare consumes the exact local surface, launches the fixed installed worker
// with empty destination-free INIT, establishes its qualified local Grant and
// joins it before yielding provenance for permission bootstrap.
func (owner *Owner) Prepare(ctx context.Context, principal [32]byte, surface execution.Surface) (_ *Preparation, result error) {
	if owner == nil || ctx == nil || ctx.Err() != nil {
		return nil, errors.New("execution preparation is unavailable")
	}
	capability, err := owner.authority.Admit(principal, surface)
	if err != nil {
		return nil, err
	}
	session, err := owner.supervisor.Activate(ctx, capability, principal, surface)
	if err != nil {
		return nil, err
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, session.Close())
		}
	}()
	job, err := session.BeginJob()
	if err != nil {
		return nil, err
	}
	if err := owner.prepareJob(job, surface); err != nil {
		return nil, err
	}
	preparation := &Preparation{session: session, job: job}
	if err := errors.Join(preparation.Check(), ctx.Err()); err != nil {
		return nil, err
	}
	return preparation, nil
}

func (owner *Owner) prepareJob(job *execution.Job, surface execution.Surface) error {
	invocation, err := owner.launchJob(job, surface)
	if err != nil {
		return err
	}
	return invocation.Close()
}

func (owner *Owner) launchJob(job *execution.Job, surface execution.Surface) (_ *Invocation, result error) {
	bounded, cancel := context.WithTimeout(job.Context(), 15*time.Second)
	defer cancel()
	var activation *worker.Activation
	var cleanup *worker.Cleanup
	var grant *execution.Authority
	var lease *execution.ActiveSession
	var stop func() bool
	gateHeld := false
	transferred := false
	callbackDone := make(chan struct{})
	defer func() {
		if transferred {
			return
		}
		job.Retire()
		if grant != nil {
			grant.Close()
		}
		if lease != nil {
			lease.Release()
		}
		if stop != nil && !stop() {
			<-callbackDone
		}
		var physical error
		if activation != nil && activation.Attachment != nil {
			physical = activation.Attachment.Close()
			job.FailCleanup(physical)
		}
		if cleanup != nil {
			physical = errors.Join(physical, cleanup.Close())
		} else if activation != nil && activation.Queued {
			physical = errors.Join(physical, errors.New("original activation cleanup is unverified"))
		}
		result = errors.Join(result, job.Finish(physical))
		// An ambiguous or failed cleanup retains the global inventory slot;
		// a second owner cannot activate against an unjoined original baseline.
		if gateHeld && physical == nil {
			<-launchGate
		}
	}()
	select {
	case launchGate <- struct{}{}:
		gateHeld = true
	case <-bounded.Done():
		return nil, bounded.Err()
	}
	if err := job.Check(); err != nil {
		return nil, err
	}
	role := "reader"
	if surface == execution.Administration {
		role = "publisher"
	}
	activation, err := worker.Activate(bounded, role)
	if err != nil {
		return nil, err
	}
	if err := job.ClaimLifetime(); err != nil {
		return nil, err
	}
	cleanup, err = worker.NewCleanup(activation.Instance, job.FailCleanup)
	if err != nil {
		return nil, err
	}
	stop = context.AfterFunc(job.Context(), func() { defer close(callbackDone); _ = activation.Attachment.Close() })
	if err := activation.Artifact.Verify(); err != nil {
		return nil, err
	}
	deadline, _ := bounded.Deadline()
	if err := activation.Attachment.SetDeadline(deadline); err != nil {
		return nil, err
	}
	if err := initializeWorker(bounded, activation.Attachment, role, job.Nonce()); err != nil {
		return nil, err
	}
	if err := activation.Artifact.Verify(); err != nil {
		return nil, err
	}
	current, err := worker.ObserveInstance(bounded, activation.Instance.Name, role)
	if err != nil || current != activation.Instance {
		return nil, errors.Join(errors.New("original worker changed after READY"), err)
	}
	if err := errors.Join(job.Check(), bounded.Err()); err != nil {
		return nil, err
	}
	var principal, generation [32]byte
	if _, err := rand.Read(principal[:]); err != nil || principal == [32]byte{} {
		return nil, errors.New("worker Principal is unavailable")
	}
	if _, err := rand.Read(generation[:]); err != nil || generation == [32]byte{} {
		return nil, errors.New("worker Grant is unavailable")
	}
	grant, err = execution.New(execution.Config{ID: generation, Grants: []execution.Grant{{Principal: principal, Surface: execution.Connection}}})
	if err != nil {
		return nil, err
	}
	capability, err := grant.Admit(principal, execution.Connection)
	if err != nil {
		return nil, err
	}
	lease, _, err = grant.Activate(job.Context(), capability, principal, execution.Connection)
	if err != nil {
		return nil, err
	}
	if err := errors.Join(job.Check(), lease.Context().Err(), bounded.Err()); err != nil {
		return nil, err
	}
	if err := activation.Attachment.SetDeadline(time.Time{}); err != nil {
		return nil, err
	}
	invocation := &Invocation{job: job, activation: activation, cleanup: cleanup, grant: grant, lease: lease,
		stop: stop, callbackDone: callbackDone, done: make(chan struct{})}
	invocation.observeAttachment()
	invocation.beginJoin()
	transferred = true
	// The exact candidate is pinned and privately bound. Sibling activations
	// may now use a new baseline including this live original invocation.
	<-launchGate
	return invocation, nil
}
