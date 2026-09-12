//go:build linux

package endpoint

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
)

// initializeTextWorker checks the current local owner on both sides of the
// fixed readiness exchange. It creates no Principal or Grant and cannot replace
// installed-artifact verification or the separate joined cleanup owner.
func initializeTextWorker(ctx context.Context, attachment *textWorkerAttachment, instance textWorkerInstance, job *textJobIdentity, snapshot []byte) error {
	surface, mode := broker.Connection, textdocument.ReaderWorker
	if instance.role == "publisher" {
		surface, mode = broker.Administration, textdocument.PublisherWorker
	}
	if ctx == nil || attachment == nil || attachment.connection == nil || job == nil || job.owner == nil ||
		!textWorkerUnit(instance.name, instance.role) || attachment.pid != instance.pid || attachment.uid != instance.uid {
		return errors.New("text worker initialization is unavailable")
	}
	owner := job.owner
	owner.mu.Lock()
	nonce := job.nonce
	current := owner.liveLocked(owner.endpoint, surface) && owner.job == job && !job.retired && nonce != [32]byte{}
	owner.mu.Unlock()
	if !current {
		return errors.New("text worker initialization belongs to a retired job")
	}
	bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	stopOwner := context.AfterFunc(owner.lease.Context(), cancel)
	defer stopOwner()
	deadline, _ := bounded.Deadline()
	if err := attachment.connection.SetDeadline(deadline); err != nil {
		return errors.New("text worker initialization deadline is unavailable")
	}
	if err := textdocument.InitializeWorker(bounded, attachment, mode, nonce, snapshot); err != nil {
		return err
	}
	if bounded.Err() != nil || !owner.currentJob(owner.endpoint, surface, job, nonce) {
		return errors.New("text worker readiness belongs to a retired job")
	}
	if err := attachment.connection.SetDeadline(time.Time{}); err != nil {
		return errors.New("text worker readiness deadline could not be released")
	}
	return nil
}
