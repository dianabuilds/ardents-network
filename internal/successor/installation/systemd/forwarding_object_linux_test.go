package systemd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"
)

func TestForwardingAbsentUnitCancellationIsNotChangedIdentity(t *testing.T) {
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	expired, stop := context.WithDeadline(t.Context(), time.Unix(1, 0))
	defer stop()
	for _, ctx := range []context.Context{canceled, expired} {
		// The real request refuses before bus access. No connection, absent
		// reply, manager fact or successful native authority is supplied.
		reference := &managerReference{}
		for _, name := range []string{"ardents-issuer.service", "ardents-forwarding.socket"} {
			err := nodeNameAbsent(ctx, reference, name)
			if !errors.Is(err, ctx.Err()) || errors.Is(err, ErrObservation) {
				t.Fatal("original canceled absence read invented changed identity", err)
			}
		}
	}
}

func TestForwardingAbsentUnitCancellationKeepsIndependentFailures(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for _, err := range []error{context.Canceled, errors.Join(context.Canceled, errors.Join(context.Canceled, context.Canceled))} {
		if !nodeAbsenceContextOnly(ctx, err) {
			t.Fatal("exact original context leaves were changed")
		}
	}
	for _, err := range []error{nil, io.EOF, ErrObservation, context.DeadlineExceeded,
		errors.New(context.Canceled.Error()), fmt.Errorf("original physical operation: %w", context.Canceled),
		errors.Join(context.Canceled, io.EOF), errors.Join(context.Canceled, ErrObservation)} {
		if nodeAbsenceContextOnly(ctx, err) {
			t.Fatal("independent or wrapped failure became context-only", err)
		}
	}
	if nodeAbsenceContextOnly(t.Context(), context.Canceled) {
		t.Fatal("foreign cancellation became this live request's cancellation")
	}
}

// This independent D-Bus frame has the exact LoadUnit object observed on the
// private initial-stopped stand. A Source-only reply grammar rejected it before
// the fixed Forwarding owner could compare any loaded Unit/Service policy.
func TestForwardingLoadUnitReplyIndependentVector(t *testing.T) {
	frame := sourceObjectReplyVector(":1.4", "/org/freedesktop/systemd1/unit/ardents_2dforwarding_2eservice")
	if err := verifyManagerFrame(frame, ":1.4", true, false, forwardingUnitReply); err != nil {
		t.Fatal("actual fixed Forwarding LoadUnit reply refused", err)
	}
	if verifyManagerFrame(frame, ":1.4", true, true, sourceUnitReply) == nil || verifyManagerFrame(frame, ":1.4", false, false, forwardingUnitReply) == nil {
		t.Fatal("reply crossed a closed connection vocabulary")
	}
	for _, path := range []string{
		"/org/freedesktop/systemd1/unit/ardents_2dsource_2eservice",
		"/org/freedesktop/systemd1/unit/ardents_2dissuer_2eservice",
		"/org/freedesktop/systemd1/unit/ardents_2dendpoint_2eservice",
		"/org/freedesktop/systemd1/unit/ardents_2dforwarding_2eserviceX",
		"/org/freedesktop/systemd1/job/42",
	} {
		for _, sourceJobs := range []bool{false, true} {
			if verifyManagerFrame(sourceObjectReplyVector(":1.4", path), ":1.4", true, sourceJobs, forwardingUnitReply) == nil {
				t.Fatal("foreign unit or Source job reached Forwarding", path)
			}
		}
	}
	for length := 0; length < len(frame); length++ {
		if verifyManagerFrame(frame[:length], ":1.4", true, false, forwardingUnitReply) == nil {
			t.Fatal("truncated Forwarding object accepted", length)
		}
	}
	if verifyManagerFrame(sourceObjectReplyVector(":1.5", forwardingUnitReply.object()), ":1.4", true, false, forwardingUnitReply) == nil ||
		verifyManagerFrame(frame, ":1.4", true, false, managerUnitReply(255)) == nil {
		t.Fatal("foreign manager or unknown reply vocabulary accepted")
	}
}
