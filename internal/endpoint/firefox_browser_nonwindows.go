//go:build !windows

package endpoint

import "os/exec"

// startFirefox asks the explicitly selected Firefox executable to open one
// validated Reference origin. Firefox remains outside Endpoint ownership.
func startFirefox(executable, referenceURL string) error {
	command := exec.Command(executable, "-new-window", referenceURL)
	if err := command.Start(); err != nil {
		return err
	}
	return command.Process.Release()
}
