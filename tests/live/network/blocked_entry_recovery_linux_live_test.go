//go:build linux && live

package network_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

func runBlockedRecoveryParent(t *testing.T) {
	t.Helper()
	timeline := startBlockedTimeline(t)
	prepareBlockedState(t, "route-state", "route-network")
	prepareBlockedState(t, "bridge-network", "bridge-network")
	prepareBlockedState(t, "local-roles", "local-roles")
	runBlockedCommand(t, "/usr/local/bin/ardents-bridge", "import", "/run/secure/import.json")
	initial := exec.Command("/usr/local/bin/ardents-route", "run", "/run/secure/route.json")
	var initialOutput bytes.Buffer
	initial.Stdout, initial.Stderr = &initialOutput, &initialOutput
	if err := initial.Start(); err != nil {
		t.Fatal(err)
	}
	cutoffPath := blockedSync() + "/recovery-cutoff.json"
	waitBlockedFile(t, cutoffPath, 20*time.Second)
	var cutoff blockedRecoveryCutoff
	readBlockedJSON(t, cutoffPath, &cutoff)
	if cutoff.LastByte.IsZero() || time.Since(cutoff.LastByte) > 2*time.Second {
		t.Fatalf("recovery cutoff is stale: %+v", cutoff)
	}
	_ = initial.Wait()
	prepareBlockedObservation(t, blockedNegativeManifest("recovery"))
	sendBlockedPathControl(t)
	commandOutput, commandDone := runBlockedRecoveryBridge(t, cutoff.LastByte, timeline)
	if commandDone.Sub(cutoff.LastByte) > 14*time.Second {
		t.Fatalf("recovery Bridge cleanup missed +14s: %s\n%s", commandDone.Sub(cutoff.LastByte), commandOutput)
	}
	if !strings.Contains(commandOutput, "bridge-deadline-exceeded") &&
		!strings.Contains(commandOutput, "context canceled") {
		t.Fatalf("recovery command terminal is not clipped:\n%s", commandOutput)
	}
	evidence := assertBlockedRecoveryEvidence(t)
	finishBlockedObservation(t)
	if _, err := os.Stat("/run/state/candidate"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("recovery candidate residue: %v", err)
	}
	result := map[string]any{"kind": "recovery-result", "class": "bridge-deadline-exceeded",
		"attempt": fmt.Sprintf("%x", evidence.AttemptDigest), "contact_starts": 1,
		"later_ordinals": 0, "cleanup": true, "last_byte": cutoff.LastByte,
		"command_done": commandDone}
	raw, _ := json.Marshal(result)
	fmt.Println(string(raw))
}

func runBlockedRecoveryBridge(t *testing.T, lastByte time.Time, timeline blockedTimeline) (string, time.Time) {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	transition, err := os.ReadFile("/run/secure/transition.bin")
	if err != nil {
		t.Fatal(err)
	}
	transition = stampBlockedTransition(t, transition, timeline)
	command := exec.Command("/usr/local/bin/ardents-route", "run", "/run/secure/route.json",
		"--entry-plan", "/run/secure/entry.json")
	command.ExtraFiles = []*os.File{reader}
	var output bytes.Buffer
	command.Stdout, command.Stderr = &output, &output
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	_, writeErr := writer.Write(transition)
	_ = writer.Close()
	_ = reader.Close()
	if writeErr != nil {
		_ = command.Process.Kill()
		t.Fatal(writeErr)
	}
	stopAt := lastByte.Add(8 * time.Second)
	time.AfterFunc(max(time.Duration(0), time.Until(stopAt)), func() { _ = command.Process.Signal(syscall.SIGTERM) })
	_ = command.Wait()
	return output.String(), time.Now()
}
