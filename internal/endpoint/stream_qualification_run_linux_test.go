//go:build linux

package endpoint

import (
	"context"
	"errors"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
)

func TestCancelledQualificationRunCannotPublishIntoReplacement(t *testing.T) {
	endpoint, principal := textContextEndpoint(t)
	owner := admittedTextContext(t, endpoint, principal, broker.Connection)
	firstJob, err := beginTextTestJob(t, owner, endpoint, broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	first, err := newTextQualificationRun(streamqualification.ReaderRole, streamqualification.ClientToPublisher, fixtureID(211))
	if err != nil {
		t.Fatal(err)
	}
	if err := first.bindInvocation(firstJob.nonce); err != nil {
		t.Fatal(err)
	}
	firstJob.qualification = first
	var firstReport streamqualification.Report
	stopFailure := errors.New("sampling cleanup failed")
	stopCalls := 0
	releaseSampling := make(chan struct{})
	if err := first.configure(&firstReport, func(context.Context) error { return nil },
		func(context.Context) (func(), error) { return func() {}, nil },
		func() error { stopCalls++; <-releaseSampling; return stopFailure },
		func(context.Context, streamqualification.Report) error { return nil }); err != nil {
		t.Fatal(err)
	}

	lateDone := make(chan struct{})
	go func() {
		_ = first.stopSamples()
		first.publishReport(streamqualification.Report{Failure: "old qualification"})
		close(lateDone)
	}()
	owner.retireJob(firstJob)
	if err := owner.finishJobCleanup(firstJob, nil); err != nil {
		t.Fatal(err)
	}

	replacementJob, err := beginTextTestJob(t, owner, endpoint, broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		owner.retireJob(replacementJob)
		_ = owner.finishJobCleanup(replacementJob, nil)
	})
	replacement, err := newTextQualificationRun(streamqualification.ReaderRole, streamqualification.ClientToPublisher, fixtureID(212))
	if err != nil {
		t.Fatal(err)
	}
	if err := replacement.bindInvocation(replacementJob.nonce); err != nil {
		t.Fatal(err)
	}
	replacementJob.qualification = replacement
	var replacementReport streamqualification.Report
	if err := replacement.configure(&replacementReport, func(context.Context) error { return nil },
		func(context.Context) (func(), error) { return func() {}, nil }, func() error { return nil },
		func(context.Context, streamqualification.Report) error { return nil }); err != nil {
		t.Fatal(err)
	}

	close(releaseSampling)
	<-lateDone
	if firstReport.Failure != "old qualification" || replacementReport.Failure != "" || len(replacementReport.Streams) != 0 || !replacementReport.Started.IsZero() {
		t.Fatalf("late report crossed runs: first=%q replacement=%q", firstReport.Failure, replacementReport.Failure)
	}
	if err := first.stopSamples(); !errors.Is(err, stopFailure) {
		t.Fatalf("sampling cleanup result = %v", err)
	}
	if err := first.stopSamples(); !errors.Is(err, stopFailure) || stopCalls != 1 {
		t.Fatalf("sampling cleanup was not immutable: err=%v calls=%d", err, stopCalls)
	}
}
