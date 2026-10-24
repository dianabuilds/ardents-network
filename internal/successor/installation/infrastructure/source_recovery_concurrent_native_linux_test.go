//go:build linux && amd64 && network_signing_native

package infrastructure

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// Separate sibling helpers are essential: each tracer owns Wait4(-1) only for
// its own root. No tracer can reap the other caller or confuse its lifetime.
// These are real file/process owners, not authenticated installed recovery.
func TestSourceRecoveryConcurrentNative(t *testing.T) {
	program := os.Getenv("ARDENTS_SOURCE_RECOVERY_CONCURRENT_PROGRAM")
	if program == "" {
		return // The dedicated native file profile selects the actual driver ELF.
	}
	marker, err := os.ReadFile("/etc/ardents-source-stopped-native-stand")
	if err != nil || string(marker) != "ardents-source-stopped-native-v1\n" || os.Geteuid() != 0 || !sourcePath(program) {
		t.Fatal("original native file/process stand required", err)
	}
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "original")
	t.Cleanup(func() {
		for _, name := range []string{"first", "second"} {
			if raw, err := os.ReadFile(root + "-" + name + ".log"); err == nil {
				t.Logf("actual %s caller output:\n%s", name, raw)
			}
		}
	})
	command := func(mode, name, selectedRoot string) *exec.Cmd {
		cmd := exec.Command(helper, "-test.run=^TestSourceRefusalObserverNative$", "-test.v", "-test.timeout=60s")
		for _, value := range os.Environ() {
			if !strings.HasPrefix(value, "ARDENTS_SOURCE_REFUSAL_OBSERVER_MODE=") && !strings.HasPrefix(value, "ARDENTS_SOURCE_CUT_OBSERVER_") {
				cmd.Env = append(cmd.Env, value)
			}
		}
		cmd.Env = append(cmd.Env, "ARDENTS_SOURCE_REFUSAL_OBSERVER_MODE="+mode, "ARDENTS_SOURCE_CUT_OBSERVER_PROGRAM="+program, "ARDENTS_SOURCE_CUT_OBSERVER_ROOT="+selectedRoot, "ARDENTS_SOURCE_CUT_OBSERVER_OUTPUT="+root+"-"+name+".log")
		return cmd
	}
	first := command("native-recovery-hold", "first", root)
	input, err := first.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := first.StdoutPipe()
	if err != nil {
		t.Fatal(errors.Join(err, input.Close()))
	}
	first.Stderr = first.Stdout
	if err := first.Start(); err != nil {
		t.Fatal(errors.Join(err, input.Close(), output.Close()))
	}
	ready := make(chan sourceCutObservation, 1)
	done := make(chan struct{})
	var captured bytes.Buffer
	var readErr error
	go func() {
		defer close(done)
		scanner := bufio.NewScanner(output)
		scanner.Buffer(make([]byte, 4096), 64<<10)
		for scanner.Scan() {
			line := scanner.Bytes()
			if captured.Len()+len(line)+1 > 1<<20 {
				readErr = errors.New("finite recovery helper output exceeded")
				_, drained := io.Copy(io.Discard, output)
				readErr = errors.Join(readErr, drained)
				return
			}
			captured.Write(line)
			captured.WriteByte('\n')
			if bytes.HasPrefix(line, []byte("SOURCE_RECOVERY_HOLD_READY ")) {
				var observation sourceCutObservation
				if err := json.Unmarshal(bytes.TrimPrefix(line, []byte("SOURCE_RECOVERY_HOLD_READY ")), &observation); err != nil {
					readErr = err
					continue
				}
				select {
				case ready <- observation:
				default:
					readErr = errors.New("repeated recovery hold readiness")
				}
			}
		}
		readErr = errors.Join(readErr, scanner.Err())
	}()
	joined := false
	defer func() {
		_ = input.Close() // An abandoned controller makes the helper kill/join.
		if !joined {
			<-done
			if err := first.Wait(); err != nil {
				t.Error("first recovery helper cleanup", err, captured.String())
			}
			if readErr != nil {
				t.Error(readErr)
			}
		}
	}()
	var held sourceCutObservation
	select {
	case held = <-ready:
	case <-done:
		t.Fatal("actual recovery did not reach held record entry", captured.String(), readErr)
	}
	if !held.RecoveryHeldBeforeRecord || !held.RecoveryRootOnlyAtHold || !held.RefusalAdmissionObserved || held.RootPID <= 0 || held.HeldRootThread <= 0 || held.InstalledExecution || held.RefusalOwnershipReleasedBeforeExit || held.RefusalOriginalScopeJoined || held.RootExit != -1 {
		t.Fatal("invalid actual original held lifetime", held)
	}
	var secondOutput sourceRefusalBusOutput
	second := command("native-held-both", "second", root)
	second.Stdout, second.Stderr = &secondOutput, &secondOutput
	if err := second.Run(); err != nil {
		t.Fatal("actual second helper exclusion", err, secondOutput.String())
	}
	decode := func(raw string, processes int) sourceCutObservation {
		t.Helper()
		var result sourceCutObservation
		found := 0
		for _, line := range strings.Split(raw, "\n") {
			if strings.HasPrefix(line, "SOURCE_REFUSAL_OBSERVATION ") {
				if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "SOURCE_REFUSAL_OBSERVATION ")), &result); err != nil {
					t.Fatal(err)
				}
				found++
			}
		}
		if found != 1 || !result.RefusalAdmissionObserved || !result.RefusalOwnershipReleasedBeforeExit || !result.RefusalOriginalScopeJoined || result.RefusalStoppedThreads < 2 || result.RootExit != 0 || result.ThreadsJoined < result.RefusalStoppedThreads || result.ThreadsJoined < result.Processes ||
			(processes == 1 && result.Processes != 1 || processes > 1 && result.Processes < processes) || result.InstalledExecution {
			t.Fatal("actual native before-return result incomplete", found, result, raw)
		}
		return result
	}
	other := decode(secondOutput.String(), 4)
	if other.RootPID == held.RootPID || other.RecoveryHeldBeforeRecord {
		t.Fatal("second caller borrowed original lifetime", held, other)
	}
	lock, err := os.Stat(filepath.Join(root, "release-lock"))
	if err != nil {
		t.Fatal(err)
	}
	lockNative := lock.Sys().(*syscall.Stat_t)
	if !other.RefusalExclusionObserved || other.RefusalExclusionKind != "release" || other.RefusalExclusionDevice != uint64(lockNative.Dev) || other.RefusalExclusionInode != lockNative.Ino {
		t.Fatal("second caller did not actually receive exclusion on the original lock inode", other)
	}
	if _, err := io.WriteString(input, "cancel\n"); err != nil {
		t.Fatal(err)
	}
	if err := input.Close(); err != nil {
		t.Fatal(err)
	}
	<-done
	err = first.Wait()
	joined = true
	t.Logf("actual first helper output:\n%s", captured.String())
	t.Logf("actual second helper output:\n%s", secondOutput.String())
	if err != nil || readErr != nil {
		t.Fatal("actual cancelled caller/helper join", err, readErr, captured.String())
	}
	completed := decode(captured.String(), 4)
	joinedOutput, err := os.ReadFile(root + "-first.log")
	if err != nil {
		t.Fatal("original helper join report", err)
	}
	joinedChildren := make(map[int]bool)
	for _, line := range strings.Split(string(joinedOutput), "\n") {
		if strings.HasPrefix(line, "SOURCE_RECOVERY_NATIVE_JOINED_CHILD ") {
			var pid int
			if n, err := fmt.Sscanf(line, "SOURCE_RECOVERY_NATIVE_JOINED_CHILD pid=%d", &pid); err != nil || n != 1 || pid <= 0 || joinedChildren[pid] {
				t.Fatal("three distinct actual joined helper processes required", line, err)
			}
			joinedChildren[pid] = true
		}
	}
	if len(joinedChildren) != 3 {
		t.Fatal("original helpers did not all reach actual Wait", string(joinedOutput))
	}
	if completed.RootPID != held.RootPID || !completed.RecoveryHeldBeforeRecord || !completed.RecoveryHeldImagePreserved || !completed.ExternalCancellationAcknowledged || completed.SignalledRootThread == 0 || completed.SignalledRootThread == completed.HeldRootThread {
		t.Fatal("actual recovery cancellation or original custody incomplete", held, completed)
	}
	if completed.RecoveryCancellationElapsedNS <= 0 || completed.RecoveryCancellationElapsedNS >= 120_000_000_000 {
		t.Fatal("processed actual cancellation did not precede earliest original deadline", completed)
	}
	if os.Getenv("ARDENTS_SOURCE_REFUSAL_CHILD_STOP_FIRST") == "1" && (completed.CloneNotificationsAfterChildStop == 0 || other.CloneNotificationsAfterChildStop == 0) {
		t.Fatal("actual reversed child-stop/clone-notification order was not observed for both roots", completed, other)
	}
	for _, path := range []string{filepath.Join(root, "original-journal"), filepath.Join(root, "original-journal.recovery-completion"), filepath.Join(root, "release-lock")} {
		if err := probeSourceRefusalLease(path, "", 0600, false); err != nil {
			t.Fatal("native owner lease still retained after joined refusal", err)
		}
	}
	// A fresh read owner still cannot adopt or append that partial envelope.
	prefix, err := openSourceRecoveryPrefix(t.Context(), filepath.Join(root, "original-journal"), "initial")
	if err != nil {
		t.Fatal(err)
	}
	defer prefix.close()
	partial, err := os.ReadFile(prefix.journal.path + ".recovery-completion")
	if err != nil || json.Valid(partial) {
		t.Fatal("original partial completion was not retained", err)
	}
	writer, err := openSourceRecoveryBirths(prefix, strings.Repeat("a", 64), strings.Repeat("b", 64), nil)
	defer writer.close()
	if !errors.Is(err, os.ErrExist) {
		t.Fatal("later fresh owner adopted partial completion", err)
	}
	if err := writer.close(); !errors.Is(err, os.ErrExist) && err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(prefix.journal.path + ".recovery-completion")
	if err != nil || !bytes.Equal(after, partial) {
		t.Fatal("later refused owner changed partial completion", err)
	}
	t.Logf("actual sibling roots first=%d second=%d; first cancellation joined, both before-return proofs, partial envelope retained", held.RootPID, other.RootPID)

	// The same record entry with an actually live descendant must still refuse.
	// EOF alone cannot satisfy this negative: require the precise scope guard.
	live := command("native-recovery-hold", "live", root+"-live")
	live.Env = append(live.Env, "ARDENTS_SOURCE_RECOVERY_NATIVE_LIVE_CHILD=1")
	var liveOutput sourceRefusalBusOutput
	live.Stdout, live.Stderr = &liveOutput, &liveOutput
	if err := live.Run(); err == nil || !strings.Contains(liveOutput.String(), "recovery hold has foreign or repeated lifetime") || strings.Contains(liveOutput.String(), "SOURCE_RECOVERY_HOLD_READY ") {
		t.Fatal("actual live descendant escaped recovery hold refusal", err, liveOutput.String())
	}
	t.Logf("actual live-descendant refusal:\n%s", liveOutput.String())
}

func TestSourceRecoveryUnexpectedNoticeNative(t *testing.T) {
	program := os.Getenv("ARDENTS_SOURCE_RECOVERY_CONCURRENT_PROGRAM")
	if program == "" {
		return
	}
	marker, err := os.ReadFile("/etc/ardents-source-stopped-native-stand")
	if err != nil || string(marker) != "ardents-source-stopped-native-v1\n" || os.Geteuid() != 0 || !sourcePath(program) {
		t.Fatal("actual native file/process stand required", err)
	}
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "early-notice")
	t.Cleanup(func() {
		if raw, err := os.ReadFile(root + "-original.log"); err == nil {
			t.Logf("actual early-notice caller output:\n%s", raw)
		}
	})
	command := exec.Command(helper, "-test.run=^TestSourceRefusalObserverNative$", "-test.v", "-test.timeout=60s")
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "ARDENTS_SOURCE_REFUSAL_OBSERVER_MODE=") && !strings.HasPrefix(value, "ARDENTS_SOURCE_CUT_OBSERVER_") {
			command.Env = append(command.Env, value)
		}
	}
	command.Env = append(command.Env, "ARDENTS_SOURCE_REFUSAL_OBSERVER_MODE=native-early-notice", "ARDENTS_SOURCE_CUT_OBSERVER_PROGRAM="+program, "ARDENTS_SOURCE_CUT_OBSERVER_ROOT="+root, "ARDENTS_SOURCE_CUT_OBSERVER_OUTPUT="+root+"-original.log")
	var output sourceRefusalBusOutput
	command.Stdout, command.Stderr = &output, &output
	if err := command.Run(); err == nil || !strings.Contains(output.String(), "processed cancellation notice preceded outside signal") {
		t.Fatal("actual early notice escaped causal rejection", err, output.String())
	}
	t.Logf("actual early-notice helper output:\n%s", output.String())
	var observed sourceCutObservation
	found := 0
	for _, line := range strings.Split(output.String(), "\n") {
		if strings.HasPrefix(line, "SOURCE_REFUSAL_OBSERVATION ") {
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "SOURCE_REFUSAL_OBSERVATION ")), &observed); err != nil {
				t.Fatal(err)
			}
			found++
		}
	}
	if found != 1 || !observed.RefusalAdmissionObserved || !observed.RecoveryHeldBeforeRecord || !observed.RecoveryRootOnlyAtHold || observed.ExternalCancellationAcknowledged || observed.RefusalOwnershipReleasedBeforeExit || observed.RefusalOriginalScopeJoined || observed.SignalledRootThread != 0 || observed.RecoveryCancellationElapsedNS != 0 || observed.Processes < 4 || observed.ThreadsJoined < observed.Processes {
		t.Fatal("early notice claimed a signal, accepted cleanup, or lost native joins", observed)
	}
}
