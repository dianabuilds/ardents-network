package release

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestOriginalAuthorizationHandoffRefusesForeignClosedAndCanceled(t *testing.T) {
	h := freshSignedHistory(t)
	v, err := Open(filepath.Join(t.TempDir(), "history"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := v.Close(); err != nil {
			t.Error(err)
		}
	})
	ctx, cancel := context.WithCancel(t.Context())
	d := v.Evaluate(ctx, h.input)
	a, ok := d.Authorization()
	if !ok {
		t.Fatalf("healthy actual evaluation %s %v", d.Outcome, d.Err())
	}
	if err := v.CheckAuthorizations(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := v.CheckAuthorizations(ctx, Authorization{}); err == nil {
		t.Fatal("zero accepted")
	}
	foreign, err := Open(filepath.Join(t.TempDir(), "foreign"))
	if err != nil {
		t.Fatal(err)
	}
	if err := foreign.CheckAuthorizations(ctx, a); err == nil {
		t.Fatal("foreign owner accepted")
	}
	if err := foreign.Close(); err != nil {
		t.Fatal(err)
	}
	// Detached mutation cannot alter private evidence.
	d.Digest[0] ^= 1
	if err := v.CheckAuthorizations(ctx, a); err != nil {
		t.Fatal(err)
	}
	cancel()
	if err := v.CheckAuthorizations(t.Context(), a); !errors.Is(err, context.Canceled) {
		t.Fatal("original cancellation lost", err)
	}
	d = v.Evaluate(t.Context(), h.input)
	a, ok = d.Authorization()
	if !ok {
		t.Fatal("fresh no-update failed", d.Err())
	}
	if err := v.CheckAuthorizations(t.Context(), a); err != nil {
		t.Fatal(err)
	}
	if err := v.Close(); err != nil {
		t.Fatal(err)
	}
	if err := v.CheckAuthorizations(t.Context(), a); !errors.Is(err, ErrClosed) {
		t.Fatal("late close accepted", err)
	}
}
