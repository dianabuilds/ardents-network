//go:build installation_native

package generation

import (
	"bytes"
	"context"
	"debug/elf"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func snapshotFixture(t *testing.T) (string, map[string][]byte) {
	t.Helper()
	directory, writer := nativeGeneration(t)
	body := fillGeneration(t, writer)
	parent := filepath.Dir(directory)
	if err := os.Chown(parent, 0, 65534); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(parent, 0750); err != nil {
		t.Fatal(err)
	}
	return directory, body
}

func openSnapshotFixture(t *testing.T, directory string, ctx context.Context) *Snapshot {
	t.Helper()
	parent, parentErr := os.Lstat(filepath.Dir(directory))
	info, err := os.Lstat(directory)
	if err != nil || parentErr != nil {
		t.Fatal(err, parentErr)
	}
	s, err := OpenSnapshot(ctx, filepath.Dir(directory), parent, info, filepath.Base(directory), 65534)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestInstallationNativeSnapshotOwnsReadCustodyAndDetachedBytes(t *testing.T) {
	directory, bodies := snapshotFixture(t)
	s := openSnapshotFixture(t, directory, t.Context())
	for name, wanted := range bodies {
		body := s.Bytes(name)
		if !bytes.Equal(body, wanted) {
			t.Fatal("snapshot differs", name)
		}
		body[0] ^= 1
		if !bytes.Equal(s.Bytes(name), wanted) {
			t.Fatal("caller mutated snapshot")
		}
		info, err := os.Lstat(filepath.Join(directory, name))
		if err != nil || !s.Matches(name, info) {
			t.Fatal("original inode comparison differs", err)
		}
	}
	parent, root, file := s.parent, s.root, s.file
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	for _, handle := range []*os.Root{parent, root} {
		if _, err := handle.Stat("."); err == nil {
			t.Fatal("snapshot root survived Close")
		}
	}
	if _, err := file.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatal("directory descriptor survived Close", err)
	}
	if s.Bytes("binding.json") != nil {
		t.Fatal("closed snapshot exposed bytes")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestInstallationNativeSnapshotRefusesForeignInventoryInodeAndBytes(t *testing.T) {
	for _, mutation := range []string{"extra", "missing", "inode", "bytes", "hardlink"} {
		t.Run(mutation, func(t *testing.T) {
			directory, _ := snapshotFixture(t)
			s := openSnapshotFixture(t, directory, t.Context())
			name := filepath.Join(directory, "headless.json")
			var err error
			switch mutation {
			case "extra":
				err = os.WriteFile(filepath.Join(directory, "foreign"), []byte("extra"), 0640)
			case "missing":
				err = os.Remove(name)
			case "inode":
				err = os.Rename(name, filepath.Join(filepath.Dir(directory), "old-headless"))
				if err == nil {
					err = os.WriteFile(name, s.Bytes("headless.json"), 0640)
				}
				if err == nil {
					err = os.Chown(name, 0, 65534)
				}
			case "bytes":
				body := s.Bytes("headless.json")
				body[0] ^= 1
				err = os.WriteFile(name, body, 0640)
				if err == nil {
					original := s.files["headless.json"].identity.ModTime()
					err = os.Chtimes(name, original, original)
				}
			case "hardlink":
				err = os.Link(name, filepath.Join(filepath.Dir(directory), "alias"))
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Observe(); !errors.Is(err, ErrBinding) {
				t.Fatal("foreign snapshot accepted", err)
			}
			if s.Bytes("binding.json") != nil || !errors.Is(s.Close(), ErrBinding) {
				t.Fatal("failure renewed snapshot")
			}
		})
	}
}

func TestInstallationNativeSnapshotRetainsOriginalCancellation(t *testing.T) {
	directory, _ := snapshotFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	s := openSnapshotFixture(t, directory, ctx)
	cancel()
	if !errors.Is(s.Observe(), context.Canceled) || s.Bytes("binding.json") != nil || !errors.Is(s.Close(), context.Canceled) {
		t.Fatal("snapshot renewed original caller")
	}
}

// Actual kernel credential drop in an owned chroot proves read access without
// root or writer.lock. This is not a systemd invocation or startup qualification.
func TestInstallationNativeSnapshotServiceAccount(t *testing.T) {
	if os.Getenv("ARDENTS_SNAPSHOT_READER_CHILD") == "1" {
		if os.Geteuid() != 65534 || os.Getegid() != 65534 {
			t.Fatal("actual service credentials differ")
		}
		directory := filepath.Join("/generations", strings.Repeat("a", 64))
		s := openSnapshotFixture(t, directory, t.Context())
		if len(s.Bytes("binding.json")) == 0 || s.Observe() != nil {
			t.Fatal("service account failed immutable read")
		}
		return
	}
	directory, _ := snapshotFixture(t)
	root := filepath.Dir(filepath.Dir(directory))
	if err := os.Chown(root, 0, 65534); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0750); err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "reader.test"), body, 0555); err != nil {
		t.Fatal(err)
	}
	// The race executable uses the native ELF loader/libc. Copy its declared
	// runtime into this owned chroot; missing runtime is an invalid environment.
	image, err := elf.Open(exe)
	if err != nil {
		t.Fatal(err)
	}
	libraries, libraryErr := image.ImportedLibraries()
	var runtimePaths []string
	for _, program := range image.Progs {
		if program.Type != elf.PT_INTERP {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(program.Open(), 4096))
		if err != nil || len(data) == 4096 {
			_ = image.Close()
			t.Fatal("invalid ELF interpreter", err)
		}
		runtimePaths = append(runtimePaths, strings.TrimRight(string(data), "\x00"))
	}
	if err := errors.Join(libraryErr, image.Close()); err != nil {
		t.Fatal(err)
	}
	for _, library := range libraries {
		if filepath.Base(library) != library {
			t.Fatal("invalid native library name")
		}
		var found string
		for _, parent := range []string{"/lib/x86_64-linux-gnu", "/usr/lib/x86_64-linux-gnu"} {
			path := filepath.Join(parent, library)
			if _, err := os.Stat(path); err == nil {
				found = path
				break
			}
		}
		if found == "" {
			t.Fatal("invalid environment: native ELF library unavailable", library)
		}
		runtimePaths = append(runtimePaths, found)
	}
	for _, source := range runtimePaths {
		if !filepath.IsAbs(source) || filepath.Clean(source) != source || (!strings.HasPrefix(source, "/lib/") && !strings.HasPrefix(source, "/lib64/") && !strings.HasPrefix(source, "/usr/lib/")) {
			t.Fatal("invalid native runtime path")
		}
		data, err := os.ReadFile(source)
		if err != nil {
			t.Fatal("invalid environment: native runtime unavailable", err)
		}
		destination := filepath.Join(root, strings.TrimPrefix(source, "/"))
		if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(destination, data, 0555); err != nil {
			t.Fatal(err)
		}
	}
	command := exec.CommandContext(t.Context(), filepath.Join(root, "reader.test"), "-test.run=^TestInstallationNativeSnapshotServiceAccount$", "-test.count=1")
	// Linux resolves the executable after chroot, so use the inner pathname.
	command.Path = "/reader.test"
	command.SysProcAttr = &syscall.SysProcAttr{Chroot: root, Credential: &syscall.Credential{Uid: 65534, Gid: 65534, Groups: []uint32{65534}}}
	command.Dir = "/"
	command.Env = append(os.Environ(), "ARDENTS_SNAPSHOT_READER_CHILD=1")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("actual service-account snapshot failed: %v\n%s", err, output)
	}
}
