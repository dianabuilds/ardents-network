//go:build linux && network_signing_native

package infrastructure

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Each child runs the real file/journal owner, then stops its entire process
// at an actual I/O or admission boundary. The parent observes kernel STOP,
// kills and waits for that original child before independently reopening the
// retained inode and prefix. No installed Source, successful semantic neighbor
// or controlled-original-cut proof is constructed. These native counterexamples
// do not establish equivalence between different installed effect windows.
func TestSourceRecoveryPhysicalNativeProcessCuts(t *testing.T) {
	marker, err := os.ReadFile("/etc/ardents-source-stopped-native-stand")
	if err != nil || string(marker) != "ardents-source-stopped-native-v1\n" || os.Geteuid() != 0 {
		t.Fatal("invalid native file stand", err)
	}
	for _, boundary := range []string{"intent-synced-before-readback", "empty-born", "body-before-promotion", "body-synced-before-ready", "ready-synced-before-readback", "ready-readback"} {
		t.Run(boundary, func(t *testing.T) {
			root, err := os.MkdirTemp(os.Getenv("ARDENTS_SOURCE_STOPPED_FUTURE_PARENT"), "physical-cut-")
			if err != nil {
				t.Fatal(err)
			}
			var fs syscall.Statfs_t
			if err := syscall.Statfs(root, &fs); err != nil || fs.Type != 0xef53 {
				t.Fatal("actual ext4 required", err)
			}
			self, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			child := exec.Command(self, "-test.run=^TestSourceRecoveryPhysicalNativeWorker$", "-test.v")
			child.Env = append(os.Environ(), "ARDENTS_SOURCE_PHYSICAL_ROOT="+root, "ARDENTS_SOURCE_PHYSICAL_BOUNDARY="+boundary)
			child.Stdout, child.Stderr = &output, &output
			if err := child.Start(); err != nil {
				t.Fatal(err)
			}
			joined := false
			defer func() {
				if !joined {
					killed := child.Process.Kill()
					waited := child.Wait() // Also joins the original output reader.
					if killed != nil && !errors.Is(killed, os.ErrProcessDone) {
						t.Error("original child interruption", killed)
					}
					if waited == nil {
						t.Error("interrupted child unexpectedly completed")
					}
				}
			}()
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			for {
				var status syscall.WaitStatus
				pid, err := syscall.Wait4(child.Process.Pid, &status, syscall.WNOHANG|syscall.WUNTRACED, nil)
				if errors.Is(err, syscall.EINTR) {
					continue
				}
				if err != nil {
					t.Fatal("original kernel stop observation", err)
				}
				if pid != 0 {
					if !status.Stopped() || status.StopSignal() != syscall.SIGSTOP {
						t.Fatal("worker exited without selected physical stop", status)
					}
					break
				}
				if err := waitSourceTransitionTick(ctx); err != nil {
					t.Fatal("physical boundary unavailable", err)
				}
			}
			journal, path := filepath.Join(root, "original.source-initial-journal"), filepath.Join(root, "payload")
			before, err := os.ReadFile(journal)
			if err != nil {
				t.Fatal(err)
			}
			journalInfo, err := os.Lstat(journal)
			if err != nil {
				t.Fatal(err)
			}
			probe, err := os.Open(journal)
			if err != nil {
				t.Fatal(err)
			}
			lockErr := syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
			if closeErr := probe.Close(); !errors.Is(lockErr, syscall.EWOULDBLOCK) || closeErr != nil {
				t.Fatal("stopped original writer did not retain exclusion", lockErr, closeErr)
			}
			var body []byte
			fileInfo, fileErr := os.Lstat(path)
			if fileErr == nil {
				body, err = os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
			} else if !errors.Is(fileErr, os.ErrNotExist) {
				t.Fatal(fileErr)
			}
			if err := child.Process.Kill(); err != nil {
				t.Fatal(err)
			}
			waitErr := child.Wait()
			joined = true
			status, ok := child.ProcessState.Sys().(syscall.WaitStatus)
			if waitErr == nil || !ok || !status.Signaled() || status.Signal() != syscall.SIGKILL {
				t.Fatal("original physical cut did not join SIGKILL", waitErr, status)
			}
			if !bytes.Contains(output.Bytes(), []byte("SOURCE_FILE_PHYSICAL_STOP "+boundary+"\n")) {
				t.Fatal("kernel stop did not follow selected real boundary", output.String())
			}
			fresh, err := openSourceRecoveryPrefix(t.Context(), journal, "initial")
			if err != nil {
				t.Fatal("fresh post-join prefix custody", err)
			}
			defer fresh.close()
			if !bytes.Equal(fresh.journal.raw, before) || !sameSourceFile(journalInfo, fresh.journal.info) {
				t.Fatal("join or fresh read changed original prefix")
			}
			last := fresh.records[len(fresh.records)-1]
			phase, count, mode, wanted := "file-born", 3, os.FileMode(0600), []byte("native original bytes\n")
			switch boundary {
			case "intent-synced-before-readback":
				phase, count, wanted = "file-intent", 2, nil
			case "empty-born":
				wanted = nil
			case "body-synced-before-ready":
				mode = 0444
			case "ready-synced-before-readback", "ready-readback":
				phase, count, mode = "file-ready", 4, 0444
			}
			if last.Phase != phase || len(fresh.records) != count || !bytes.Equal(body, wanted) {
				t.Fatal("actual cut differs from selected effect window", last.Phase, len(fresh.records), len(body))
			}
			if phase == "file-intent" {
				if !errors.Is(fileErr, os.ErrNotExist) {
					t.Fatal("intent cut performed next exclusive birth")
				}
			} else {
				after, err := os.Lstat(path)
				actual, readErr := os.ReadFile(path)
				if err != nil || readErr != nil || !sameSourceFile(fileInfo, after) || !sourceAccess(after, mode, 0, 0, false) || !bytes.Equal(actual, body) {
					t.Fatal("cut file inode/access/bytes changed after original join", err, readErr)
				}
			}
			// A canonical visible prefix and fresh lease after native join cannot
			// manufacture the missing original controlled-cut provenance.
			missing := fresh.readOriginalCut(strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("c", 64))
			if closed := fresh.close(); !sourcePhysicalRefusalClosed(missing, closed) {
				t.Fatal("uncontrolled physical cut supplied Source provenance or lost refusal", missing)
			}
			for _, suffix := range []string{".original-cut-journal", ".original-cut-result", ".original-cut-pending", ".recovery-completion"} {
				if _, err := os.Lstat(journal + suffix); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("refused physical cut created acceptance evidence", suffix, err)
				}
			}
			after, err := os.ReadFile(journal)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("fresh refusal rewrote original journal", err)
			}
			t.Log("actual original stop/SIGKILL/join; fresh framing/exclusion retained; missing original-cut proof refused", boundary, root)
		})
	}
}

// This checks the concrete prefix Close contract: Close joins the retained
// original refusal with a separate cleanup branch. A nonempty cleanup branch
// must fail the observer even when errors.Is still finds the original refusal.
func sourcePhysicalRefusalClosed(original, closed error) bool {
	joined, ok := closed.(interface{ Unwrap() []error })
	if original == nil || !ok {
		return false
	}
	children := joined.Unwrap()
	return len(children) == 1 && errors.Is(children[0], original)
}

func TestSourcePhysicalRefusalCloseRetainsIndependentFailure(t *testing.T) {
	for _, failClose := range []bool{false, true} {
		t.Run(fmt.Sprint(failClose), func(t *testing.T) {
			births, _ := nativeSourceBirths(t, t.Context())
			if err := births.close(); err != nil {
				t.Fatal(err)
			}
			fresh, err := openSourceRecoveryPrefix(t.Context(), births.journal.path, "initial")
			if err != nil {
				t.Fatal(err)
			}
			defer fresh.close()
			missing := fresh.readOriginalCut(strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("c", 64))
			if !errors.Is(missing, os.ErrNotExist) {
				t.Fatal("actual missing original-cut result required", missing)
			}
			if failClose {
				// Physically close the retained original descriptor, so the real
				// owner's later Close must report its independent close failure.
				if err := fresh.journal.file.Close(); err != nil {
					t.Fatal(err)
				}
			}
			closed := fresh.close()
			if !errors.Is(closed, missing) || sourcePhysicalRefusalClosed(missing, closed) == failClose {
				t.Fatal("observer lost original refusal or masked actual cleanup failure", missing, closed)
			}
			if failClose && !errors.Is(closed, os.ErrClosed) {
				t.Fatal("actual descriptor close failure missing", closed)
			}
		})
	}
}

func TestSourceRecoveryPhysicalNativeWorker(t *testing.T) {
	root, boundary := os.Getenv("ARDENTS_SOURCE_PHYSICAL_ROOT"), os.Getenv("ARDENTS_SOURCE_PHYSICAL_BOUNDARY")
	if root == "" && boundary == "" {
		return
	}
	if !sourcePath(root) || boundary == "" {
		t.Fatal("invalid private file worker")
	}
	journal, path := filepath.Join(root, "original.source-initial-journal"), filepath.Join(root, "payload")
	births, err := openSourceRecoverableInitialBirths(t.Context(), journal, map[string]bool{path: true})
	if err != nil {
		t.Fatal(errors.Join(err, births.close()))
	}
	defer births.close()
	stop := func() error {
		if _, err := fmt.Fprintln(os.Stdout, "SOURCE_FILE_PHYSICAL_STOP "+boundary); err != nil {
			return err
		}
		if err := syscall.Kill(os.Getpid(), syscall.SIGSTOP); err != nil {
			return err
		}
		// A process-directed stop may be delivered to another runtime thread:
		// Kill can return before the whole process has stopped. Never unwind
		// into births.close while the outside owner is retaining this cut.
		// Even an unexpected continuation keeps the original writer until the
		// parent kills and joins it; it cannot run the next effect or cleanup.
		for {
			if err := syscall.Pause(); err != nil && !errors.Is(err, syscall.EINTR) {
				return err
			}
		}
	}
	births.afterSync = func(file *os.File) error {
		if file == births.journal.file {
			if boundary == "intent-synced-before-readback" && bytes.Contains(births.journal.body, []byte(`"phase":"file-intent"`)) || boundary == "ready-synced-before-readback" && bytes.Contains(births.journal.body, []byte(`"phase":"file-ready"`)) {
				return stop()
			}
		} else if file.Name() == path && boundary == "body-synced-before-ready" {
			return stop()
		}
		return nil
	}
	err = births.write(path, []byte("native original bytes\n"), 0444, 0, 0, func() error {
		if boundary == "empty-born" && births.sequence == 3 || boundary == "ready-readback" && births.sequence == 4 {
			return stop()
		}
		if boundary == "body-before-promotion" {
			info, err := os.Lstat(path)
			if err == nil && info.Size() != 0 {
				return stop()
			}
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
		return nil
	})
	t.Fatal("worker completed without original physical cut", err)
}
