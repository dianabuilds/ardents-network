package enrollment

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func exactExecutable(actual, expected string, artifact []byte, packageArtifact bool) error {
	actualInfo, err := os.Stat(actual)
	if err != nil {
		return fmt.Errorf("inspect running executable: %w", err)
	}
	expectedInfo, err := os.Stat(expected)
	if err != nil {
		return fmt.Errorf("inspect enrolled executable: %w", err)
	}
	if !os.SameFile(actualInfo, expectedInfo) {
		return errors.New("running executable is not the enrolled artifact")
	}
	running, err := readEnrollmentFile(actual, packageArtifact)
	if err != nil || !bytes.Equal(running, artifact) {
		return errors.New("running executable does not match the enrolled artifact")
	}
	return nil
}

// VerifyRunningCompanion proves that the current process is the exact named
// regular-file companion from the already verified enrollment inventory. It is
// intended for a bounded participant tool such as ardents-control; it does not
// re-verify or execute the companion.
func VerifyRunningCompanion(request Request, name string, artifact []byte) error {
	if request.BundleRoot == "" || !validName(name) || len(artifact) == 0 {
		return errors.New("alpha enrollment companion is invalid")
	}
	root, err := filepath.Abs(request.BundleRoot)
	if err != nil {
		return errors.New("resolve alpha bundle")
	}
	running, err := os.Executable()
	if err != nil {
		return errors.New("resolve alpha enrollment companion")
	}
	return exactExecutable(running, filepath.Join(root, name), artifact, false)
}
