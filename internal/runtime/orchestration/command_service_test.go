package orchestration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type runtimeCommandStub struct {
	startErr error
	stopErr  error
	fails    []string
	stops    int
}

func (s *runtimeCommandStub) StartLocked(context.Context) error { return s.startErr }
func (s *runtimeCommandStub) StopLocked(context.Context) error {
	s.stops++
	return s.stopErr
}
func (s *runtimeCommandStub) FailLocked(code, _, _, _, _, _ string) { s.fails = append(s.fails, code) }

type transportStopperStub struct {
	state   string
	stopErr error
	stops   int
}

func (s *transportStopperStub) State() string { return s.state }
func (s *transportStopperStub) Stop(context.Context) error {
	s.stops++
	return s.stopErr
}

func TestCommandServiceStartLockedRollsBackBlobExchangeFailure(t *testing.T) {
	runtime := &runtimeCommandStub{}
	transport := &transportStopperStub{state: "ready"}
	svc := NewCommandService(runtime, transport)

	_, _, err := svc.StartLocked(context.Background(), func(context.Context) error {
		return errors.New("blob exchange failed")
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "blob exchange failed")
	require.Equal(t, 1, runtime.stops)
	require.Equal(t, 1, transport.stops)
	require.Empty(t, runtime.fails)
}

func TestCommandServiceStartLockedFailsRollbackExplicitly(t *testing.T) {
	runtime := &runtimeCommandStub{stopErr: errors.New("runtime stop failed")}
	transport := &transportStopperStub{state: "ready", stopErr: errors.New("transport stop failed")}
	svc := NewCommandService(runtime, transport)

	_, _, err := svc.StartLocked(context.Background(), func(context.Context) error {
		return errors.New("blob exchange failed")
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "rollback runtime")
	require.Equal(t, []string{"node.data_plane.rollback_failed"}, runtime.fails)
}

func TestDiscoveryRefreshIntervalUsesConfiguredValue(t *testing.T) {
	require.Equal(t, 3*time.Second, DiscoveryRefreshInterval(3*time.Second))
}

func TestCommandServiceStartDiscoveryRefreshLoopTicksUntilCanceled(t *testing.T) {
	svc := NewCommandService(&runtimeCommandStub{}, &transportStopperStub{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ticks := make(chan struct{}, 2)
	svc.StartDiscoveryRefreshLoop(ctx, 10*time.Millisecond, func(context.Context) {
		select {
		case ticks <- struct{}{}:
		default:
		}
	})

	require.Eventually(t, func() bool {
		return len(ticks) > 0
	}, 500*time.Millisecond, 10*time.Millisecond)
}
