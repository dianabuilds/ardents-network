//go:build linux && amd64 && network_signing_native

package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"syscall"
	"testing"
	"time"
)

const sourcePhysicalRootReturnSymbol = "github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure.sourcePhysicalRootNativeReturn"

func verifySourcePhysicalRootFDs(pid int, original *sourcePhysicalImage) error {
	path := filepath.Join("/proc", fmt.Sprint(pid), "fd")
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	entries, read := directory.ReadDir(257)
	if errors.Is(read, io.EOF) {
		read = nil
	}
	if err := errors.Join(read, directory.Close()); err != nil {
		return err
	}
	if len(entries) > 256 {
		return errors.New("native root descriptor inventory exceeds bound")
	}
	for _, entry := range entries {
		info, err := os.Stat(filepath.Join(path, entry.Name()))
		if errors.Is(err, os.ErrNotExist) {
			continue // An unrelated runtime descriptor already closed.
		}
		if err != nil {
			return err
		}
		if os.SameFile(info, original.journalInfo) || original.payloadInfo != nil && os.SameFile(info, original.payloadInfo) || original.pendingInfo != nil && os.SameFile(info, original.pendingInfo) {
			return errors.New("original native writer descriptor still live after Close")
		}
	}
	return nil
}

// This is a real post-Close call in the native-only driver. The observer must
// locate its actual compiled entry, without noinline flags or PC rewriting.
// Holding here keeps the original process alive while a different process
// independently probes the original journal's actual exclusion and bytes.
func sourcePhysicalRootNativeReturn() error {
	file, err := os.Open("/proc/self/exe")
	if err != nil {
		return err
	}
	_, observed := file.Stat()
	return errors.Join(observed, file.Close())
}

// A direct original writer, not a Source installation or semantic owner. No
// accepting account/manager/Release/State or transition proof is constructed.
// The real sourceBirths cancellation/Close must release its own native lease
// before this driver returns, with GC disabled and the original process alive.
func TestSourcePhysicalRootNativeDriver(t *testing.T) {
	if os.Getenv("ARDENTS_SOURCE_TRANSITION_NATIVE_CASE") != "unsafe-root-cut" {
		return
	}
	root := os.Getenv("ARDENTS_SOURCE_TRANSITION_NATIVE_ROOT")
	marker, err := os.ReadFile("/etc/ardents-source-stopped-native-stand")
	if err != nil || string(marker) != "ardents-source-stopped-native-v1\n" || os.Geteuid() != 0 || !sourcePath(root) {
		t.Fatal("private root native file stand required", err)
	}
	previous := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(previous)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	base, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	ctx, stop := signal.NotifyContext(base, syscall.SIGTERM)
	noticed := make(chan error, 1)
	go func() {
		<-ctx.Done()
		_, err := fmt.Fprintln(os.Stdout, "native-original-cancellation-observed")
		noticed <- err
	}()
	joined := false
	joinNotice := func() {
		if !joined {
			joined = true
			if err := <-noticed; err != nil {
				t.Error(err)
			}
		}
	}
	defer func() { cancel(); stop(); joinNotice() }()
	joinSourceRecoveryNativeHelpers(t)
	childMode := os.Getenv("ARDENTS_SOURCE_PHYSICAL_NATIVE_CHILD")
	if childMode != "" {
		if childMode != "live" && childMode != "late" {
			t.Fatal("closed native physical child fixture required")
		}
		childContext, cancelChild := context.WithCancel(ctx)
		ready := make(chan struct{})
		var child *exec.Cmd
		var childError error
		go func() {
			defer close(ready)
			for childMode == "late" {
				if childContext.Err() != nil {
					return
				}
				_, err := os.Stat(filepath.Join(root, "native-child-after-cut"))
				if err == nil {
					break
				}
				if !errors.Is(err, os.ErrNotExist) {
					childError = err
					return
				}
				time.Sleep(time.Millisecond)
			}
			child = exec.Command("/bin/sleep", "60")
			childError = child.Start()
		}()
		defer func() {
			cancelChild()
			<-ready
			if childError != nil {
				t.Error("native physical child birth", childError)
			}
			if child != nil && child.Process != nil {
				killed, waited := child.Process.Kill(), child.Wait()
				if killed != nil || waited == nil {
					t.Error("native physical child interruption/join", killed, waited)
				}
			}
		}()
		if childMode == "live" {
			<-ready
			if childError != nil {
				t.Fatal("native live descendant birth", childError)
			}
		}
	}
	journal, payload := filepath.Join(root, "original.source-initial-journal"), filepath.Join(root, "next-effect")
	births, err := openSourceRecoverableInitialBirths(ctx, journal, map[string]bool{payload: true})
	if err != nil {
		t.Fatal(errors.Join(err, births.close()))
	}
	defer births.close()
	err = births.record("admitted", root, nil, strings.Repeat("c", 64), 0, 999, 999)
	if err == nil {
		err = births.write(payload, []byte("original next effect\n"), 0600, 0, 0, ctx.Err)
	}
	closed := births.close()
	if ctx.Err() != context.Canceled || !errors.Is(closed, context.Canceled) || !sourceJoinedCancellation(closed) || !sourceJoinedCancellation(err) {
		t.Fatal("original file operation/Close lost actual cancellation", ctx.Err(), err, closed)
	}
	joinNotice()
	if _, err := fmt.Fprintln(os.Stdout, "native-root-writer-closed"); err != nil {
		t.Fatal(err)
	}
	if err := sourcePhysicalRootNativeReturn(); err != nil {
		t.Fatal(err)
	}
	if !births.closed {
		t.Fatal("original writer object was not closed before return")
	}
}
