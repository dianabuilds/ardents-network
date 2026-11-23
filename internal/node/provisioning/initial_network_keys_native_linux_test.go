//go:build linux && network_signing_native

package provisioning

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	artifactio "github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

func initialKeyArguments(t *testing.T, count int) (string, string) {
	t.Helper()
	parent := sourceArtifactNativeParent(t)
	plan, root := filepath.Join(parent, "plan.json"), filepath.Join(parent, "keys")
	raw := fmt.Sprintf(`{"schema":"ardents-initial-keys-plan-v1","operation":"prepare-initial-keys","node_count":%d}`, count)
	if e := os.WriteFile(plan, []byte(raw), 0600); e != nil {
		t.Fatal(e)
	}
	return plan, root
}

// Independent stdlib reader verifies every exact receipt entry, full canonical
// PKCS8 encoding, derived key, native mode and closed inventory.
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
				raw, e := artifactio.EncodeEd25519Key(key)
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
			if _, e := PrepareInitialNetworkKeys(t.Context(), control, destination); e != nil {
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
			_, e := prepareInitialKeys(ctx, plan, root, func() (ed25519.PublicKey, ed25519.PrivateKey, error) { return ed25519.GenerateKey(rand.Reader) }, artifactio.EncodeEd25519Key, func(step string, f *os.File) {
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
			if _, e := PrepareInitialNetworkKeys(t.Context(), plan, root); e == nil {
				t.Fatal("prefix adopted")
			}
			control, destination := initialKeyArguments(t, 1)
			if _, e := PrepareInitialNetworkKeys(t.Context(), control, destination); e != nil {
				t.Fatal("healthy control", e)
			}
		})
	}
}

func TestNetworkSigningInitialKeysFinalOutputReobservation(t *testing.T) {
	for _, fault := range []string{"bytes", "extra", "replacement", "cancel and close", "healthy"} {
		t.Run(fault, func(t *testing.T) {
			plan, root := initialKeyArguments(t, 1)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			acted := false
			_, e := prepareInitialKeys(ctx, plan, root, func() (ed25519.PublicKey, ed25519.PrivateKey, error) { return ed25519.GenerateKey(rand.Reader) }, artifactio.EncodeEd25519Key, func(stage string, f *os.File) {
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
			if _, e = PrepareInitialNetworkKeys(t.Context(), plan, root); e == nil {
				t.Fatal("prefix adopted")
			}
			p, d := initialKeyArguments(t, 1)
			if _, e = PrepareInitialNetworkKeys(t.Context(), p, d); e != nil {
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
	_, e := prepareInitialKeys(t.Context(), plan, root, func() (ed25519.PublicKey, ed25519.PrivateKey, error) { return ed25519.GenerateKey(rand.Reader) }, artifactio.EncodeEd25519Key, func(step string, _ *os.File) {
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
			if _, e := PrepareInitialNetworkKeys(t.Context(), plan, root); e == nil {
				t.Fatal("interrupted root adopted")
			}
			control, destination := initialKeyArguments(t, 1)
			if _, e := PrepareInitialNetworkKeys(t.Context(), control, destination); e != nil {
				t.Fatal("healthy control", e)
			}
		})
	}
}

func TestNetworkSigningInitialKeysNonrootChild(t *testing.T) {
	if os.Getenv("ARDENTS_INITIAL_KEYS_NONROOT") != "1" {
		return
	}
	if os.Geteuid() == 0 {
		t.Fatal("actual nonroot prerequisite")
	}
	if _, e := PrepareInitialNetworkKeys(t.Context(), "/missing-public-plan", "/missing-output"); e != errInitialKeys {
		t.Fatal("nonroot must refuse before input I/O", e)
	}
}

func TestNetworkSigningInitialKeysNonroot(t *testing.T) {
	sourceArtifactNativeParent(t)
	// /proc/self/exe addresses this exact original test image after credentials
	// change, without creating an executable under an insecure ancestor.
	cmd := exec.Command("/proc/self/exe", "-test.run=^TestNetworkSigningInitialKeysNonrootChild$", "-test.timeout=10s")
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 65534, Gid: 65534}}
	cmd.Env = append(os.Environ(), "ARDENTS_INITIAL_KEYS_NONROOT=1")
	if raw, e := cmd.CombinedOutput(); e != nil {
		t.Fatal("actual nonroot child", e, string(raw))
	}
}
