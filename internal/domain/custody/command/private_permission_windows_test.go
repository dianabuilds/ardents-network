package command

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/domain/custody/delivery"
	"golang.org/x/sys/windows"
)

func TestWindowsPrivatePermissionUnsafeRetryRefusesBeforeVault(t *testing.T) {
	directory := t.TempDir()
	root, destination := filepath.Join(directory, "absent-vault"), filepath.Join(directory, "permission")
	original := []byte("untrusted existing output")
	if err := os.WriteFile(destination, original, 0600); err != nil {
		t.Fatal(err)
	}
	descriptor, err := windows.SecurityDescriptorFromString("D:P(A;;GA;;;WD)")
	if err != nil {
		t.Fatal(err)
	}
	acl, _, err := descriptor.DACL()
	if err != nil || acl == nil {
		t.Fatal("construct broad control", err)
	}
	if err := windows.SetNamedSecurityInfo(destination, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		t.Fatal("set broad control", err)
	}
	before, err := windows.GetNamedSecurityInfo(destination, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	probe := &privateDeliveryProbe{}
	var output bytes.Buffer
	err = run(t.Context(), []string{"issue-admission-permission", "-vault-root", root,
		"-record", "record", "-request", "absent-request", "-permission-output", destination}, &output, probe)
	if !errors.Is(err, delivery.ErrPrivatePermissionUnavailable) || probe.calls != 0 || output.Len() != 0 {
		t.Fatal("unsafe existing output reached private authority or output", err, probe.calls)
	}
	if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("unsafe retry opened or created the Vault", err)
	}
	after, err := windows.GetNamedSecurityInfo(destination, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil || before.String() != after.String() {
		t.Fatal("refusal changed the existing DACL", err)
	}
	contents, err := os.ReadFile(destination)
	if err != nil || !bytes.Equal(contents, original) {
		t.Fatal("refusal changed existing bytes", err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 1 {
		t.Fatal("refusal left a staging artifact", err, entries)
	}
}
