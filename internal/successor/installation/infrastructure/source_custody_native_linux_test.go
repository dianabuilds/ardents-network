//go:build linux && network_signing_native

package infrastructure

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

func nativeSourceInputs(t *testing.T, ctx context.Context) (SourceInputs, SourceRequest) {
	t.Helper()
	base := t.TempDir()
	r := SourceRequest{Schema: "ardents-source-preflight-v1", Invocation: "source", Profile: "ardents-source-process-v1", InstallationRoot: filepath.Join(base, "future-installation"), StateRoot: filepath.Join(base, "future-state"), ParticipationRoot: filepath.Join(base, "future-participation"), CredentialDirectory: filepath.Join(base, "future-credentials"), Plan: filepath.Join(base, "plan"), Certificate: filepath.Join(base, "certificate"), PrivateKey: filepath.Join(base, "key"), ClientCA: filepath.Join(base, "ca"), Clock: filepath.Join(base, "clock"), Address: "127.0.0.1:4567"}
	// Custody-only controls retain actual files, not a successful TLS/runtime
	// claim. The compiled consumer fixture separately uses genuine TLS material.
	for _, p := range []string{r.Plan, r.Certificate, r.PrivateKey, r.ClientCA} {
		if err := os.WriteFile(p, []byte("original bounded custody fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	plan := map[string]any{"network": map[string]any{"root": r.StateRoot, "network_id": [32]byte{1}, "authorities": [][32]byte{{2}}, "threshold": 1, "profile_authority": [32]byte{3}, "clock_observation_file": r.Clock}, "process_profile": r.Profile, "participation_root": r.ParticipationRoot, "address": r.Address, "certificate": filepath.Join(r.CredentialDirectory, "certificate.pem"), "private_key": filepath.Join(r.CredentialDirectory, "private-key.pem"), "client_root": filepath.Join(r.CredentialDirectory, "client-ca.pem"), "client_pins": [][32]byte{{4}}}
	planRaw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(r.Plan, planRaw, 0600); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	request := filepath.Join(base, "request")
	if err = os.WriteFile(request, append(raw, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := ReadSourceInputs(ctx, request)
	if err != nil {
		t.Fatal("invalid native custody environment", errors.Join(err, s.Close()))
	}
	return s, r
}

func TestSourcePreflightNativeHeldOriginalInputCancellationAndClose(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	s, r := nativeSourceInputs(t, ctx)
	copy := s
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	var original *os.File
	s.original.observe = func(f *os.File) {
		if f.Name() == r.PrivateKey {
			once.Do(func() { original = f; close(entered); <-release })
		}
	}
	checked := make(chan error, 1)
	go func() { checked <- s.Check() }()
	<-entered
	cancel()
	if err := copy.Check(); !errors.Is(err, context.Canceled) {
		t.Fatal("original cancellation did not synchronously deny new use", err)
	}
	// Close the real original native fd, independently of logical cancellation.
	if err := original.Close(); err != nil {
		t.Fatal(err)
	}
	closed := make(chan error, 1)
	go func() { closed <- copy.Close() }()
	for {
		s.original.mu.Lock()
		denied := s.original.closed
		s.original.mu.Unlock()
		if denied {
			break
		}
		runtime.Gosched()
	}
	select {
	case <-closed:
		t.Fatal("input Close released before original physical reader joined")
	default:
	}
	select {
	case <-checked:
		t.Fatal("held original input returned before callback join")
	default:
	}
	close(release)
	if err := <-checked; !errors.Is(err, os.ErrClosed) {
		t.Fatal("actual original read/Stat failure lost", err)
	}
	err := <-closed
	if !errors.Is(err, context.Canceled) || !errors.Is(err, os.ErrClosed) {
		t.Fatal("first denial or independent native Close cause lost", err)
	}
	if s.Check() == nil || copy.Check() == nil {
		t.Fatal("late/copied input revived custody")
	}
	if err := s.Close(); !errors.Is(err, os.ErrClosed) {
		t.Fatal("repeat Close lost retained failure", err)
	}
}

func TestSourcePreflightNativeInputMutationAndAncestorAccess(t *testing.T) {
	for _, target := range []string{"bytes", "file-access", "ancestor-access", "symlink"} {
		t.Run(target, func(t *testing.T) {
			s, r := nativeSourceInputs(t, t.Context())
			defer s.Close()
			switch target {
			case "bytes":
				if err := os.WriteFile(r.Certificate, []byte("substitute original bytes"), 0600); err != nil {
					t.Fatal(err)
				}
			case "file-access":
				if err := os.Chmod(r.PrivateKey, 0644); err != nil {
					t.Fatal(err)
				}
			case "ancestor-access":
				if err := os.Chmod(filepath.Dir(r.PrivateKey), 0755); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Rename(r.ClientCA, r.ClientCA+"-original"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(r.ClientCA+"-original", r.ClientCA); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.Check(); err == nil {
				t.Fatal("mutated original input accepted")
			}
			if s.Check() == nil {
				t.Fatal("denied original input revived")
			}
		})
	}
}

func TestSourcePreflightNativeHeldLastInputReobservesOriginalPaths(t *testing.T) {
	for _, fault := range []string{"ancestor-access", "ancestor-inode", "named-file"} {
		t.Run(fault, func(t *testing.T) {
			s, r := nativeSourceInputs(t, t.Context())
			defer s.Close()
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			s.original.observe = func(f *os.File) {
				if f.Name() == r.ClientCA {
					once.Do(func() { close(entered); <-release })
				}
			}
			checked := make(chan error, 1)
			go func() { checked <- s.Check() }()
			<-entered
			var mutation error
			switch fault {
			case "ancestor-access":
				mutation = os.Chmod(filepath.Dir(r.ClientCA), 0755)
			case "ancestor-inode":
				parent := filepath.Dir(r.ClientCA)
				mutation = os.Rename(parent, parent+"-moved")
				if mutation == nil {
					defer func() {
						if err := os.Rename(parent+"-moved", parent); err != nil {
							t.Error(err)
						}
					}()
				}
			case "named-file":
				// An already-read different file is replaced while the final CA
				// fd remains healthy; rechecking only that final fd cannot catch it.
				mutation = os.Rename(r.Certificate, r.Certificate+"-original")
				if mutation == nil {
					mutation = os.WriteFile(r.Certificate, []byte("foreign named input"), 0600)
				}
			}
			close(release)
			observed := <-checked
			if mutation != nil {
				t.Fatal(mutation)
			}
			if !errors.Is(observed, ErrBinding) {
				t.Fatal("post-I/O path change accepted", observed)
			}
			if s.Check() == nil {
				t.Fatal("denied original paths revived")
			}
		})
	}
}
