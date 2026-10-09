package instance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func nativeConfig(t *testing.T) Config {
	t.Helper()
	before := time.Now().UTC().Truncate(time.Second)
	return Config{Root: filepath.Join(t.TempDir(), "instance"), NetworkID: [32]byte{3}, NotBefore: before, NotAfter: before.Add(time.Hour)}
}

func TestNativeInstanceExactAcceptanceAndTerminalResponse(t *testing.T) {
	for _, mode := range []string{"accepted", "foreign", "malformed", "changed-after-acceptance"} {
		t.Run(mode, func(t *testing.T) {
			config := nativeConfig(t)
			root, err := Initialize(context.Background(), config)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = Open(context.Background(), config.Root); err == nil {
				t.Fatal("exclusive Instance lease reused")
			}
			raw, err := root.Request(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			view, err := ParseRequest(raw)
			if err != nil {
				t.Fatal(err)
			}
			response := fixtureResponse(view, 7, 3)
			phase := Accepted
			switch mode {
			case "foreign":
				view.InstancePublic[0] ^= 1
				response = fixtureResponse(view, 7, 3)
				phase = Conflicting
			case "malformed":
				response[len(response)-1] ^= 1
				phase = Rejected
			}
			result, acceptErr := root.Accept(context.Background(), response)
			if phase == Accepted {
				if acceptErr != nil || result.Generation != 7 || result.State != Accepted {
					t.Fatal(result, acceptErr)
				}
				again, err := root.Accept(context.Background(), response)
				if err != nil || again != result {
					t.Fatal("exact retry changed", again, err)
				}
				credential, err := root.Credential(context.Background())
				if err != nil || credential.Digest() != result.CredentialDigest {
					t.Fatal(err)
				}
				if mode == "changed-after-acceptance" {
					response[len(response)-1] ^= 1
					_, acceptErr = root.Accept(context.Background(), response)
					phase = Conflicting
					if !errors.Is(acceptErr, ErrUnavailable) {
						t.Fatal(acceptErr)
					}
				}
			} else if !errors.Is(acceptErr, ErrUnavailable) {
				t.Fatal(acceptErr)
			}
			if err = root.Close(); err != nil {
				t.Fatal(err)
			}
			root, err = Open(context.Background(), config.Root)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			if root.state.phase != phase {
				t.Fatal(root.state.phase, phase)
			}
			kept, err := root.Request(context.Background())
			if err != nil || !bytes.Equal(raw, kept) {
				t.Fatal("stable request changed", err)
			}
			if phase != Accepted {
				if _, err = root.Credential(context.Background()); !errors.Is(err, ErrUnavailable) {
					t.Fatal("terminal response retained authority", err)
				}
				if len(root.state.private) != 0 {
					t.Fatal("terminal root retained key")
				}
			}
		})
	}
}

func TestNativeConsumedRestartAndRetainedClose(t *testing.T) {
	config := nativeConfig(t)
	root, err := Initialize(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	response := fixtureResponse(root.state.request, 7, 3)
	if _, err = root.Accept(context.Background(), response); err != nil {
		t.Fatal(err)
	}
	if err = root.Close(); err != nil {
		t.Fatal(err)
	}
	// Isolated persisted-grammar oracle. Actual floor-backed consumption is a
	// separate live Publisher obligation; this fixture establishes no readiness.
	path := filepath.Join(config.Root, stateName)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(raw)
	var stored storedState
	if err = json.Unmarshal(raw, &stored); err != nil {
		t.Fatal(err)
	}
	stored.Phase, stored.Private = Consumed, ""
	raw, err = json.Marshal(stored)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, append(raw, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	root, err = Open(context.Background(), config.Root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = root.Credential(context.Background()); !errors.Is(err, ErrSuccessorRequired) {
		t.Fatal("consumed private authority revived", err)
	}
	if len(root.state.private) != 0 {
		t.Fatal("consumed root has private key")
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := root.Close(); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	root, err = Open(context.Background(), config.Root)
	if err != nil {
		t.Fatal(err)
	}
	if err = root.lease.file.Close(); err != nil {
		t.Fatal(err)
	}
	first := root.Close()
	if first == nil || root.Close() != first {
		t.Fatal("physical Close error not retained")
	}
}

func TestNativeInstanceLostLeaseAndCanceledAcceptance(t *testing.T) {
	config := nativeConfig(t)
	root, err := Initialize(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	before, err := os.ReadFile(filepath.Join(config.Root, stateName))
	if err != nil {
		t.Fatal(err)
	}
	defer clear(before)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = root.Accept(cancelled, fixtureResponse(root.state.request, 7, 3)); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(config.Root, stateName))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("canceled response changed state", err)
	}
	clear(after)
	lock := filepath.Join(config.Root, lockName)
	if err = os.Rename(lock, lock+"-original"); err != nil {
		t.Fatal(err)
	}
	if err = writeExclusive(lock, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = root.Request(context.Background()); err == nil {
		t.Fatal("lost original lease supplied request")
	}
}

func TestNativeInstancePostRenameRefusesUntilReopen(t *testing.T) {
	for _, mode := range []string{"flush-failure", "caller-loss"} {
		t.Run(mode, func(t *testing.T) {
			config := nativeConfig(t)
			root, err := Initialize(context.Background(), config)
			if err != nil {
				t.Fatal(err)
			}
			response := fixtureResponse(root.state.request, 7, 3)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			fault := errors.New("post-rename parent sync failed")
			root.flush = func(path string) error {
				if mode == "flush-failure" {
					return fault
				}
				if err := syncDirectory(path); err != nil {
					return err
				}
				cancel()
				return nil
			}
			_, err = root.Accept(ctx, response)
			wanted := fault
			if mode == "caller-loss" {
				wanted = context.Canceled
			}
			if !errors.Is(err, wanted) {
				t.Fatalf("post-I/O refusal absent: %v", err)
			}
			if len(root.state.private) != 0 {
				t.Fatal("ambiguous owner retained usable private material")
			}
			if _, err = root.Request(context.Background()); err == nil {
				t.Fatal("ambiguous owner returned request receipt")
			}
			if _, err = root.Credential(context.Background()); err == nil {
				t.Fatal("ambiguous owner returned Credential receipt")
			}
			first := root.Close()
			if !errors.Is(first, wanted) || root.Close() != first {
				t.Fatal("terminal first failure changed", first)
			}
			root, err = Open(context.Background(), config.Root)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			if root.state.phase != Accepted {
				t.Fatal("reopen lost committed phase", root.state.phase)
			}
			// This response transition was not a publication/consumption. Its
			// independently reopened public acceptance can reconcile a floor;
			// it grants no accepting runtime or revived consumed lifetime.
			if _, err = root.Credential(context.Background()); err != nil {
				t.Fatal(err)
			}
		})
	}
}
