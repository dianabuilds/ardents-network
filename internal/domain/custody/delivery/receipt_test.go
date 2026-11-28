package delivery

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/custody"
)

func TestInvalidReceiptCannotPublish(t *testing.T) {
	for _, tc := range []struct {
		name    string
		publish func(string) error
	}{
		{"service wrong operation", func(path string) error {
			return PublishServiceResponse(path, custody.Receipt{Operation: custody.OperationIssueAdmissionPermission, ServiceResponse: []byte("response")})
		}},
		{"service empty response", func(path string) error {
			return PublishServiceResponse(path, custody.Receipt{Operation: custody.OperationIssueServiceCredential})
		}},
		{"permission wrong operation", func(path string) error {
			return PublishAdmissionPermission(path, custody.Receipt{Operation: custody.OperationIssueServiceCredential, AdmissionPermission: make([]byte, admission.PermissionSize)})
		}},
		{"permission wrong size", func(path string) error {
			return PublishAdmissionPermission(path, custody.Receipt{Operation: custody.OperationIssueAdmissionPermission, AdmissionPermission: []byte{1}})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := tc.publish(filepath.Join(dir, "output")); err == nil || errors.Is(err, ErrPrivatePermissionUnavailable) {
				t.Fatalf("receipt was not refused before platform preflight: %v", err)
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 0 {
				t.Fatalf("invalid receipt produced filesystem effects: %v, %v", entries, err)
			}
		})
	}
}
