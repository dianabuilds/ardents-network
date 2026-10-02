//go:build linux

package nodeidentity

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"
)

func importedIdentity(t *testing.T) (string, string, Binding) {
	t.Helper()
	b, pem, _ := identityFixture(t)
	base := t.TempDir()
	if e := os.Chmod(base, 0700); e != nil {
		t.Fatal(e)
	}
	source := filepath.Join(base, "source.pem")
	if e := os.WriteFile(source, pem, 0600); e != nil {
		t.Fatal(e)
	}
	root := filepath.Join(base, "identity")
	if e := Import(t.Context(), root, source, b); e != nil {
		t.Fatal(e)
	}
	return root, source, b
}
func TestIdentityLeaseCopyAndPinnedReopen(t *testing.T) {
	root, source, b := importedIdentity(t)
	before, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	s, e := Open(t.Context(), root, b)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = Open(t.Context(), root, b); !errors.Is(e, ErrBusy) {
		t.Fatal(e)
	}
	copyHandle := s
	if got, e := s.Public(); e != nil || got != b {
		t.Fatal(e)
	}
	if e = s.Close(); e != nil {
		t.Fatal(e)
	}
	if e = copyHandle.Close(); e != nil {
		t.Fatal(e)
	}
	if _, e = copyHandle.Public(); !errors.Is(e, ErrClosed) {
		t.Fatal(e)
	}
	s, e = Open(t.Context(), root, b)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Close(); e != nil {
		t.Fatal(e)
	}
	bad := b
	bad.Signer[0] ^= 1
	if _, e = Open(t.Context(), root, bad); e == nil {
		t.Fatal("wrong pin")
	}
	after, e := os.ReadFile(source)
	if e != nil || !bytes.Equal(before, after) {
		t.Fatal("source modified")
	}
}
func TestIdentityImportSourceAndRootFailures(t *testing.T) {
	for _, name := range []string{"source-mode", "directory-mode", "symlink", "hardlink", "pending", "extra", "key-mode", "missing", "substitution"} {
		t.Run(name, func(t *testing.T) {
			root, source, b := importedIdentity(t)
			switch name {
			case "source-mode":
				_ = os.Chmod(source, 0644)
			case "directory-mode":
				_ = os.Chmod(filepath.Dir(source), 0755)
			case "symlink":
				_ = os.Rename(source, source+".real")
				if e := os.Symlink(source+".real", source); e != nil {
					t.Fatal(e)
				}
			case "hardlink":
				if e := os.Link(source, source+".link"); e != nil {
					t.Fatal(e)
				}
			case "pending", "extra":
				_ = os.WriteFile(filepath.Join(root, name), nil, 0600)
			case "key-mode":
				_ = os.Chmod(filepath.Join(root, "identity.key"), 0644)
			case "missing":
				_ = os.Remove(filepath.Join(root, "identity.key"))
			case "substitution":
				_ = os.Rename(filepath.Join(root, "identity.key"), filepath.Join(root, "old"))
				_ = os.WriteFile(filepath.Join(root, "identity.key"), []byte("replacement"), 0600)
			}
			if name == "source-mode" || name == "directory-mode" || name == "symlink" || name == "hardlink" {
				newRoot := filepath.Join(filepath.Dir(root), "new")
				if e := Import(t.Context(), newRoot, source, b); e == nil {
					t.Fatal("unsafe source accepted")
				}
				if _, e := os.Lstat(newRoot); !errors.Is(e, os.ErrNotExist) {
					t.Fatal("source failure created root")
				}
			} else {
				if _, e := Open(t.Context(), root, b); e == nil {
					t.Fatal("bad root accepted")
				}
			}
		})
	}
}
func TestIdentityFaultsAndCancellation(t *testing.T) {
	b, _, der := identityFixture(t)
	for _, phase := range []string{"write", "sync", "close", "rename", "directory-sync"} {
		t.Run(phase, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "identity")
			hit := false
			e := initialize(t.Context(), root, b, der, func(p string) error {
				if p == phase {
					hit = true
					return errors.New("fault")
				}
				return nil
			})
			if !hit || !errors.Is(e, ErrUncertain) {
				t.Fatal(e)
			}
			if _, e = Open(t.Context(), root, b); e == nil {
				t.Fatal("partial reopened")
			}
			if e = initialize(t.Context(), root, b, der, nil); !errors.Is(e, ErrUnavailable) {
				t.Fatal("replaced partial", e)
			}
		})
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	root := filepath.Join(t.TempDir(), "identity")
	if e := Import(ctx, root, "/missing", b); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if _, e := os.Lstat(root); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("canceled effects")
	}
	root, _, b = importedIdentity(t)
	if _, e := openStore(t.Context(), root, b, func(p string) error {
		if p == "open-sync" {
			return errors.New("fault")
		}
		return nil
	}); e == nil {
		t.Fatal("failed barrier accepted")
	}
}
func TestIdentityKilledProcessRetainsKey(t *testing.T) {
	if root := os.Getenv("ARDENTS_IDENTITY_CHILD"); root != "" {
		b, _, _ := identityFixture(t)
		s, e := Open(context.Background(), root, b)
		if e != nil {
			os.Exit(2)
		}
		if e = os.WriteFile(root+".ready", []byte("ready"), 0600); e != nil {
			os.Exit(3)
		}
		for {
			time.Sleep(time.Second)
			runtime.KeepAlive(s)
		}
	}
	root, _, b := importedIdentity(t)
	before, e := os.ReadFile(filepath.Join(root, "identity.key"))
	if e != nil {
		t.Fatal(e)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestIdentityKilledProcessRetainsKey$")
	cmd.Env = append(os.Environ(), "ARDENTS_IDENTITY_CHILD="+root)
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	joined := false
	join := func() {
		if joined {
			return
		}
		joined = true
		killErr := cmd.Process.Kill()
		if killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
			t.Errorf("kill child: %v", killErr)
		}
		waitErr := cmd.Wait()
		if cmd.ProcessState == nil {
			t.Errorf("child not joined: %v", waitErr)
			return
		}
		status, ok := cmd.ProcessState.Sys().(syscall.WaitStatus)
		if !ok || !status.Signaled() || status.Signal() != syscall.SIGKILL {
			t.Errorf("unexpected child completion: %v", waitErr)
		}
	}
	t.Cleanup(join)
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, e = os.Stat(root + ".ready"); e == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("child not ready")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, e = Open(t.Context(), root, b); !errors.Is(e, ErrBusy) {
		t.Fatal(e)
	}
	join()
	s, e := Open(t.Context(), root, b)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	after, e := os.ReadFile(filepath.Join(root, "identity.key"))
	if e != nil || !bytes.Equal(before, after) {
		t.Fatal("crash replaced key")
	}
}
