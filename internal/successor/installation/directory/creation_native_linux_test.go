//go:build installation_native

package directory

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// These actors create actual non-root-owned filesystem directories. They
// supply no Endpoint account, writer lease, Release or installation authority.
func TestInstallationNativeDirectoryBirthAndRefusal(t *testing.T) {
	parent := nativeDirectory(t)
	name := filepath.Join(parent, "nested", "state")
	created, err := New(t.Context(), 65534, 65534)
	if err != nil {
		t.Fatal(err)
	}
	if err := created.Create(name); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(name)
	identity, identityErr := created.Identity(name)
	if err != nil || identityErr != nil || !Matches(info, 65534, 65534) || identity.Device == 0 || identity.Inode == 0 {
		t.Fatal("native birth identity differs", err, identityErr)
	}
	if err := created.Create(name); err != nil {
		t.Fatal("own sibling reuse refused", err)
	}
	foreign, err := New(t.Context(), 65534, 65534)
	if err != nil {
		t.Fatal(err)
	}
	if err := foreign.Create(name); err == nil {
		t.Fatal("foreign root adopted")
	}
	identity.Device, identity.Inode = 0, 0
	again, err := created.Identity(name)
	if err != nil || again.Device == 0 || again.Inode == 0 {
		t.Fatal("detached facts changed original custody", err)
	}
	if err := created.Observe(); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(name, name+".original"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(name, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(name, 65534, 65534); err != nil {
		t.Fatal(err)
	}
	if err := created.Create(name); err == nil {
		t.Fatal("matching mode/owner substituted an inode")
	}
	if _, err := created.Identity(name); err == nil {
		t.Fatal("substituted identity exposed")
	}
	if err := created.Observe(); err == nil {
		t.Fatal("substitution escaped original observation")
	}
}

func TestInstallationNativeDirectoryOriginalCancellation(t *testing.T) {
	parent := nativeDirectory(t)
	ctx, cancel := context.WithCancel(t.Context())
	created, err := New(ctx, 65534, 65534)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	name := filepath.Join(parent, "never-born")
	if err := created.Create(name); !errors.Is(err, context.Canceled) {
		t.Fatal("original cancellation lost", err)
	}
	if _, err := os.Lstat(name); !os.IsNotExist(err) {
		t.Fatal("cancelled work created a root")
	}
	if _, err := New(ctx, 65534, 65534); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled constructor admitted", err)
	}
}

func nativeDirectory(t *testing.T) string {
	t.Helper()
	parent := os.Getenv("ARDENTS_INSTALLATION_NATIVE_ROOT")
	if os.Geteuid() != 0 || parent == "" || !canonicalPath(parent) {
		t.Fatal("invalid environment: installation_native requires root and a trusted external temporary parent")
	}
	for current := parent; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil || !trustedDirectory(info) {
			t.Fatal("invalid environment: untrusted temporary ancestor", err)
		}
		if current == filepath.Dir(current) {
			break
		}
	}
	name, err := os.MkdirTemp(parent, "installation-directory-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(name); err != nil {
			t.Error(err)
		}
	})
	return name
}

func TestInstallationNativeDirectoryZeroValueHasNoBirth(t *testing.T) {
	name := filepath.Join(nativeDirectory(t), "unchecked")
	var unchecked Creation
	if err := unchecked.Create(name); !errors.Is(err, ErrInput) {
		t.Fatal("unchecked creation admitted", err)
	}
	if _, err := os.Lstat(name); !os.IsNotExist(err) {
		t.Fatal("unchecked value created a directory")
	}
}
