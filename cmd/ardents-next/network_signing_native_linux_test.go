//go:build linux && network_signing_native

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
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

func signingNativeParent(t *testing.T) string {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Fatal("invalid environment: Linux root required")
	}
	p := t.TempDir()
	if err := os.Chmod(p, 0700); err != nil {
		t.Fatal(err)
	}
	ancestors, err := openPreparationParent(p)
	if err != nil {
		t.Fatal("invalid environment", err)
	}
	if err = errors.Join(signingAncestors(ancestors), ancestors.close()); err != nil {
		t.Fatal("invalid environment: root-controlled direct ancestors required", err)
	}
	var fs syscall.Statfs_t
	if err = syscall.Statfs(p, &fs); err != nil || fs.Type != 0xef53 {
		t.Fatal("invalid environment: native ext4 required", err)
	}
	return p
}

func writeSigningTestKey(t *testing.T, path string, key ed25519.PrivateKey) {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(der)
	raw := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	defer clear(raw)
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func nativeSigningArguments(t *testing.T) []string {
	t.Helper()
	parent := signingNativeParent(t)
	p, key, _, _ := signingRecordFixture(t)
	now := time.Now().UTC().Truncate(time.Second)
	p["valid_from"], p["valid_until"] = now.Add(-time.Minute).Format(time.RFC3339), now.Add(time.Hour).Format(time.RFC3339)
	_, message, err := prepareNetworkPublic("prepare-initial-node-record", preparationJSON(t, p))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(message)
	plan, prepared, keyPath := filepath.Join(parent, "plan"), filepath.Join(parent, "prepared"), filepath.Join(parent, "key")
	if err = os.WriteFile(plan, signingDeclaration(t, "node-record", p, now), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(prepared, message, 0600); err != nil {
		t.Fatal(err)
	}
	writeSigningTestKey(t, keyPath, key)
	return []string{"sign-initial-node-record", plan, prepared, preparationHex(digest[:]), keyPath, filepath.Join(parent, "signed")}
}

func changeSigningTestTime(t *testing.T, args []string, from, until, reference time.Time, reprepare bool) {
	t.Helper()
	raw, err := os.ReadFile(args[1])
	if err != nil {
		t.Fatal(err)
	}
	var plan map[string]json.RawMessage
	var record map[string]any
	if json.Unmarshal(raw, &plan) != nil || json.Unmarshal(plan["record"], &record) != nil {
		t.Fatal("test declaration")
	}
	record["valid_from"], record["valid_until"] = from.Format(time.RFC3339Nano), until.Format(time.RFC3339Nano)
	plan["record"] = preparationJSON(t, record)
	plan["reference_time"] = preparationJSON(t, reference.Format(time.RFC3339Nano))
	if reprepare {
		_, message, e := prepareNetworkPublic("prepare-initial-node-record", plan["record"])
		if e != nil {
			t.Fatal(e)
		}
		digest := sha256.Sum256(message)
		args[3] = preparationHex(digest[:])
		if e = os.WriteFile(args[2], message, 0600); e != nil {
			t.Fatal(e)
		}
	}
	if err = os.WriteFile(args[1], preparationJSON(t, plan), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestNetworkSigningOriginalTimeAfterIO(t *testing.T) {
	for _, stage := range []string{"inputs-read", "before-directory-sync"} {
		t.Run(stage, func(t *testing.T) {
			args := nativeSigningArguments(t)
			now := time.Now().UTC().Truncate(time.Second)
			until := now.Add(3 * time.Second)
			changeSigningTestTime(t, args, now.Add(-time.Minute), until, now, true)
			observed := false
			_, err := executeObservedNetworkSigning(t.Context(), args, func(step string, _ *os.File) {
				if step != stage {
					return
				}
				observed = true
				// This waits for an actual clock boundary, not physical completion.
				// The original synchronous operation retains every input descriptor.
				if d := time.Until(until); d > 0 {
					time.Sleep(d + time.Millisecond)
				}
				if time.Now().Before(until) {
					t.Fatal("original clock prerequisite changed")
				}
			})
			if !observed || err == nil {
				t.Fatal("expired original time accepted after native IO", err)
			}
			_, statErr := os.Stat(args[5])
			if stage == "inputs-read" && !os.IsNotExist(statErr) {
				t.Fatal("expiry before signature created output", statErr)
			}
			if stage == "before-directory-sync" && statErr != nil {
				t.Fatal("uncertain output prefix removed", statErr)
			}
			control := nativeSigningArguments(t)
			if _, e := executeNetworkSigning(t.Context(), control); e != nil {
				t.Fatal("independent healthy time control", e)
			}
		})
	}
}

func TestNetworkSigningCommandsGenuineStateChain(t *testing.T) {
	signingNativeParent(t)
	runNetworkPreparedStateChain(t, func(t *testing.T, purpose string, plan any, key ed25519.PrivateKey, message []byte) ([]byte, [][]byte) {
		parent := signingNativeParent(t)
		planPath, prepared, keyPath, root := filepath.Join(parent, "plan"), filepath.Join(parent, "prepared"), filepath.Join(parent, "key"), filepath.Join(parent, "signed")
		declaration := signingDeclaration(t, purpose, plan, time.Now().UTC().Truncate(time.Second))
		operation := "sign-initial-" + purpose
		if purpose == "closed-profile" {
			operation = "sign-closed-profile"
			declaration = profileSigningDeclaration(t, plan, key.Public().(ed25519.PublicKey), time.Now().UTC().Truncate(time.Second))
		}
		if err := os.WriteFile(planPath, declaration, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(prepared, message, 0600); err != nil {
			t.Fatal(err)
		}
		writeSigningTestKey(t, keyPath, key)
		digest := sha256.Sum256(message)
		cmd := exec.CommandContext(t.Context(), compiledCommand(t), "network", operation, planPath, prepared, preparationHex(digest[:]), keyPath, root)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatal("genuine signing command", err, string(out))
		}
		name := "node-record.bin"
		var materials [][]byte
		if purpose == "epoch" {
			name = "epoch.bin"
		}
		if purpose == "closed-profile" {
			name = "profile.bin"
		}
		got, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		if purpose == "epoch" {
			records := plan.(map[string]any)["records"].([]string)
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 1+2*len(records) {
				t.Fatal("finite Epoch inventory", err)
			}
			for i, record := range records {
				material, e := os.ReadFile(filepath.Join(root, fmt.Sprintf("material-%02d.bin", i)))
				if e != nil {
					t.Fatal(e)
				}
				materials = append(materials, material)
				input, e := os.ReadFile(filepath.Join(root, fmt.Sprintf("input-%02d.bin", i)))
				if e != nil || preparationHex(input) != record {
					t.Fatal("product Source input differs", e)
				}
			}
		}
		retry := exec.CommandContext(t.Context(), compiledCommand(t), "network", operation, planPath, prepared, preparationHex(digest[:]), keyPath, root)
		if retry.Run() == nil {
			t.Fatal("existing output adopted")
		}
		return got, materials
	})
}

func TestNetworkSigningOriginalInputCustody(t *testing.T) {
	for name, arguments := range map[string]func(*testing.T) []string{"record": nativeSigningArguments, "profile": nativeProfileSigningArguments} {
		t.Run(name, func(t *testing.T) { testSigningOriginalInputCustody(t, arguments) })
	}
}

func testSigningOriginalInputCustody(t *testing.T, arguments func(*testing.T) []string) {
	for _, fault := range []string{"healthy", "key mode", "key hardlink", "key replacement", "key changed", "public replacement", "public changed", "ancestor writable", "cancel", "late key replacement"} {
		t.Run(fault, func(t *testing.T) {
			args := arguments(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			observed := false
			artifacts, err := executeObservedNetworkSigning(ctx, args, func(step string, _ *os.File) {
				at := "inputs-read"
				if fault == "late key replacement" {
					at = "signed"
				}
				if step != at {
					return
				}
				observed = true
				var e error
				switch fault {
				case "key mode":
					e = os.Chmod(args[4], 0644)
				case "key hardlink":
					e = os.Link(args[4], args[4]+"-link")
				case "key replacement", "late key replacement":
					e = os.Rename(args[4], args[4]+"-original")
					if e == nil {
						e = os.WriteFile(args[4], []byte("replacement"), 0600)
					}
				case "key changed":
					e = os.WriteFile(args[4], []byte("corrupt"), 0600)
				case "public replacement":
					e = os.Rename(args[2], args[2]+"-original")
					if e == nil {
						e = os.WriteFile(args[2], []byte("replacement"), 0600)
					}
				case "public changed":
					e = os.WriteFile(args[1], []byte("{}"), 0600)
				case "ancestor writable":
					e = os.Chmod(filepath.Dir(args[4]), 0770)
				case "cancel":
					cancel()
				}
				if e != nil {
					t.Fatal(e)
				}
			})
			if !observed || (err == nil) != (fault == "healthy") {
				t.Fatal("causal input verdict", err)
			}
			if fault == "healthy" {
				if artifacts.digest == [32]byte{} {
					t.Fatal("missing actual artifact")
				}
			} else if _, e := os.Lstat(args[5]); !os.IsNotExist(e) {
				t.Fatal("pre-output refusal created destination", e)
			}
			if fault == "ancestor writable" {
				if e := os.Chmod(filepath.Dir(args[4]), 0700); e != nil {
					t.Fatal(e)
				}
			}
			control := arguments(t)
			if _, e := executeNetworkSigning(t.Context(), control); e != nil {
				t.Fatal("independent healthy control", e)
			}
		})
	}
}

func TestNetworkSigningDurableInventory(t *testing.T) {
	for _, fault := range []string{"healthy", "existing", "file replacement", "hardlink", "extra", "late bytes", "directory replacement", "write close", "dir sync close", "cancel"} {
		t.Run(fault, func(t *testing.T) {
			parent := signingNativeParent(t)
			root := filepath.Join(parent, "signed")
			if fault == "existing" {
				if err := os.Mkdir(root, 0700); err != nil {
					t.Fatal(err)
				}
			}
			artifacts := []networkSignedArtifact{{"epoch.bin", bytes.Repeat([]byte{3}, 512)}, {"input-00.bin", []byte("input")}, {"material-00.bin", []byte("material")}}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			acted := false
			err := persistNetworkSigning(ctx, root, artifacts, func() error { return ctx.Err() }, func(step string, f *os.File) {
				if acted {
					return
				}
				var e error
				if step == "file-created" {
					switch fault {
					case "file replacement":
						e = os.Rename(filepath.Join(root, "epoch.bin"), filepath.Join(parent, "original"))
						if e == nil {
							e = os.WriteFile(filepath.Join(root, "epoch.bin"), artifacts[0].raw, 0600)
						}
						acted = true
					case "hardlink":
						e = os.Link(filepath.Join(root, "epoch.bin"), filepath.Join(parent, "link"))
						acted = true
					case "write close":
						e = f.Close()
						acted = true
					case "cancel":
						cancel()
						acted = true
					}
				}
				if step == "before-directory-sync" {
					switch fault {
					case "extra":
						e = os.WriteFile(filepath.Join(root, "foreign"), []byte("keep"), 0600)
						acted = true
					case "late bytes":
						e = os.WriteFile(filepath.Join(root, "epoch.bin"), []byte("changed"), 0600)
						acted = true
					case "directory replacement":
						e = os.Rename(root, root+"-original")
						if e == nil {
							e = os.Mkdir(root, 0700)
						}
						acted = true
					}
				}
				if step == "directory-sync" && fault == "dir sync close" {
					e = f.Close()
					acted = true
				}
				if e != nil {
					t.Fatal(e)
				}
			})
			if (err == nil) != (fault == "healthy") {
				t.Fatal("durability verdict", err)
			}
			if fault == "write close" && !errors.Is(err, os.ErrClosed) {
				t.Fatal("physical errors not retained", err)
			}
			if fault != "healthy" && fault != "existing" && !acted {
				t.Fatal("fault did not execute")
			}
			if e := persistNetworkSigning(t.Context(), root, artifacts, func() error { return nil }, nil); e == nil {
				t.Fatal("prefix adopted")
			}
		})
	}
}

func TestNetworkSigningConcurrentOutput(t *testing.T) {
	parent := signingNativeParent(t)
	root := filepath.Join(parent, "signed")
	var wg sync.WaitGroup
	results := make(chan error, 2)
	start := make(chan struct{})
	for range 2 {
		wg.Go(func() {
			<-start
			results <- persistNetworkSigning(t.Context(), root, []networkSignedArtifact{{"node-record.bin", []byte("signed")}}, func() error { return nil }, nil)
		})
	}
	close(start)
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatal("exclusive output winners", success)
	}
}

func TestNetworkSigningOriginalRefusalCannotRevive(t *testing.T) {
	for name, arguments := range map[string]func(*testing.T) []string{"record": nativeSigningArguments, "profile": nativeProfileSigningArguments} {
		t.Run(name, func(t *testing.T) { testSigningOriginalRefusalCannotRevive(t, arguments) })
	}
}

func testSigningOriginalRefusalCannotRevive(t *testing.T, arguments func(*testing.T) []string) {
	args := arguments(t)
	input, err := openSigningInput(t.Context(), args[4], 16<<10, true)
	if err != nil {
		t.Fatal(err)
	}
	borrowed := input.raw
	if err = os.Chmod(args[4], 0644); err != nil {
		t.Fatal(err)
	}
	if input.check(t.Context()) == nil {
		t.Fatal("changed original access accepted")
	}
	if err = os.Chmod(args[4], 0600); err != nil {
		t.Fatal(err)
	}
	if input.check(t.Context()) == nil {
		t.Fatal("restoration revived original private input")
	}
	if err = input.close(); err != nil {
		t.Fatal(err)
	}
	for _, b := range borrowed {
		if b != 0 {
			t.Fatal("owned plaintext input not cleared")
		}
	}
	fresh, err := openSigningInput(t.Context(), args[4], 16<<10, true)
	if err != nil {
		t.Fatal("fresh independent input control", err)
	}
	if err = fresh.close(); err != nil {
		t.Fatal(err)
	}
}

func TestNetworkSigningUnsupportedMount(t *testing.T) {
	signingNativeParent(t)
	parent, err := os.MkdirTemp("/dev/shm", "ardents-signing-mount-")
	if err != nil {
		t.Fatal("tmpfs refusal prerequisite", err)
	}
	t.Cleanup(func() {
		if e := os.RemoveAll(parent); e != nil {
			t.Error(e)
		}
	})
	root := filepath.Join(parent, "signed")
	if err = persistNetworkSigning(t.Context(), root, []networkSignedArtifact{{"node-record.bin", []byte("signed")}}, func() error { return nil }, nil); err == nil {
		t.Fatal("unsupported mount accepted")
	}
	if _, err = os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("mount refusal created output", err)
	}
}

func TestNetworkSigningReceiptBlockedOutput(t *testing.T) {
	signingNativeParent(t)
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	fd := int(w.Fd())
	if err = syscall.SetNonblock(fd, true); err != nil {
		t.Fatal(err)
	}
	for {
		_, err = syscall.Write(fd, bytes.Repeat([]byte{1}, 4096))
		if errors.Is(err, syscall.EAGAIN) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	joined := make(chan error, 1)
	go func() { joined <- writeNetworkPreparationReceipt(ctx, w, bytes.Repeat([]byte{1}, 4<<20)) }()
	select {
	case err := <-joined:
		t.Fatal("full pipe completed before cancellation", err)
	case <-time.After(20 * time.Millisecond):
	}
	cancel()
	if err := <-joined; !errors.Is(err, context.Canceled) {
		t.Fatal("blocked output accepted")
	}
}

func TestNetworkSigningInterruptedChild(t *testing.T) {
	if os.Getenv("ARDENTS_SIGNING_CHILD") != "1" {
		return
	}
	root, stage := os.Getenv("ARDENTS_SIGNING_ROOT"), os.Getenv("ARDENTS_SIGNING_STAGE")
	var args []string
	if json.Unmarshal([]byte(os.Getenv("ARDENTS_SIGNING_ARGS")), &args) != nil || len(args) != 6 || args[5] != root {
		t.Fatal("original child declaration unavailable")
	}
	if stage == "file-limit" {
		if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &syscall.Rlimit{Cur: 16, Max: 16}); err != nil {
			t.Fatal(err)
		}
		_, err := executeObservedNetworkSigning(t.Context(), args, nil)
		if !errors.Is(err, syscall.EFBIG) {
			t.Fatal("actual partial write cause lost", err)
		}
		name := "node-record.bin"
		if args[0] == "sign-closed-profile" {
			name = "profile.bin"
		}
		saved, e := os.ReadFile(filepath.Join(root, name))
		if e != nil || len(saved) != 16 {
			t.Fatal("actual partial prefix", len(saved), e)
		}
		return
	}
	// The parent reads this actual barrier, kills and Waits the original child.
	// A pipe read holds the writer; no timer, path or PID observation grants join.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	_, err = executeObservedNetworkSigning(t.Context(), args, func(step string, _ *os.File) {
		if step == stage {
			if _, e := os.Stdout.Write([]byte("held\n")); e != nil {
				t.Fatal(e)
			}
			var b [1]byte
			_, _ = r.Read(b[:])
		}
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestNetworkSigningActualPartialAndProcessCuts(t *testing.T) {
	for name, arguments := range map[string]func(*testing.T) []string{"record": nativeSigningArguments, "profile": nativeProfileSigningArguments} {
		t.Run(name, func(t *testing.T) { testSigningActualPartialAndProcessCuts(t, arguments) })
	}
}

func testSigningActualPartialAndProcessCuts(t *testing.T, arguments func(*testing.T) []string) {
	for _, stage := range []string{"file-limit", "inputs-read", "signed", "directory-created", "file-created", "written", "closed", "before-directory-sync", "commit"} {
		t.Run(stage, func(t *testing.T) {
			args := arguments(t)
			root := args[5]
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestNetworkSigningInterruptedChild$", "-test.count=1")
			cmd.Env = append(os.Environ(), "ARDENTS_SIGNING_CHILD=1", "ARDENTS_SIGNING_ROOT="+root, "ARDENTS_SIGNING_STAGE="+stage, "ARDENTS_SIGNING_ARGS="+string(preparationJSON(t, args)))
			if stage == "file-limit" {
				if out, e := cmd.CombinedOutput(); e != nil {
					t.Fatal(e, string(out))
				}
			} else {
				pipe, err := cmd.StdoutPipe()
				if err != nil {
					t.Fatal(err)
				}
				var diagnostic bytes.Buffer
				cmd.Stderr = &diagnostic
				if err = cmd.Start(); err != nil {
					t.Fatal(err)
				}
				mark := make([]byte, 5)
				_, re := io.ReadFull(pipe, mark)
				ke := cmd.Process.Kill()
				we := cmd.Wait()
				if re != nil || string(mark) != "held\n" || ke != nil || we == nil {
					t.Fatal("original cut/join unavailable", re, ke, we, diagnostic.String())
				}
			}
			if stage == "inputs-read" || stage == "signed" {
				if _, e := os.Stat(root); !os.IsNotExist(e) {
					t.Fatal("pre-output cut created destination", e)
				}
				return
			}
			if _, e := os.Stat(root); e != nil {
				t.Fatal("original prefix missing", e)
			}
			if e := persistNetworkSigning(t.Context(), root, []networkSignedArtifact{{"node-record.bin", []byte("signed")}}, func() error { return nil }, nil); e == nil {
				t.Fatal("partial prefix adopted")
			}
		})
	}
}

func TestNetworkSigningStaticNativeRefusals(t *testing.T) {
	for _, fault := range []string{"symlink", "hardlink", "owner", "mode", "special mode", "type", "encoding", "wrong key", "oversize", "prepared pin", "QUIC", "stale", "future", "expired", "not yet live", "oversized interval", "fractional interval", "cancel"} {
		t.Run(fault, func(t *testing.T) {
			args := nativeSigningArguments(t)
			var e error
			switch fault {
			case "symlink":
				e = os.Rename(args[4], args[4]+"-original")
				if e == nil {
					e = os.Symlink(args[4]+"-original", args[4])
				}
			case "hardlink":
				e = os.Link(args[4], args[4]+"-other")
			case "owner":
				e = os.Chown(args[4], 65534, 65534)
			case "mode":
				e = os.Chmod(args[4], 0644)
			case "special mode":
				e = os.Chmod(args[4], 0600|os.ModeSetuid)
			case "type":
				e = os.Remove(args[4])
				if e == nil {
					e = os.Mkdir(args[4], 0700)
				}
			case "encoding":
				e = os.WriteFile(args[4], []byte("PRIVATE KEY must not be echoed"), 0600)
			case "wrong key":
				writeSigningTestKey(t, args[4], ed25519.NewKeyFromSeed(bytes.Repeat([]byte{98}, 32)))
			case "oversize":
				e = os.WriteFile(args[4], bytes.Repeat([]byte{1}, (16<<10)+1), 0600)
			case "prepared pin":
				args[3] = preparationIdentifier(99)
			case "QUIC":
				raw, err := os.ReadFile(args[1])
				if err != nil {
					t.Fatal(err)
				}
				raw = bytes.ReplaceAll(raw, []byte("ardents-carrier-tcp-tls-v2"), []byte("ardents-carrier-quic-v2"))
				e = os.WriteFile(args[1], raw, 0600)
			case "stale", "future", "expired", "not yet live", "oversized interval", "fractional interval":
				now := time.Now().UTC().Truncate(time.Second)
				from, until, ref := now.Add(-time.Minute), now.Add(time.Hour), now
				switch fault {
				case "stale":
					from, ref = now.Add(-10*time.Minute), now.Add(-6*time.Minute)
				case "future":
					ref = now.Add(2 * time.Minute)
				case "expired":
					from, until, ref = now.Add(-2*time.Minute), now.Add(-time.Second), now.Add(-30*time.Second)
				case "not yet live":
					from, ref = now.Add(40*time.Second), now.Add(50*time.Second)
				case "oversized interval":
					until = from.Add(7*24*time.Hour + time.Second)
				case "fractional interval":
					from = from.Add(100 * time.Millisecond)
				}
				changeSigningTestTime(t, args, from, until, ref, fault != "fractional interval")
			}
			if e != nil {
				t.Fatal(e)
			}
			ctx, cancel := context.WithCancel(t.Context())
			if fault == "cancel" {
				cancel()
			}
			defer cancel()
			var out bytes.Buffer
			if code := runNetworkSigning(ctx, args, &out); code == 0 || out.Len() != 0 {
				t.Fatal("static refusal accepted or leaked diagnostic", code)
			}
			if _, e = os.Lstat(args[5]); !os.IsNotExist(e) {
				t.Fatal("invalid input created output", e)
			}
			control := nativeSigningArguments(t)
			if _, e = executeNetworkSigning(t.Context(), control); e != nil {
				t.Fatal("healthy control", e)
			}
		})
	}
}
