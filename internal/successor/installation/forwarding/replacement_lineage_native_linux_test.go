//go:build network_signing_native

package forwarding

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Actual directory/namespace/descriptor custody only. This disposable image
// has no systemd broker, genuine Reload, Release or installed authority. An
// existing generator directory makes the environment invalid before mutation.
func TestForwardingReplacementContextNativeGeneratorJoin(t *testing.T) {
	marker, err := os.ReadFile("/etc/ardents-forwarding-preparation-native-stand")
	if err != nil || string(marker) != "ardents-forwarding-preparation-native-v1\n" || os.Geteuid() != 0 || os.Getegid() != 0 {
		t.Fatal("invalid environment: disposable root native stand required", err)
	}
	for _, mode := range []string{"complete", "late-close", "cancel-close"} {
		t.Run(mode, func(t *testing.T) { nativeReplacementContextJoin(t, mode) })
	}
}

func nativeReplacementContextJoin(t *testing.T, mode string) {
	t.Helper()
	base, finish := context.WithTimeout(context.Background(), 10*time.Second)
	defer finish()
	ctx, cancel := context.WithCancel(base)
	defer cancel()
	paths := []string{"/run/systemd/generator", "/run/systemd/generator.early", "/run/systemd/generator.late"}
	for _, path := range paths {
		for _, name := range []string{path, path + ".prior"} {
			if _, err := os.Lstat(name); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("invalid environment: generator fixture path must be absent", name, err)
			}
		}
	}
	parent := "/run/systemd"
	createdParent := false
	if _, err := os.Lstat(parent); errors.Is(err, os.ErrNotExist) {
		if err := os.Mkdir(parent, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(parent, 0755); err != nil {
			t.Fatal(err)
		}
		createdParent = true
	} else if err != nil {
		t.Fatal(err)
	}
	var created []string
	t.Cleanup(func() {
		for n := len(created) - 1; n >= 0; n-- {
			if err := os.Remove(created[n]); err != nil && !errors.Is(err, os.ErrNotExist) {
				t.Error(err)
			}
		}
		if createdParent {
			if err := os.Remove(parent); err != nil {
				t.Error(err)
			}
		}
	})
	for _, path := range paths {
		if err := os.Mkdir(path, 0755); err != nil {
			t.Fatal(err)
		}
		created = append(created, path)
		if err := os.Chmod(path, 0755); err != nil {
			t.Fatal(err)
		}
	}
	identity, err := openRootIdentity(base)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := identity.close(); err != nil {
			t.Error(err)
		}
	}()
	namespace, err := openStartNamespace(base, paths)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := namespace.close(); err != nil {
			t.Error(err)
		}
	}()
	raw, err := captureNativeContext(identity, "physical-test-only", namespace, &inputs{ctx: base, byPath: map[string]*inputDirectory{}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	old, err := openRecoveryContext(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = old.close() }()
	for _, path := range paths {
		if err := os.Rename(path, path+".prior"); err != nil {
			t.Fatal(err)
		}
		created = append(created, path+".prior")
		if err := os.Mkdir(path, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := namespace.reobserveAfterReload(); err != nil {
		t.Fatal("physical generator observation", err)
	}
	late := errors.New("original generator descriptor Close failure")
	held, release := make(chan struct{}), make(chan struct{})
	old.beforeClose = func(file *os.File) error {
		if file.Name() != paths[2] {
			return nil
		}
		close(held)
		<-release
		if mode == "late-close" {
			return late
		}
		return nil
	}
	type outcome struct {
		fresh *recoveryContext
		raw   []byte
		err   error
	}
	done := make(chan outcome, 1)
	go func() { fresh, raw, err := old.afterOwnedReload(namespace); done <- outcome{fresh, raw, err} }()
	select {
	case <-held:
	case got := <-done:
		old.beforeClose = nil
		close(release)
		if got.fresh != nil {
			_ = got.fresh.close()
		}
		t.Fatal("old physical Close was not held", got.err)
	}
	_, heldErr := old.dirs[paths[2]].Stat()
	if mode == "cancel-close" {
		cancel()
	}
	close(release)
	got := <-done
	if heldErr != nil {
		if got.fresh != nil {
			_ = got.fresh.close()
		}
		t.Fatal("old descriptor released before Close join", heldErr)
	}
	if mode != "complete" {
		cause := late
		if mode == "cancel-close" {
			cause = context.Canceled
		}
		if got.fresh != nil || !errors.Is(got.err, cause) {
			if got.fresh != nil {
				_ = got.fresh.close()
			}
			t.Fatal("uncertain original Close supplied fresh observation", got.err)
		}
		for _, file := range old.order {
			if _, err := file.Stat(); !errors.Is(err, os.ErrClosed) {
				t.Fatal("old physical descriptor not joined", file.Name(), err)
			}
		}
		return
	}
	if got.err != nil || got.fresh == nil {
		t.Fatal("checked physical continuation unavailable", got.err)
	}
	defer func() {
		if err := got.fresh.close(); err != nil {
			t.Error(err)
		}
	}()
	if err := compareReloadContexts(raw, got.raw); err != nil {
		t.Fatal("non-generator binding changed", err)
	}
	if err := got.fresh.check(); err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		oldInfo, err := os.Lstat(path + ".prior")
		if err != nil {
			t.Fatal(err)
		}
		newInfo, err := got.fresh.dirs[path].Stat()
		if err != nil {
			t.Fatal(err)
		}
		if os.SameFile(oldInfo, newInfo) || filepath.Clean(got.fresh.dirs[path].Name()) != path {
			t.Fatal("original generator inode was silently reused")
		}
	}
}
