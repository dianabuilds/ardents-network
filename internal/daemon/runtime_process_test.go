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
	started := make(chan struct{})
	releaseWriter := make(chan struct{})
	writerStopped := make(chan struct{})
	done := (&RuntimeManager{}).StartDiscoveryRefreshLoop(ctx, time.Nanosecond, func(ctx context.Context) {
		close(started)
		<-ctx.Done()
		<-releaseWriter
		close(writerStopped)
	})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("discovery refresh writer did not start")
	}
	cancel()
	select {
	case <-done:
		t.Fatal("refresh loop reported stopped while its writer was still active")
	default:
	}
	close(releaseWriter)
	select {
	case <-writerStopped:
	case <-time.After(time.Second):
		t.Fatal("discovery refresh writer did not stop")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("refresh loop did not report writer shutdown")
	}
}
