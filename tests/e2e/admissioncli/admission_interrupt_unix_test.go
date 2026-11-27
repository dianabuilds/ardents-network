//go:build unix

package admissioncli_test

import (
	"encoding/json"
	"errors"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestAdmissionHolderInterruptJoinsInputAndReleasesLease(t *testing.T) {
	binary, _, config := admissionConsoleFixture(t)
	command := exec.CommandContext(t.Context(), binary, "admission", "holder", "--config", config)
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	joined := false
	defer func() {
		if !joined {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
	}()
	if err := json.NewEncoder(input).Encode(map[string]string{"operation": "status"}); err != nil {
		t.Fatal(err)
	}
	var status processfixture.AdmissionReply
	if err := json.NewDecoder(output).Decode(&status); err != nil || status.Outcome != "completed" {
		t.Fatalf("holder status: %v, %+v", err, status)
	}
	if err := command.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	select {
	case err := <-done:
		joined = true
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 130 {
			t.Fatal("interrupt result", err)
		}
	case <-time.After(5 * time.Second):
		_ = command.Process.Kill()
		<-done
		joined = true
		t.Fatal("interrupted holder retained stdin or lease")
	}
	send, closeConsole := processfixture.OpenAdmissionConsole(t, binary, "holder", config)
	if reply := send(map[string]string{"operation": "status"}); reply.Outcome != "completed" {
		t.Fatal("interrupt retained lease", reply)
	}
	closeConsole()
}
