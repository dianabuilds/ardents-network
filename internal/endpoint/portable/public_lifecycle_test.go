package portable

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// The per-user Portable command surface is retired (ADR-0125, #617 S3.5); this
// Module is retained with its behavior tests until the separately gated Endpoint
// cohort removal. These contracts cover the public lifecycle API whose former
// command and process callers are retired, so the retained Module keeps genuine
// behavior evidence rather than test-only reachability.

func TestDefaultConfigResolvesAbsoluteProfileRoots(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	config, err := DefaultConfig()
	if err != nil {
		t.Fatalf("default portable config: %v", err)
	}
	if config.StateHome == "" || config.RuntimeHome == "" ||
		!filepath.IsAbs(config.StateHome) || !filepath.IsAbs(config.RuntimeHome) {
		t.Fatalf("default portable profile roots are not absolute: %+v", config)
	}
}

func TestFailureEventClassifiesLifecycleReasons(t *testing.T) {
	for _, test := range []struct {
		reason Reason
		state  State
	}{
		{ReasonStaleRecoveryFailed, StateStale},
		{ReasonUnexpectedRuntimeEntry, StateStale},
		{ReasonOwnerBusy, StateBlocked},
		{ReasonLockError, StateBlocked},
		{ReasonAttachmentBindFailed, StateBlocked},
		{ReasonLocalProfileInvalid, StateIncompatible},
	} {
		event := FailureEvent(lifecycleError(test.reason, errors.New("cause")))
		if event.State != test.state || event.Reason != test.reason {
			t.Fatalf("failure event for %s = %+v, want state %s", test.reason, event, test.state)
		}
	}
	if event := FailureEvent(errors.New("plain failure")); event.State != StateIncompatible || event.Reason != ReasonLocalProfileInvalid {
		t.Fatalf("non-lifecycle failure event = %+v", event)
	}
}

func TestWaitStopsOnCancellationAndRefusesUnavailableInput(t *testing.T) {
	if err := (*Runtime)(nil).Wait(context.Background()); err == nil {
		t.Fatal("nil runtime Wait was accepted")
	}
	runtime, err := Open(testConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.Wait(nil); err == nil {
		t.Fatal("nil context Wait was accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := runtime.Wait(ctx); err != nil {
		t.Fatalf("Wait terminal cleanup: %v", err)
	}
	if _, err := os.Lstat(runtime.Attachment()); !os.IsNotExist(err) {
		t.Fatalf("attachment remains after Wait cleanup: %v", err)
	}
}
