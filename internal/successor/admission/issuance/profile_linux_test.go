//go:build linux

package issuance

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuerprofile"
	"github.com/dianabuilds/ardents-network/internal/successor/nodeidentity"
)

func profileOwners(t *testing.T) (Store, nodeidentity.Store, string, Inventory, string, string) {
	t.Helper()
	base := t.TempDir()
	if e := os.Chmod(base, 0700); e != nil {
		t.Fatal(e)
	}
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{11}, 32))
	b := testBinding(1)
	b.Signer = [32]byte(private.Public().(ed25519.PublicKey))
	keyRoot := filepath.Join(base, "keys")
	if e := Initialize(t.Context(), keyRoot, b); e != nil {
		t.Fatal(e)
	}
	keys, e := Open(t.Context(), keyRoot, b)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = keys.Close() })
	v, e := keys.Inventory()
	if e != nil {
		t.Fatal(e)
	}
	der, e := x509.MarshalPKCS8PrivateKey(private)
	if e != nil {
		t.Fatal(e)
	}
	source := filepath.Join(base, "source.pem")
	if e = os.WriteFile(source, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0600); e != nil {
		t.Fatal(e)
	}
	identityRoot := filepath.Join(base, "identity")
	binding := nodeidentity.Binding{Network: b.Network, Node: b.Issuer, Signer: b.Signer}
	if e = nodeidentity.Import(t.Context(), identityRoot, source, binding); e != nil {
		t.Fatal(e)
	}
	identity, e := nodeidentity.Open(t.Context(), identityRoot, binding)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = identity.Close() })
	return keys, identity, filepath.Join(base, "profile"), v, keyRoot, identityRoot
}
func TestProfileRealSigningCopyAndReopen(t *testing.T) {
	keys, identity, root, v, _, _ := profileOwners(t)
	if _, e := identity.SignIssuerProfile(t.Context(), issuerprofile.Request{}); !errors.Is(e, nodeidentity.ErrInvalid) {
		t.Fatal(e)
	}
	if e := InitializeProfile(t.Context(), root, keys, identity); e != nil {
		t.Fatal(e)
	}
	p, e := OpenProfile(t.Context(), root, keys)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = OpenProfile(t.Context(), root, keys); !errors.Is(e, ErrBusy) {
		t.Fatal(e)
	}
	raw, e := p.Bytes()
	if e != nil {
		t.Fatal(e)
	}
	if !ed25519.Verify(v.Binding.Signer[:], append([]byte("ardents-closed-issuer-keys-v1\x00"), raw[:len(raw)-64]...), raw[len(raw)-64:]) {
		t.Fatal("independent Node signature")
	}
	original := append([]byte(nil), raw...)
	raw[0] ^= 1
	again, e := p.Bytes()
	if e != nil || !bytes.Equal(again, original) {
		t.Fatal("mutable profile")
	}
	copyHandle := p
	if e = p.Close(); e != nil {
		t.Fatal(e)
	}
	if e = copyHandle.Close(); e != nil {
		t.Fatal(e)
	}
	if e = InitializeProfile(t.Context(), root, keys, identity); !errors.Is(e, ErrUnavailable) {
		t.Fatal("existing profile replaced", e)
	}
	p, e = OpenProfile(t.Context(), root, keys)
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	again, e = p.Bytes()
	if e != nil || !bytes.Equal(again, original) {
		t.Fatal("reopen changed profile")
	}
	b, transcript, ok := func() (issuerprofile.Binding, []byte, bool) {
		r, e := profileRequest(v)
		if e != nil {
			t.Fatal(e)
		}
		return r.Snapshot()
	}()
	if !ok || b.Signer != v.Binding.Signer || len(transcript) == 0 {
		t.Fatal("request")
	}
	bad := v
	bad.Binding.Issuer[0] ^= 1
	request, e := profileRequest(bad)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = identity.SignIssuerProfile(t.Context(), request); !errors.Is(e, nodeidentity.ErrInvalid) {
		t.Fatal("wrong purpose binding")
	}
}
func TestProfileFaultsTamperingAndForeignRoots(t *testing.T) {
	keys, identity, _, v, keyRoot, identityRoot := profileOwners(t)
	request, e := profileRequest(v)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := identity.SignIssuerProfile(t.Context(), request)
	if e != nil {
		t.Fatal(e)
	}
	for _, phase := range []string{"write", "sync", "close", "rename", "directory-sync"} {
		t.Run(phase, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "profile")
			hit := false
			e := initializeProfileStorage(t.Context(), root, v.Binding, raw, v.Digest, func(p string) error {
				if p == phase {
					hit = true
					return errors.New("fault")
				}
				return nil
			})
			if !hit || !errors.Is(e, ErrUncertain) {
				t.Fatal(e)
			}
			if _, e = OpenProfile(t.Context(), root, keys); e == nil {
				t.Fatal("partial root accepted")
			}
		})
	}
	for _, name := range []string{"pin", "signature", "pending", "symlink", "hardlink", "mode", "root-mode", "missing"} {
		t.Run(name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "profile")
			if e := InitializeProfile(t.Context(), root, keys, identity); e != nil {
				t.Fatal(e)
			}
			path := filepath.Join(root, "profile.bytes")
			switch name {
			case "pin":
				_ = os.WriteFile(filepath.Join(root, "profile.pin"), []byte("changed"), 0600)
			case "signature":
				bad := append([]byte(nil), raw...)
				bad[len(bad)-1] ^= 1
				_ = os.WriteFile(path, bad, 0600)
			case "pending":
				_ = os.WriteFile(filepath.Join(root, "profile.pending"), nil, 0600)
			case "symlink":
				_ = os.Rename(path, path+".real")
				if e := os.Symlink(path+".real", path); e != nil {
					t.Fatal(e)
				}
			case "hardlink":
				if e := os.Link(path, filepath.Join(filepath.Dir(root), "link")); e != nil {
					t.Fatal(e)
				}
			case "mode":
				_ = os.Chmod(path, 0644)
			case "root-mode":
				_ = os.Chmod(root, 0755)
			case "missing":
				_ = os.Remove(path)
			}
			if _, e := OpenProfile(t.Context(), root, keys); e == nil {
				t.Fatal("bad profile accepted")
			}
		})
	}
	for _, root := range []string{keyRoot, identityRoot} {
		if _, e := OpenProfile(t.Context(), root, keys); e == nil {
			t.Fatal("foreign root accepted")
		}
	}
	root := filepath.Join(t.TempDir(), "foreign")
	if e := os.Mkdir(root, 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(root, "issuer.keys"), []byte("legacy"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := OpenProfile(t.Context(), root, keys); e == nil {
		t.Fatal("legacy accepted")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	fresh := filepath.Join(t.TempDir(), "profile")
	if e := InitializeProfile(ctx, fresh, keys, identity); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if _, e := os.Stat(fresh); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("cancel effects")
	}
	fresh = filepath.Join(t.TempDir(), "profile")
	if e := InitializeProfile(t.Context(), fresh, keys, identity); e != nil {
		t.Fatal(e)
	}
	if _, e := openProfileStorage(t.Context(), fresh, keys, v, func(phase string) error {
		if phase == "open-sync" {
			return errors.New("barrier")
		}
		return nil
	}); e == nil {
		t.Fatal("failed barrier accepted")
	}
}

type profileProcessPlan struct {
	Keys, Identity, Profile string
	Binding                 Binding
}

func TestProfileProcessCrashPreservesExactBytes(t *testing.T) {
	if path := os.Getenv("ARDENTS_PROFILE_CHILD"); path != "" {
		raw, e := os.ReadFile(path)
		if e != nil {
			os.Exit(2)
		}
		var plan profileProcessPlan
		if json.Unmarshal(raw, &plan) != nil {
			os.Exit(3)
		}
		ctx := context.Background()
		identity, e := nodeidentity.Open(ctx, plan.Identity, nodeidentity.Binding{Network: plan.Binding.Network, Node: plan.Binding.Issuer, Signer: plan.Binding.Signer})
		if e != nil {
			os.Exit(4)
		}
		keys, e := Open(ctx, plan.Keys, plan.Binding)
		if e != nil {
			os.Exit(5)
		}
		profile, e := OpenProfile(ctx, plan.Profile, keys)
		if e != nil {
			os.Exit(6)
		}
		if e = os.WriteFile(path+".ready", []byte("ready"), 0600); e != nil {
			os.Exit(7)
		}
		for {
			time.Sleep(time.Second)
			runtime.KeepAlive(identity)
			runtime.KeepAlive(keys)
			runtime.KeepAlive(profile)
		}
	}
	keys, identity, root, v, keyRoot, identityRoot := profileOwners(t)
	if e := InitializeProfile(t.Context(), root, keys, identity); e != nil {
		t.Fatal(e)
	}
	before, e := os.ReadFile(filepath.Join(root, "profile.bytes"))
	if e != nil {
		t.Fatal(e)
	}
	if e = identity.Close(); e != nil {
		t.Fatal(e)
	}
	if e = keys.Close(); e != nil {
		t.Fatal(e)
	}
	plan := profileProcessPlan{keyRoot, identityRoot, root, v.Binding}
	raw, e := json.Marshal(plan)
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "plan")
	if e = os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestProfileProcessCrashPreservesExactBytes$")
	cmd.Env = append(os.Environ(), "ARDENTS_PROFILE_CHILD="+path)
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
		if _, e = os.Stat(path + ".ready"); e == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("child not ready")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, e = Open(t.Context(), keyRoot, v.Binding); !errors.Is(e, ErrBusy) {
		t.Fatal(e)
	}
	join()
	keys, e = Open(t.Context(), keyRoot, v.Binding)
	if e != nil {
		t.Fatal(e)
	}
	defer keys.Close()
	profile, e := OpenProfile(t.Context(), root, keys)
	if e != nil {
		t.Fatal(e)
	}
	defer profile.Close()
	after, e := profile.Bytes()
	if e != nil || !bytes.Equal(before, after) {
		t.Fatal("crash changed profile")
	}
}
