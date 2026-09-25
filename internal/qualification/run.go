//go:build linux

package qualification

import (
	"context"
	"errors"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
	"github.com/dianabuilds/ardents-network/internal/route"
)

// Run owns state used only by the fixed qualification caller. An ordinary
// text Job has no report, observer, sampling, or joined stream state; a
// qualification Job only retains its exact run owner.
type Run struct {
	mu                  sync.Mutex
	init                streamqualification.Init
	acquireIntroduction func(context.Context) (func(), error)
	acquireSetup        func(context.Context) (func(), error)
	stopSampling        func() error
	joins               map[*route.ClosedJoinedStream]struct{}
	observe             func(context.Context, streamqualification.Report) error
	report              *streamqualification.Report
	stopOnce            sync.Once
	stopErr             error
}

func NewRun(role streamqualification.Role, profile streamqualification.Profile, seed [32]byte) (*Run, error) {
	if seed == [32]byte{} {
		return nil, errors.New("qualification workload seed is absent")
	}
	if _, err := profile.Definition(role); err != nil {
		return nil, err
	}
	return &Run{init: streamqualification.Init{Role: role, Profile: profile, Seed: seed}}, nil
}

func (run *Run) Init() streamqualification.Init {
	if run == nil {
		return streamqualification.Init{}
	}
	return run.init
}

func (run *Run) BindInvocation(nonce [32]byte) error {
	if run == nil || nonce == [32]byte{} {
		return errors.New("qualification invocation is unavailable")
	}
	run.mu.Lock()
	defer run.mu.Unlock()
	if run.init.Nonce != [32]byte{} {
		return errors.New("qualification invocation is already bound")
	}
	run.init.Nonce = nonce
	return nil
}

func (run *Run) Configure(report *streamqualification.Report,
	acquireIntroduction func(context.Context) (func(), error),
	acquireSetup func(context.Context) (func(), error),
	stopSampling func() error,
	observe func(context.Context, streamqualification.Report) error,
) error {
	if run == nil || report == nil || acquireIntroduction == nil || acquireSetup == nil || stopSampling == nil || observe == nil {
		return errors.New("qualification run configuration is unavailable")
	}
	run.mu.Lock()
	defer run.mu.Unlock()
	if run.report != nil {
		return errors.New("qualification run is already configured")
	}
	run.report = report
	run.acquireIntroduction = acquireIntroduction
	run.acquireSetup = acquireSetup
	run.stopSampling = stopSampling
	run.observe = observe
	return nil
}

func (run *Run) AcquireIntroduction(ctx context.Context) (func(), error) {
	if run == nil || run.acquireIntroduction == nil {
		return nil, errors.New("qualification introduction acquisition unavailable")
	}
	return run.acquireIntroduction(ctx)
}

func (run *Run) AcquireSetup(ctx context.Context) (func(), error) {
	if run == nil || run.acquireSetup == nil {
		return nil, errors.New("qualification setup acquisition unavailable")
	}
	return run.acquireSetup(ctx)
}

func (run *Run) Observe(ctx context.Context, report streamqualification.Report) error {
	if run == nil || run.observe == nil {
		return nil
	}
	return run.observe(ctx, report)
}

func (run *Run) StopSamples() error {
	if run == nil {
		return nil
	}
	run.stopOnce.Do(func() {
		run.mu.Lock()
		stop := run.stopSampling
		run.mu.Unlock()
		if stop != nil {
			run.stopErr = stop()
		}
	})
	return run.stopErr
}

func (run *Run) PublishReport(report streamqualification.Report) {
	if run == nil {
		return
	}
	run.mu.Lock()
	defer run.mu.Unlock()
	if run.report != nil {
		*run.report = report
	}
}

func (run *Run) RetainJoin(joined *route.ClosedJoinedStream) {
	if run == nil || joined == nil {
		return
	}
	run.mu.Lock()
	defer run.mu.Unlock()
	if run.joins == nil {
		run.joins = make(map[*route.ClosedJoinedStream]struct{})
	}
	run.joins[joined] = struct{}{}
}

func (run *Run) ReleaseJoin(joined *route.ClosedJoinedStream) {
	if run == nil {
		return
	}
	run.mu.Lock()
	delete(run.joins, joined)
	run.mu.Unlock()
}

func (run *Run) JoinedStreams() []*route.ClosedJoinedStream {
	if run == nil {
		return nil
	}
	run.mu.Lock()
	defer run.mu.Unlock()
	joins := make([]*route.ClosedJoinedStream, 0, len(run.joins))
	for joined := range run.joins {
		joins = append(joins, joined)
	}
	return joins
}

func (run *Run) Retains(joined *route.ClosedJoinedStream) bool {
	if run == nil {
		return false
	}
	run.mu.Lock()
	defer run.mu.Unlock()
	_, retained := run.joins[joined]
	return retained
}
