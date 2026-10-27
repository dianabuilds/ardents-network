//go:build linux && network_signing_native

package forwarding

import (
	"os"
	"strings"
	"testing"
)

func TestForwardingStartupNativeRefusesOrdinaryProcess(t *testing.T) {
	if os.Getenv("ARDENTS_FORWARDING_RUNTIME_SELECTION_EXPECT") != "protected" {
		// This named case is invoked only by the protected private controller;
		// selecting it without that exact prerequisite is an invalid environment.
		t.Fatal("invalid environment: genuine protected non-root file component required")
	}
	if os.Geteuid() < 100 || os.Geteuid() > 999 || os.Getegid() != os.Geteuid() || os.Getenv("INVOCATION_ID") == "" {
		t.Fatal("invalid environment: actual non-root caller and declared invocation required")
	}
	h, err := OpenStartup(t.Context(), os.Getenv("ARDENTS_FORWARDING_INITIAL_BINDING"))
	if h != nil {
		_ = h.Close()
		t.Fatal("ordinary test process impersonated installed canonical Forwarding")
	}
	if err == nil || !strings.Contains(err.Error(), "installation process: observation mismatch") {
		t.Fatal("negative control did not reach actual executable/process matching", err)
	}
}
