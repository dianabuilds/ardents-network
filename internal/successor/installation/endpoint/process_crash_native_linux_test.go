//go:build installation_native

package endpoint

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

type crashWriterObservation struct {
	Root       string
	Generation string
	Fixed      string
}

// This child exercises only actual filesystem mechanisms with the existing
// staging fixture. Its bytes supply no Enrollment, Release, account or manager
// authority. The native profile selects the parent; only it selects this role.
func TestInstallationCrashWriterChild(t *testing.T) {
	phase := os.Getenv("ARDENTS_INSTALLATION_CRASH_PHASE")
	ready := os.Getenv("ARDENTS_INSTALLATION_CRASH_READY")
	if (phase != "generation-staged" && phase != "fixed-written") || ready == "" {
		t.Fatal("invalid environment: crash writer requires its selected parent")
	}
	lease, request, files, selected := nativeStagingFixture(t)
	stage, err := stageInitialGeneration(t.Context(), lease, request, files, selected, 65534)
	if err != nil {
		t.Fatal(err)
	}
	defer stage.close()
	observation := crashWriterObservation{Root: lease.path, Generation: selected.GenerationDigest}
	if phase == "fixed-written" {
		if err := stage.fixedPhase(t.Context(), "0003.json", "installing-fixed-resources"); err != nil {
			t.Fatal(err)
		}
		observation.Fixed = filepath.Join(filepath.Dir(lease.path), "fixed-resource")
		if err := stage.createFixedFile(t.Context(), observation.Fixed, []byte("owned crash-test resource\n"), 0644, 0); err != nil {
			t.Fatal(err)
		}
	}
	body, err := json.Marshal(observation)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ready+".writing", body, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(ready+".writing", ready); err != nil {
		t.Fatal(err)
	}
	// The parent's pipe remains open without data. SIGKILL must bypass both
	// normal staging cleanup and the test fixture's lease cleanup.
	var input [1]byte
	if _, err := os.Stdin.Read(input[:]); err != nil {
		t.Fatal("crash writer lost its parent before the selected kill", err)
	}
	t.Fatal("crash writer unexpectedly resumed")
}

func TestInstallationNativeProcessCrashRetainsOwnedStageAndRefusesInitialAdoption(t *testing.T) {
	for _, phase := range []string{"generation-staged", "fixed-written"} {
		t.Run(phase, func(t *testing.T) {
			parent := nativeRequestDirectory(t)
			ready := filepath.Join(parent, "ready.json")
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			input, hold, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer input.Close()
			defer hold.Close()
			command := exec.CommandContext(ctx, executable, "-test.run=^TestInstallationCrashWriterChild$", "-test.count=1")
			command.Dir = "/"
			command.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "ARDENTS_INSTALLATION_NATIVE_ROOT=" + parent,
				"ARDENTS_INSTALLATION_CRASH_PHASE=" + phase, "ARDENTS_INSTALLATION_CRASH_READY=" + ready}
			command.Stdin = input
			command.WaitDelay = 5 * time.Second
			var output bytes.Buffer
			command.Stdout, command.Stderr = &output, &output
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			wait := make(chan error, 1)
			go func() { wait <- command.Wait() }()
			joined := false
			defer func() {
				if !joined {
					_ = command.Process.Kill()
					<-wait
				}
			}()
			var observation crashWriterObservation
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()
		readyLoop:
			for {
				body, err := os.ReadFile(ready)
				if err == nil {
					if len(body) > 4096 || json.Unmarshal(body, &observation) != nil {
						t.Fatal("invalid crash-writer observation")
					}
					break readyLoop
				}
				if !errors.Is(err, os.ErrNotExist) {
					t.Fatal(err)
				}
				select {
				case err := <-wait:
					joined = true
					t.Fatalf("crash writer ended before readiness: %v\n%s", err, output.String())
				case <-ctx.Done():
					t.Fatal("crash writer did not reach the selected boundary", ctx.Err())
				case <-ticker.C:
				}
			}
			relative, err := filepath.Rel(parent, observation.Root)
			if err != nil || !canonicalPath(observation.Root) || relative == "." || filepath.IsAbs(relative) ||
				relative == ".." || len(relative) < 3 || relative[:3] == "../" || !canonicalDigest(observation.Generation) {
				t.Fatal("child root escaped its owned parent", err)
			}
			writer, err := os.Open(filepath.Join(observation.Root, "writer.lock"))
			if err != nil {
				t.Fatal(err)
			}
			defer writer.Close()
			if err := syscall.Flock(int(writer.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); !errors.Is(err, syscall.EWOULDBLOCK) {
				t.Fatal("original writer lease was not retained before the crash", err)
			}
			journal := filepath.Join(observation.Root, "journals", observation.Generation)
			paths := []string{filepath.Join(observation.Root, "transition.json"), filepath.Join(journal, "generation-directory.json"), filepath.Join(journal, "0002.json")}
			if phase == "fixed-written" {
				if observation.Fixed != filepath.Join(filepath.Dir(observation.Root), "fixed-resource") {
					t.Fatal("fixed resource escaped its owned parent")
				}
				paths = append(paths, observation.Fixed, filepath.Join(journal, "creations", digestHex([]byte(observation.Fixed))+".json"))
			} else if observation.Fixed != "" {
				t.Fatal("generation-only phase performed a fixed write")
			}
			before := make(map[string][]byte)
			identities := make(map[string]os.FileInfo)
			for _, path := range paths {
				before[path], err = os.ReadFile(path)
				if err != nil || len(before[path]) == 0 {
					t.Fatal("owned record or resource absent before crash", path, err)
				}
				identities[path], err = os.Lstat(path)
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := command.Process.Kill(); err != nil {
				t.Fatal(err)
			}
			err = <-wait
			joined = true
			var exited *exec.ExitError
			if !errors.As(err, &exited) || command.ProcessState == nil {
				t.Fatal("original crash writer did not retain a failing terminal outcome", err)
			}
			status, ok := command.ProcessState.Sys().(syscall.WaitStatus)
			if !ok || !status.Signaled() || status.Signal() != syscall.SIGKILL || ctx.Err() != nil {
				t.Fatal("selected crash was replaced by another terminal outcome", err, ctx.Err())
			}
			if err := syscall.Flock(int(writer.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
				t.Fatal("joined crash writer retained the kernel lease", err)
			}
			if err := syscall.Flock(int(writer.Fd()), syscall.LOCK_UN); err != nil {
				t.Fatal(err)
			}
			// The parent no longer holds a lock: a refusal cannot be credited to
			// contention instead of the initial owner's no-adoption boundary.
			if adopted, err := createInitialLease(t.Context(), observation.Root); adopted != nil || !errors.Is(err, os.ErrExist) {
				if adopted != nil {
					_ = adopted.close()
				}
				t.Fatal("another initial operation adopted crash residue", err)
			}
			for _, path := range paths {
				body, readErr := os.ReadFile(path)
				info, statErr := os.Lstat(path)
				if readErr != nil || statErr != nil || !bytes.Equal(before[path], body) || !os.SameFile(identities[path], info) || info.Mode() != identities[path].Mode() {
					t.Fatal("crash or refused adoption changed original bytes/inode/access", path, readErr, statErr)
				}
			}
			if _, err := os.Lstat(filepath.Join(observation.Root, "selection.json")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("staged crash residue became selected", err)
			}
		})
	}
}
