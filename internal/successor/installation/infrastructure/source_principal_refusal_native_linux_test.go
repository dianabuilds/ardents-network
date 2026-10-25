//go:build linux && network_signing_native

package infrastructure

import (
	"context"
	"errors"
	"os"
	"runtime/debug"
	"testing"
	"time"
)

// The outside native fixture supplies only failing NSS results (or caller
// cancellation). A successful fake NSS response cannot qualify this profile.
// Positive name/number observations use the separate actual-getent workload.
func TestSourcePrincipalNativeRefusal(t *testing.T) {
	mode := os.Getenv("ARDENTS_SOURCE_PRINCIPAL_NATIVE_REFUSAL")
	if mode == "" {
		return // Dedicated native helper, selected by its outside caller.
	}
	if mode != "binding" && mode != "canceled" && mode != "deadline" {
		t.Fatal("closed native principal refusal mode required")
	}
	marker, err := os.ReadFile("/etc/ardents-source-principal-native-stand")
	if err != nil || string(marker) != "ardents-source-principal-native-v1\n" || os.Geteuid() != 0 {
		t.Fatal("dedicated native principal fixture required", err)
	}
	previous := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(previous)
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	if mode == "canceled" {
		cancel()
	}
	observe := func() {
		t.Helper()
		err := observeSourcePrincipal(ctx, 900, 900)
		wanted := ErrBinding
		if mode == "canceled" {
			wanted = context.Canceled
		} else if mode == "deadline" {
			wanted = context.DeadlineExceeded
		}
		if !errors.Is(err, wanted) {
			t.Fatal("actual principal refusal did not retain its original cause", err)
		}
	}
	// Initialise only ordinary Go process/poll machinery. This is an actual
	// independently refused observation, not a replacement successful permit.
	observe()
	before, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		observe()
		after, err := os.ReadDir("/proc/self/fd")
		if err != nil || len(after) != len(before) {
			t.Fatal("original failed principal observation retained descriptors without GC", len(before), len(after), err)
		}
	}
	t.Log("three actual principal refusals; original cause and before-return FD release retained without GC")
}
