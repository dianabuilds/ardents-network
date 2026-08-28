//go:build h4_3b_multihost || h4_8_a11

package service_test

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type h48A11Status struct{ root string }

func openH48A11Status(t *testing.T, scenario referenceC2Scenario, environment h43MultiHostEnvironment) h48A11Status {
	t.Helper()
	if !scenario.dynamicWorkload.configured() {
		return h48A11Status{}
	}
	if os.Getenv("ARDENTS_H4_8_A11_SUFFIX") == "" {
		t.Fatal("A11 configured workload requires ARDENTS_H4_8_A11_SUFFIX")
	}
	root := os.Getenv("ARDENTS_H4_8_A11_STATUS_ROOT")
	info, err := os.Stat(root)
	if root == "" || !filepath.IsAbs(root) || err != nil || !info.IsDir() {
		t.Fatal("ARDENTS_H4_8_A11_STATUS_ROOT must name an existing absolute directory")
	}
	status := h48A11Status{root: root}
	status.create(t, "contract.ready", fmt.Sprintf("test=%s\ncontainer=%s\nremote_directory=%s\n",
		t.Name(), environment.container, environment.remoteDirectory))
	return status
}

func (status h48A11Status) remoteReady(t *testing.T, environment h43MultiHostEnvironment) {
	t.Helper()
	status.create(t, "remote.ready", fmt.Sprintf("container=%s\nremote_directory=%s\n", environment.container, environment.remoteDirectory))
}

func (status h48A11Status) userReady(t *testing.T, running *killableCommand) {
	t.Helper()
	if running == nil || running.command == nil || running.command.Process == nil || running.command.Process.Pid <= 0 {
		t.Fatal("A11 User process PID is unavailable")
	}
	status.create(t, "user.ready", fmt.Sprintf("pid=%d\n", running.command.Process.Pid))
}

func (status h48A11Status) complete(t *testing.T) {
	t.Helper()
	status.create(t, "complete", "passed=true\n")
}

func (status h48A11Status) retainRemote(t *testing.T, remote h43RemoteC2) {
	t.Helper()
	if status.root == "" {
		return
	}
	directory := filepath.Join(status.root, "remote-evidence")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatalf("create A11 remote-evidence directory: %v", err)
	}
	evidence := remote.captureEvidence(t)
	retained := h48A11Status{root: directory}
	retained.create(t, "capture.txt", string(evidence))
	status.create(t, "remote-evidence.complete", "schema=ardents-h4-8-a11-remote-evidence-v1\nretained=remote-evidence/capture.txt\n")
}

func (status h48A11Status) retainRemoteFailure(t *testing.T, remote h43RemoteC2) {
	t.Helper()
	if status.root == "" || !t.Failed() {
		return
	}
	if err := status.retainRemoteFailureWith(remote.captureFailureEvidence); err != nil {
		t.Errorf("retain failed A11 remote attempt before cleanup: %v", err)
	}
}

func (status h48A11Status) retainRemoteFailureWith(capture func() ([]byte, error)) error {
	if status.root == "" {
		return nil
	}
	if _, err := os.Stat(filepath.Join(status.root, "remote-evidence.complete")); err == nil {
		return nil
	}
	marker := filepath.Join(status.root, "remote-evidence.failed-attempt")
	if _, err := os.Stat(marker); err == nil {
		return nil
	}
	directory := filepath.Join(status.root, "remote-evidence")
	if err := os.Mkdir(directory, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	evidence, captureErr := capture()
	result := "schema=ardents-h4-8-a11-failed-remote-evidence-v1\nstatus=capture-error\n"
	if len(evidence) != 0 {
		path := filepath.Join(directory, "capture.txt")
		if err := writeH48A11Exclusive(path, evidence); err != nil {
			return err
		}
	}
	if captureErr == nil {
		result = "schema=ardents-h4-8-a11-failed-remote-evidence-v1\nstatus=retained\nretained=remote-evidence/capture.txt\n"
	} else {
		detail := strings.ReplaceAll(captureErr.Error(), "\n", " ")
		if len(detail) > 4096 {
			detail = detail[:4096]
		}
		result += "error=" + detail + "\n"
		if len(evidence) != 0 {
			result += "retained=remote-evidence/capture.txt\n"
		}
	}
	if err := writeH48A11Exclusive(marker, []byte(result)); err != nil {
		return err
	}
	return captureErr
}

func (status h48A11Status) retainUser(t *testing.T, result commandResult) {
	t.Helper()
	if status.root == "" {
		return
	}
	if len(result.stdout)+len(result.stderr) > 1<<20 {
		t.Fatalf("A11 User evidence size = %d, want at most 1048576 bytes", len(result.stdout)+len(result.stderr))
	}
	directory := filepath.Join(status.root, "user-evidence")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatalf("create A11 user-evidence directory: %v", err)
	}
	retained := h48A11Status{root: directory}
	retained.create(t, "stdout.log", string(result.stdout))
	retained.create(t, "stderr.log", string(result.stderr))
	disposition, exitCode := h48A11ProcessStatus(result.err)
	retained.create(t, "process.txt", fmt.Sprintf("schema=ardents-h4-8-a11-user-evidence-v1\nstreams=separate\ndisposition=%s\nexit_code=%d\n", disposition, exitCode))
	status.create(t, "user-evidence.complete", "schema=ardents-h4-8-a11-user-evidence-v1\nretained=user-evidence/stdout.log,user-evidence/stderr.log,user-evidence/process.txt\n")
}

func h48A11ProcessStatus(err error) (string, int) {
	if err == nil {
		return "success", 0
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return "exit-error", exitError.ExitCode()
	}
	return "wait-error", -1
}

func (status h48A11Status) create(t *testing.T, name, contents string) {
	t.Helper()
	if status.root == "" {
		return
	}
	path := filepath.Join(status.root, name)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			t.Fatalf("A11 status path already exists: %s", path)
		}
		t.Fatal(err)
	}
	if _, err := file.WriteString(contents); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeH48A11Exclusive(path string, contents []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(contents); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
