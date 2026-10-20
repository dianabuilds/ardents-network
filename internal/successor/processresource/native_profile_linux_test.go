//go:build linux && node_process_native

package processresource

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

// This selected native test requires the actual sole-process leaf profile.
// Missing placement is an invalid environment, never a successful skip. The
// real kernel, proc, Go runtime and original descriptors supply every success.
func TestOriginalNativeProcessProfile(t *testing.T) {
	testOriginalNativeProcessProfile(t, Profile)
}

func TestOriginalNativeIssuerProcessProfile(t *testing.T) {
	testOriginalNativeProcessProfile(t, IssuerProfile)
}

func TestOriginalNativeDescriptorProcessProfile(t *testing.T) {
	testOriginalNativeProcessProfile(t, DescriptorProfile)
}

func testOriginalNativeProcessProfile(t *testing.T, profile string) {
	t.Helper()
	guard, err := Open(context.Background(), profile)
	if err != nil {
		t.Fatal("selected native process environment unavailable:", err)
	}
	observation, err := guard.Check()
	if err != nil || observation.RSSBytes == 0 || observation.MemoryBytes == 0 || observation.Threads == 0 || observation.FDs == 0 || observation.CPUWindowStarted.IsZero() {
		t.Fatal("genuine native evidence unavailable", err)
	}
	copy := *guard
	prior := runtime.GOMAXPROCS(2)
	_, first := guard.Check()
	runtime.GOMAXPROCS(prior)
	if first == nil {
		t.Fatal("weakened original Go placement accepted")
	}
	_, later := copy.Check()
	if later != first {
		t.Fatal("restored placement erased original failure")
	}
	if err := copy.Close(); err != nil {
		t.Fatal(err)
	}
	if guard.Close() != copy.Close() {
		t.Fatal("copied native owner changed close result")
	}
	if _, err := guard.Check(); err == nil {
		t.Fatal("closed original pins admitted work")
	}
}

func TestOriginalNativeEvidenceLossRetainsActualCloseFailure(t *testing.T) {
	testOriginalNativeEvidenceLoss(t, Profile)
}

func TestOriginalNativeIssuerEvidenceLossRetainsActualCloseFailure(t *testing.T) {
	testOriginalNativeEvidenceLoss(t, IssuerProfile)
}

func TestOriginalNativeDescriptorEvidenceLossRetainsActualCloseFailure(t *testing.T) {
	testOriginalNativeEvidenceLoss(t, DescriptorProfile)
}

func testOriginalNativeEvidenceLoss(t *testing.T, profile string) {
	t.Helper()
	guard, err := Open(context.Background(), profile)
	if err != nil {
		t.Fatal("selected native process environment unavailable:", err)
	}
	if err := guard.original.native.group.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := guard.Check(); err == nil {
		t.Fatal("lost actual cgroup pin supplied admission")
	} else if facts := Failure(err); facts == nil || facts.Cause != "native-evidence" || facts.Observation != nil {
		t.Fatal("lost evidence became measured zeros or lost first diagnostic", facts)
	}
	first := guard.Close()
	if !errors.Is(first, os.ErrClosed) {
		t.Fatal("native double-close failure was suppressed")
	}
	if guard.Close() != first {
		t.Fatal("late Close erased actual native failure")
	}
}

func TestOriginalNativePlacementAndCallerRetire(t *testing.T) {
	for _, mode := range []string{"foreign-process", "caller"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			guard, err := Open(ctx, Profile)
			if err != nil {
				t.Fatal("selected native environment unavailable", err)
			}
			t.Cleanup(func() {
				if err := guard.Close(); err != nil {
					t.Error(err)
				}
			})
			var first error
			if mode == "caller" {
				cancel()
				_, first = guard.Check()
				if first != context.Canceled {
					t.Fatal("original caller cause changed", first)
				}
			} else {
				// This real child deliberately violates sole-process placement.
				// It owns no authority and is physically joined before pin release.
				child := exec.Command("sleep", "2")
				if err := child.Start(); err != nil {
					t.Fatal(err)
				}
				_, first = guard.Check()
				killErr := child.Process.Kill()
				waitErr := child.Wait()
				if killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
					t.Fatal("owned foreign-placement child interruption failed", killErr)
				}
				var exited *exec.ExitError
				if child.ProcessState == nil || waitErr != nil && !errors.As(waitErr, &exited) {
					t.Fatal("owned foreign-placement child join unavailable", waitErr)
				}
				if first == nil || Failure(first) == nil {
					t.Fatal("foreign native process admitted work", first)
				}
			}
			if first == nil {
				t.Fatal("original admission remained live")
			}
			time.Sleep(time.Millisecond)
			if _, later := guard.Check(); later != first {
				t.Fatal("restored placement/caller revived original owner")
			}
		})
	}
}

func TestOriginalNativeManagedMemoryPressureRetires(t *testing.T) {
	testOriginalNativeManagedMemoryPressure(t, Profile)
}

func TestOriginalNativeIssuerManagedMemoryPressureRetires(t *testing.T) {
	testOriginalNativeManagedMemoryPressure(t, IssuerProfile)
}

func TestOriginalNativeDescriptorManagedMemoryPressureRetires(t *testing.T) {
	testOriginalNativeManagedMemoryPressure(t, DescriptorProfile)
}

func testOriginalNativeManagedMemoryPressure(t *testing.T, profile string) {
	t.Helper()
	guard, err := Open(context.Background(), profile)
	if err != nil {
		t.Fatal("selected native process environment unavailable:", err)
	}
	t.Cleanup(func() { _ = guard.Close(); runtime.GC() })
	// Retain real resident Go memory above the selected soft limit, while below
	// the kernel emergency limit. This is actual process pressure, not Measure.
	payload := make([]byte, 513<<20)
	for index := 0; index < len(payload); index += os.Getpagesize() {
		payload[index] = 1
	}
	observation, first := guard.Check()
	runtime.KeepAlive(payload)
	if observation.GoMemoryBytes < 512<<20 || first == nil {
		t.Fatal("actual managed-memory pressure admitted work", observation.GoMemoryBytes, first)
	}
	expectedCause := "resource-bound"
	if observation.HighEvents != 0 || observation.EmergencyEvents != 0 || observation.PIDEvents != 0 {
		expectedCause = "kernel-limit-event"
	}
	if facts := Failure(first); facts == nil || facts.Cause != expectedCause || facts.Observation == nil || facts.Observation.GoMemoryBytes != observation.GoMemoryBytes {
		t.Fatal("original managed-memory first cause lost", facts)
	} else {
		facts.Observation.GoMemoryBytes = 0
		if Failure(first).Observation.GoMemoryBytes != observation.GoMemoryBytes {
			t.Fatal("diagnostic copy changed original evidence")
		}
	}
	payload = nil
	runtime.GC()
	if _, later := guard.Check(); later != first {
		t.Fatal("released pressure revived the original owner")
	}
}

func TestOriginalNativeIntroductionProcessProfile(t *testing.T) {
	testOriginalNativeProcessProfile(t, IntroductionProfile)
}
func TestOriginalNativeIntroductionEvidenceLossRetainsActualCloseFailure(t *testing.T) {
	testOriginalNativeEvidenceLoss(t, IntroductionProfile)
}
func TestOriginalNativeIntroductionManagedMemoryPressureRetires(t *testing.T) {
	testOriginalNativeManagedMemoryPressure(t, IntroductionProfile)
}

func TestOriginalNativeJoinProcessProfile(t *testing.T) {
	testOriginalNativeProcessProfile(t, JoinProfile)
}
func TestOriginalNativeJoinEvidenceLossRetainsActualCloseFailure(t *testing.T) {
	testOriginalNativeEvidenceLoss(t, JoinProfile)
}
func TestOriginalNativeJoinManagedMemoryPressureRetires(t *testing.T) {
	testOriginalNativeManagedMemoryPressure(t, JoinProfile)
}
