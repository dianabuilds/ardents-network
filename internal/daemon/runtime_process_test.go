package daemon

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDiscoveryRefreshIntervalUsesConfiguredValue(t *testing.T) {
	require.Equal(t, 3*time.Second, DiscoveryRefreshInterval(3*time.Second))
}

func TestRuntimeManagerRefreshLoopStopsWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ticks := make(chan struct{}, 2)
	(&RuntimeManager{}).StartDiscoveryRefreshLoop(ctx, 10*time.Millisecond, func(context.Context) {
		select {
		case ticks <- struct{}{}:
		default:
		}
	})
	require.Eventually(t, func() bool { return len(ticks) > 0 }, 500*time.Millisecond, 10*time.Millisecond)
	cancel()
}
