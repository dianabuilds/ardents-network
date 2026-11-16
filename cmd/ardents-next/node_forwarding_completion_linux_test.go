package main

import (
	"testing"
	"time"
)

func TestForwardingTimeoutRetainsOriginalCompletion(t *testing.T) {
	joined := make(chan forwardingCompletion, 1)
	timeout := make(chan time.Time, 1)
	notified := make(chan struct{})
	type observed struct {
		result   forwardingCompletion
		timedOut bool
	}
	done := make(chan observed, 1)
	go func() {
		result, timedOut := awaitForwardingCompletion(joined, timeout, func() { close(notified) })
		done <- observed{result, timedOut}
	}()
	timeout <- time.Now()
	<-notified
	select {
	case <-done:
		t.Fatal("timeout returned before the original completion")
	case <-time.After(25 * time.Millisecond):
	}
	joined <- forwardingCompletion{outcome: "failed", cleanup: "participation"}
	select {
	case result := <-done:
		if !result.timedOut || result.result.outcome != "failed" || result.result.cleanup != "participation" {
			t.Fatal("original completion was lost", result)
		}
	case <-time.After(time.Second):
		t.Fatal("actual completion did not release the waiter")
	}
}
