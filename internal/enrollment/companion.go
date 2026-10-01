package enrollment

import (
	"bytes"
	"errors"
	"fmt"
	"os"
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
