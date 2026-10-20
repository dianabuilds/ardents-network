//go:build linux && network_signing_native

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"
)

func initialKeyArguments(t *testing.T, count int) (string, string) {
	t.Helper()
	parent := signingNativeParent(t)
	plan, root := filepath.Join(parent, "plan.json"), filepath.Join(parent, "keys")
	raw := fmt.Sprintf(`{"schema":"ardents-initial-keys-plan-v1","operation":"prepare-initial-keys","node_count":%d}`, count)
	if e := os.WriteFile(plan, []byte(raw), 0600); e != nil {
		t.Fatal(e)
	}
	return plan, root
}

// Independent stdlib reader verifies every exact receipt entry, full canonical
// PKCS8 encoding, derived key, native mode and closed inventory.
func verifyInitialKeys(t *testing.T, root string, count int, stdout []byte) []ed25519.PrivateKey {
	t.Helper()
	raw, e := os.ReadFile(filepath.Join(root, "public-receipt.json"))
	if e != nil {
		t.Fatal(e)
	}
	if stdout != nil && !bytes.Equal(raw, stdout) {
		t.Fatal("receipt mismatch")
	}
	var receipt initialKeyReceipt
	if json.Unmarshal(raw, &receipt) != nil || len(receipt.Keys) != count+1 || receipt.Schema != "ardents-initial-keys-receipt-v1" || receipt.Operation != "prepare-initial-keys" || receipt.State || receipt.Signed || receipt.Installed || receipt.Independent {
		t.Fatal("receipt claims")
	}
	entries, e := os.ReadDir(root)
	if e != nil || len(entries) != count+2 {
		t.Fatal("inventory", e)
	}
	seen := map[string]bool{}
	var keys []ed25519.PrivateKey
	for i, k := range receipt.Keys {
		want := "state-key.pem"
		purpose := "state"
		if i > 0 {
			want = fmt.Sprintf("node-%02d-key.pem", i-1)
			purpose = "node"
		}
		if k.File != want || k.Purpose != purpose || (i == 0 && k.Index != nil) || (i > 0 && (k.Index == nil || *k.Index != i-1)) {
			t.Fatal("role/index inventory")
		}
		private, e := os.ReadFile(filepath.Join(root, k.File))
		if e != nil {
			t.Fatal(e)
		}
		block, rest := pem.Decode(private)
		if block == nil || len(rest) != 0 || block.Type != "PRIVATE KEY" || len(block.Headers) != 0 {
			t.Fatal("PEM")
		}
		parsed, e := x509.ParsePKCS8PrivateKey(block.Bytes)
		if e != nil {
			t.Fatal("PKCS8")
		}
		key, ok := parsed.(ed25519.PrivateKey)
		if !ok {
			t.Fatal("algorithm")
		}
		der, e := x509.MarshalPKCS8PrivateKey(key)
		if e != nil || !bytes.Equal(der, block.Bytes) || !bytes.Equal(private, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})) {
			t.Fatal("canonical bytes")
		}
		derived := ed25519.NewKeyFromSeed(key[:32])
		if !bytes.Equal(derived, key) {
			t.Fatal("seed consistency")
		}
		clear(derived)
		pub := key.Public().(ed25519.PublicKey)
		digest := sha256.Sum256(pub)
		if k.Public != hex.EncodeToString(pub) || k.Digest != hex.EncodeToString(digest[:]) || seen[k.Public] {
			t.Fatal("public facts/distinctness")
		}
		seen[k.Public] = true
		info, e := os.Lstat(filepath.Join(root, k.File))
		if e != nil || !signingFileAllowed(info, true) {
			t.Fatal("native private file", e)
		}
		clear(private)
		clear(block.Bytes)
		clear(der)
		keys = append(keys, key)
	}
	for _, key := range keys {
		t.Cleanup(func() { clear(key) })
	}
	return keys
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
			verifyInitialKeys(t, root, count, out)
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
	keys := verifyInitialKeys(t, root, 2, out)
	paths := map[string]string{}
	for i, key := range keys {
		name := "state-key.pem"
		if i > 0 {
			name = fmt.Sprintf("node-%02d-key.pem", i-1)
		}
		paths[hex.EncodeToString(key.Public().(ed25519.PublicKey))] = filepath.Join(root, name)
	}
	runNetworkPreparedStateChain(t, func(t *testing.T, purpose string, public any, key ed25519.PrivateKey, message []byte) ([]byte, [][]byte) {
		parent := signingNativeParent(t)
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

func TestNetworkSigningInitialKeysSecretFailures(t *testing.T) {
	for _, fault := range []string{"generation", "encoding", "duplicate", "inconsistent", "input replacement", "cancel", "healthy"} {
		t.Run(fault, func(t *testing.T) {
			plan, root := initialKeyArguments(t, 2)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			cause := errors.New("isolated producer failure")
			var owned [][]byte
			var duplicate ed25519.PrivateKey
			calls := 0
			generate := func() (ed25519.PublicKey, ed25519.PrivateKey, error) {
				pub, key, e := ed25519.GenerateKey(rand.Reader)
				calls++
				owned = append(owned, key)
				if fault == "generation" && calls == 2 {
					return pub, key, cause
				}
				if fault == "duplicate" {
					if calls == 1 {
						duplicate = bytes.Clone(key)
					} else {
						clear(key)
						key = bytes.Clone(duplicate)
						owned = append(owned, key)
						pub = key.Public().(ed25519.PublicKey)
					}
				}
				if fault == "inconsistent" {
					key[63] ^= 1
				}
				if fault == "cancel" && calls == 2 {
					cancel()
				}
				if fault == "input replacement" && calls == 1 {
					raw, e := os.ReadFile(plan)
					if e != nil {
						t.Fatal(e)
					}
					if e = os.Rename(plan, plan+"-original"); e != nil {
						t.Fatal(e)
					}
					if e = os.WriteFile(plan, raw, 0600); e != nil {
						t.Fatal(e)
					}
				}
				return pub, key, e
			}
			encode := func(key ed25519.PrivateKey) ([]byte, error) {
				raw, e := encodeInitialKey(key)
				owned = append(owned, raw)
				if fault == "encoding" && calls == 2 {
					return raw, cause
				}
				return raw, e
			}
			receipt, e := prepareInitialKeys(ctx, plan, root, generate, encode, nil)
			clear(duplicate)
			if (e == nil) != (fault == "healthy") || (receipt != nil) != (fault == "healthy") {
				t.Fatal("causal producer outcome", e)
			}
			if (fault == "generation" || fault == "encoding") && !errors.Is(e, cause) {
				t.Fatal("cause lost", e)
			}
			for _, raw := range owned {
				for _, b := range raw {
					if b != 0 {
						t.Fatal("owned plaintext remains")
					}
				}
			}
			if fault != "healthy" {
				if _, e := os.Stat(root); !os.IsNotExist(e) {
					t.Fatal("failed generation created output", e)
				}
			}
			control, destination := initialKeyArguments(t, 1)
			if _, e := executeInitialKeys(t.Context(), control, destination); e != nil {
				t.Fatal("healthy producer control", e)
			}
		})
	}
}

func TestNetworkSigningInitialKeysNativeFaults(t *testing.T) {
	for _, fault := range []string{"healthy", "file mode", "hardlink", "symlink", "replacement", "late bytes", "extra", "directory replacement", "ancestor", "write close", "directory close", "cancel"} {
		t.Run(fault, func(t *testing.T) {
			plan, root := initialKeyArguments(t, 2)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			acted := false
			_, e := prepareInitialKeys(ctx, plan, root, func() (ed25519.PublicKey, ed25519.PrivateKey, error) { return ed25519.GenerateKey(rand.Reader) }, encodeInitialKey, func(step string, f *os.File) {
				if acted {
					return
				}
				var err error
				leaf := filepath.Join(root, "state-key.pem")
				if step == "file-created" {
					switch fault {
					case "file mode":
						err = os.Chmod(leaf, 0644)
						acted = true
					case "hardlink":
						err = os.Link(leaf, root+"-link")
						acted = true
					case "symlink", "replacement":
						err = os.Rename(leaf, root+"-original")
						if err == nil {
							if fault == "symlink" {
								err = os.Symlink(root+"-original", leaf)
							} else {
								err = os.WriteFile(leaf, []byte("foreign"), 0600)
							}
						}
						acted = true
					case "write close":
						err = f.Close()
						acted = true
					case "cancel":
						cancel()
						acted = true
					}
				}
				if step == "before-directory-sync" {
					switch fault {
					case "late bytes":
						err = os.WriteFile(leaf, []byte("foreign"), 0600)
						acted = true
					case "extra":
						err = os.WriteFile(filepath.Join(root, "foreign"), nil, 0600)
						acted = true
					case "directory replacement":
						err = os.Rename(root, root+"-original")
						if err == nil {
							err = os.Mkdir(root, 0700)
						}
						acted = true
					case "ancestor":
						err = os.Chmod(filepath.Dir(root), 0770)
						acted = true
					}
				}
				if step == "directory-sync" && fault == "directory close" {
					err = f.Close()
					acted = true
				}
				if err != nil {
					t.Fatal(err)
				}
			})
			if fault == "ancestor" {
				if e := os.Chmod(filepath.Dir(root), 0700); e != nil {
					t.Fatal(e)
				}
			}
			if (e == nil) != (fault == "healthy") || (fault != "healthy" && !acted) {
				t.Fatal("native causal verdict", e)
			}
			if fault == "write close" && !errors.Is(e, os.ErrClosed) {
				t.Fatal("independent physical causes lost", e)
			}
			if _, e := executeInitialKeys(t.Context(), plan, root); e == nil {
				t.Fatal("prefix adopted")
			}
			control, destination := initialKeyArguments(t, 1)
			if _, e := executeInitialKeys(t.Context(), control, destination); e != nil {
				t.Fatal("healthy control", e)
			}
		})
	}
}

func TestNetworkSigningInitialKeysContentionAndOutput(t *testing.T) {
	plan, root := initialKeyArguments(t, 2)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	start := make(chan struct{})
	for range 2 {
		wg.Go(func() { <-start; _, e := executeInitialKeys(t.Context(), plan, root); results <- e })
	}
	close(start)
	wg.Wait()
	close(results)
	wins := 0
	for e := range results {
		if e == nil {
			wins++
		}
	}
	if wins != 1 {
		t.Fatal("exclusive winners", wins)
	}
	verifyInitialKeys(t, root, 2, nil)
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

func TestNetworkSigningInitialKeysFinalOutputReobservation(t *testing.T) {
	for _, fault := range []string{"bytes", "extra", "replacement", "cancel and close", "healthy"} {
		t.Run(fault, func(t *testing.T) {
			plan, root := initialKeyArguments(t, 1)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			acted := false
			_, e := prepareInitialKeys(ctx, plan, root, func() (ed25519.PublicKey, ed25519.PrivateKey, error) { return ed25519.GenerateKey(rand.Reader) }, encodeInitialKey, func(stage string, f *os.File) {
				if stage != "commit" {
					return
				}
				acted = true
				var err error
				switch fault {
				case "bytes":
					err = os.WriteFile(filepath.Join(root, "state-key.pem"), []byte("changed"), 0600)
				case "extra":
					err = os.WriteFile(filepath.Join(root, "foreign"), []byte("retain"), 0600)
				case "replacement":
					err = os.Rename(filepath.Join(root, "node-00-key.pem"), root+"-original")
					if err == nil {
						err = os.WriteFile(filepath.Join(root, "node-00-key.pem"), []byte("foreign"), 0600)
					}
				case "cancel and close":
					cancel()
					err = f.Close()
				}
				if err != nil {
					t.Fatal(err)
				}
			})
			if !acted || (e == nil) != (fault == "healthy") {
				t.Fatal("final output reobservation", e)
			}
			if fault == "cancel and close" && (!errors.Is(e, context.Canceled) || !errors.Is(e, os.ErrClosed)) {
				t.Fatal("independent cancellation/physical errors lost", e)
			}
			if _, e = executeInitialKeys(t.Context(), plan, root); e == nil {
				t.Fatal("prefix adopted")
			}
			p, d := initialKeyArguments(t, 1)
			if _, e = executeInitialKeys(t.Context(), p, d); e != nil {
				t.Fatal("healthy control", e)
			}
		})
	}
}

func TestNetworkSigningInitialKeysChild(t *testing.T) {
	stage := os.Getenv("ARDENTS_INITIAL_KEYS_STAGE")
	if stage == "" {
		return
	}
	plan, root := os.Getenv("ARDENTS_INITIAL_KEYS_PLAN"), os.Getenv("ARDENTS_INITIAL_KEYS_ROOT")
	if stage == "partial" {
		if e := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &syscall.Rlimit{Cur: 16, Max: 16}); e != nil {
			t.Fatal(e)
		}
	}
	_, e := prepareInitialKeys(t.Context(), plan, root, func() (ed25519.PublicKey, ed25519.PrivateKey, error) { return ed25519.GenerateKey(rand.Reader) }, encodeInitialKey, func(step string, _ *os.File) {
		if step == stage {
			if e := os.WriteFile(root+"-ready", []byte("ready"), 0600); e != nil {
				t.Fatal(e)
			}
			select {}
		}
	})
	if stage == "partial" {
		if !errors.Is(e, syscall.EFBIG) {
			t.Fatal("partial cause", e)
		}
		raw, e := os.ReadFile(filepath.Join(root, "state-key.pem"))
		defer clear(raw)
		if e != nil || len(raw) != 16 {
			t.Fatal("partial prefix", e)
		}
		return
	}
	t.Fatal("cut not reached", e)
}

func TestNetworkSigningInitialKeysProcessCuts(t *testing.T) {
	for _, stage := range []string{"partial", "directory-created", "file-created", "written", "closed", "directory-sync", "commit"} {
		t.Run(stage, func(t *testing.T) {
			plan, root := initialKeyArguments(t, 2)
			cmd := exec.Command(os.Args[0], "-test.run=^TestNetworkSigningInitialKeysChild$", "-test.timeout=20s")
			cmd.Env = append(os.Environ(), "ARDENTS_INITIAL_KEYS_STAGE="+stage, "ARDENTS_INITIAL_KEYS_PLAN="+plan, "ARDENTS_INITIAL_KEYS_ROOT="+root)
			var diagnostic bytes.Buffer
			cmd.Stdout = &diagnostic
			cmd.Stderr = &diagnostic
			if e := cmd.Start(); e != nil {
				t.Fatal(e)
			}
			if stage == "partial" {
				if e := cmd.Wait(); e != nil {
					t.Fatal(e, diagnostic.String())
				}
			} else {
				ready := false
				deadline := time.Now().Add(10 * time.Second)
				for time.Now().Before(deadline) {
					if _, e := os.Stat(root + "-ready"); e == nil {
						ready = true
						break
					}
					time.Sleep(5 * time.Millisecond)
				}
				kill := cmd.Process.Kill()
				joined := cmd.Wait()
				if !ready || kill != nil || joined == nil {
					t.Fatal("original child cut/join unavailable", ready, kill, joined, diagnostic.String())
				}
			}
			if _, e := os.Stat(root); e != nil {
				t.Fatal("prefix lost", e)
			}
			if _, e := executeInitialKeys(t.Context(), plan, root); e == nil {
				t.Fatal("interrupted root adopted")
			}
			control, destination := initialKeyArguments(t, 1)
			if _, e := executeInitialKeys(t.Context(), control, destination); e != nil {
				t.Fatal("healthy control", e)
			}
		})
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
			if _, e = executeInitialKeys(t.Context(), p, r); e != nil {
				t.Fatal("healthy control", e)
			}
		})
	}
	plan, _ := initialKeyArguments(t, 1)
	parent, e := os.MkdirTemp("/dev/shm", "ardents-key-mount-")
	if e != nil {
		t.Fatal("tmpfs prerequisite", e)
	}
	t.Cleanup(func() {
		if e := os.RemoveAll(parent); e != nil {
			t.Error(e)
		}
	})
	root := filepath.Join(parent, "keys")
	if _, e = executeInitialKeys(t.Context(), plan, root); e == nil {
		t.Fatal("tmpfs accepted")
	}
	if _, e = os.Stat(root); !os.IsNotExist(e) {
		t.Fatal("mount refusal created root", e)
	}
}

func TestNetworkSigningInitialKeysNonrootChild(t *testing.T) {
	if os.Getenv("ARDENTS_INITIAL_KEYS_NONROOT") != "1" {
		return
	}
	if os.Geteuid() == 0 {
		t.Fatal("actual nonroot prerequisite")
	}
	if _, e := executeInitialKeys(t.Context(), "/missing-public-plan", "/missing-output"); e != errInitialKeys {
		t.Fatal("nonroot must refuse before input I/O", e)
	}
}

func TestNetworkSigningInitialKeysNonroot(t *testing.T) {
	signingNativeParent(t)
	// /proc/self/exe addresses this exact original test image after credentials
	// change, without creating an executable under an insecure ancestor.
	cmd := exec.Command("/proc/self/exe", "-test.run=^TestNetworkSigningInitialKeysNonrootChild$", "-test.timeout=10s")
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 65534, Gid: 65534}}
	cmd.Env = append(os.Environ(), "ARDENTS_INITIAL_KEYS_NONROOT=1")
	if raw, e := cmd.CombinedOutput(); e != nil {
		t.Fatal("actual nonroot child", e, string(raw))
	}
}

type initialKeysFailedWriter struct{}

func (initialKeysFailedWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

type initialKeysShortWriter struct{}

func (initialKeysShortWriter) Write([]byte) (int, error) { return 0, nil }
