package blockedentry

import (
	"errors"
	"os"
	"path/filepath"
)

const finalRunnerPath = "runtime/network-live.test"

func exportFinalRunner(image, outputRoot, expectedHash string) (path string, returnErr error) {
	token, err := receiptToken()
	if err != nil {
		return "", err
	}
	name := "ardents-s5-runner-export-" + token
	if _, err := boundedReceiptCommand("docker", "container", "inspect", name); err == nil {
		return "", errors.New("random final runner export name already exists")
	}
	_, err = boundedReceiptCommand("docker", "container", "create", "--name", name,
		"--label", "io.ardents.stage5.receipt-owner="+token, "--entrypoint", "/bin/true", image)
	if err != nil {
		return "", err
	}
	defer func() { returnErr = errors.Join(returnErr, removeOwnedReceiptContainer(name, token)) }()
	path = filepath.Join(outputRoot, filepath.FromSlash(finalRunnerPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	complete := false
	defer func() {
		if !complete {
			removeErr := os.Remove(path)
			if os.IsNotExist(removeErr) {
				removeErr = nil
			}
			returnErr = errors.Join(returnErr, removeErr)
		}
	}()
	if _, err := boundedReceiptCommand("docker", "container", "cp",
		name+":/usr/local/bin/network-live.test", path); err != nil {
		return "", err
	}
	hash, _, err := hashFile(path)
	if err != nil || hash != expectedHash {
		return "", errors.Join(err, errors.New("exported final runner differs from the product receipt"))
	}
	if err := os.Chmod(path, 0o400); err != nil {
		return "", err
	}
	complete = true
	return path, nil
}
