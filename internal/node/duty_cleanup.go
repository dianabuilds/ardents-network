package node

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/node/hosting"
)

// dutyCleanup owns the process's shared Hosting handle and local-role record.
// Run is its sole caller. A timed-out, unjoined role leaves the conflict record
// to expire at its authenticated NotAfter rather than claiming completed cleanup.
type dutyCleanup struct {
	host      *hosting.Lifetime
	release   func() error
	retained  bool
	unjoined  bool
	attempted bool
	err       error
}

func (cleanup *dutyCleanup) deferUntil(joined <-chan struct{}) {
	if cleanup == nil || joined == nil || dutyJoined(joined) {
		return
	}
	cleanup.unjoined = true
	cleanup.host.DeferCloseUntil(joined)
}

func (cleanup *dutyCleanup) Close() error {
	if cleanup == nil {
		return nil
	}
	if cleanup.attempted {
		return cleanup.err
	}
	cleanup.attempted = true
	if cleanup.unjoined {
		return nil
	}
	if cleanup.host != nil {
		cleanup.err = cleanup.host.Close()
		if cleanup.err != nil {
			return cleanup.err
		}
	}
	if cleanup.retained {
		if cleanup.release == nil {
			cleanup.err = errors.New("local role release is unavailable")
			return cleanup.err
		}
		cleanup.err = cleanup.release()
	}
	return cleanup.err
}
