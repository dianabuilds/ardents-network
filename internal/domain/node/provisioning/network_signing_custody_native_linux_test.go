//go:build linux && network_signing_native

package provisioning

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/node/provisioning/networkdraft"
	"github.com/dianabuilds/ardents-network/internal/domain/node/provisioning/networkpreparation"
	"github.com/dianabuilds/ardents-network/internal/domain/node/provisioning/networksigning"
	artifactio "github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	provisionfixture "github.com/dianabuilds/ardents-network/tests/networkfixture"
)

// Keep the certificate control on actual key creation, public preparation and
// signed Record publication. The separate CLI acceptance drives that chain
// through the compiled command.
func nativeCertificateArguments(t *testing.T) []string {
	t.Helper()
	parent := provisionfixture.ProtectedExt4Parent(t)
	keyPlan := filepath.Join(parent, "initial-keys")
	keyRoot := filepath.Join(parent, "keys")
	write := func(path string, payload []byte) {
		t.Helper()
		if err := os.WriteFile(path, payload, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(keyPlan, []byte(`{"schema":"ardents-initial-keys-plan-v1","operation":"prepare-initial-keys","node_count":1}`))
	if _, err := PrepareInitialNetworkKeys(t.Context(), keyPlan, keyRoot); err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(keyRoot, "node-00-key.pem")
	encoded, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(encoded)
	key, err := artifactio.ParseEd25519Key(encoded)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(key)
	recordPlan := provisionfixture.RecordPlan(key.Public().(ed25519.PublicKey))
	now := time.Now().UTC().Truncate(time.Second)
	recordPlan["valid_from"], recordPlan["valid_until"] = now.Add(-time.Minute).Format(time.RFC3339), now.Add(time.Hour).Format(time.RFC3339)
	publicPlan, prepared := filepath.Join(parent, "record-plan"), filepath.Join(parent, "prepared")
	write(publicPlan, provisionfixture.PlanJSON(t, recordPlan))
	artifact, err := networkpreparation.Prepare(t.Context(), "prepare-initial-node-record", publicPlan, prepared)
	if err != nil {
		t.Fatal(err)
	}
	declaration, signed := filepath.Join(parent, "signing-plan"), filepath.Join(parent, "signed")
	write(declaration, provisionfixture.SigningDeclaration(t, "node-record", recordPlan, now))
	if _, err := networksigning.Sign(t.Context(), []string{"sign-initial-node-record", declaration, filepath.Join(prepared, artifact.Name), hex.EncodeToString(artifact.Digest[:]), keyPath, signed}, nil); err != nil {
		t.Fatal(err)
	}
	record := filepath.Join(signed, "node-record.bin")
	raw, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	certificatePlan := filepath.Join(parent, "certificate-plan")
	write(certificatePlan, provisionfixture.CertificateDeclaration(t, recordPlan, now))
	return []string{"prepare-node-certificate", certificatePlan, record, hex.EncodeToString(digest[:]), keyPath, filepath.Join(parent, "certificate")}
}

func nativeSigningArguments(t *testing.T) []string {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	return timedNetworkSigningArguments(t, now, now.Add(time.Hour))
}

func TestNetworkSigningOriginalInputCustody(t *testing.T) {
	for name, arguments := range map[string]func(*testing.T) []string{"record": nativeSigningArguments, "profile": nativeProfileSigningArguments, "certificate": nativeCertificateArguments} {
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
			artifacts, err := networksigning.Sign(ctx, args, func(step string, _ *os.File) {
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
				if artifacts.Digest == [32]byte{} {
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
			if _, e := networksigning.Sign(t.Context(), control, nil); e != nil {
				t.Fatal("independent healthy control", e)
			}
		})
	}
}

func TestNetworkSigningOriginalRefusalCannotRevive(t *testing.T) {
	for name, arguments := range map[string]func(*testing.T) []string{"record": nativeSigningArguments, "profile": nativeProfileSigningArguments, "certificate": nativeCertificateArguments} {
		t.Run(name, func(t *testing.T) { testSigningOriginalRefusalCannotRevive(t, arguments) })
	}
}

func testSigningOriginalRefusalCannotRevive(t *testing.T, arguments func(*testing.T) []string) {
	args := arguments(t)
	input, err := artifactio.OpenProtectedInput(t.Context(), args[4], 16<<10, true)
	if err != nil {
		t.Fatal(err)
	}
	borrowed := input.Bytes()
	if err = os.Chmod(args[4], 0644); err != nil {
		t.Fatal(err)
	}
	if input.Check(t.Context()) == nil {
		t.Fatal("changed original access accepted")
	}
	if err = os.Chmod(args[4], 0600); err != nil {
		t.Fatal(err)
	}
	if input.Check(t.Context()) == nil {
		t.Fatal("restoration revived original private input")
	}
	if err = input.Close(); err != nil {
		t.Fatal(err)
	}
	for _, b := range borrowed {
		if b != 0 {
			t.Fatal("owned plaintext input not cleared")
		}
	}
	fresh, err := artifactio.OpenProtectedInput(t.Context(), args[4], 16<<10, true)
	if err != nil {
		t.Fatal("fresh independent input control", err)
	}
	if err = fresh.Close(); err != nil {
		t.Fatal(err)
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
		_, err := networksigning.Sign(t.Context(), args, nil)
		if !errors.Is(err, syscall.EFBIG) {
			t.Fatal("actual partial write cause lost", err)
		}
		name := "node-record.bin"
		if args[0] == "sign-closed-profile" {
			name = "profile.bin"
		}
		if args[0] == "prepare-node-certificate" {
			name = "node-certificate.pem"
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
	_, err = networksigning.Sign(t.Context(), args, func(step string, _ *os.File) {
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
	for name, arguments := range map[string]func(*testing.T) []string{"record": nativeSigningArguments, "profile": nativeProfileSigningArguments, "certificate": nativeCertificateArguments} {
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
			cmd.Env = append(os.Environ(), "ARDENTS_SIGNING_CHILD=1", "ARDENTS_SIGNING_ROOT="+root, "ARDENTS_SIGNING_STAGE="+stage, "ARDENTS_SIGNING_ARGS="+string(provisionfixture.PlanJSON(t, args)))
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
			if _, e := networksigning.Sign(t.Context(), args, nil); e == nil {
				t.Fatal("partial prefix adopted")
			}
		})
	}
}

func changeCertificateTestTime(t *testing.T, args []string, from, until, reference time.Time) {
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
	plan["record"] = provisionfixture.PlanJSON(t, record)
	plan["reference_time"] = provisionfixture.PlanJSON(t, reference.Format(time.RFC3339Nano))
	_, message, err := networkdraft.Prepare("prepare-initial-node-record", plan["record"])
	if err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(args[4])
	if err != nil {
		t.Fatal(err)
	}
	key, err := artifactio.ParseEd25519Key(raw)
	clear(raw)
	if err != nil {
		t.Fatal(err)
	}
	message = append(message, ed25519.Sign(key, message)...)
	clear(key)
	digest := sha256.Sum256(message)
	args[3] = hex.EncodeToString(digest[:])
	if err = os.WriteFile(args[2], message, 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(args[1], provisionfixture.PlanJSON(t, plan), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestNetworkSigningNodeCertificateLateTime(t *testing.T) {
	for _, stage := range []string{"inputs-read", "before-directory-sync"} {
		t.Run(stage, func(t *testing.T) {
			args := nativeCertificateArguments(t)
			now := time.Now().UTC().Truncate(time.Second)
			until := now.Add(3 * time.Second)
			changeCertificateTestTime(t, args, now.Add(-time.Minute), until, now)
			acted := false
			_, err := networksigning.Sign(t.Context(), args, func(step string, _ *os.File) {
				if step == stage {
					acted = true
					if delay := time.Until(until); delay > 0 {
						time.Sleep(delay + time.Millisecond)
					}
				}
			})
			if !acted || err == nil {
				t.Fatal("late original certificate accepted", err)
			}
			if stage == "inputs-read" {
				if _, err := os.Stat(args[5]); !os.IsNotExist(err) {
					t.Fatal("expired before crypto created output", err)
				}
			}
		})
	}
}

func TestNetworkSigningNodeCertificateFinalOutputAndPhysicalFailure(t *testing.T) {
	for _, fault := range []string{"bytes", "inventory", "canceled-close"} {
		t.Run(fault, func(t *testing.T) {
			args := nativeCertificateArguments(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			acted := false
			_, err := networksigning.Sign(ctx, args, func(step string, f *os.File) {
				if fault == "canceled-close" && step == "signed" {
					acted = true
					if e := f.Close(); e != nil {
						t.Fatal(e)
					}
					cancel()
				}
				if step == "commit" && fault != "canceled-close" {
					acted = true
					name := "node-certificate.pem"
					if fault == "inventory" {
						name = "foreign"
					}
					if e := os.WriteFile(filepath.Join(args[5], name), []byte("changed after original input IO"), 0600); e != nil {
						t.Fatal(e)
					}
				}
			})
			if !acted || err == nil {
				t.Fatal("late original failure accepted", err)
			}
			if fault == "canceled-close" && (!errors.Is(err, context.Canceled) || !errors.Is(err, os.ErrClosed)) {
				t.Fatal("independent physical/cancellation errors lost", err)
			}
		})
	}
}

func TestNetworkSigningNodeCertificateContention(t *testing.T) {
	args := nativeCertificateArguments(t)
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() { <-start; _, err := networksigning.Sign(t.Context(), args, nil); results <- err })
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
		t.Fatal("fresh output winners", success)
	}
}

func TestNetworkSigningOriginalCancellationBeforeEffects(t *testing.T) {
	for name, arguments := range map[string]func(*testing.T) []string{
		"record":      nativeSigningArguments,
		"certificate": nativeCertificateArguments,
	} {
		t.Run(name, func(t *testing.T) {
			args := arguments(t)
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			if _, err := networksigning.Sign(ctx, args, nil); !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled signer: %v", err)
			}
			if _, err := os.Lstat(args[5]); !os.IsNotExist(err) {
				t.Fatalf("canceled signer created output: %v", err)
			}
		})
	}
}
