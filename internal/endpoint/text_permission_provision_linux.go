//go:build linux

package endpoint

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint/permissionfile"
)

// provisionTextPermission is the participant's finite offline handover. The
// observer receives only the public request digest; it supplies no success or
// permission authority. Its synchronous implementation must honor the supplied bounded context and join its output before return. Only an actual matching Custody file can complete provisioning.
func (owner *textContext) provisionTextPermission(ctx context.Context, requestPath, responsePath string, maxima [3]uint32, report func(context.Context, [32]byte) error) (outcome error) {
	if owner == nil || ctx == nil || ctx.Err() != nil || report == nil || requestPath == responsePath {
		return errors.New("text permission provisioning unavailable")
	}
	response, err := permissionfile.Open(responsePath)
	if err != nil {
		return err
	}
	defer func() { outcome = errors.Join(outcome, response.Close()) }()
	// A new authorized context has no qualification receipt. Obtain one from
	// the real installed launcher, then join the preparation invocation before
	// exporting anything or waiting for offline approval. Empty INIT is valid
	// for both roles; it never opens a Service stream or publishes a document.
	owner.mu.Lock()
	qualified := owner.verifiedJob != nil && owner.verifiedJob.owner == owner && owner.verifiedJob.workerGrant != nil
	owner.mu.Unlock()
	if !qualified {
		worker, err := owner.launchTextWorker(ctx, nil)
		if err != nil {
			return err
		}
		if err := errors.Join(worker.Close(), ctx.Err()); err != nil {
			return err
		}
		if !worker.completedCurrent() {
			return errors.New("text permission preparation context changed")
		}
	}

	digest, err := owner.exportTextPermissionFile(ctx, requestPath, maxima)
	if err != nil {
		return err
	}
	owner.mu.Lock()
	pending := owner.permission
	if pending == nil || pending.digest != digest {
		owner.mu.Unlock()
		return errors.New("text permission request owner changed")
	}
	expiry := pending.request.Permission.NotAfter
	owner.mu.Unlock()
	bounded, cancel := context.WithDeadline(ctx, expiry)
	defer cancel()
	interrupted := make(chan struct{})
	stop := context.AfterFunc(owner.lease.Context(), func() { defer close(interrupted); cancel() })
	defer func() {
		if !stop() {
			<-interrupted
		}
	}()
	if err := report(bounded, digest); err != nil {
		return err
	}
	for {
		if err := bounded.Err(); err != nil {
			return err
		}
		present, err := response.Present()
		if err != nil {
			return err
		}
		if present {
			// The import reopens and identity-checks the canonical owner-only path.
			// Invalid or partially written responses fail; they are never retried into
			// success. The operator installs a complete response before exposing it.
			return owner.importTextPermissionFile(bounded, responsePath, digest)
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-bounded.Done():
			timer.Stop()
			return bounded.Err()
		case <-timer.C:
		}
	}
}
