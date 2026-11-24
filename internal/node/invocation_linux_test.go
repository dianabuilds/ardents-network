package node

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestNodeTimeoutRetainsOriginalCompletion(t *testing.T) {
	joined := make(chan InvocationCompletion, 1)
	timeout := make(chan time.Time, 1)
	notified := make(chan struct{})
	type observed struct {
		result   InvocationCompletion
		timedOut bool
	}
	done := make(chan observed, 1)
	go func() {
		result, timedOut := awaitNodeCompletion(joined, timeout, func() { close(notified) })
		done <- observed{result, timedOut}
	}()
	timeout <- time.Now()
	<-notified
	select {
	case <-done:
		t.Fatal("timeout returned before the original completion")
	case <-time.After(25 * time.Millisecond):
	}
	joined <- InvocationCompletion{Outcome: "failed", CleanupOwner: "participation"}
	select {
	case result := <-done:
		if !result.timedOut || result.result.Outcome != "failed" || result.result.CleanupOwner != "participation" {
			t.Fatal("original completion was lost", result)
		}
	case <-time.After(time.Second):
		t.Fatal("actual completion did not release the waiter")
	}
}

func TestNodeCompletionBeforeTimeoutKeepsOriginalResult(t *testing.T) {
	joined := make(chan InvocationCompletion, 1)
	joined <- InvocationCompletion{Outcome: "canceled"}
	result, timedOut := awaitNodeCompletion(joined, make(chan time.Time), func() { t.Error("unexpected timeout notice") })
	if timedOut || result.Outcome != "canceled" {
		t.Fatal("completed original was not retained", result, timedOut)
	}
}

type invocationTestRole struct {
	stopped chan struct{}
	joined  chan struct{}
}

func (r invocationTestRole) Stop()                                 { close(r.stopped) }
func (r invocationTestRole) Done() <-chan struct{}                 { return r.joined }
func (r invocationTestRole) Close() error                          { <-r.joined; return nil }
func (r invocationTestRole) JoinedOutcome() string                 { return "canceled" }
func (r invocationTestRole) JoinedFailureOwners() (string, string) { return "", "" }

func TestInvocationRetainsAuthorityUntilPhysicalJoin(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	role := invocationTestRole{stopped: make(chan struct{}), joined: make(chan struct{})}
	join := sync.OnceFunc(func() { close(role.joined) })
	defer join()
	unavailable := make(chan struct{})
	released := make(chan struct{})
	result := make(chan InvocationCompletion, 1)
	go func() {
		result <- SuperviseInvocation(ctx, role, time.Millisecond, func() error {
			cancel()
			return nil
		}, func() {
			close(unavailable)
		}, func() error {
			close(released)
			return nil
		})
	}()
	select {
	case <-unavailable:
	case <-time.After(time.Second):
		t.Fatal("supervisor did not announce retirement timeout")
	}
	select {
	case <-role.stopped:
	default:
		t.Fatal("supervisor reported retirement before stopping the role")
	}
	select {
	case <-released:
		t.Fatal("timeout released authority before physical join")
	default:
	}
	join()
	select {
	case completion := <-result:
		if !completion.TimedOut || completion.Outcome != "canceled" {
			t.Fatalf("lost original joined completion: %+v", completion)
		}
	case <-time.After(time.Second):
		t.Fatal("physical join did not release the supervisor")
	}
	select {
	case <-released:
	default:
		t.Fatal("authority remained held after physical join")
	}
}
