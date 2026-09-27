//go:build linux

package endpoint

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
	"github.com/dianabuilds/ardents-network/internal/endpoint/worker"
)

// initializeTextWorker checks the current local owner on both sides of the
// fixed readiness exchange. It creates no Principal or Grant and cannot replace
// installed-artifact verification or the separate joined cleanup owner.
func initializeTextWorker(ctx context.Context, attachment *worker.Attachment, instance worker.Instance, job *textJobIdentity, snapshot []byte) error {
	surface, mode := broker.Connection, textdocument.ReaderWorker
	if instance.Role == "publisher" {
		surface, mode = broker.Administration, textdocument.PublisherWorker
	}
	if ctx == nil || !attachment.Connected() || job == nil || job.owner == nil ||
		!worker.ValidUnit(instance.Name, instance.Role) || attachment.PID() != instance.PID || attachment.UID() != instance.UID {
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
	if err := attachment.SetDeadline(deadline); err != nil {
		return errors.New("text worker initialization deadline is unavailable")
	}
	var initializationErr error
	if job.qualification != nil {
		if worker.OfUnit(instance.Name) != worker.Stream || job.qualification.Init().Nonce != nonce {
			return errors.New("qualification worker inventory or invocation differs")
		}
		initializationErr = streamqualification.InitializeWorker(attachment, job.qualification.Init())
	} else {
		if worker.OfUnit(instance.Name) != worker.Text {
			return errors.New("text worker inventory differs")
		}
		initializationErr = textdocument.InitializeWorker(bounded, attachment, mode, nonce, snapshot)
	}
	if initializationErr != nil {
		return initializationErr
	}
	if bounded.Err() != nil || !owner.currentJob(owner.endpoint, surface, job, nonce) {
		return errors.New("text worker readiness belongs to a retired job")
	}
	if err := attachment.SetDeadline(time.Time{}); err != nil {
		return errors.New("text worker readiness deadline could not be released")
	}
	return nil
}
