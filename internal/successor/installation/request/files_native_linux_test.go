//go:build linux && network_signing_native

package request

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
)

func prepareDeliveryFiles(t *testing.T) (Files, string) {
	t.Helper()
	marker, err := os.ReadFile("/etc/ardents-join-preparation-native-stand")
	if err != nil || string(marker) != "ardents-join-preparation-native-v1\n" || os.Geteuid() != 0 {
		t.Fatal("invalid environment: original root JOIN preparation stand required", err)
	}
	directory := t.TempDir()
	if err := os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}
	var filesystem syscall.Statfs_t
	if err := syscall.Statfs(directory, &filesystem); err != nil || filesystem.Type != 0xef53 {
		t.Fatal("invalid environment: actual protected ext4 input required", err)
	}
	path := filepath.Join(directory, "private")
	if err := os.WriteFile(path, []byte("original private input\n"), 0600); err != nil {
		t.Fatal(err)
	}
	files, err := OpenFiles(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = files.Close() })
	raw, err := files.Read(path, 128, true)
	if err != nil || string(raw) != "original private input\n" {
		t.Fatal("original protected read", err)
	}
	clear(raw)
	return files, path
}

func TestDeliveryFilesNativeDetachedBytesAndOriginalInode(t *testing.T) {
	files, path := prepareDeliveryFiles(t)
	raw, err := files.Read(path, 128, true)
	if err != nil {
		t.Fatal(err)
	}
	raw[0] ^= 1
	clear(raw)
	if err := files.Check(); err != nil {
		t.Fatal("detached copy changed retained original bytes", err)
	}
	if err := os.Rename(path, path+".original"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("original private input\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := files.Check(); !errors.Is(err, ErrChanged) {
		t.Fatal("same bytes on a foreign inode accepted", err)
	}
	if err := files.Close(); !errors.Is(err, ErrChanged) {
		t.Fatal("original identity refusal lost at join", err)
	}
}

func TestDeliveryFilesNativeCloseJoinsHeldReadAndRetainsLateFailure(t *testing.T) {
	for _, closeFault := range []bool{false, true} {
		t.Run(map[bool]string{false: "joined-cancel", true: "joined-cancel-and-close"}[closeFault], func(t *testing.T) {
			files, _ := prepareDeliveryFiles(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			files.original.ctx = ctx
			entered, release, denied := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var releaseOnce, readOnce sync.Once
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			defer unblock()
			files.original.observe = func(stage string, _ *os.File) {
				if stage == "read" {
					readOnce.Do(func() { close(entered); <-release })
				}
				if stage == "denied" {
					close(denied)
				}
			}
			checked, closed := make(chan error, 1), make(chan error, 1)
			go func() { checked <- files.Check() }()
			<-entered
			cancel()
			copy := files
			go func() { closed <- copy.Close() }()
			<-denied
			select {
			case <-closed:
				t.Fatal("Close returned while the original reader remained held")
			default:
			}
			original := files.original.files[0]
			if bytes.Equal(original.raw, make([]byte, len(original.raw))) {
				t.Fatal("original secret erased before reader join")
			}
			if err := files.Check(); !errors.Is(err, context.Canceled) {
				t.Fatal("denial blocked behind input I/O or lost original caller", err)
			}
			if closeFault {
				if err := original.file.Close(); err != nil {
					t.Fatal(err)
				}
			}
			unblock()
			if err := <-checked; !errors.Is(err, context.Canceled) {
				t.Fatal("held read lost cancellation", err)
			}
			result := <-closed
			if !errors.Is(result, context.Canceled) || errors.Is(result, os.ErrClosed) != closeFault {
				t.Fatal("original cancellation or independent physical Close failure lost", result)
			}
			if files.Close() != result || !bytes.Equal(original.raw, make([]byte, len(original.raw))) {
				t.Fatal("copied join changed result or retained private bytes")
			}
		})
	}
}
