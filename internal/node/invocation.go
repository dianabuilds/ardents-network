package node

import (
	"context"
	"errors"
	"time"
)

// A timeout reports unavailability but cannot make the original Node or State
// appear joined. The final result is retained until physical completion.
func awaitNodeCompletion(joined <-chan InvocationCompletion, timeout <-chan time.Time, notify func()) (InvocationCompletion, bool) {
	select {
	case result := <-joined:
		return result, false
	case <-timeout:
		notify()
		return <-joined, true
	}
}

// InvocationRole is the termination contract of an already running Node role.
type InvocationRole interface {
	Stop()
	Done() <-chan struct{}
	Close() error
	JoinedOutcome() string
	JoinedFailureOwners() (string, string)
}

// InvocationCompletion describes the physically joined invocation. The original
// authority is retained until the role has joined, including after a timeout.
type InvocationCompletion struct {
	Outcome               string
	FirstOwner            string
	CleanupOwner          string
	AuthorityReleaseError error
	AnnouncementError     error
	TimedOut              bool
}

// SuperviseInvocation announces a running role, waits for cancellation or role
// retirement, and joins it before releasing its borrowed authority. The timeout
// callback reports slow retirement without releasing the authority early.
func SuperviseInvocation(ctx context.Context, original InvocationRole, wait time.Duration, announce func() error, unavailable func(), releaseAuthority func() error) InvocationCompletion {
	announcementErr := announce()
	if announcementErr == nil {
		select {
		case <-ctx.Done():
		case <-original.Done():
		}
	}
	original.Stop()
	joined := make(chan InvocationCompletion, 1)
	go func() {
		_ = original.Close()
		first, cleanup := original.JoinedFailureOwners()
		joined <- InvocationCompletion{
			Outcome: original.JoinedOutcome(), FirstOwner: first, CleanupOwner: cleanup,
			AuthorityReleaseError: releaseAuthority(), AnnouncementError: announcementErr,
		}
	}()
	timer := time.NewTimer(wait)
	defer timer.Stop()
	result, timedOut := awaitNodeCompletion(joined, timer.C, unavailable)
	result.TimedOut = timedOut
	return result
}

// Route's watcher can observe this invocation's own already-denied admission
// while SIGINT cleanup is joining. Only that exact unwrapped local notice is
// not a cleanup failure. Foreign causes and single-error physical wrappers are
// retained, even if they wrap cancellation or this notice. This does not infer
// successful physical completion: Receiver.Close has already joined its users.
func originalRetirementResult(result, notice error) error {
	retained, _ := removeOriginalRetirementNotice(result, notice)
	return retained
}

// Preserve every unchanged error node: another owner may subsequently remove
// its exact retained operational subtree while keeping physical wrappers.
func removeOriginalRetirementNotice(result, notice error) (error, bool) {
	if result == notice {
		return nil, result != nil
	}
	if joined, ok := result.(interface{ Unwrap() []error }); ok {
		var retained []error
		changed := false
		for _, cause := range joined.Unwrap() {
			next, removed := removeOriginalRetirementNotice(cause, notice)
			retained = append(retained, next)
			changed = changed || removed
		}
		if changed {
			return errors.Join(retained...), true
		}
	}
	return result, false
}
