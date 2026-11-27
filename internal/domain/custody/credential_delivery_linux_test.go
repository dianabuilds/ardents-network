package custody

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
)

func TestCredentialDeliveryKeepsSeparateRetryAccessPolicies(t *testing.T) {
	directory := t.TempDir()
	body := bytes.Repeat([]byte{1}, admission.PermissionSize)
	public := Receipt{Operation: OperationIssueServiceCredential, ServiceResponse: body}
	private := Receipt{Operation: OperationIssueAdmissionPermission, AdmissionPermission: body}
	path := filepath.Join(directory, "response")

	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := public.PublishServiceResponse(path); err != nil {
		t.Fatalf("exact public retry: %v", err)
	}
	if err := private.PublishAdmissionPermission(path); err == nil {
		t.Fatal("private delivery accepted a group-readable destination with identical bytes")
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := private.PublishAdmissionPermission(path); err != nil {
		t.Fatalf("exact private retry: %v", err)
	}
}

func TestPrivatePermissionFirstDeliveryAndExactRetry(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "permission")
	body := bytes.Repeat([]byte{7}, admission.PermissionSize)
	receipt := Receipt{Operation: OperationIssueAdmissionPermission, AdmissionPermission: body}
	if err := receipt.PublishAdmissionPermission(path); err != nil {
		t.Fatal("first private delivery", err)
	}
	original, err := os.Stat(path)
	if err != nil || !privatePermissionFile(original) {
		t.Fatal("published file lacks owner-only access", err)
	}
	if err := receipt.PublishAdmissionPermission(path); err != nil {
		t.Fatal("exact retry", err)
	}
	after, err := os.Stat(path)
	if err != nil || !os.SameFile(original, after) {
		t.Fatal("exact retry replaced the original", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, body) {
		t.Fatal("permission bytes differ", err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 1 {
		t.Fatal("staging file remained", err)
	}
}

func TestPrivatePermissionRefusesUnsupportedFilesystemBeforeStaging(t *testing.T) {
	receipt := Receipt{Operation: OperationIssueAdmissionPermission, AdmissionPermission: make([]byte, admission.PermissionSize)}
	if err := receipt.PublishAdmissionPermission("/proc/ardents-private-permission-test"); !errors.Is(err, ErrPrivatePermissionUnavailable) {
		t.Fatal("unsupported filesystem did not return the typed refusal", err)
	}
}

// The kernel mount ID, rather than a path or a shared ext-family magic,
// decides which filesystem protects the held object.
func TestPrivatePermissionMountIdentity(t *testing.T) {
	const ext4 = "12 1 8:1 / /storage rw - ext4 /dev/sda rw\n"
	const ext3 = "13 1 8:2 / /storage rw - ext3 /dev/sdb rw\n"
	tests := []struct {
		name   string
		mounts string
		id     uint64
		want   bool
	}{
		{"original ext4 despite another mount at its path", ext4 + ext3, 12, true},
		{"other ext family refuses", ext4 + ext3, 13, false},
		{"unmounted original refuses", ext3, 12, false},
		{"duplicate identity refuses", ext4 + ext4, 12, false},
		{"malformed table refuses", ext4 + "bad record\n", 12, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := requireExt4Mount(strings.NewReader(test.mounts), test.id)
			if (err == nil) != test.want {
				t.Fatalf("mount acceptance = %v, want %v: %v", err == nil, test.want, err)
			}
		})
	}
}
