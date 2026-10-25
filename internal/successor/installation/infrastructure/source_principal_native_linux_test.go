//go:build linux && network_signing_native

package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// This is a native-only NSS mechanism workload, selected by an outside caller.
// The disposable native container supplies one fixed host-owned principal. No
// Source installation, Release, manager, State or accepting root is constructed.
// Repeated genuine observations must retain their normal file/NSS validation and
// physical helper joins within the observer's unchanged 256-TID lifetime bound.
func sourcePrincipalNativeLookups(t *testing.T) {
	t.Helper()
	marker, err := os.ReadFile("/etc/ardents-source-principal-native-stand")
	load := os.Getenv("ARDENTS_SOURCE_PRINCIPAL_NATIVE_LOOKUPS")
	if err != nil || string(marker) != "ardents-source-principal-native-v1\n" || os.Geteuid() != 0 || (load != "1" && load != "112") {
		t.Fatal("dedicated native principal fixture required", err)
	}
	observations := 64
	if load == "112" {
		observations = 112
	}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	var workers sync.WaitGroup
	failures := make(chan error, 4)
	for range 4 {
		workers.Go(func() {
			for range observations / 4 {
				if err := observeSourcePrincipal(ctx, 900, 900); err != nil {
					failures <- err
					return
				}
			}
		})
	}
	workers.Wait()
	close(failures)
	var failed error
	for err := range failures {
		failed = errors.Join(failed, err)
	}
	if err := errors.Join(failed, ctx.Err()); err != nil {
		t.Fatal("original native principal observations failed", err)
	}
	root := os.Getenv("ARDENTS_SOURCE_TRANSITION_NATIVE_ROOT")
	if !filepath.IsAbs(root) || filepath.Clean(root) != root {
		t.Fatal("exact native-only evidence root required")
	}
	f, err := os.OpenFile(filepath.Join(root, "native-principal-lookups-complete"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, written := fmt.Fprintf(f, "%d original principal observations joined\n", observations)
	if err := errors.Join(written, f.Sync(), f.Close()); err != nil {
		t.Fatal("native-only completion evidence failed", err)
	}
}
