//go:build linux

package endpoint

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/instance"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

// SIGKILL bypasses every deferred cleanup. The real durable Publication and
// Instance owners are reopened together, as Endpoint must do after a crash.
// Registration acknowledgement is an explicit fixture here; this does not
// qualify installed worker cleanup or remote registration loss.
func TestTextPublicationCrashRetainsCrossOwnerFloors(t *testing.T) {
	if stage := os.Getenv("ARDENTS_PUBLICATION_CRASH_STAGE"); stage != "" {
		runTextPublicationCrashChild(t, stage, os.Getenv("ARDENTS_PUBLICATION_CRASH_ROOT"))
		return
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, stage := range []string{"publication floor", "consumed Instance", "local publication"} {
		t.Run(stage, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Chmod(root, 0700); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, binary, "-test.run=^TestTextPublicationCrashRetainsCrossOwnerFloors$", "-test.timeout=30s")
			command.Env = append(os.Environ(), "ARDENTS_PUBLICATION_CRASH_STAGE="+stage, "ARDENTS_PUBLICATION_CRASH_ROOT="+root)
			var output bytes.Buffer
			command.Stdout, command.Stderr = &output, &output
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- command.Wait() }()
			joined := false
			defer func() {
				if !joined {
					_ = command.Process.Kill()
					<-done
				}
			}()
			marker := filepath.Join(root, "boundary.json")
			tick := time.NewTicker(10 * time.Millisecond)
			defer tick.Stop()
			var public ed25519.PublicKey
			for public == nil {
				raw, err := os.ReadFile(marker)
				if err == nil {
					if err := json.Unmarshal(raw, &public); err != nil {
						t.Fatal(err)
					}
					break
				}
				if !errors.Is(err, os.ErrNotExist) {
					t.Fatal(err)
				}
				select {
				case err := <-done:
					joined = true
					t.Fatalf("child ended before crash boundary: %v / %s", err, output.Bytes())
				case <-ctx.Done():
					t.Fatal("crash boundary deadline")
				case <-tick.C:
				}
			}
			if err := command.Process.Kill(); err != nil {
				t.Fatal(err)
			}
			killed := <-done
			joined = true
			var exit *exec.ExitError
			if !errors.As(killed, &exit) {
				t.Fatalf("child was not killed: %v", killed)
			}
			status, ok := exit.Sys().(syscall.WaitStatus)
			if !ok || !status.Signaled() || status.Signal() != syscall.SIGKILL {
				t.Fatalf("not SIGKILL: %v", killed)
			}
			config := publication.Config{Root: filepath.Join(root, "publication"), NetworkID: fixtureID(201), Authority: public, Clock: time.Now}
			publisher, err := publication.Open(config)
			if err != nil {
				t.Fatal(err)
			}
			defer publisher.Close()
			floor, err := publisher.Floor()
			if err != nil || floor != 1 {
				t.Fatalf("durable floor after crash: %d / %v", floor, err)
			}
			if lease, err := publisher.Acquire(t.Context()); err == nil {
				_ = lease.Close()
				t.Fatal("crash resurrected publication readiness")
			}
			signer, err := instance.Open(filepath.Join(root, "instance"))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := signer.OpenBinding(floor); !errors.Is(err, instance.ErrSuccessorRequired) {
				t.Fatalf("crashed Instance reusable: %v", err)
			}
			if err := signer.Close(); err != nil {
				t.Fatal(err)
			}
			// Reconciliation must itself be durable; even a stale lower caller floor
			// cannot revive the original Instance after the next process opens it.
			signer, err = instance.Open(filepath.Join(root, "instance"))
			if err != nil {
				t.Fatal(err)
			}
			defer signer.Close()
			if _, err := signer.OpenBinding(0); !errors.Is(err, instance.ErrSuccessorRequired) {
				t.Fatalf("reconciliation not durable: %v", err)
			}
		})
	}
}

func runTextPublicationCrashChild(t *testing.T, stage, root string) {
	t.Helper()
	for _, name := range []string{"instance", "publication"} {
		if err := os.Mkdir(filepath.Join(root, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	public, authority, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	signer, binding := acceptedInstanceBinding(t, filepath.Join(root, "instance"), fixtureID(201), authority, now.Add(-time.Second), now.Add(time.Hour))
	defer signer.Close()
	clear(authority)
	publisher, err := publication.Open(publication.Config{Root: filepath.Join(root, "publication"), NetworkID: fixtureID(201), Authority: public, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	defer publisher.Close()
	boundary := func() {
		raw, err := json.Marshal(public)
		if err != nil {
			t.Fatal(err)
		}
		pending := filepath.Join(root, "boundary.pending")
		if err := os.WriteFile(pending, raw, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(pending, filepath.Join(root, "boundary.json")); err != nil {
			t.Fatal(err)
		}
		<-time.After(time.Minute)
		t.Fatal("parent did not terminate child")
	}
	_, err = publisher.PublishAfterReadiness(t.Context(), publication.PublishInput{Credential: binding.Credential(), InstanceSigner: binding, At: now}, func(context.Context) ([]byte, error) {
		if stage == "publication floor" {
			boundary()
		}
		if err := binding.CommitPublished(1); err != nil {
			return nil, err
		}
		if stage == "consumed Instance" {
			boundary()
		}
		return []byte("registration acknowledgement fixture"), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if stage != "local publication" {
		t.Fatal("unexpected crash stage")
	}
	boundary()
}
