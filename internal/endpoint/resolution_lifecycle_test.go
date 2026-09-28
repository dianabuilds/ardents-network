//go:build linux

package endpoint

import (
	"context"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/endpoint/source"
)

func TestResolutionLifecycleExcludesSecondFlightAndReleasesOnce(t *testing.T) {
	var lifecycle resolutionLifecycle
	caller, cancelCaller := context.WithCancel(context.Background())
	defer cancelCaller()
	flight := lifecycle.BeginLocked(context.Background(), caller, &source.ResolutionAcquisition{})
	if flight == nil || !lifecycle.BusyLocked() || !lifecycle.CurrentLocked(flight) {
		t.Fatal("first Descriptor flight was not admitted")
	}
	if second := lifecycle.BeginLocked(context.Background(), caller, &source.ResolutionAcquisition{}); second != nil {
		t.Fatal("lookup and publication could occupy the same resolution slot")
	}
	cancelCaller()
	<-flight.context.Done()
	lifecycle.JoinCaller(flight)
	retained := 0
	lifecycle.FinishLocked(flight, func() { retained++ })
	lifecycle.FinishLocked(flight, func() { retained++ })
	if retained != 1 || lifecycle.BusyLocked() || lifecycle.CurrentLocked(flight) || flight.source != nil {
		t.Fatal("Descriptor completion did not release the acquisition and slot exactly once")
	}
	select {
	case <-flight.done:
	default:
		t.Fatal("Descriptor completion did not close its join barrier")
	}
}

func TestResolutionLifecycleJoinsCallerCallbackBeforeCompletion(t *testing.T) {
	var lifecycle resolutionLifecycle
	flight := lifecycle.BeginLocked(context.Background(), context.Background(), &source.ResolutionAcquisition{})
	if flight == nil {
		t.Fatal("Descriptor flight was not admitted")
	}
	if !flight.stopCaller() {
		<-flight.callerStopped
	}
	entered, release, joined := make(chan struct{}), make(chan struct{}), make(chan struct{})
	flight.stopCaller = func() bool { close(entered); return false }
	flight.callerStopped = release
	go func() { lifecycle.JoinCaller(flight); close(joined) }()
	<-entered
	select {
	case <-joined:
		t.Fatal("caller cancellation callback was not joined")
	default:
	}
	close(release)
	<-joined
	lifecycle.FinishLocked(flight, nil)
	flight.Join()
}

func TestResolutionLifecycleStopRevokesButRetainsJoinIdentity(t *testing.T) {
	var lifecycle resolutionLifecycle
	flight := lifecycle.BeginLocked(context.Background(), context.Background(), &source.ResolutionAcquisition{})
	if flight == nil || lifecycle.StopLocked() != flight {
		t.Fatal("retirement did not retain the exact Descriptor flight")
	}
	<-flight.context.Done()
	if !lifecycle.BusyLocked() || lifecycle.CurrentLocked(flight) {
		t.Fatal("stopped owner lost join identity or accepted stale currentness")
	}
	if replacement := lifecycle.BeginLocked(context.Background(), context.Background(), &source.ResolutionAcquisition{}); replacement != nil {
		t.Fatal("stopped owner admitted a replacement Descriptor flight")
	}
	select {
	case <-flight.done:
		t.Fatal("retirement completed before flight release")
	default:
	}
	lifecycle.JoinCaller(flight)
	lifecycle.JoinCaller(flight)
	lifecycle.FinishLocked(flight, nil)
	flight.Join()
	if lifecycle.BusyLocked() {
		t.Fatal("completed retirement retained the resolution slot")
	}
}
