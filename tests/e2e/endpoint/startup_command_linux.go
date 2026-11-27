//go:build linux && (document_worker_installed || endpoint_launch_native)

package endpoint

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"runtime"
	"syscall"
	"testing"
)

// RunStartupCommand runs the exact CLI operation and checks its declared outcome.
func RunStartupCommand(t testing.TB, program string, credential *syscall.Credential, outcome string, args ...string) []byte {
	t.Helper()
	out, diagnostic, err := runStartupCommand(t, program, credential, args...)
	var response struct{ Outcome string }
	decodeErr := json.Unmarshal(out, &response)
	if err != nil || decodeErr != nil || response.Outcome != outcome {
		t.Fatalf("command %v: exit=%v JSON=%v outcome=%q want=%q stdout=%s stderr=%s",
			args, err, decodeErr, response.Outcome, outcome, out, diagnostic)
	}
	return out
}

const prepareHistoryOperation = "endpoint.prepare-history"

// The history command publishes its finite result on the diagnostic stream.
func prepareEndpointHistory(t testing.TB, program string, credential *syscall.Credential, expectedOperation, plan, bundle, profile string) {
	t.Helper()
	out, diagnostic, err := runStartupCommand(t, program, credential, "endpoint", "prepare-history", plan, bundle, profile)
	var event struct{ Operation, Phase, Outcome string }
	decodeErr := json.Unmarshal(diagnostic, &event)
	if err != nil || len(out) != 0 || decodeErr != nil ||
		event.Operation != expectedOperation || event.Phase != "history-preparation" || event.Outcome != "prepared" {
		t.Fatalf("prepare Endpoint history: want operation=%q exit=%v JSON=%v stdout=%s stderr=%s",
			expectedOperation, err, decodeErr, out, diagnostic)
	}
}

func runStartupCommand(t testing.TB, program string, credential *syscall.Credential, args ...string) (output, diagnostic []byte, err error) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), program, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: credential, Pdeathsig: syscall.SIGKILL}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	out, err := cmd.Output()
	return out, stderr.Bytes(), err
}
