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
	return startRetained(ctx, v, authorization, bindingPath, distribution, history, false)
}

// StartSelected separately admits the first invocation of a newly selected
// replacement after its exact predecessor retired. Replacement itself never
// calls this operation, and ordinary same-generation Restart stays distinct.
func StartSelected(ctx context.Context, v *release.Verifier, authorization infrastructure.ForwardingAuthorization, bindingPath, distribution, history string) error {
	return startRetained(ctx, v, authorization, bindingPath, distribution, history, true)
}

func startRetained(ctx context.Context, v *release.Verifier, authorization infrastructure.ForwardingAuthorization, bindingPath, distribution, history string, replacement bool) (returned error) {
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
		returned = errors.Join(returned, v.Close(), finishTransition(ctx))
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
	head := h.head()
	if h.snapshot == nil || h.latestRetirement == nil || head == nil || head.number+1 >= attemptLimit {
		return ErrBinding
	}
	predecessor, err := head.predecessor()
	if err != nil {
		return err
	}
	changed := h.completed.original.binding.Generation != h.snapshot.archive.Generation
	if changed != replacement {
		return ErrBinding
	}
	s.completed = h.completed // this new root attempt owns the independently opened chain
	s.nextNumber = head.number + 1
	s.predecessor = predecessor
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
