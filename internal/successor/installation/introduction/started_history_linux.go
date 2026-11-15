package introduction

import (
	"context"
	"errors"
	"os"

	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// Started retains independently opened initial, Hosting and complete invocation
// history. It observes no live process or domain and grants no Stop, restart or
// replacement permission. Copies share the same joined completion.
type Started struct{ original *started }

type started struct {
	ctx       context.Context
	verifier  *release.Verifier
	completed Completed
}

// ReadStarted requires the original retained Release history, but not current
// artifact permission: expired Release metadata must not hide shutdown evidence.
// Every preceding invocation requires complete retirement; partial, foreign or
// noncontiguous histories refuse.
func ReadStarted(ctx context.Context, verifier *release.Verifier, bindingPath, history string) (result Started, returned error) {
	if ctx == nil || verifier == nil || os.Geteuid() != 0 || !validPath(history) {
		return result, ErrInput
	}
	completed, err := readCompleted(ctx, bindingPath, true)
	if err != nil {
		return result, err
	}
	result = Started{&started{ctx: ctx, verifier: verifier, completed: completed}}
	defer func() {
		if returned != nil {
			returned = errors.Join(returned, result.Close())
			result = Started{}
		}
	}()
	if completed.original.binding.History != history || completed.original.started == nil {
		return result, ErrBinding
	}
	return result, result.Check()
}

// Check reobserves original native custody between independent Release-history
// checks. Closing joins both observers before releasing either journal lease.
func (h Started) Check() error {
	if h.original == nil {
		return ErrInput
	}
	s := h.original
	return s.completed.operationObserved(func() error {
		return s.verifier.CheckHistoryPath(s.ctx, s.completed.original.binding.History)
	}, nil)
}

func (h Started) Close() error {
	if h.original == nil {
		return nil
	}
	return h.original.completed.Close()
}

// CheckRetirement requires the complete independently reopened latest retirement,
// including original domain/native joins, file births and absent publication
// blocker. A result file alone cannot pass. No new invocation is admitted.
func (h Started) CheckRetirement() error {
	if h.original == nil {
		return ErrInput
	}
	s := h.original
	return s.completed.operationObserved(func() error {
		return s.verifier.CheckHistoryPath(s.ctx, s.completed.original.binding.History)
	}, func(c *completedInitial) error {
		if c.retired == nil {
			return ErrBinding
		}
		return c.retired.check()
	})
}
