//go:build linux

package publication

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTextPublicationRefreshLifecycleStartsOnceAndRepeatedStopJoins(t *testing.T) {
	const callers = 16
	var lifecycle RefreshLifecycle
	var starts atomic.Int32
	started := make(chan struct{}, callers)
	stopping := make(chan struct{}, 1)
	cleanup := make(chan struct{})
	var cleanupOnce sync.Once
	releaseCleanup := func() { cleanupOnce.Do(func() { close(cleanup) }) }
	run := func(flight *Refresh) {
		starts.Add(1)
		started <- struct{}{}
		<-flight.Context.Done()
		stopping <- struct{}{}
		<-cleanup
	}
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(func() {
		cancel()
		releaseCleanup()
		_ = lifecycle.Stop()
	})
	flights := make([]*Refresh, callers)
	var group sync.WaitGroup
	for index := range callers {
		group.Add(1)
		go func() {
			defer group.Done()
			flights[index] = lifecycle.Start(ctx, run)
			lifecycle.Wake()
		}()
	}
	group.Wait()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("refresh scheduler did not start")
	}
	if starts.Load() != 1 {
		t.Fatalf("refresh scheduler starts = %d", starts.Load())
	}
	for _, flight := range flights {
		if flight == nil || flight != flights[0] {
			t.Fatal("concurrent start did not retain one scheduler")
		}
	}
	stopped := make(chan error, 2)
	go func() { stopped <- lifecycle.Stop() }()
	go func() { stopped <- lifecycle.Stop() }()
	select {
	case <-stopping:
	case <-time.After(time.Second):
		t.Fatal("Stop did not cancel scheduler")
	}
	select {
	case err := <-stopped:
		t.Fatalf("Stop returned before scheduler cleanup: %v", err)
	default:
	}
	releaseCleanup()
	for range 2 {
		if err := <-stopped; err != nil {
			t.Fatal(err)
		}
	}
	lifecycle.Wake()
	if got := lifecycle.Start(ctx, run); got != flights[0] || starts.Load() != 1 {
		t.Fatal("wake or start after Stop replaced the terminal scheduler")
	}
}
