//go:build network_signing_native

package systemd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

// Failure-only comparisons of the actual pinned library sentinel. No manager,
// connection, successful observation or Source/Forwarding authority is supplied.
func TestSourceReferenceLibraryCloseCause(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for _, err := range []error{dbus.ErrClosed, fmt.Errorf("original call: %w", dbus.ErrClosed)} {
		if observed := startupCallFailure(ctx, err, true); observed != context.Canceled {
			t.Fatal("original D-Bus interruption invented a second failure", observed)
		}
	}
	for _, err := range []error{io.EOF, ErrObservation, errors.Join(dbus.ErrClosed, ErrObservation), errors.New(dbus.ErrClosed.Error())} {
		if observed := startupCallFailure(ctx, err, true); observed != err {
			t.Fatal("original interruption hid an independent failure", observed)
		}
	}
	if startupCallFailure(ctx, dbus.ErrClosed, false) != dbus.ErrClosed || startupCallFailure(t.Context(), dbus.ErrClosed, true) != dbus.ErrClosed {
		t.Fatal("foreign or uncanceled D-Bus close became owned cancellation")
	}
	expired, stop := context.WithDeadline(t.Context(), time.Unix(1, 0))
	defer stop()
	if startupCallFailure(expired, dbus.ErrClosed, true) != context.DeadlineExceeded {
		t.Fatal("original D-Bus deadline became ordinary cancellation")
	}
}

// The startup profile supplies its genuinely selected, never-started Source.
// This component test starts nothing and grants no installed startup authority.
func TestSourceControlRetainsStoppedUnitNative(t *testing.T) {
	marker, err := os.ReadFile("/etc/ardents-source-startup-native-stand")
	if os.Geteuid() != 0 || err != nil || string(marker) != "ardents-source-startup-native-v1\n" {
		t.Fatal("invalid private Source startup native environment", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	observer, err := openManagerReference(ctx, sourceUnitReply)
	if observer != nil {
		defer func() {
			if err := observer.close(); err != nil {
				t.Error("original observation close", err)
			}
		}()
	}
	if err != nil {
		t.Fatal(err)
	}
	missing := func() bool {
		t.Helper()
		_, err := observer.client.call(ctx, "/org/freedesktop/systemd1", "org.freedesktop.systemd1.Manager.GetUnit", "ardents-source.service")
		if err == nil {
			return false
		}
		var reply dbus.Error
		if !errors.As(err, &reply) || reply.Name != "org.freedesktop.systemd1.NoSuchUnit" {
			t.Fatal("unexpected original manager refusal", err)
		}
		return true
	}
	waitMissing := func() {
		t.Helper()
		end := time.Now().Add(2 * time.Second)
		for !missing() {
			if time.Now().After(end) {
				t.Fatal("unreferenced stopped Source was not collected")
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	waitMissing() // An already loaded fixture cannot hide the original defect.
	original, stop := context.WithCancel(ctx)
	defer stop()
	control, err := OpenSourceControl(original)
	closeChecked := false
	if control != nil {
		defer func() {
			if !closeChecked {
				if err := control.Close(); err != nil {
					t.Error("partial control close", err)
				}
			}
		}()
	}
	if err != nil {
		t.Fatal("control could not retain the genuine stopped unit", err)
	}
	copy := *control
	for range 3 {
		time.Sleep(100 * time.Millisecond)
		facts, err := control.Observe()
		if err != nil {
			t.Fatal("original reference lost across manager collection", err)
		}
		if !Matches(facts.Unit, "ActiveState", "s", "inactive") || !Matches(facts.Service, "MainPID", "u", uint32(0)) || control.original.startAttempted || control.original.stopAttempted {
			t.Fatal("reference acquisition started or stopped Source")
		}
	}
	stop()
	closed := control.Close()
	closeChecked = true
	if !sourceReferenceCancellationOnly(closed) || copy.Close() != closed {
		t.Fatal("original cancellation or cached physical close result lost", closed)
	}
	waitMissing() // Original bus closure must actually return its server reference.
}

func sourceReferenceCancellationOnly(err error) bool {
	if err == context.Canceled {
		return true
	}
	if group, ok := err.(interface{ Unwrap() []error }); ok {
		children := group.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if !sourceReferenceCancellationOnly(child) {
				return false
			}
		}
		return true
	}
	if wrapper, ok := err.(interface{ Unwrap() error }); ok {
		return sourceReferenceCancellationOnly(wrapper.Unwrap())
	}
	return false
}
