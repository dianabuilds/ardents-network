//go:build linux

package main

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
