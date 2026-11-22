package custody

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/admission"
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
