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
	cmd := exec.CommandContext(t.Context(), program, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: credential, Pdeathsig: syscall.SIGKILL}
	var diagnostic bytes.Buffer
	cmd.Stderr = &diagnostic
	runtime.LockOSThread()
	out, err := cmd.Output()
	runtime.UnlockOSThread()
	var response struct{ Outcome string }
	decodeErr := json.Unmarshal(out, &response)
	if err != nil || decodeErr != nil || response.Outcome != outcome {
		t.Fatalf("command %v: exit=%v JSON=%v outcome=%q want=%q stdout=%s stderr=%s",
			args, err, decodeErr, response.Outcome, outcome, out, diagnostic.String())
	}
	return out
}
