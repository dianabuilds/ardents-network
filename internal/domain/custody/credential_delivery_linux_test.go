package custody

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
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
