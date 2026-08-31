//go:build referencec2 && (h4_3b_multihost || h4_8_a11)

package service_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestH48A11StatusRetainsSeparateUserStreamsAndExitStatus(t *testing.T) {
	root := t.TempDir()
	status := h48A11Status{root: root}
	status.retainUser(t, commandResult{stdout: []byte("bounded-user-output\n"), stderr: []byte("bounded-user-error\n")})

	assertA11EvidenceFile(t, filepath.Join(root, "user-evidence", "stdout.log"), "bounded-user-output\n")
	assertA11EvidenceFile(t, filepath.Join(root, "user-evidence", "stderr.log"), "bounded-user-error\n")
	assertA11EvidenceFile(t, filepath.Join(root, "user-evidence", "process.txt"),
		"schema=ardents-h4-8-a11-user-evidence-v1\nstreams=separate\ndisposition=success\nexit_code=0\n")
	assertA11EvidenceFile(t, filepath.Join(root, "user-evidence.complete"),
		"schema=ardents-h4-8-a11-user-evidence-v1\nretained=user-evidence/stdout.log,user-evidence/stderr.log,user-evidence/process.txt\n")
}

func TestH48A11StatusRetainsFailedUserExitCode(t *testing.T) {
	if os.Getenv("ARDENTS_A11_TEST_EXIT_7") == "1" {
		os.Exit(7)
	}
	command := exec.Command(os.Args[0], "-test.run=^TestH48A11StatusRetainsFailedUserExitCode$")
	command.Env = append(os.Environ(), "ARDENTS_A11_TEST_EXIT_7=1")
	err := command.Run()
	if err == nil {
		t.Fatal("A11 User evidence child unexpectedly succeeded")
	}

	root := t.TempDir()
	status := h48A11Status{root: root}
	status.retainUser(t, commandResult{stdout: []byte("failed-user-output\n"), stderr: []byte("failed-user-error\n"), err: err})
	assertA11EvidenceFile(t, filepath.Join(root, "user-evidence", "process.txt"),
		"schema=ardents-h4-8-a11-user-evidence-v1\nstreams=separate\ndisposition=exit-error\nexit_code=7\n")
}

func TestH48A11CommandCaptureRetainsSeparateStreamsAndCombinedOrder(t *testing.T) {
	capture := new(commandCapture)
	stdout := commandStreamCapture{capture: capture}
	stderr := commandStreamCapture{capture: capture, stderr: true}
	_, _ = stdout.Write([]byte("out-1\n"))
	_, _ = stderr.Write([]byte("err-1\n"))
	_, _ = stdout.Write([]byte("out-2\n"))
	result := capture.result(nil)
	if string(result.stdout) != "out-1\nout-2\n" || string(result.stderr) != "err-1\n" ||
		string(result.output) != "out-1\nerr-1\nout-2\n" {
		t.Fatalf("A11 command capture = stdout %q stderr %q combined %q", result.stdout, result.stderr, result.output)
	}
}

func assertA11EvidenceFile(t *testing.T, path, expected string) {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != expected {
		t.Fatalf("A11 evidence %s = %q, want %q", path, contents, expected)
	}
}
