package state

import (
	"context"
	"testing"
	"time"
)

func TestAutomaticRefreshRecordsTerminalFailure(t *testing.T) {
	owner := &networkState{}
	ticks := make(chan time.Time)
	owner.work.Add(1)
	go owner.runAutomaticRefresh(context.Background(), ticks, nil)
	ticks <- time.Now()
	owner.work.Wait()
	if owner.automaticErr == nil {
		t.Fatal("terminal automatic refresh failure was not retained")
	}
	if _, err := owner.Current(); err == nil {
		t.Fatal("Current accepted State after terminal automatic refresh failure")
	}
}

func TestWaitReportsTerminalAutomaticFailureAlongsideSourceServer(t *testing.T) {
	owner := &networkState{config: config{automatic: time.Second}, serverDone: make(chan struct{}), automaticDone: make(chan struct{})}
	ticks := make(chan time.Time)
	owner.work.Add(1)
	go func() {
		defer close(owner.automaticDone)
		owner.runAutomaticRefresh(context.Background(), ticks, nil)
	}()
	result := make(chan error, 1)
	go func() { result <- owner.Wait(context.Background()) }()
	ticks <- time.Now()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("Wait accepted terminal automatic refresh failure")
		}
	case <-time.After(time.Second):
		t.Fatal("Wait did not report terminal automatic refresh failure")
	}
	owner.work.Wait()
}
