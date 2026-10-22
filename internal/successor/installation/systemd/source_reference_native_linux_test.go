//go:build network_signing_native

package systemd

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

// The startup profile supplies its genuinely selected, never-started Source.
// This component test starts nothing and grants no installed startup authority.
func TestSourceControlRetainsStoppedUnitNative(t *testing.T) {
	marker, err := os.ReadFile("/etc/ardents-source-startup-native-stand")
	if os.Geteuid() != 0 || err != nil || string(marker) != "ardents-source-startup-native-v1\n" {
		t.Fatal("invalid private Source startup native environment", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	observer, err := openSourceReference(ctx)
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
