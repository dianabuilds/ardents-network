//go:build linux

package route

import (
	"errors"
	"time"
)

const closedSourceRetention = 120 * time.Second

// expire owns the finite post-work idle interval and original parent deadline.
// An admitted child's completion renews only the idle interval, never its
// authority. Pending or rejected children cannot renew idle readiness.
func (owner *closedSourceChannels) expire() {
	defer owner.workers.Done()
	for {
		owner.mu.Lock()
		if owner.terminal != nil {
			owner.mu.Unlock()
			return
		}
		until := owner.end
		if len(owner.lanes) == 0 && owner.idleUntil.Before(until) {
			until = owner.idleUntil
		}
		if !time.Now().Before(until) {
			owner.mu.Unlock()
			owner.fail(errors.New("closed source lifetime ended"))
			return
		}
		changed := owner.changed
		owner.mu.Unlock()
		_ = waitClosedSourceChange(changed, until)
	}
}

// finishAfterChannels gives autonomous expiry the same joined terminal path
// as explicit Close. Public Close also joins this watcher, without waiting
// on a watcher which is itself waiting for that public Close.
func (prefix *ClosedSourcePrefix) finishAfterChannels() {
	<-prefix.channels.done
	prefix.finish()
	close(prefix.done)
}

// Done closes only after expiry, failure or explicit Close has joined the
// source's transport workers. It conveys no permission to replace a pending
// operation or reuse its admission.
func (prefix *ClosedSourcePrefix) Done() <-chan struct{} {
	if prefix == nil {
		return nil
	}
	return prefix.done
}
