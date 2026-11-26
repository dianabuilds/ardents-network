//go:build linux

package issuance

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestStoreObservedLossCannotReviveAfterPathRestoration(t *testing.T) {
	for _, name := range []string{"issuer.lock", "issuer.keys", "issuer.pin", "root"} {
		t.Run(name, func(t *testing.T) {
			base := t.TempDir()
			root := filepath.Join(base, "keys")
			binding := testBinding(1)
			if err := Initialize(t.Context(), root, binding); err != nil {
				t.Fatal(err)
			}
			s, err := Open(t.Context(), root, binding)
			if err != nil {
				t.Fatal(err)
			}
			copyOwner := s
			original := root
			if name != "root" {
				original = filepath.Join(root, name)
			}
			moved := filepath.Join(base, "held")
			if err := os.Rename(original, moved); err != nil {
				t.Fatal(err)
			}
			first := s.Check()
			if first == nil {
				t.Fatal("actual custody loss accepted")
			}
			if err := os.Rename(moved, original); err != nil {
				t.Fatal(err)
			}
			if err := copyOwner.Check(); err != first {
				t.Fatal("copy forgot original loss", err)
			}
			if _, err := s.Inventory(); err == nil {
				t.Fatal("restored path revived inventory")
			}
			if err := s.Close(); !errors.Is(err, first) {
				t.Fatal("close lost original cause", err)
			}
			fresh, err := Open(t.Context(), root, binding)
			if err != nil {
				t.Fatal("independent verified reopen", err)
			}
			if err := fresh.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Metadata equality cannot replace exact immutable-byte equality. This control
// changes real material while restoring the inode's original size/time, so only
// the retained material digest rejects before any dependent debit/sign effect.
func TestStoreMaterialDigestRefusesChangedBytesWithOriginalMetadata(t *testing.T) {
	root := filepath.Join(t.TempDir(), "keys")
	binding := testBinding(1)
	if err := Initialize(t.Context(), root, binding); err != nil {
		t.Fatal(err)
	}
	s, err := Open(t.Context(), root, binding)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "issuer.keys")
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(original)
	changed := append([]byte(nil), original...)
	defer clear(changed)
	changed[len(changed)-1] ^= 1
	if err := os.WriteFile(path, changed, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, before.ModTime(), before.ModTime()); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil || !sameIdentity(before, after) {
		t.Fatal("control did not preserve observed metadata", err)
	}
	first := s.Check()
	if first == nil {
		t.Fatal("changed private bytes accepted under original metadata")
	}
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, before.ModTime(), before.ModTime()); err != nil {
		t.Fatal(err)
	}
	if err := s.Check(); err != first {
		t.Fatal("restored material revived original owner", err)
	}
	if err := s.Close(); !errors.Is(err, first) {
		t.Fatal("original material loss forgotten", err)
	}
	fresh, err := Open(t.Context(), root, binding)
	if err != nil {
		t.Fatal("independent verified reopen", err)
	}
	if err := fresh.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestStoreDurableIdentityAndCopies(t *testing.T) {
	b := testBinding(1)
	root := filepath.Join(t.TempDir(), "keys")
	if err := Initialize(t.Context(), root, b); err != nil {
		t.Fatal(err)
	}
	s, err := Open(t.Context(), root, b)
	if err != nil {
		t.Fatal(err)
	}
	v, err := s.Inventory()
	if err != nil {
		t.Fatal(err)
	}
	digest := v.Digest
	v.Keys[0].SPKI[0] ^= 1
	again, err := s.Inventory()
	if err != nil || again.Keys[0].SPKI[0] == v.Keys[0].SPKI[0] {
		t.Fatal("mutable inventory", err)
	}
	if _, e := Open(t.Context(), root, b); !errors.Is(e, ErrBusy) {
		t.Fatal(e)
	}
	copied := s
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if e := copied.Close(); e != nil {
				t.Error(e)
			}
		})
	}
	wg.Wait()
	if _, e := s.Inventory(); !errors.Is(e, ErrClosed) {
		t.Fatal(e)
	}
	s, err = Open(t.Context(), root, b)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := s.Inventory()
	if got.Digest != digest {
		t.Fatal("identity changed")
	}
	if e := s.Close(); e != nil {
		t.Fatal(e)
	}
	before, _ := os.ReadFile(filepath.Join(root, "issuer.keys"))
	if e := Initialize(t.Context(), root, b); !errors.Is(e, ErrUnavailable) {
		t.Fatal(e)
	}
	after, _ := os.ReadFile(filepath.Join(root, "issuer.keys"))
	if !bytes.Equal(before, after) {
		t.Fatal("replaced")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	absent := filepath.Join(t.TempDir(), "canceled")
	if e := Initialize(ctx, absent, b); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if _, e := os.Lstat(absent); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("cancel effect")
	}
}

func TestStorageFaultsRefuseUnsafeRecovery(t *testing.T) {
	b := testBinding(1)
	raw, err := generate(t.Context(), b)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(raw)
	v, err := decodeMaterial(raw, b)
	if err != nil {
		t.Fatal(err)
	}
	for _, phase := range []string{"write", "sync", "close", "rename", "directory-sync"} {
		t.Run(phase, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "keys")
			// Target the material write, rather than just lock creation.
			calls := 0
			fault := func(p string) error {
				if p == phase {
					calls++
					if phase == "rename" || phase == "directory-sync" || calls == 2 {
						return errors.New("injected")
					}
				}
				return nil
			}
			if e := initializeStorage(t.Context(), root, b, raw, v.Digest, fault); !errors.Is(e, ErrUncertain) {
				t.Fatal(e)
			}
			if _, e := Open(t.Context(), root, b); e == nil {
				t.Fatal("partial root opened")
			}
			if e := Initialize(t.Context(), root, b); !errors.Is(e, ErrUnavailable) {
				t.Fatal("partial root replaced", e)
			}
		})
	}
	root := filepath.Join(t.TempDir(), "keys")
	if e := initializeStorage(t.Context(), root, b, raw, v.Digest, nil); e != nil {
		t.Fatal(e)
	}
	if _, e := openStorage(t.Context(), root, b, func(string) error { return errors.New("barrier") }); e == nil {
		t.Fatal("unflushed open acknowledged")
	}
	s, e := Open(t.Context(), root, b)
	if e != nil {
		t.Fatal(e)
	}
	_ = s.Close()
}

func TestStorageTampering(t *testing.T) {
	b := testBinding(1)
	raw, err := generate(t.Context(), b)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(raw)
	v, _ := decodeMaterial(raw, b)
	mutations := map[string]func(string){
		"pending":   func(p string) { _ = os.WriteFile(filepath.Join(p, "issuer.pending"), []byte("partial"), 0600) },
		"unknown":   func(p string) { _ = os.WriteFile(filepath.Join(p, "extra"), nil, 0600) },
		"missing":   func(p string) { _ = os.Remove(filepath.Join(p, "issuer.keys")) },
		"mode":      func(p string) { _ = os.Chmod(filepath.Join(p, "issuer.keys"), 0644) },
		"root-mode": func(p string) { _ = os.Chmod(p, 0755) },
		"digest":    func(p string) { _ = os.WriteFile(filepath.Join(p, "issuer.pin"), []byte("wrong"), 0600) },
		"material":  func(p string) { _ = os.WriteFile(filepath.Join(p, "issuer.keys"), []byte("bad"), 0600) },
		"symlink": func(p string) {
			name := filepath.Join(p, "issuer.keys")
			_ = os.Rename(name, name+".saved")
			_ = os.Symlink(name+".saved", name)
		},
		"hardlink": func(p string) { _ = os.Link(filepath.Join(p, "issuer.keys"), filepath.Join(filepath.Dir(p), "linked")) },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "keys")
			if e := initializeStorage(t.Context(), root, b, raw, v.Digest, nil); e != nil {
				t.Fatal(e)
			}
			mutate(root)
			if s, e := Open(t.Context(), root, b); e == nil {
				_ = s.Close()
				t.Fatal("tamper accepted")
			}
		})
	}
	root := filepath.Join(t.TempDir(), "keys")
	if e := initializeStorage(t.Context(), root, b, raw, v.Digest, nil); e != nil {
		t.Fatal(e)
	}
	s, e := Open(t.Context(), root, b)
	if e != nil {
		t.Fatal(e)
	}
	original := filepath.Join(root, "issuer.keys")
	if e := os.Rename(original, original+".saved"); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(original, raw, 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Inventory(); e == nil {
		t.Fatal("substitution undetected")
	}
	if e := s.Close(); e == nil {
		t.Fatal("cleanup failure lost")
	}
	if s.Close() == nil {
		t.Fatal("cleanup identity lost on retry")
	}
	foreign := t.TempDir()
	file := filepath.Join(foreign, ".ardents-closed-issuer-v1")
	_ = os.WriteFile(file, []byte("legacy"), 0600)
	if _, e := Open(t.Context(), foreign, b); e == nil {
		t.Fatal("legacy root opened")
	}
	files, _ := os.ReadDir(foreign)
	if len(files) != 1 {
		t.Fatal("foreign effect")
	}
}

func TestLeaseCrashProcess(t *testing.T) {
	if root := os.Getenv("ARDENTS_ISSUANCE_LEASE_ROOT"); root != "" {
		s, e := Open(context.Background(), root, testBinding(1))
		if e != nil {
			os.Exit(3)
		}
		_ = s
		if _, e = os.Stdout.WriteString("locked\n"); e != nil {
			os.Exit(4)
		}
		for {
			time.Sleep(time.Hour)
			runtime.KeepAlive(s)
		}
	}
	root := filepath.Join(t.TempDir(), "keys")
	b := testBinding(1)
	if e := Initialize(t.Context(), root, b); e != nil {
		t.Fatal(e)
	}
	before, _ := os.ReadFile(filepath.Join(root, "issuer.keys"))
	command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestLeaseCrashProcess$")
	command.Env = append(os.Environ(), "ARDENTS_ISSUANCE_LEASE_ROOT="+root)
	stdout, e := command.StdoutPipe()
	if e != nil {
		t.Fatal(e)
	}
	if e = command.Start(); e != nil {
		t.Fatal(e)
	}
	defer func() { _ = command.Process.Kill(); _ = command.Wait() }()
	ready := make([]byte, 7)
	if _, e := io.ReadFull(stdout, ready); e != nil || string(ready) != "locked\n" {
		t.Fatal("child", e, string(ready))
	}
	if _, e := Open(t.Context(), root, b); !errors.Is(e, ErrBusy) {
		t.Fatal(e)
	}
	_ = command.Process.Kill()
	_ = command.Wait()
	s, e := Open(t.Context(), root, b)
	if e != nil {
		t.Fatal(e)
	}
	_ = s.Close()
	after, _ := os.ReadFile(filepath.Join(root, "issuer.keys"))
	if !bytes.Equal(before, after) {
		t.Fatal("crash replaced keys")
	}
}
