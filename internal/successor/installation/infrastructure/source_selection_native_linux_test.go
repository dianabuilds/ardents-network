//go:build linux && network_signing_native

package infrastructure

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// These fixtures exercise only original native byte/inode custody. They do not
// construct a Startup, an Authorization or a successful State/Source neighbor.
// The compiled stopped command separately exercises complete selected readback.
func nativeSelectionCustody(t *testing.T, ctx context.Context) *sourceSelection {
	t.Helper()
	births, path := nativeSourceBirths(t, ctx)
	t.Cleanup(func() { _ = births.close() })
	base := filepath.Dir(path)
	sealed := filepath.Join(base, "sealed")
	if err := os.Mkdir(sealed, 0700); err != nil {
		t.Fatal(err)
	}
	clock := filepath.Join(base, "clock")
	if err := os.WriteFile(clock, []byte("explicit live external evidence"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(clock, 0644); err != nil {
		t.Fatal(err)
	}
	s := &sourceSelection{ctx: ctx, sealed: sealed, binding: sourceBinding{Request: SourceRequest{Clock: clock}}, files: map[string]*sourceFile{}, dirs: map[string]*os.File{}, identities: map[string]os.FileInfo{}}
	t.Cleanup(func() { _ = s.close() })
	// Independent closed native inventory, with no production name constructor.
	for _, name := range []string{"ardents-node-linux-amd64", "ardents-source.service", "binding.json", "infrastructure-enrollment.json", "infrastructure-source.json", "rendered-source.service", "root.json", "snapshot.json", "source-plan.json", "source-request.json", "targets.json", "timestamp.json"} {
		file := filepath.Join(sealed, name)
		mode := os.FileMode(0444)
		if name == "ardents-node-linux-amd64" {
			mode = 0555
		}
		if err := os.WriteFile(file, []byte("bounded physical custody vector: "+name), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(file, mode); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(sealed, 0555); err != nil {
		t.Fatal(err)
	}
	for _, name := range sourceGenerationNames() {
		mode := os.FileMode(0444)
		if name == "ardents-node-linux-amd64" {
			mode = 0555
		}
		if _, err := s.read(filepath.Join(sealed, name), 4096, mode, 0, 0); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.observe(); err != nil {
		t.Fatal("physical custody vector refused", err)
	}
	return s
}

func TestSourceStoppedNativeSelectionOriginalFilesAndLiveClock(t *testing.T) {
	for _, fault := range []string{"bytes", "access", "ancestor", "extra", "missing", "live-clock", "clock-writer-access"} {
		t.Run(fault, func(t *testing.T) {
			s := nativeSelectionCustody(t, t.Context())
			path := filepath.Join(s.sealed, "source-plan.json")
			var err error
			switch fault {
			case "bytes":
				err = os.WriteFile(path, []byte("foreign source plan"), 0444)
			case "access":
				err = os.Chmod(path, 0644)
			case "ancestor":
				err = os.Chmod(filepath.Dir(s.sealed), 0755)
			case "extra":
				err = os.WriteFile(filepath.Join(s.sealed, "foreign"), []byte("extra inventory"), 0600)
			case "missing":
				err = os.Rename(path, filepath.Join(filepath.Dir(s.sealed), "retained-original"))
			case "live-clock":
				clock := s.binding.Request.Clock
				if err = os.Rename(clock, clock+".original"); err == nil {
					err = os.WriteFile(clock, []byte("new explicitly supplied clock observation"), 0600)
				}
				if err == nil {
					err = os.Chmod(clock, 0644)
				}
			case "clock-writer-access":
				err = os.Chmod(s.binding.Request.Clock, 0666)
			}
			if err != nil {
				t.Fatal(err)
			}
			observed := s.observe()
			if fault == "live-clock" {
				if observed != nil {
					t.Fatal("live clock inode/contents were frozen", observed)
				}
				return
			}
			if observed == nil || s.observe() == nil {
				t.Fatal("changed original selection admitted or revived")
			}
		})
	}
}

func TestSourceStoppedNativeSelectionHeldLastReadAndCloseFailure(t *testing.T) {
	for _, fault := range []string{"named-original", "cancellation-close"} {
		t.Run(fault, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			s := nativeSelectionCustody(t, ctx)
			entered, release := make(chan struct{}), make(chan struct{})
			var first *os.File
			count := 0
			s.observed = func(f *os.File) {
				count++
				if count == 1 {
					first = f
				}
				if count == 12 {
					close(entered)
					<-release
				}
			}
			checked := make(chan error, 1)
			go func() { checked <- s.observe() }()
			<-entered
			var mutation error
			if fault == "named-original" {
				original := s.files[first.Name()]
				mutation = os.Rename(first.Name(), filepath.Join(filepath.Dir(s.sealed), "retained-original"))
				if mutation == nil {
					mutation = os.WriteFile(first.Name(), original.raw, 0600)
				}
				if mutation == nil {
					mutation = os.Chmod(first.Name(), original.info.Mode().Perm())
				}
			} else {
				cancel()
				mutation = first.Close() // actual independent physical failure
			}
			select {
			case <-checked:
				t.Fatal("held physical read published a result before join")
			default:
			}
			close(release)
			observed := <-checked
			if mutation != nil {
				t.Fatal(mutation)
			}
			if observed == nil || s.observe() == nil {
				t.Fatal("post-I/O substitution/cancellation admitted")
			}
			closed := s.close()
			if fault == "cancellation-close" && (!errors.Is(observed, context.Canceled) || !errors.Is(closed, os.ErrClosed) || !errors.Is(closed, context.Canceled)) {
				t.Fatal("original cancellation or physical Close error lost", observed, closed)
			}
			if s.close() != closed {
				t.Fatal("repeated Close changed retained result")
			}
		})
	}
}
