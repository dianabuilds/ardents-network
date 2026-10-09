//go:build installation_native

package request

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// This selected profile exercises actual root ownership and filesystem changes.
// It neither invokes systemd nor grants installed generation authority.
func nativeRequestDirectory(t *testing.T) string {
	t.Helper()
	parent := os.Getenv("ARDENTS_INSTALLATION_NATIVE_ROOT")
	if os.Geteuid() != 0 || parent == "" {
		t.Fatal("invalid environment: installation_native requires root and an explicit trusted temporary parent")
	}
	if _, err := rootDirectoryAncestors(parent); err != nil {
		t.Fatalf("invalid environment: untrusted temporary parent: %v", err)
	}
	directory, err := os.MkdirTemp(parent, "installation-request-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(directory); err != nil {
			t.Error(err)
		}
	})
	return directory
}

func writeNativeRequest(t *testing.T, directory string) string {
	t.Helper()
	filename := filepath.Join(directory, "request.json")
	if err := os.WriteFile(filename, requestFixture(), 0600); err != nil {
		t.Fatal(err)
	}
	return filename
}

func TestInstallationNativeRequestDirectFile(t *testing.T) {
	filename := writeNativeRequest(t, nativeRequestDirectory(t))
	before, err := os.Lstat(filename)
	if err != nil {
		t.Fatal(err)
	}
	request, err := ReadOwned(t.Context(), filename, true)
	if err != nil {
		t.Fatal(err)
	}
	if request.declared == nil || request.custody == nil || request.custody.path != filename ||
		request.custody.digest != sha256.Sum256(requestFixture()) || !os.SameFile(before, request.custody.identity) {
		t.Fatal("direct owned input did not retain its exact private provenance")
	}
}

func TestInstallationNativeRequestRefusesForeignCustody(t *testing.T) {
	for _, name := range []string{"symlink", "hardlink", "file-writable", "file-owner", "file-group", "parent-writable", "parent-symlink", "empty", "oversized"} {
		t.Run(name, func(t *testing.T) {
			directory := nativeRequestDirectory(t)
			filename := writeNativeRequest(t, directory)
			var err error
			switch name {
			case "symlink":
				err = os.Rename(filename, filename+".original")
				if err == nil {
					err = os.Symlink(filename+".original", filename)
				}
			case "hardlink":
				err = os.Link(filename, filename+".alias")
			case "file-writable":
				err = os.Chmod(filename, 0620)
			case "file-owner":
				err = os.Chown(filename, 65534, 0)
			case "file-group":
				err = os.Chown(filename, 0, 65534)
			case "parent-writable":
				err = os.Chmod(directory, 0770)
			case "parent-symlink":
				alias := filepath.Join(directory, "alias")
				err = os.Symlink(directory, alias)
				filename = filepath.Join(alias, "request.json")
			case "empty":
				err = os.WriteFile(filename, nil, 0600)
			case "oversized":
				err = os.WriteFile(filename, make([]byte, (64<<10)+1), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			request, err := ReadOwned(t.Context(), filename, true)
			if !errors.Is(err, ErrNativeUnavailable) || request.declared != nil || request.custody != nil {
				t.Fatalf("foreign custody returned a request: %v", err)
			}
		})
	}
}

type nativeRequestReadMutation struct {
	context.Context
	calls  int
	at     int
	mutate func()
}

func (c *nativeRequestReadMutation) Err() error {
	c.calls++
	if c.calls == c.at {
		c.mutate()
	}
	return c.Context.Err()
}

func TestInstallationNativeRequestRefusesMutationDuringRead(t *testing.T) {
	for _, name := range []string{"replace-same-bytes", "rewrite-same-bytes", "grow", "hardlink", "mode", "parent-replacement"} {
		t.Run(name, func(t *testing.T) {
			directory := nativeRequestDirectory(t)
			parent := filepath.Join(directory, "input")
			if err := os.Mkdir(parent, 0700); err != nil {
				t.Fatal(err)
			}
			filename := writeNativeRequest(t, parent)
			// The third original-context observation is Decode's handoff,
			// after physical read and before native identity reobservation.
			ctx := &nativeRequestReadMutation{Context: t.Context(), at: 3, mutate: func() {
				var err error
				switch name {
				case "replace-same-bytes":
					err = os.Rename(filename, filename+".original")
					if err == nil {
						err = os.WriteFile(filename, requestFixture(), 0600)
					}
				case "rewrite-same-bytes":
					// A same-byte rewrite must change an observable native fact.
					// Timestamp resolution cannot attest every physical write.
					before, statErr := os.Stat(filename)
					if statErr != nil {
						t.Fatal(statErr)
					}
					err = os.WriteFile(filename, requestFixture(), 0600)
					if err == nil {
						err = os.Chtimes(filename, before.ModTime(), before.ModTime().Add(time.Second))
					}
				case "grow":
					err = os.WriteFile(filename, append(requestFixture(), ' '), 0600)
				case "hardlink":
					err = os.Link(filename, filename+".alias")
				case "mode":
					err = os.Chmod(filename, 0640)
				case "parent-replacement":
					err = os.Rename(parent, parent+".original")
					if err == nil {
						err = os.Mkdir(parent, 0700)
					}
					if err == nil {
						err = os.WriteFile(filename, requestFixture(), 0600)
					}
				}
				if err != nil {
					t.Fatal(err)
				}
			}}
			request, err := ReadOwned(ctx, filename, true)
			if !errors.Is(err, ErrNativeUnavailable) || request.declared != nil || request.custody != nil {
				t.Fatalf("changed native input acquired custody: %v", err)
			}
		})
	}
}

func TestInstallationNativeRequestOriginalCancellation(t *testing.T) {
	for _, at := range []int{4, 5} {
		t.Run(string(rune('0'+at)), func(t *testing.T) {
			filename := writeNativeRequest(t, nativeRequestDirectory(t))
			original, cancel := context.WithCancel(t.Context())
			defer cancel()
			ctx := &nativeRequestReadMutation{Context: original, at: at, mutate: cancel}
			request, err := ReadOwned(ctx, filename, true)
			if !errors.Is(err, context.Canceled) || request.declared != nil || request.custody != nil {
				t.Fatalf("cancelled handoff returned native provenance: %v", err)
			}
		})
	}
}

func TestInstallationNativeRequestOriginReobservation(t *testing.T) {
	for _, change := range []string{"unchanged", "declaration", "same-byte-replacement"} {
		t.Run(change, func(t *testing.T) {
			filename := writeNativeRequest(t, nativeRequestDirectory(t))
			document, err := ReadOwned(t.Context(), filename, true)
			if err != nil {
				t.Fatal(err)
			}
			declared, ok := document.Declaration()
			if !ok || document.Origin() == nil {
				t.Fatal("owned read lost provenance")
			}
			if change == "declaration" {
				declared.Source.Sources[0].Family = "different"
			}
			if change == "same-byte-replacement" {
				if err := os.Rename(filename, filename+".original"); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filename, requestFixture(), 0600); err != nil {
					t.Fatal(err)
				}
			}
			err = document.Origin().Observe(t.Context(), declared)
			if change == "unchanged" && err != nil {
				t.Fatal(err)
			}
			if change != "unchanged" && !errors.Is(err, ErrChanged) {
				t.Fatalf("changed origin admitted: %v", err)
			}
		})
	}
}
