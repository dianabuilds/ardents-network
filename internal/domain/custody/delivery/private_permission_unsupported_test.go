//go:build !linux && !windows

package delivery

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/custody"
)

func TestPrivatePermissionUnavailableBeforeStaging(t *testing.T) {
	directory := t.TempDir()
	if runtime.GOOS == "darwin" {
		if output, err := exec.Command("/bin/chmod", "+a", "everyone allow read,file_inherit", directory).CombinedOutput(); err != nil {
			t.Fatalf("create inherited ACL: %v: %s", err, output)
		}
	}
	body := bytes.Repeat([]byte{7}, admission.PermissionSize)
	path := filepath.Join(directory, "permission")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "darwin" {
		output, err := exec.Command("/bin/ls", "-le", path).CombinedOutput()
		if err != nil || !strings.Contains(string(output), "inherited allow read") {
			t.Fatalf("fixture did not inherit the ACL: %v: %s", err, output)
		}
	}
	receipt := custody.Receipt{Operation: custody.OperationIssueAdmissionPermission, AdmissionPermission: body}
	if err := PublishAdmissionPermission(path, receipt); !errors.Is(err, ErrPrivatePermissionUnavailable) {
		t.Fatal("unverified private output was accepted", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, body) {
		t.Fatal("refusal changed existing output", err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 1 {
		t.Fatal("refusal created a staging file", err)
	}
}
