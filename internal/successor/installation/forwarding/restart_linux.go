package forwarding

import (
	"context"
	"errors"
	"os"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// Restart admits a new invocation of the same immutable generation only after
// independently reading every predecessor start and committed retirement. Fresh
// Release/native/startup admission is repeated; no old runtime owner transfers.
func Restart(ctx context.Context, v *release.Verifier, authorization infrastructure.ForwardingAuthorization, bindingPath, distribution, history string) (returned error) {
	if ctx == nil || v == nil || os.Geteuid() != 0 || !validPath(bindingPath) || !validPath(distribution) || !validPath(history) {
		return ErrInput
	}
	if _, bounded := ctx.Deadline(); !bounded {
		return ErrInput
	}
	s := &startAttempt{ctx: ctx, verifier: v, authorization: authorization}
	defer func() {
		s.terminal = errors.Join(s.terminal, returned)
		if s.attempted && !s.handed {
			s.joinFailedAttempt()
		}
		returned = errors.Join(s.terminal, s.close())
		if returned == nil && s.handed {
			returned = s.storage.publish()
		}
	}()
	if err := installedPlatform(ctx); err != nil {
		return err
	}
	var err error
	s.history, err = ReadStarted(ctx, v, bindingPath, history)
	if err != nil {
		return err
	}
	h := s.history.original
	if h.latestRetirement == nil || h.snapshot.number+1 >= attemptLimit {
		return ErrBinding
	}
	s.completed = h.completed // this new root attempt owns the independently opened chain
	s.nextNumber = h.snapshot.number + 1
	s.predecessor = h.latestRetirement.sha256
	if err := s.completed.IndependentPaths(distribution, history); err != nil {
		return err
	}
	if err := s.completed.CheckAuthorization(v, authorization); err != nil {
		return err
	}
	if err := s.history.Check(); err != nil {
		return err
	}
	return s.launch()
}
