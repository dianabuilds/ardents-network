//go:build ignore

package r050

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var (
	recordA = bytes.Repeat([]byte("record-A\n"), 256)
	recordB = bytes.Repeat([]byte("record-B\n"), 256)
)

func TestHostAndSuccessfulReplacement(t *testing.T) {
	root := secureTestRoot(t)
	manifest, err := hostManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(manifest)
	if err := replaceActivation(root, recordA, nil); err != nil {
		t.Fatal(err)
	}
	if err := replaceActivation(root, recordB, nil); err != nil {
		t.Fatal(err)
	}
	assertActivation(t, root, recordB)
}

func TestConcurrentReadersNeverSeePartialRecord(t *testing.T) {
	root := secureTestRoot(t)
	if err := replaceActivation(root, recordA, nil); err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{string(recordA): true, string(recordB): true}
	stop := make(chan struct{})
	errorsSeen := make(chan error, 1)
	var readers sync.WaitGroup
	for reader := 0; reader < 8; reader++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				value, err := activationBytes(root)
				if err != nil || !allowed[string(value)] {
					select {
					case errorsSeen <- fmt.Errorf("read=%d bytes error=%v", len(value), err):
					default:
					}
					return
				}
			}
		}()
	}
	busy := 0
	for iteration := 0; iteration < 100; iteration++ {
		next := recordA
		if iteration%2 == 0 {
			next = recordB
		}
		if err := replaceActivation(root, next, nil); errors.Is(err, errActivationBusy) {
			busy++
			continue
		} else if err != nil {
			close(stop)
			readers.Wait()
			t.Fatal(err)
		}
	}
	close(stop)
	readers.Wait()
	select {
	case err := <-errorsSeen:
		t.Fatal(err)
	default:
	}
	if err := replaceActivation(root, recordB, nil); err != nil {
		t.Fatalf("replacement after readers stopped: %v", err)
	}
	t.Logf("bounded activation-busy outcomes during concurrent opens: %d/100", busy)
}

func TestProcessInterruptionBoundaries(t *testing.T) {
	points := []string{afterCreate, afterWrite, afterFileSync, afterReplace, afterDurability}
	for _, point := range points {
		t.Run(point, func(t *testing.T) {
			root := secureTestRoot(t)
			if err := replaceActivation(root, recordA, nil); err != nil {
				t.Fatal(err)
			}
			command := exec.Command(os.Args[0], "-test.run=^TestInterruptionHelper$")
			command.Env = append(os.Environ(), "R050_HELPER=1", "R050_ROOT="+root, "R050_STOP="+point)
			err := command.Run()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 86 {
				t.Fatalf("helper exit=%v, want 86", err)
			}
			want := recordA
			if point == afterReplace || point == afterDurability {
				want = recordB
			}
			assertActivation(t, root, want)
			temps, err := activationTemps(root)
			if err != nil || len(temps) > 1 {
				t.Fatalf("temporary residue=%v error=%v", temps, err)
			}
			for _, temporary := range temps {
				if err := os.Remove(temporary); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestInterruptionHelper(t *testing.T) {
	if os.Getenv("R050_HELPER") != "1" {
		return
	}
	stop := os.Getenv("R050_STOP")
	err := replaceActivation(os.Getenv("R050_ROOT"), recordB, func(point string) {
		if point == stop {
			os.Exit(86)
		}
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(87)
	}
	os.Exit(0)
}

func TestHeldHandleHasExplicitOutcome(t *testing.T) {
	root := secureTestRoot(t)
	if err := replaceActivation(root, recordA, nil); err != nil {
		t.Fatal(err)
	}
	release, blocks, err := platformHoldActivation(root)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	err = replaceActivation(root, recordB, nil)
	if blocks {
		if !errors.Is(err, errActivationBusy) {
			t.Fatalf("error=%v, want activation-busy", err)
		}
		assertActivation(t, root, recordA)
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	assertActivation(t, root, recordB)
}

func TestLinkedRootIsRejectedBeforeMutation(t *testing.T) {
	parent := t.TempDir()
	realRoot := filepath.Join(parent, "real")
	if err := os.Mkdir(realRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := preparePlatformRoot(realRoot); err != nil {
		t.Fatal(err)
	}
	linkedRoot := filepath.Join(parent, "linked")
	if err := platformCreateLinkedRoot(realRoot, linkedRoot); err != nil {
		t.Fatal(err)
	}
	err := replaceActivation(linkedRoot, recordA, nil)
	if !errors.Is(err, errActivationUnsupported) {
		t.Fatalf("error=%v, want activation-unsupported", err)
	}
	if _, err := os.Stat(filepath.Join(realRoot, activationFile)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("linked target mutated: %v", err)
	}
}

func TestBoundedInputAndPath(t *testing.T) {
	root := secureTestRoot(t)
	if err := replaceActivation(root, make([]byte, maximumActivationLen+1), nil); !errors.Is(err, errActivationUnsupported) {
		t.Fatalf("oversize error=%v", err)
	}
	component := strings.Repeat("x", maximumComponentLen+1)
	if err := replaceActivation(filepath.Join(root, component), recordA, nil); !errors.Is(err, errActivationUnsupported) {
		t.Fatalf("long component error=%v", err)
	}
}

func TestUnsafeRootPermissionsAreRejectedBeforeMutation(t *testing.T) {
	root := secureTestRoot(t)
	if err := makePlatformRootUnsafe(root); err != nil {
		t.Fatal(err)
	}
	if err := replaceActivation(root, recordA, nil); !errors.Is(err, errActivationUnsupported) {
		t.Fatalf("unsafe root error=%v", err)
	}
	if _, err := os.Stat(filepath.Join(root, activationFile)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unsafe root mutated: %v", err)
	}
}

func TestHardlinkedCommittedRecordIsRejectedBeforeMutation(t *testing.T) {
	root := secureTestRoot(t)
	if err := replaceActivation(root, recordA, nil); err != nil {
		t.Fatal(err)
	}
	shadow := filepath.Join(root, "activation-shadow")
	if err := os.Link(filepath.Join(root, activationFile), shadow); err != nil {
		t.Fatal(err)
	}
	if err := replaceActivation(root, recordB, nil); !errors.Is(err, errActivationUnsupported) {
		t.Fatalf("hardlink error=%v", err)
	}
	got, err := os.ReadFile(filepath.Join(root, activationFile))
	if err != nil || !bytes.Equal(got, recordA) {
		t.Fatalf("committed bytes changed after hardlink rejection: bytes=%d error=%v", len(got), err)
	}
}

func secureTestRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := preparePlatformRoot(root); err != nil {
		t.Fatal(err)
	}
	return root
}

func assertActivation(t *testing.T, root string, want []byte) {
	t.Helper()
	got, err := activationBytes(root)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("activation=%d bytes, want %d", len(got), len(want))
	}
}
