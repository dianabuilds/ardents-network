//go:build linux && network_signing_native

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	sourcefixture "github.com/dianabuilds/ardents-network/tests/e2e/source"
)

func initialKeyArguments(t *testing.T, count int) (string, string) {
	t.Helper()
	return sourcefixture.InitialKeyArguments(t, sourcefixture.SigningWorkspace(t), count)
}

func TestNetworkSigningInitialKeysCommand(t *testing.T) {
	for _, count := range []int{1, 64} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			plan, root := initialKeyArguments(t, count)
			cmd := exec.CommandContext(t.Context(), compiledCommand(t), "network", "prepare-initial-keys", plan, root)
			out, e := cmd.Output()
			if e != nil {
				t.Fatal("real producer", e)
			}
			sourcefixture.VerifyInitialKeys(t, root, count, out)
			if e = exec.CommandContext(t.Context(), compiledCommand(t), "network", "prepare-initial-keys", plan, root).Run(); e == nil {
				t.Fatal("completed output adopted")
			}
		})
	}
}

func TestNetworkSigningInitialKeysStateChain(t *testing.T) {
	plan, root := initialKeyArguments(t, 2)
	out, e := exec.CommandContext(t.Context(), compiledCommand(t), "network", "prepare-initial-keys", plan, root).Output()
	if e != nil {
		t.Fatal(e)
	}
	keys := sourcefixture.VerifyInitialKeys(t, root, 2, out)
	paths := map[string]string{}
	for i, key := range keys {
		name := "state-key.pem"
		if i > 0 {
			name = fmt.Sprintf("node-%02d-key.pem", i-1)
		}
		paths[hex.EncodeToString(key.Public().(ed25519.PublicKey))] = filepath.Join(root, name)
	}
	runNetworkPreparedStateChain(t, func(t *testing.T, purpose string, public any, key ed25519.PrivateKey, message []byte) ([]byte, [][]byte) {
		parent := sourcefixture.SigningWorkspace(t)
		planPath, prepared, destination := filepath.Join(parent, "plan"), filepath.Join(parent, "prepared"), filepath.Join(parent, "signed")
		operation := "sign-initial-" + purpose
		declaration := signingDeclaration(t, purpose, public, time.Now().UTC().Truncate(time.Second))
		if purpose == "closed-profile" {
			operation = "sign-closed-profile"
			declaration = profileSigningDeclaration(t, public, key.Public().(ed25519.PublicKey), time.Now().UTC().Truncate(time.Second))
		}
		if e := os.WriteFile(planPath, declaration, 0600); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(prepared, message, 0600); e != nil {
			t.Fatal(e)
		}
		digest := sha256.Sum256(message)
		keyPath := paths[hex.EncodeToString(key.Public().(ed25519.PublicKey))]
		if keyPath == "" {
			t.Fatal("signer lacks original generated key")
		}
		args := []string{"network", operation, planPath, prepared, hex.EncodeToString(digest[:]), keyPath, destination}
		// Independent signer admission rejects a different generated role/key
		// and altered reviewed pin before the same healthy actual command.
		bad := append([]string(nil), args...)
		bad[5] = paths[hex.EncodeToString(keys[0].Public().(ed25519.PublicKey))]
		if bad[5] == keyPath {
			bad[5] = paths[hex.EncodeToString(keys[1].Public().(ed25519.PublicKey))]
		}
		if exec.CommandContext(t.Context(), compiledCommand(t), bad...).Run() == nil {
			t.Fatal("foreign generated role/key accepted")
		}
		bad = append([]string(nil), args...)
		bad[4] = fmt.Sprintf("%064x", 0)
		if exec.CommandContext(t.Context(), compiledCommand(t), bad...).Run() == nil {
			t.Fatal("wrong reviewed pin accepted")
		}
		if e := exec.CommandContext(t.Context(), compiledCommand(t), args...).Run(); e != nil {
			t.Fatal("actual generated-key signer", purpose, e)
		}
		name := "node-record.bin"
		if purpose == "epoch" {
			name = "epoch.bin"
		}
		if purpose == "closed-profile" {
			name = "profile.bin"
		}
		raw, e := os.ReadFile(filepath.Join(destination, name))
		if e != nil {
			t.Fatal(e)
		}
		var materials [][]byte
		if purpose == "epoch" {
			for i := 0; i < 2; i++ {
				m, e := os.ReadFile(filepath.Join(destination, fmt.Sprintf("material-%02d.bin", i)))
				if e != nil {
					t.Fatal(e)
				}
				materials = append(materials, m)
			}
		}
		return raw, materials
	}, keys...)
}

func TestNetworkSigningInitialKeysReceiptFailure(t *testing.T) {
	for _, writer := range []io.Writer{initialKeysFailedWriter{}, initialKeysShortWriter{}} {
		p, r := initialKeyArguments(t, 1)
		if runNetwork(t.Context(), []string{"prepare-initial-keys", p, r}, writer) == 0 {
			t.Fatal("failed output accepted")
		}
		if runNetwork(t.Context(), []string{"prepare-initial-keys", p, r}, io.Discard) == 0 {
			t.Fatal("failed receipt reused")
		}
	}
}

func TestNetworkSigningInitialKeysBlockedReceipt(t *testing.T) {
	plan, root := initialKeyArguments(t, 1)
	r, w, e := os.Pipe()
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	defer w.Close()
	fd := int(w.Fd())
	if e = syscall.SetNonblock(fd, true); e != nil {
		t.Fatal(e)
	}
	for {
		_, e = syscall.Write(fd, bytes.Repeat([]byte{1}, 4096))
		if errors.Is(e, syscall.EAGAIN) {
			break
		}
		if e != nil {
			t.Fatal(e)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	joined := make(chan int, 1)
	go func() { joined <- runNetwork(ctx, []string{"prepare-initial-keys", plan, root}, w) }()
	// Wait for the actual finite files before cancelling original output. A
	// receipt on disk grants neither command success nor adopting retry rights.
	deadline := time.Now().Add(10 * time.Second)
	ready := false
	for time.Now().Before(deadline) {
		if _, e := os.Stat(filepath.Join(root, "public-receipt.json")); e == nil {
			ready = true
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	select {
	case code := <-joined:
		t.Fatal("full receipt pipe completed", code)
	case <-time.After(20 * time.Millisecond):
	}
	cancel()
	code := <-joined
	if !ready || code == 0 {
		t.Fatal("original output not cancelled/joined", ready, code)
	}
	if runNetwork(t.Context(), []string{"prepare-initial-keys", plan, root}, io.Discard) == 0 {
		t.Fatal("failed receipt adopted")
	}
	p, d := initialKeyArguments(t, 1)
	if runNetwork(t.Context(), []string{"prepare-initial-keys", p, d}, io.Discard) != 0 {
		t.Fatal("healthy receipt control")
	}
}

func TestNetworkSigningInitialKeysStaticRefusals(t *testing.T) {
	for _, fault := range []string{"existing", "parent mode", "ancestor", "owner", "plan symlink", "plan hardlink", "plan directory", "plan owner", "duplicate declaration", "unknown declaration", "trailing declaration", "mixed declaration", "count", "fraction", "overflow", "cancel"} {
		t.Run(fault, func(t *testing.T) {
			plan, root := initialKeyArguments(t, 1)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			var e error
			switch fault {
			case "existing":
				e = os.Mkdir(root, 0700)
			case "parent mode":
				e = os.Chmod(filepath.Dir(root), 0755)
			case "ancestor":
				e = os.Chmod(filepath.Dir(root), 0770)
			case "owner":
				e = os.Chown(filepath.Dir(root), 65534, 65534)
			case "plan symlink":
				e = os.Rename(plan, plan+"-original")
				if e == nil {
					e = os.Symlink(plan+"-original", plan)
				}
			case "plan hardlink":
				e = os.Link(plan, plan+"-link")
			case "plan directory":
				e = os.Remove(plan)
				if e == nil {
					e = os.Mkdir(plan, 0700)
				}
			case "plan owner":
				e = os.Chown(plan, 65534, 65534)
			case "cancel":
				cancel()
			default:
				raw, err := os.ReadFile(plan)
				if err != nil {
					t.Fatal(err)
				}
				switch fault {
				case "duplicate declaration":
					raw = bytes.Replace(raw, []byte(`"node_count":1`), []byte(`"node_count":1,"node_count":1`), 1)
				case "unknown declaration":
					raw = bytes.Replace(raw, []byte(`"node_count":1`), []byte(`"node_count":1,"seed":"private-input-forbidden"`), 1)
				case "trailing declaration":
					raw = append(raw, []byte(` {}`)...)
				case "mixed declaration":
					raw = bytes.Replace(raw, []byte("prepare-initial-keys"), []byte("sign-initial-epoch"), 1)
				case "count":
					raw = bytes.Replace(raw, []byte(`"node_count":1`), []byte(`"node_count":65`), 1)
				case "fraction":
					raw = bytes.Replace(raw, []byte(`"node_count":1`), []byte(`"node_count":1.5`), 1)
				case "overflow":
					raw = bytes.Replace(raw, []byte(`"node_count":1`), []byte(`"node_count":18446744073709551616`), 1)
				}
				e = os.WriteFile(plan, raw, 0600)
			}
			if e != nil {
				t.Fatal(e)
			}
			var out bytes.Buffer
			if runNetwork(ctx, []string{"prepare-initial-keys", plan, root}, &out) == 0 || out.Len() != 0 {
				t.Fatal("refusal or private diagnostic")
			}
			if fault != "existing" {
				if _, e = os.Lstat(root); !os.IsNotExist(e) {
					t.Fatal("pre-effect refusal", e)
				}
			}
			if fault == "owner" {
				if e = os.Chown(filepath.Dir(root), 0, 0); e != nil {
					t.Fatal(e)
				}
			}
			if fault == "ancestor" || fault == "parent mode" {
				if e = os.Chmod(filepath.Dir(root), 0700); e != nil {
					t.Fatal(e)
				}
			}
			p, r := initialKeyArguments(t, 1)
			if code := runNetwork(t.Context(), []string{"prepare-initial-keys", p, r}, io.Discard); code != 0 {
				t.Fatal("healthy control", code)
			}
		})
	}
}

type initialKeysFailedWriter struct{}

func (initialKeysFailedWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

type initialKeysShortWriter struct{}

func (initialKeysShortWriter) Write([]byte) (int, error) { return 0, nil }
