//go:build linux

package endpoint

import (
	"context"
	"crypto/rand"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
)

// qualifiedTextWorker is private to the verified installed launch. It is not
// an Application-provided attestation and cannot be reconstructed from wire
// identities. Its only delegated use is the already-admitted byte stream;
// Publisher administration and all network selection stay with its context.
type qualifiedTextWorker struct {
	job      *textJobIdentity
	lifetime *textWorkerLifetime
	grant    *broker.Broker
	lease    *broker.ActiveSession
}

// bindTextWorker is called only at the end of launchTextWorker's artifact,
// activation, credentials and readiness checks. Broker keeps generic mechanics;
// this Endpoint owner is responsible for the verified launch assertion.
func (owner *textContext) bindTextWorker(job *textJobIdentity, lifetime *textWorkerLifetime) (*qualifiedTextWorker, error) {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if job == nil || lifetime == nil || !owner.liveLocked(owner.endpoint, owner.surface) || owner.job != job ||
		job.retired || !job.bound || job.workerGrant != nil || lifetime.context.Err() != nil {
		return nil, errors.New("text worker Grant owner is unavailable")
	}
	var principal, generation [32]byte
	if _, err := rand.Read(principal[:]); err != nil || principal == [32]byte{} {
		return nil, errors.New("text worker Principal is unavailable")
	}
	if _, err := rand.Read(generation[:]); err != nil || generation == [32]byte{} {
		return nil, errors.New("text worker Grant generation is unavailable")
	}
	grant, err := broker.New(broker.Config{ID: generation, Grants: []broker.Grant{{Principal: principal, Surface: broker.Connection}}})
	if err != nil {
		return nil, err
	}
	capability, err := grant.Admit(principal, broker.Connection)
	if err != nil {
		grant.Close()
		return nil, err
	}
	lease, _, err := grant.Activate(job.context, capability, principal, broker.Connection)
	if err != nil {
		grant.Close()
		return nil, err
	}
	job.workerGrant = grant
	owner.verifiedJob = job
	return &qualifiedTextWorker{job: job, lifetime: lifetime, grant: grant, lease: lease}, nil
}

func (worker *qualifiedTextWorker) beginOperation(ctx context.Context, surface broker.Surface) (context.Context, func(), error) {
	if worker == nil || worker.job == nil || worker.lease == nil || ctx == nil {
		return nil, nil, errors.New("text worker operation is unavailable")
	}
	owner := worker.job.owner
	owner.mu.Lock()
	current := owner.liveLocked(owner.endpoint, surface) && owner.job == worker.job && !worker.job.retired &&
		worker.job.workerGrant == worker.grant && worker.lease.Context().Err() == nil && ctx.Err() == nil
	owner.mu.Unlock()
	if !current {
		return nil, nil, errors.New("text worker operation belongs to a retired job")
	}
	finish, err := worker.lifetime.beginUse()
	if err != nil {
		return nil, nil, err
	}
	bounded, cancel := context.WithCancel(worker.lease.Context())
	callbackDone := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { cancel(); close(callbackDone) })
	return bounded, func() {
		cancel()
		if !stop() {
			<-callbackDone
		}
		finish()
	}, nil
}

func (worker *qualifiedTextWorker) completedCurrent() bool {
	if worker == nil || worker.job == nil || worker.job.owner == nil {
		return false
	}
	owner := worker.job.owner
	owner.mu.Lock()
	defer owner.mu.Unlock()
	return owner.liveLocked(owner.endpoint, owner.surface) && owner.lastJob == worker.job && owner.job == nil &&
		worker.job.finished && worker.job.cleanupErr == nil
}

func (worker *qualifiedTextWorker) Close() error {
	if worker == nil {
		return nil
	}
	return worker.lifetime.Close()
}
