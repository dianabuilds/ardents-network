package enrollment

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
)

// VerifyRunningCompanion proves that this process is the already verified
// native-host companion without granting any Network or release authority.
func VerifyRunningCompanion(request Request, name string, artifact []byte) error {
	if request.BundleRoot == "" || !validName(name) || len(artifact) == 0 {
		return errors.New("browser enrollment companion is invalid")
	}
	running, err := os.Executable()
	if err != nil {
		return errors.New("resolve browser enrollment companion")
	}
	return exactExecutable(running, filepath.Join(request.BundleRoot, name), artifact)
}

func exactExecutable(actual, expected string, artifact []byte) error {
	actualInfo, err := os.Stat(actual)
	if err != nil {
		return err
	}
	expectedInfo, err := os.Stat(expected)
	if err != nil || !os.SameFile(actualInfo, expectedInfo) {
		return errors.New("running executable is not the enrolled artifact")
	}
	running, err := readRegular(actual)
	if err != nil || !bytes.Equal(running, artifact) {
		return errors.New("running executable does not match the enrolled artifact")
	}
	return nil
}
