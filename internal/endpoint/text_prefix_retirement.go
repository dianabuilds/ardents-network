//go:build linux

package endpoint

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// Observe fully joined role prefixes only. Retiring an idle prefix does not
// rotate its retained members or issue tokens; a later explicit operation owns
// any new open. Source and Publisher Introduction have separate lifetimes.
func (owner *textContext) retireTextPrefixLocked() error {
	var result error
	for _, role := range []struct {
		prefix **route.ClosedSourcePrefix
		cancel *context.CancelFunc
	}{{&owner.prefix, &owner.prefixCancel}, {&owner.introduction.prefix, &owner.introduction.cancel}, {&owner.responder.prefix, &owner.responder.cancel}} {
		prefix := *role.prefix
		if prefix == nil {
			continue
		}
		select {
		case <-prefix.Done():
		default:
			continue
		}
		err := prefix.Close()
		if *role.cancel != nil {
			(*role.cancel)()
		}
		*role.prefix, *role.cancel = nil, nil
		result = errors.Join(result, err)
	}
	if result != nil {
		owner.closeErr = errors.Join(owner.closeErr, result)
		owner.closed = true
		owner.endpoint.failTextContexts(result)
	}
	return result
}
