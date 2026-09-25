//go:build linux

package endpoint

import (
	"context"
	"errors"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
	"github.com/dianabuilds/ardents-network/internal/qualification"
)

func TestCancelledQualificationRunCannotPublishIntoReplacement(t *testing.T) {
	endpoint, principal := textContextEndpoint(t)
	owner := admittedTextContext(t, endpoint, principal, broker.Connection)
	firstJob, err := beginTextTestJob(t, owner, endpoint, broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	first, err := qualification.NewRun(streamqualification.ReaderRole, streamqualification.ClientToPublisher, fixtureID(211))
	if err != nil {
		t.Fatal(err)
	}
	if err := first.BindInvocation(firstJob.nonce); err != nil {
		t.Fatal(err)
	}
	firstJob.qualification = first
	var firstReport streamqualification.Report
	stopFailure := errors.New("sampling cleanup failed")
	stopCalls := 0
	if err := first.Configure(&firstReport, func(context.Context) (func(), error) { return func() {}, nil },
		func(context.Context) (func(), error) { return func() {}, nil },
		func() error { stopCalls++; return stopFailure },
		func(context.Context, streamqualification.Report) error { return nil }); err != nil {
		t.Fatal(err)
	}
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
	replacement, err := qualification.NewRun(streamqualification.ReaderRole, streamqualification.ClientToPublisher, fixtureID(212))
	if err != nil {
		t.Fatal(err)
	}
	if err := replacement.BindInvocation(replacementJob.nonce); err != nil {
		t.Fatal(err)
	}
	replacementJob.qualification = replacement
	var replacementReport streamqualification.Report
	if err := replacement.Configure(&replacementReport, func(context.Context) (func(), error) { return func() {}, nil },
		func(context.Context) (func(), error) { return func() {}, nil }, func() error { return nil },
		func(context.Context, streamqualification.Report) error { return nil }); err != nil {
		t.Fatal(err)
	}

	if err := first.StopSamples(); !errors.Is(err, stopFailure) {
		t.Fatalf("sampling cleanup result = %v", err)
	}
	first.PublishReport(streamqualification.Report{Failure: "old qualification"})
	if firstReport.Failure != "old qualification" || replacementReport.Failure != "" || len(replacementReport.Streams) != 0 || !replacementReport.Started.IsZero() {
		t.Fatalf("late report crossed runs: first=%q replacement=%q", firstReport.Failure, replacementReport.Failure)
	}
	if err := first.StopSamples(); !errors.Is(err, stopFailure) || stopCalls != 1 {
		t.Fatalf("sampling cleanup was not immutable: err=%v calls=%d", err, stopCalls)
	}
}
