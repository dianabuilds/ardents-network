package custody

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
)

// ErrPrivatePermissionUnavailable means that owner-only Permission delivery
// cannot be established on the selected platform or filesystem.
var ErrPrivatePermissionUnavailable = errors.New("private admission permission delivery unavailable")

// CheckAdmissionPermissionDestination checks whether the selected destination
// can support private delivery. Publication rechecks the original objects; this
// preflight is not a reservation and does not grant authority.
func CheckAdmissionPermissionDestination(path string) error {
	return checkPrivatePermissionDestination(path)
}

// PublishServiceResponse delivers the public response from a committed
// Service Credential operation. An exact existing response permits retry.
func (receipt Receipt) PublishServiceResponse(path string) error {
	const maximumResponseBytes = 1024
	if receipt.Operation != OperationIssueServiceCredential ||
		len(receipt.ServiceResponse) == 0 || len(receipt.ServiceResponse) > maximumResponseBytes {
		return errors.New("service Credential response is invalid")
	}
	return publishCredential(path, "service Credential response", ".ardents-service-credential-response-", receipt.ServiceResponse, false)
}

// PublishAdmissionPermission delivers the owner-only Permission from a committed
// Admission operation. An exact existing Permission permits retry.
func (receipt Receipt) PublishAdmissionPermission(path string) error {
	if receipt.Operation != OperationIssueAdmissionPermission || len(receipt.AdmissionPermission) != admission.PermissionSize {
		return errors.New("admission permission is invalid")
	}
	if err := CheckAdmissionPermissionDestination(path); err != nil {
		return err
	}
	return publishCredential(path, "admission permission", ".ardents-admission-permission-", receipt.AdmissionPermission, true)
}

// A complete flushed staging file is linked without replacing the destination.
// The original Vault operation has already finished and does not depend on
// whether delivery succeeds. A retry may only reuse identical output bytes.
func publishCredential(path, name, stagingPrefix string, body []byte, private bool) (resultErr error) {
	destination, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve %s destination: %w", name, err)
	}
	parent := filepath.Dir(destination)
	staging, err := os.CreateTemp(parent, stagingPrefix)
	if err != nil {
		return fmt.Errorf("create %s staging file: %w", name, err)
	}
	stagingPath, stagingOpen := staging.Name(), true
	defer func() {
		if stagingOpen {
			resultErr = errors.Join(resultErr, staging.Close())
		}
		if err := os.Remove(stagingPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			resultErr = errors.Join(resultErr, fmt.Errorf("remove %s staging file: %w", name, err))
			return
		}
		if err := syncDirectory(parent); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("flush %s directory: %w", name, err))
		}
	}()
	if err := staging.Chmod(0o600); err != nil {
		return fmt.Errorf("protect %s staging file: %w", name, err)
	}
	if private {
		opened, statErr := staging.Stat()
		named, nameErr := os.Lstat(stagingPath)
		if statErr != nil || nameErr != nil || !privatePermissionFile(opened) ||
			!privatePermissionFile(named) || !os.SameFile(opened, named) {
			return errors.Join(ErrPrivatePermissionUnavailable, statErr, nameErr)
		}
		if err := checkPrivatePermissionFilesystem(staging); err != nil {
			return err
		}
	}
	written, err := staging.Write(body)
	if err == nil && written != len(body) {
		err = io.ErrShortWrite
	}
	if err != nil {
		return fmt.Errorf("write %s staging file: %w", name, err)
	}
	if err := staging.Sync(); err != nil {
		return fmt.Errorf("flush %s staging file: %w", name, err)
	}
	err = staging.Close()
	stagingOpen = false
	if err != nil {
		return fmt.Errorf("close %s staging file: %w", name, err)
	}
	if err := os.Link(stagingPath, destination); err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("publish %s: %w", name, err)
	}
	if !credentialMatches(destination, body, private) {
		return errors.New(name + " destination conflicts")
	}
	return nil
}

func credentialMatches(path string, expected []byte, private bool) bool {
	accepts := func(info os.FileInfo) bool {
		return info.Mode().IsRegular() && info.Size() == int64(len(expected)) &&
			(!private || privatePermissionFile(info))
	}
	named, err := os.Lstat(path)
	if err != nil || !accepts(named) {
		return false
	}
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	opened, err := file.Stat()
	if private && checkPrivatePermissionFilesystem(file) != nil {
		_ = file.Close()
		return false
	}
	if err != nil || !accepts(opened) || !os.SameFile(named, opened) {
		_ = file.Close()
		return false
	}
	body, readErr := io.ReadAll(io.LimitReader(file, int64(len(expected))+1))
	defer clear(body)
	final, statErr := file.Stat()
	closeErr := file.Close()
	named, namedErr := os.Lstat(path)
	return readErr == nil && statErr == nil && closeErr == nil && namedErr == nil &&
		accepts(final) && accepts(named) && os.SameFile(opened, final) && os.SameFile(opened, named) &&
		bytes.Equal(body, expected)
}
