//go:build linux

package endpoint

import (
	"errors"
)

// Observe fully joined role prefixes only. Retiring an idle prefix does not
// rotate its retained members or issue tokens; a later explicit operation owns
// any new open. Source and Publisher Introduction have separate lifetimes.
func (owner *dutyContext) retirePrefixLocked() error {
	result := errors.Join(owner.source.retireIdleLocked(), owner.introduction.prefix.retireIdleLocked(), owner.responder.retireIdleLocked())
	if result != nil {
		owner.closeErr = errors.Join(owner.closeErr, result)
		owner.closed = true
		owner.endpoint.failDutyContexts(result)
	}
	return result
}
