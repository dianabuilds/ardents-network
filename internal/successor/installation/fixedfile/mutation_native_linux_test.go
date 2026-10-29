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

// Root0444 is an immutable public-file birth, not permission to replace an
// existing complete read-only file or assign a foreign public-file group.
func TestInstallationNativeFixedFileReadOnlyBirths(t *testing.T) {
	for _, kind := range []string{"create", "continue"} {
		t.Run(kind, func(t *testing.T) {
			directory, parent := mutationDirectory(t)
			filename := filepath.Join(directory, "public-metadata")
			body := []byte("original public metadata\n")
			before := matchingDescriptors(t, parent)
			var owned *Mutation
			var birth os.FileInfo
			var err error
			if kind == "create" {
				owned, err = Create(t.Context(), filename, parent, body, 0444, 0)
			} else {
				f, openErr := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
				if openErr != nil {
					t.Fatal(openErr)
				}
				birth, err = f.Stat()
				if err := errors.Join(err, f.Sync(), f.Close()); err != nil {
					t.Fatal(err)
				}
				owned, err = ContinueBirth(t.Context(), filename, parent, birth, body, 0444, 0)
			}
			if owned != nil {
				defer owned.Close()
			}
			if err != nil {
				t.Fatal("root0444 birth refused", err)
			}
			if birth == nil {
				birth = owned.Identity()
			}
			if owned.Identity().Mode() != 0600 || owned.Identity().Size() != 0 || !os.SameFile(birth, owned.Identity()) {
				t.Fatal("original empty birth lost")
			}
			committed, err := owned.Commit()
			if err != nil || committed == nil || !os.SameFile(birth, committed) || committed.Mode() != 0444 || committed.Sys().(*syscall.Stat_t).Gid != 0 {
				t.Fatal("root0444 same-inode commit", err)
			}
			raw, err := os.ReadFile(filename)
			if err != nil || !bytes.Equal(raw, body) {
				t.Fatal("read-only birth bytes", err)
			}
			if err := owned.Close(); err != nil {
				t.Fatal(err)
			}
			if matchingDescriptors(t, birth) != 0 || matchingDescriptors(t, parent) != before {
				t.Fatal("read-only birth descriptors survived Close")
			}
			replacement, err := Replace(t.Context(), filename, parent, committed, body, []byte("replacement"), 0444, 0)
			if replacement != nil {
				defer replacement.Close()
			}
			if replacement != nil || !errors.Is(err, ErrInput) {
				t.Fatal("birth extension granted read-only replacement", err)
			}
			foreign, err := Create(t.Context(), filepath.Join(directory, "foreign-group"), parent, body, 0444, 65534)
			if foreign != nil {
				defer foreign.Close()
			}
			if foreign != nil || !errors.Is(err, ErrInput) {
				t.Fatal("public read-only foreign group accepted", err)
			}
			if _, err := os.Lstat(filepath.Join(directory, "foreign-group")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("refusal created a foreign-group file", err)
			}
			raw, err = os.ReadFile(filename)
			after, statErr := os.Lstat(filename)
			if err != nil || statErr != nil || !bytes.Equal(raw, body) || !sameFile(committed, after) {
				t.Fatal("refusals changed original read-only file", err, statErr)
			}
		})
	}
}

// Only physical recorded-birth custody is exercised; the fixture supplies no
// original process cut, transaction journal, Release or recovery admission.
func TestInstallationNativeFixedFileContinuesRecordedEmptyBirth(t *testing.T) {
	for _, change := range []string{"complete", "bytes-before", "inode-before", "mode-before", "link-before", "bytes-after", "cancel-after"} {
		t.Run(change, func(t *testing.T) {
			directory, parent := mutationDirectory(t)
			filename := filepath.Join(directory, "recorded-empty")
			file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
			if err != nil {
				t.Fatal(err)
			}
			if err := file.Sync(); err != nil {
				file.Close()
				t.Fatal(err)
			}
			birth, err := file.Stat()
			closeErr := file.Close()
			if err != nil || closeErr != nil {
				t.Fatal(err, closeErr)
			}
			baseline := matchingDescriptors(t, parent)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			switch change {
			case "bytes-before":
				if err := os.WriteFile(filename, []byte("unknown"), 0600); err != nil {
					t.Fatal(err)
				}
			case "inode-before":
				if err := os.Rename(filename, filename+"-original"); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filename, nil, 0600); err != nil {
					t.Fatal(err)
				}
			case "mode-before":
				if err := os.Chmod(filename, 0644); err != nil {
					t.Fatal(err)
				}
			case "link-before":
				if err := os.Link(filename, filename+"-alias"); err != nil {
					t.Fatal(err)
				}
			}
			body := []byte("exact continued fixed unit\n")
			frozen := bytes.Clone(body)
			owned, err := ContinueBirth(ctx, filename, parent, birth, body, 0644, 0)
			if owned != nil {
				defer owned.Close()
			}
			if change == "bytes-before" || change == "inode-before" || change == "mode-before" || change == "link-before" {
				if err == nil {
					t.Fatal("changed original birth adopted")
				}
				if owned != nil {
					_ = owned.Close()
				}
				if matchingDescriptors(t, parent) != baseline {
					t.Fatal("partial parent custody leaked")
				}
				actual, readErr := os.ReadFile(filename)
				if readErr != nil {
					t.Fatal(readErr)
				}
				if change == "bytes-before" && string(actual) != "unknown" || change != "bytes-before" && len(actual) != 0 {
					t.Fatal("unknown birth changed")
				}
				return
			}
			if err != nil || owned == nil || !os.SameFile(birth, owned.Identity()) || matchingDescriptors(t, birth) != 1 {
				t.Fatal("independent recorded birth custody", err)
			}
			body[0] = '!'
			if change == "bytes-after" {
				if err := os.WriteFile(filename, []byte("preserve after journal I/O"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if change == "cancel-after" {
				cancel()
			}
			committed, err := owned.Commit()
			if change != "complete" {
				if err == nil || change == "cancel-after" && !errors.Is(err, context.Canceled) {
					t.Fatal("post-opening change/cancel accepted", err)
				}
				actual, readErr := os.ReadFile(filename)
				if readErr != nil {
					t.Fatal(readErr)
				}
				if change == "bytes-after" && string(actual) != "preserve after journal I/O" || change == "cancel-after" && len(actual) != 0 {
					t.Fatal("refusal overwrote original")
				}
				if matchingDescriptors(t, birth) != 1 {
					t.Fatal("refusal released original before Close")
				}
				if closed := owned.Close(); closed == nil {
					t.Fatal("original failure lost through Close")
				}
			} else {
				if err != nil || committed == nil || !os.SameFile(birth, committed) || committed.Mode() != 0644 {
					t.Fatal("same-inode promotion", err)
				}
				actual, readErr := os.ReadFile(filename)
				if readErr != nil || !bytes.Equal(actual, frozen) {
					t.Fatal("candidate alias/bytes changed", readErr)
				}
				if err := owned.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if matchingDescriptors(t, birth) != 0 || matchingDescriptors(t, parent) != baseline {
				t.Fatal("joined descriptors leaked")
			}
		})
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
			created, err := os.Lstat(filename)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("fixture created with mode %04o; establishing required 0644 preimage", created.Mode().Perm())
			if err := os.Chmod(filename, 0644); err != nil {
				t.Fatal(err)
			}
			original, err := os.Lstat(filename)
			if err != nil || original.Mode() != 0644 {
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
					if err == nil {
						err = os.Chmod(filename, 0644)
					}
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
