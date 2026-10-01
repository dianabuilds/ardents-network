//go:build linux

package installation

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestInstallationManagerDoesNotInheritBusOrManagerOverrides(t *testing.T) {
	t.Setenv("DBUS_SYSTEM_BUS_ADDRESS", "unix:path=/foreign-bus")
	t.Setenv("SYSTEMD_UNIT_PATH", "/foreign-units")
	t.Setenv("LD_PRELOAD", "/foreign-code")
	body, err := runInstallationCommand(context.Background(), "/usr/bin/env")
	if err != nil {
		t.Fatal(err)
	}
	if body != "PATH=/usr/bin:/bin\nLANG=C\nLC_ALL=C\n" {
		t.Fatal("installation subprocess inherited foreign environment")
	}
}

func TestInstallationJoinRetainsObservationFailure(t *testing.T) {
	original := errors.New("original scope read failed")
	calls := 0
	err := joinInstallationPins(context.Background(), []*os.File{nil}, func(*os.File) (bool, bool, error) {
		calls++
		if calls == 1 {
			return false, false, original
		}
		return true, false, nil
	})
	if !errors.Is(err, original) || calls != 1 {
		t.Fatal("join retried away its first observation failure")
	}
}

func TestInstallationJoinRequiresEveryOriginalScopeEmpty(t *testing.T) {
	first, second := new(os.File), new(os.File)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err := joinInstallationPins(ctx, []*os.File{first, second}, func(pin *os.File) (bool, bool, error) {
		return pin == first, pin == second, nil
	})
	if err == nil {
		t.Fatal("one removed scope hid another populated scope")
	}
}

func TestInstallationJoinRefusesCancelledEmptyObservation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := joinInstallationPins(ctx, []*os.File{nil}, func(*os.File) (bool, bool, error) { return true, false, nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled empty observation became a passing join")
	}
}

func TestInstallationStopRefusesBeforeEffectsWithoutRootContext(t *testing.T) {
	for name, ctx := range map[string]context.Context{"absent": nil} {
		t.Run(name, func(t *testing.T) {
			if err := stopInstalledPredecessor(ctx, checkedBinding{}); err == nil || !strings.Contains(err.Error(), "root and context") {
				t.Fatal("stop admitted absent context")
			}
		})
	}
}
