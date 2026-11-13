package join

import (
	"context"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
	"os"
)

// Restart admits a new independent same-generation invocation after every
// original predecessor has committed retirement. It never initializes history.
func Restart(ctx context.Context, v *release.Verifier, a infrastructure.JoinAuthorization, bindingPath, distribution, history string) (returned error) {
	if ctx == nil || v == nil || os.Geteuid() != 0 || !validPath(bindingPath) || !validPath(distribution) || !validPath(history) {
		return ErrInput
	}
	if _, ok := ctx.Deadline(); !ok {
		return ErrInput
	}
	s := &startAttempt{ctx: ctx, verifier: v, authorization: a, historyOpening: 1}
	defer func() {
		s.terminal = errors.Join(s.terminal, returned)
		if s.attempted && !s.handed {
			s.joinFailedAttempt()
		}
		returned = errors.Join(s.terminal, s.close(), v.Close(), ctx.Err())
		if returned == nil && s.handed {
			returned = s.storage.publish()
		}
	}()
	if err := installedPlatform(ctx); err != nil {
		return err
	}
	h, err := ReadStarted(ctx, v, bindingPath, history)
	if err != nil {
		return err
	}
	s.completed = h.original.completed
	if h.original.retirement == nil || h.original.snapshot.number+1 >= attemptLimit {
		return ErrBinding
	}
	s.nextNumber = h.original.snapshot.number + 1
	s.predecessor = h.original.retirement.sha256
	if err := s.completed.IndependentPaths(distribution, history); err != nil {
		return err
	}
	if err := s.completed.CheckAuthorization(v, a); err != nil {
		return err
	}
	return s.launch()
}
