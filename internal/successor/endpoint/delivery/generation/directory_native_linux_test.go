//go:build installation_native

package generation

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func nativeContainer(t *testing.T) (string, os.FileInfo) {
	t.Helper()
	parent := os.Getenv("ARDENTS_INSTALLATION_NATIVE_ROOT")
	if os.Geteuid() != 0 || parent == "" || !filepath.IsAbs(parent) {
		t.Fatal("invalid environment: installation_native requires root and a trusted temporary parent")
	}
	for current := parent; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil || info == nil || !info.IsDir() || info.Mode().Perm()&0022 != 0 {
			t.Fatal("invalid trusted temporary parent", err)
		}
		native, ok := info.Sys().(*syscall.Stat_t)
		if !ok || native.Uid != 0 {
			t.Fatal("invalid trusted temporary parent owner")
		}
		if filepath.Dir(current) == current {
			break
		}
	}
	directory, err := os.MkdirTemp(parent, "installation-generation-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(directory); err != nil {
			t.Error(err)
		}
	})
	path := filepath.Join(directory, "generations")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	return path, info
}

func nativeGeneration(t *testing.T) (string, *Owner) {
	t.Helper()
	path, parent := nativeContainer(t)
	owned, err := Create(t.Context(), path, parent, strings.Repeat("a", 64), 65534)
	if err != nil {
		if owned != nil {
			owned.Close()
		}
		t.Fatal(err)
	}
	t.Cleanup(func() { owned.Close() })
	return filepath.Join(path, strings.Repeat("a", 64)), owned
}

func fillGeneration(t *testing.T, owned *Owner) map[string][]byte {
	t.Helper()
	bodies := map[string][]byte{}
	for _, name := range Names() {
		body := []byte("independent mechanism bytes for " + name + "\n")
		bodies[name] = bytes.Clone(body)
		if _, err := owned.CreateFile(t.Context(), name); err != nil {
			t.Fatal(err)
		}
		if err := owned.Write(t.Context(), name, body); err != nil {
			t.Fatal(err)
		}
		body[0] ^= 1
	}
	if err := owned.Seal(t.Context()); err != nil {
		t.Fatal(err)
	}
	return bodies
}

func TestInstallationNativeGenerationOwnerKeepsExactOriginalBytesAccessAndClose(t *testing.T) {
	directory, owned := nativeGeneration(t)
	born := owned.Identity()
	bodies := fillGeneration(t, owned)
	complete := owned.Identity()
	if complete.Device != born.Device || complete.Inode != born.Inode || complete.Mode != os.ModeDir|0750 || complete.GID != 65534 {
		t.Fatal("seal replaced the original directory or its intended access")
	}
	for name, wanted := range bodies {
		filename := filepath.Join(directory, name)
		actual, err := os.ReadFile(filename)
		if err != nil || !bytes.Equal(actual, wanted) {
			t.Fatal("physical bytes differ", name, err)
		}
		info, err := os.Lstat(filename)
		if err != nil {
			t.Fatal(err)
		}
		mode := os.FileMode(0640)
		if name == "ardents-linux-amd64" || name == "ardents-text-linux-amd64" {
			mode = 0555
		}
		native := info.Sys().(*syscall.Stat_t)
		if info.Mode() != mode || native.Uid != 0 || native.Gid != 65534 || native.Nlink != 1 {
			t.Fatal("physical access differs", name)
		}
		returned := owned.Bytes(name)
		if !bytes.Equal(returned, wanted) {
			t.Fatal("detached bytes differ", name)
		}
		returned[0] ^= 1
		if !bytes.Equal(owned.Bytes(name), wanted) {
			t.Fatal("caller changed retained bytes", name)
		}
	}
	// Installation separately owns this exact container promotion. The
	// generation owner must keep the same parent inode across permitted access.
	if err := os.Chown(filepath.Dir(directory), 0, 65534); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Dir(directory), 0750); err != nil {
		t.Fatal(err)
	}
	if err := owned.Observe(); err != nil {
		t.Fatal(err)
	}
	originalRoot := owned.root
	originalFile := owned.file
	originalParent := owned.parent
	if err := owned.Close(); err != nil {
		t.Fatal(err)
	}
	for _, root := range []*os.Root{originalRoot, originalParent} {
		if _, err := root.Stat("."); err == nil {
			t.Fatal("Close retained original directory descriptor")
		}
	}
	if _, err := originalFile.Stat(); err == nil {
		t.Fatal("Close retained original sync descriptor")
	}
	if err := owned.Observe(); err == nil || owned.Bytes("source.json") != nil {
		t.Fatal("closed custody became generation facts")
	}
}

func TestInstallationNativeGenerationOwnerRetainsForeignInventoryFailure(t *testing.T) {
	directory, owned := nativeGeneration(t)
	fillGeneration(t, owned)
	if err := os.WriteFile(filepath.Join(directory, "foreign"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := owned.Observe(); !errors.Is(err, ErrBinding) {
		t.Fatal("foreign entry accepted", err)
	}
	if _, err := owned.root.Stat("."); err != nil {
		t.Fatal("failed observation released original custody", err)
	}
	if err := os.Remove(filepath.Join(directory, "foreign")); err != nil {
		t.Fatal(err)
	}
	if err := owned.Observe(); !errors.Is(err, ErrBinding) {
		t.Fatal("foreign removal renewed failed generation", err)
	}
	if owned.Bytes("source.json") != nil {
		t.Fatal("failed generation returned accepting bytes")
	}
	if err := owned.Close(); !errors.Is(err, ErrBinding) {
		t.Fatal("physical close lost first failure", err)
	}
}

func TestInstallationNativeGenerationOwnerRefusesUnclosedWriteAndAdoption(t *testing.T) {
	directory, owned := nativeGeneration(t)
	if err := owned.Write(t.Context(), "../foreign", []byte("foreign")); !errors.Is(err, ErrBinding) {
		t.Fatal("escaping filename accepted", err)
	}
	if _, err := os.Lstat(filepath.Join(filepath.Dir(directory), "foreign")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("invalid write escaped generation", err)
	}
	if err := owned.Write(t.Context(), "source.json", []byte("later")); !errors.Is(err, ErrBinding) {
		t.Fatal("fresh call renewed failed owner", err)
	}
	if err := owned.Close(); !errors.Is(err, ErrBinding) {
		t.Fatal("close lost invalid write", err)
	}
	parent, err := os.Lstat(filepath.Dir(directory))
	if err != nil {
		t.Fatal(err)
	}
	adopted, err := Create(t.Context(), filepath.Dir(directory), parent, filepath.Base(directory), 65534)
	if adopted != nil || !errors.Is(err, os.ErrExist) {
		if adopted != nil {
			adopted.Close()
		}
		t.Fatal("existing generation was adopted", err)
	}
}

// This controlled original caller loses authority after birth sync. It is a
// filesystem handoff oracle, not a successful Release or manager substitute.
type birthCancellation struct {
	context.Context
	calls int
}

func (ctx *birthCancellation) Err() error {
	ctx.calls++
	if ctx.calls >= 3 {
		return context.Canceled
	}
	return ctx.Context.Err()
}

func TestInstallationNativeGenerationOwnerRetainsCancelledDurableBirth(t *testing.T) {
	path, parent := nativeContainer(t)
	ctx := &birthCancellation{Context: t.Context()}
	owned, err := Create(ctx, path, parent, strings.Repeat("b", 64), 65534)
	if owned == nil || !errors.Is(err, context.Canceled) {
		if owned != nil {
			owned.Close()
		}
		t.Fatal("late cancellation lost partial generation custody", err)
	}
	original := owned.root
	if _, err := original.Stat("."); err != nil {
		t.Fatal("cancelled birth released original root", err)
	}
	if err := owned.Write(t.Context(), "source.json", []byte("fresh")); !errors.Is(err, context.Canceled) {
		t.Fatal("fresh caller renewed cancelled birth", err)
	}
	if err := owned.Close(); !errors.Is(err, context.Canceled) {
		t.Fatal("close lost original cancellation", err)
	}
	if _, err := original.Stat("."); err == nil {
		t.Fatal("close retained cancelled original root")
	}
}

func TestInstallationNativeGenerationFileBirthPrecedesBodyAndRetainsOriginalInode(t *testing.T) {
	directory, owned := nativeGeneration(t)
	birth, err := owned.CreateFile(t.Context(), "source.json")
	if err != nil || birth == nil || birth.Size() != 0 || birth.Mode() != 0600 {
		t.Fatal("empty original birth absent", err)
	}
	file := owned.files["source.json"].file
	if file == nil {
		t.Fatal("birth lost original descriptor")
	}
	actual, err := os.ReadFile(filepath.Join(directory, "source.json"))
	if err != nil || len(actual) != 0 {
		t.Fatal("birth wrote artifact bytes before journal admission", err)
	}
	if err := owned.Write(t.Context(), "source.json", []byte("bound bytes\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatal("completed mutation did not join original writer", err)
	}
	reader := owned.files["source.json"].file
	info, err := reader.Stat()
	if err != nil || !os.SameFile(birth, info) || info.Mode() != 0640 {
		t.Fatal("write replaced birth custody", err)
	}
	if err := owned.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatal("physical close retained original leaf descriptor", err)
	}
	if _, err := reader.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatal("physical close retained original-inode reader", err)
	}
}

func TestInstallationNativeGenerationFileRequiresUnchangedBirth(t *testing.T) {
	for _, mutation := range []string{"missing-birth", "inode", "bytes", "access", "access-reset"} {
		t.Run(mutation, func(t *testing.T) {
			directory, owned := nativeGeneration(t)
			path := filepath.Join(directory, "source.json")
			var original *os.File
			if mutation != "missing-birth" {
				if _, err := owned.CreateFile(t.Context(), "source.json"); err != nil {
					t.Fatal(err)
				}
				original = owned.files["source.json"].file
			}
			switch mutation {
			case "inode":
				if err := os.Rename(path, filepath.Join(directory, "retained-original")); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, nil, 0600); err != nil {
					t.Fatal(err)
				}
			case "bytes":
				if err := os.WriteFile(path, []byte("foreign"), 0600); err != nil {
					t.Fatal(err)
				}
			case "access":
				if err := os.Chmod(path, 0640); err != nil {
					t.Fatal(err)
				}
			case "access-reset":
				// Establish an observable metadata mutation before restoring
				// access. An immediate pair can share the filesystem's ctime
				// tick and would supply no reset evidence to this snapshot test.
				born := owned.files["source.json"].identity.Sys().(*syscall.Stat_t).Ctim
				deadline := time.Now().Add(time.Second)
				for {
					if err := os.Chmod(path, 0640); err != nil {
						t.Fatal(err)
					}
					info, err := os.Lstat(path)
					if err != nil {
						t.Fatal(err)
					}
					if info.Sys().(*syscall.Stat_t).Ctim != born {
						break
					}
					if !time.Now().Before(deadline) {
						t.Fatal("invalid environment: no observable native ctime transition")
					}
					time.Sleep(time.Millisecond)
				}
				if err := os.Chmod(path, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := owned.Write(t.Context(), "source.json", []byte("candidate")); !errors.Is(err, ErrBinding) {
				t.Fatal("unowned mutation accepted", err)
			}
			if mutation == "missing-birth" {
				if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("missing birth write created leaf", err)
				}
			}
			if original != nil {
				if _, err := original.Stat(); err != nil {
					t.Fatal("failure released original custody", err)
				}
			}
			if err := owned.Close(); !errors.Is(err, ErrBinding) {
				t.Fatal("close lost first failure", err)
			}
			if original != nil {
				if _, err := original.Stat(); !errors.Is(err, os.ErrClosed) {
					t.Fatal("failed birth descriptor retained after close", err)
				}
			}
		})
	}
}

func TestInstallationNativeGenerationFileRetainsCancelledDurableBirth(t *testing.T) {
	_, owned := nativeGeneration(t)
	ctx := &birthCancellation{Context: t.Context()}
	birth, err := owned.CreateFile(ctx, "source.json")
	if birth == nil || !errors.Is(err, context.Canceled) {
		t.Fatal("late cancellation lost birth", err)
	}
	file := owned.files["source.json"].file
	if _, err := file.Stat(); err != nil {
		t.Fatal("canceled birth released physical custody", err)
	}
	if err := owned.Write(t.Context(), "source.json", []byte("fresh")); !errors.Is(err, context.Canceled) {
		t.Fatal("fresh caller renewed failed birth", err)
	}
	if err := owned.Close(); !errors.Is(err, context.Canceled) {
		t.Fatal("close lost original cancellation", err)
	}
	if _, err := file.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatal("canceled leaf survived close", err)
	}
}
