package waku

import (
	"ardents/internal/network"
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type closedReachabilitySubscription struct {
	out      chan any
	outCalls atomic.Int64
}

func (s *closedReachabilitySubscription) Out() <-chan any {
	s.outCalls.Add(1)
	return s.out
}

func (*closedReachabilitySubscription) Close() error { return nil }
func (*closedReachabilitySubscription) Name() string { return "closed-reachability-test" }

func TestRuntimeLoopDoesNotSpinOnClosedReachabilitySubscription(t *testing.T) {
	events := make(chan any)
	close(events)
	subscription := &closedReachabilitySubscription{out: events}
	service := New(network.Config{ReachabilityMode: network.ReachabilityPublicDirect})
	service.reachabilityEvents = subscription
	service.reachability = network.ReachabilitySnapshot{
		Mode: network.ReachabilityPublicDirect, State: "public", Reachable: true,
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go service.runRuntimeLoop(ctx, done)
	time.Sleep(20 * time.Millisecond)
	cancel()
	<-done

	require.LessOrEqual(t, subscription.outCalls.Load(), int64(3))
	require.Nil(t, service.reachabilityEvents)
	require.False(t, service.reachability.Reachable)
	require.Equal(t, "unknown", service.reachability.State)
	require.Equal(t, "public ingress observation stream closed", service.reachability.Reason)
}
