//go:build linux

package endpoint

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestInstalledPermissionHandover(t *testing.T) {
	directory := t.TempDir()
	if err := os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "request")
	raw := []byte("public request fixture")
	check := func() error { return nil } // Filesystem mechanism only; no authority fixture.
	if err := installedPermissionRequest(path, raw, check); err != nil {
		t.Fatal(err)
	}
	original, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := installedPermissionRequest(path, raw, check); err != nil {
		t.Fatal(err)
	}
	again, err := os.Stat(path)
	if err != nil || !os.SameFile(original, again) {
		t.Fatal("retry replaced the original request", err)
	}
	if err := installedPermissionRequest(path, []byte("another holder request"), check); err == nil {
		t.Fatal("replaced a different request")
	}
	retained, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(retained, raw) {
		t.Fatal("refusal modified retained request", err)
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if err := installedPermissionRequest(path, raw, check); err == nil {
		t.Fatal("accepted nonprivate request")
	}
}

func TestInstalledPermissionRefusesSymlinkAndCancellation(t *testing.T) {
	directory := t.TempDir()
	if err := os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}
	path, target := filepath.Join(directory, "request"), filepath.Join(directory, "target")
	if err := os.WriteFile(target, []byte("untouched"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	if err := installedPermissionRequest(path, []byte("replacement"), func() error { return nil }); err == nil {
		t.Fatal("followed request symlink")
	}
	raw, err := os.ReadFile(target)
	if err != nil || string(raw) != "untouched" {
		t.Fatal("symlink refusal changed target", err)
	}
	absent := filepath.Join(directory, "absent")
	if err := installedPermissionRequest(absent, []byte("request"), func() error { return context.Canceled }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := os.Lstat(absent); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("cancelled admission created a request", err)
	}
}

func TestInstalledPermissionRefusesOriginalParentReplacement(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "handover")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "request")
	calls := 0
	check := func() error {
		calls++
		if calls == 3 {
			if err := os.Rename(directory, directory+"-original"); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(directory, 0700); err != nil {
				t.Fatal(err)
			}
		}
		return nil
	}
	if err := installedPermissionRequest(path, []byte("public request"), check); err == nil {
		t.Fatal("accepted replacement parent")
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("wrote through replacement parent", err)
	}
	original, err := os.ReadFile(filepath.Join(directory+"-original", "request"))
	if err != nil || len(original) != 0 {
		t.Fatal("wrote request after original parent loss", err)
	}
}

func TestInstalledPermissionResponseCustody(t *testing.T) {
	directory := t.TempDir()
	if err := os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "response")
	expected := bytes.Repeat([]byte{7}, 228)
	if err := os.WriteFile(path, expected, 0600); err != nil {
		t.Fatal(err)
	}
	raw, err := installedPermissionRead(path)
	if err != nil || !bytes.Equal(raw, expected) {
		t.Fatal("lost owned response bytes", err)
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if raw, err := installedPermissionRead(path); raw != nil || err == nil {
		t.Fatal("accepted shared response")
	}
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(directory, "link")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if raw, err := installedPermissionRead(link); raw != nil || err == nil {
		t.Fatal("accepted response symlink")
	}
	if err := os.WriteFile(path, expected[:227], 0600); err != nil {
		t.Fatal(err)
	}
	if raw, err := installedPermissionRead(path); raw != nil || err == nil {
		t.Fatal("accepted partial response")
	}
}

func TestInstalledPermissionRetireRestart(t *testing.T) {
	directory := t.TempDir()
	if err := os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}
	requestPath := filepath.Join(directory, "request")
	responsePath := filepath.Join(directory, "response")
	check := func() error { return nil } // Filesystem mechanism only; no authority fixture.
	// First boot has no handover; retirement is a no-op.
	if err := installedPermissionRetire(requestPath, responsePath, check); err != nil {
		t.Fatal("first-boot retirement", err)
	}
	original := []byte("prior invocation public request")
	if err := installedPermissionRequest(requestPath, original, check); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(responsePath, bytes.Repeat([]byte{9}, 228), 0600); err != nil {
		t.Fatal(err)
	}
	// The single-shot publish rule is unchanged: a differing fresh request is
	// refused while the prior invocation's handover is retained.
	if err := installedPermissionRequest(requestPath, []byte("fresh invocation request"), check); err == nil {
		t.Fatal("replaced a prior invocation request without retirement")
	}
	// Retirement clears the prior handover so the fresh invocation can publish.
	if err := installedPermissionRetire(requestPath, responsePath, check); err != nil {
		t.Fatal("restart retirement", err)
	}
	if _, err := os.Lstat(requestPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("retirement retained the prior request", err)
	}
	if _, err := os.Lstat(responsePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("retirement retained the prior response", err)
	}
	fresh := []byte("fresh invocation request")
	if err := installedPermissionRequest(requestPath, fresh, check); err != nil {
		t.Fatal("fresh invocation publish after retirement", err)
	}
	retained, err := os.ReadFile(requestPath)
	if err != nil || !bytes.Equal(retained, fresh) {
		t.Fatal("fresh invocation request bytes", err)
	}
}

func TestInstalledPermissionRetireResponseBeforeRequest(t *testing.T) {
	directory := t.TempDir()
	if err := os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}
	requestPath := filepath.Join(directory, "request")
	responsePath := filepath.Join(directory, "response")
	if err := os.WriteFile(requestPath, []byte("prior request"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(responsePath, bytes.Repeat([]byte{9}, 228), 0600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	check := func() error {
		calls++
		if calls == 2 {
			return errors.New("interrupted after response retirement")
		}
		return nil
	}
	if err := installedPermissionRetire(requestPath, responsePath, check); err == nil {
		t.Fatal("accepted interrupted retirement")
	}
	if _, err := os.Lstat(responsePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("response not retired first", err)
	}
	if _, err := os.Lstat(requestPath); err != nil {
		t.Fatal("request removed before response retirement completed", err)
	}
}

func TestInstalledPermissionRetireRefusesForeignHandover(t *testing.T) {
	directory := t.TempDir()
	if err := os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}
	requestPath := filepath.Join(directory, "request")
	responsePath := filepath.Join(directory, "response")
	check := func() error { return nil }
	// A shared-mode request is not a private handover; retirement refuses and
	// retains it rather than removing a foreign file.
	if err := os.WriteFile(requestPath, []byte("shared request"), 0600); err != nil {
		t.Fatal(err)
	}
	// Apply the shared mode after creation so the test runner's umask cannot mask it.
	if err := os.Chmod(requestPath, 0644); err != nil {
		t.Fatal(err)
	}
	if err := installedPermissionRetire(requestPath, responsePath, check); err == nil {
		t.Fatal("retired a shared-mode request")
	}
	if _, err := os.Lstat(requestPath); err != nil {
		t.Fatal("refusal removed the foreign request", err)
	}
	if err := os.Chmod(requestPath, 0600); err != nil {
		t.Fatal(err)
	}
	// A symlink response is refused without touching its target.
	target := filepath.Join(directory, "target")
	if err := os.WriteFile(target, []byte("untouched"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, responsePath); err != nil {
		t.Fatal(err)
	}
	if err := installedPermissionRetire(requestPath, responsePath, check); err == nil {
		t.Fatal("retired a symlink response")
	}
	if raw, err := os.ReadFile(target); err != nil || string(raw) != "untouched" {
		t.Fatal("symlink refusal changed target", err)
	}
	if _, err := os.Lstat(responsePath); err != nil {
		t.Fatal("refusal removed the symlink entry", err)
	}
}

func TestInstalledPermissionRetireRefusesParentReplacementAndCancellation(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "handover")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	requestPath := filepath.Join(directory, "request")
	responsePath := filepath.Join(directory, "response")
	if err := os.WriteFile(requestPath, []byte("prior request"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := installedPermissionRetire(requestPath, responsePath, func() error { return context.Canceled }); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled retirement", err)
	}
	if _, err := os.Lstat(requestPath); err != nil {
		t.Fatal("cancelled retirement removed the request", err)
	}
	calls := 0
	check := func() error {
		calls++
		if calls == 2 {
			if err := os.Rename(directory, directory+"-original"); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(directory, 0700); err != nil {
				t.Fatal(err)
			}
		}
		return nil
	}
	if err := installedPermissionRetire(requestPath, responsePath, check); err == nil {
		t.Fatal("accepted replacement parent")
	}
}
