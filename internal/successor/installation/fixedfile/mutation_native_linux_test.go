//go:build installation_native

package fixedfile

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// Physical file fixtures grant no journal, Release, manager or startup authority.
func mutationDirectory(t *testing.T) (string, os.FileInfo) {
	t.Helper()
	parent := os.Getenv("ARDENTS_INSTALLATION_NATIVE_ROOT")
	if os.Geteuid() != 0 || parent == "" || !filepath.IsAbs(parent) {
		t.Fatal("invalid native environment")
	}
	for path := parent; ; path = filepath.Dir(path) {
		info, err := os.Lstat(path)
		if err != nil || !info.IsDir() || info.Mode().Perm()&0022 != 0 {
			t.Fatal("untrusted native parent", err)
		}
		native, ok := info.Sys().(*syscall.Stat_t)
		if !ok || native.Uid != 0 || native.Gid != 0 {
			t.Fatal("untrusted native parent owner")
		}
		if path == "/" {
			break
		}
	}
	directory, err := os.MkdirTemp(parent, "ardents-fixedfile-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(directory); err != nil {
			t.Error(err)
		}
	})
	info, err := os.Lstat(directory)
	if err != nil {
		t.Fatal(err)
	}
	return directory, info
}

func matchingDescriptors(t *testing.T, identity os.FileInfo) int {
	t.Helper()
	names, err := filepath.Glob("/proc/self/fd/*")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, name := range names {
		info, err := os.Stat(name)
		if err == nil && os.SameFile(identity, info) {
			count++
		}
	}
	return count
}

func TestInstallationNativeFixedFileCreationPinsBirthAndClosesDescriptors(t *testing.T) {
	directory, parent := mutationDirectory(t)
	filename := filepath.Join(directory, "resource")
	body := []byte("original immutable resource\n")
	frozen := bytes.Clone(body)
	before := matchingDescriptors(t, parent)
	owned, err := Create(t.Context(), filename, parent, body, 0640, 65534)
	if owned != nil {
		defer owned.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	birth := owned.Identity()
	if birth == nil || birth.Size() != 0 || birth.Mode() != 0600 || matchingDescriptors(t, birth) != 1 || matchingDescriptors(t, parent) <= before {
		t.Fatal("original birth custody absent")
	}
	body[0] = '!'
	info, err := owned.Commit()
	if err != nil || info == nil || !os.SameFile(birth, info) || info.Mode() != 0640 || info.Sys().(*syscall.Stat_t).Gid != 65534 {
		t.Fatal("same-inode commit refused", err)
	}
	actual, err := os.ReadFile(filename)
	if err != nil || !bytes.Equal(actual, frozen) {
		t.Fatal("input alias changed physical bytes", err)
	}
	if err := owned.Close(); err != nil {
		t.Fatal(err)
	}
	if matchingDescriptors(t, birth) != 0 || matchingDescriptors(t, parent) != before {
		t.Fatal("original physical descriptors survived Close")
	}
	if _, err := owned.Commit(); !errors.Is(err, ErrBinding) {
		t.Fatal("closed owner renewed", err)
	}
}

func TestInstallationNativeFixedFileCreationRefusesChangeAfterBirth(t *testing.T) {
	for _, change := range []string{"bytes", "inode", "mode", "link", "parent"} {
		t.Run(change, func(t *testing.T) {
			directory, parent := mutationDirectory(t)
			filename := filepath.Join(directory, "resource")
			owned, err := Create(t.Context(), filename, parent, []byte("candidate"), 0644, 0)
			if owned != nil {
				defer owned.Close()
			}
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "bytes":
				err = os.WriteFile(filename, []byte("foreign"), 0600)
			case "inode":
				err = os.Rename(filename, filename+"-original")
				if err == nil {
					err = os.WriteFile(filename, nil, 0600)
				}
			case "mode":
				err = os.Chmod(filename, 0644)
			case "link":
				err = os.Link(filename, filename+"-link")
			case "parent":
				err = os.Rename(directory, directory+"-original")
				if err == nil {
					t.Cleanup(func() { os.RemoveAll(directory + "-original") })
					err = os.Mkdir(directory, 0700)
				}
				if err == nil {
					err = os.WriteFile(filename, []byte("foreign"), 0600)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(filename)
			if err != nil {
				t.Fatal(err)
			}
			if info, err := owned.Commit(); info != nil || !errors.Is(err, ErrBinding) {
				t.Fatal("changed original accepted", err)
			}
			after, err := os.ReadFile(filename)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("refusal changed foreign bytes", err)
			}
			if err := owned.Close(); !errors.Is(err, ErrBinding) {
				t.Fatal("first physical failure lost", err)
			}
		})
	}
}

func TestInstallationNativeFixedFileCancellationRetainsEmptyOriginal(t *testing.T) {
	var unopened Mutation
	if err := unopened.Close(); !errors.Is(err, ErrBinding) {
		t.Fatal("unopened mutation accepted", err)
	}
	directory, parent := mutationDirectory(t)
	filename := filepath.Join(directory, "resource")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	owned, err := Create(ctx, filename, parent, []byte("candidate"), 0644, 0)
	if owned != nil {
		defer owned.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	birth := owned.Identity()
	cancel()
	if info, err := owned.Commit(); info != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("original caller renewed", err)
	}
	current, err := os.Lstat(filename)
	if err != nil || !os.SameFile(birth, current) || current.Size() != 0 || current.Mode() != 0600 {
		t.Fatal("cancelled mutation changed original", err)
	}
	if err := owned.Close(); !errors.Is(err, context.Canceled) {
		t.Fatal("original cancellation lost", err)
	}
	if matchingDescriptors(t, birth) != 0 {
		t.Fatal("cancelled original descriptor survived Close")
	}
}

func TestInstallationNativeFixedFileReplacementRechecksAfterJournalIO(t *testing.T) {
	for _, change := range []string{"none", "same-size-write", "inode", "link", "cancel"} {
		t.Run(change, func(t *testing.T) {
			directory, parent := mutationDirectory(t)
			filename := filepath.Join(directory, "resource")
			previous, candidate := []byte("old-complete"), []byte("new-complete")
			if err := os.WriteFile(filename, previous, 0644); err != nil {
				t.Fatal(err)
			}
			original, err := os.Lstat(filename)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			owned, err := Replace(ctx, filename, parent, original, previous, candidate, 0644, 0)
			if owned != nil {
				defer owned.Close()
			}
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "same-size-write":
				err = os.WriteFile(filename, []byte("bad-complete"), 0644)
			case "inode":
				err = os.Rename(filename, filename+"-original")
				if err == nil {
					err = os.WriteFile(filename, previous, 0644)
				}
			case "link":
				err = os.Link(filename, filename+"-link")
			case "cancel":
				cancel()
			}
			if err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(filename)
			if err != nil {
				t.Fatal(err)
			}
			info, commitErr := owned.Commit()
			if change == "none" {
				if commitErr != nil || info == nil || !os.SameFile(original, info) {
					t.Fatal("original replacement refused", commitErr)
				}
				actual, err := os.ReadFile(filename)
				if err != nil || !bytes.Equal(actual, candidate) {
					t.Fatal("replacement bytes differ", err)
				}
			} else {
				wanted := ErrBinding
				if change == "cancel" {
					wanted = context.Canceled
				}
				if info != nil || !errors.Is(commitErr, wanted) {
					t.Fatal("changed original accepted", commitErr)
				}
				after, err := os.ReadFile(filename)
				if err != nil || !bytes.Equal(before, after) {
					t.Fatal("refusal changed bytes", err)
				}
			}
		})
	}
}
