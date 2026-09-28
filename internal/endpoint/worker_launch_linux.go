//go:build linux

package endpoint

import (
	"context"
	"errors"
	"time"
	"unicode/utf8"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
	"github.com/dianabuilds/ardents-network/internal/endpoint/worker"
	"github.com/dianabuilds/ardents-network/internal/qualification"
)

// The installed sockets and their PID/UID instance namespace belong to the
// process, not to one Endpoint value. Keep the inventory baseline, activation
// and identity observation serial across every Endpoint in this process.
var workerLaunchGate = make(chan struct{}, 1)

// launchWorker owns a local job from reservation through verified readiness.
// No caller supplies a worker identity, artifact digest, isolation flag, socket,
// executable, Principal or Grant. All of those observations are obtained here.
func (owner *textContext) launchWorker(ctx context.Context, snapshot []byte) (*qualifiedWorker, error) {
	workload, err := documentServiceWorkloadBounds()
	if err != nil {
		return nil, err
	}
	return owner.launchInstalledWorker(ctx, snapshot, nil, workload)
}

func (owner *textContext) launchStreamQualificationWorker(ctx context.Context, run *qualification.Run) (*qualifiedWorker, error) {
	role := streamqualification.ReaderRole
	if owner != nil && owner.surface == broker.Administration {
		role = streamqualification.PublisherRole
	}
	if run == nil || run.Init().Role != role {
		return nil, errors.New("qualification workload role is unavailable")
	}
	workload, err := streamQualificationServiceWorkloadBounds()
	if err != nil {
		return nil, err
	}
	return owner.launchInstalledWorker(ctx, nil, run, workload)
}

func (owner *textContext) launchInstalledWorker(ctx context.Context, snapshot []byte, run *qualification.Run,
	workload serviceWorkloadBounds) (*qualifiedWorker, error) {
	if owner == nil || ctx == nil || ctx.Err() != nil {
		return nil, errors.New("text worker launch is unavailable")
	}
	if _, _, err := workload.direction(owner.surface); err != nil {
		return nil, err
	}
	role := "reader"
	if owner.surface == broker.Administration {
		role = "publisher"
	}
	if len(snapshot) > textdocument.MaximumBytes || !utf8.Valid(snapshot) || role == "reader" && len(snapshot) != 0 {
		return nil, errors.New("text worker snapshot is invalid")
	}
	job, err := owner.beginJob(owner.endpoint, owner.surface)
	if err != nil {
		return nil, err
	}
	// Before activation a refused launch has no process-cleanup obligation.
	// The worker mechanism reports a dialed-but-failed activation through a
	// non-nil result so this cleanup keeps its exact original position.
	transferred := false
	var activation *worker.Activation
	var release func()
	defer func() {
		if !transferred {
			owner.retireJob(job)
			var cleanupErr error
			if activation != nil && activation.Attachment != nil {
				cleanupErr = activation.Attachment.Close()
			}
			if activation != nil && activation.Queued {
				cleanupErr = errors.Join(cleanupErr, errors.New("text worker activation cleanup is unverified"))
			}
			_ = owner.finishJobCleanup(job, cleanupErr)
		}
		if release != nil {
			release()
		}
	}()
	job.workload = workload
	inventory := worker.Text
	if run != nil {
		if err := run.BindInvocation(job.nonce); err != nil {
			return nil, err
		}
		inventory = worker.Stream
		job.qualification = run
	}
	bounded, cancel := context.WithTimeout(job.context, 15*time.Second)
	defer cancel()
	stopCaller := context.AfterFunc(ctx, cancel)
	defer stopCaller()
	release, err = owner.endpoint.acquireTextLaunch(bounded)
	if err != nil {
		return nil, err
	}
	if job.context.Err() != nil {
		return nil, errors.New("text worker launch was revoked")
	}
	activation, err = worker.Activate(bounded, inventory, role)
	if err != nil {
		return nil, err
	}
	// The lifetime takes over before INIT, including failure and cancellation.
	transferred = true
	lifetime, err := initializeOwnedWorker(job.context, bounded, activation.Attachment, activation.Instance, job, snapshot, activation.Artifact)
	if err != nil {
		return nil, err
	}
	refuse := func(cause error) (*qualifiedWorker, error) { return nil, errors.Join(cause, lifetime.Close()) }
	if err := activation.Artifact.Verify(); err != nil {
		return refuse(err)
	}
	current, err := worker.ObserveInstance(bounded, activation.Instance.Name, role)
	if err != nil || current != activation.Instance {
		return refuse(errors.New("text worker invocation changed before Grant"))
	}
	if bounded.Err() != nil || ctx.Err() != nil {
		return refuse(errors.New("text worker launch was cancelled"))
	}
	// Grant creation is private to this fully observed launch, after readiness
	// and before any Connection can be given to the worker.
	bound, err := owner.bindWorker(job, lifetime)
	if err != nil {
		return refuse(err)
	}
	return bound, nil
}

func (endpoint *endpoint) acquireTextLaunch(ctx context.Context) (func(), error) {
	select {
	case workerLaunchGate <- struct{}{}:
		if ctx.Err() != nil || !endpoint.textAvailable() {
			<-workerLaunchGate
			return nil, errors.New("text worker activation is unavailable")
		}
		return func() { <-workerLaunchGate }, nil
	case <-ctx.Done():
		return nil, errors.New("text worker activation was cancelled")
	}
}
