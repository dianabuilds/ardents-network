//go:build network_signing_native

package forwarding

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime/debug"
	"testing"
	"time"
)

func TestForwardingRootIdentityNativeRequiresOriginalManagerNamespaces(t *testing.T) {
	_ = nativeCompletedBinding(t) // dedicated genuine stopped private stand, not a host fixture
	if os.Getenv("ARDENTS_FORWARDING_ROOT_NAMESPACE_EXPECT") == "foreign" {
		own, err := os.Stat("/proc/self/ns/mnt")
		if err != nil {
			t.Fatal(err)
		}
		manager, err := os.Stat("/proc/1/ns/mnt")
		if err != nil || os.SameFile(own, manager) {
			t.Fatal("invalid environment: actual foreign mount namespace required", err)
		}
		check := func() {
			h, err := openRootIdentity(t.Context())
			if h != nil {
				_ = h.close()
				t.Fatal("foreign root namespace supplied manager identity")
			}
			if !errors.Is(err, ErrBinding) {
				t.Fatal("foreign namespace did not reach actual manager comparison", err)
			}
		}
		check() // initialize native Go descriptor machinery before no-GC leak count
		oldGC := debug.SetGCPercent(-1)
		defer debug.SetGCPercent(oldGC)
		before := nativeInputFDCount(t)
		for range 3 {
			check()
		}
		if after := nativeInputFDCount(t); after != before {
			t.Fatal("denied root namespace retained native descriptors", before, after)
		}
		return
	}
	h, err := openRootIdentity(t.Context())
	if err != nil {
		t.Fatal("actual original private manager namespace observation", err)
	}
	if err := h.check(); err != nil {
		t.Fatal(err)
	}
	if err := h.close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(ctx, "/usr/bin/unshare", "--mount", "--propagation", "private", "--", program, "-test.run", "^TestForwardingRootIdentityNativeRequiresOriginalManagerNamespaces$", "-test.count=1", "-test.v", "-test.timeout=20s")
	command.Env = append(os.Environ(), "ARDENTS_FORWARDING_ROOT_NAMESPACE_EXPECT=foreign")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatal("actual borrowed-manager namespace refusal", err, string(output))
	}
}
