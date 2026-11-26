//go:build linux && network_signing_native

package endpoint

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	sourcefixture "github.com/dianabuilds/ardents-network/tests/e2e/source"
)

func sourcePreparationArguments(t *testing.T) (map[string]any, string, string) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Fatal("invalid environment: Linux root required")
	}
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	ancestors, err := artifacts.OpenDirectoryChain(t.Context(), parent, nil)
	if err != nil {
		t.Fatal("invalid environment", err)
	}
	if err := errors.Join(ancestors.CheckRootAccess(), ancestors.Close()); err != nil {
		t.Fatal("invalid environment: root-controlled direct ancestors required", err)
	}
	var fs syscall.Statfs_t
	if err := syscall.Statfs(parent, &fs); err != nil || fs.Type != 0xef53 {
		t.Fatal("invalid environment: native ext4 required", err)
	}
	pub := ed25519.NewKeyFromSeed(make([]byte, 32)).Public().(ed25519.PublicKey)
	client := HeadlessPlan{
		Schema: "ardents-headless-runtime-v2", NetworkStateRoot: filepath.Join(parent, "client-state"),
		LocalRoleStateRoot: filepath.Join(parent, "client-participation"), TimeConfidenceFile: filepath.Join(parent, "clock"),
		NetworkID: hex.EncodeToString(append([]byte{8}, make([]byte, 31)...)), NetworkThreshold: 1,
		NetworkAuthorities: []string{hex.EncodeToString(pub)}, ClosedProfileAuthority: hex.EncodeToString(pub),
	}
	p, _ := sourcefixture.TrustPreparationFixture(t, parent, client)
	plan, output := filepath.Join(parent, "plan.json"), filepath.Join(parent, "output")
	writeSourcePreparationPlan(t, plan, p)
	return p, plan, output
}

func sourcePrefixAdopted(t *testing.T, plan, output string) bool {
	t.Helper()
	_, err := PrepareSourceTrust(t.Context(), plan, output, nil)
	return err == nil
}

func writeSourcePreparationPlan(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestSourcePreparationNativeCommitmentsAndCustody(t *testing.T) {
	for _, fault := range []string{"certificate-commitment", "root-commitment", "pin", "source-duplicate", "private-collision", "state-output-collision", "public-private-collision", "symlink", "hardlink", "access", "input-replaced", "input-changed", "ancestor-replaced", "cancel", "output-final-bytes", "output-final-extra", "output-close"} {
		t.Run(fault, func(t *testing.T) {
			p, plan, output := sourcePreparationArguments(t)
			member := p["sources"].([]map[string]any)[0]
			public := member["certificate"].(map[string]any)["path"].(string)
			var observe func(string, *os.File)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			switch fault {
			case "certificate-commitment":
				member["certificate"].(map[string]any)["sha256"] = p["client_leaf_key_digest"]
			case "root-commitment":
				member["root"].(map[string]any)["sha256"] = p["client_leaf_key_digest"]
			case "pin":
				member["leaf_key_digest"] = p["client_leaf_key_digest"]
			case "source-duplicate":
				p["sources"].([]map[string]any)[1]["family"] = member["family"]
			case "private-collision":
				p["client_private_key"] = member["private_key"]
			case "state-output-collision":
				member["state_root"] = output
			case "public-private-collision":
				p["client_private_key"] = public
			case "symlink":
				if e := os.Rename(public, public+".old"); e != nil {
					t.Fatal(e)
				}
				if e := os.Symlink(public+".old", public); e != nil {
					t.Fatal(e)
				}
			case "hardlink":
				if e := os.Link(public, public+".link"); e != nil {
					t.Fatal(e)
				}
			case "access":
				if e := os.Chmod(public, 0666); e != nil {
					t.Fatal(e)
				}
			case "input-replaced", "input-changed", "ancestor-replaced", "cancel":
				observe = func(label string, f *os.File) {
					if label != "inputs-read" {
						return
					}
					switch fault {
					case "input-replaced":
						raw, e := os.ReadFile(public)
						if e != nil {
							t.Fatal(e)
						}
						if e = os.Rename(public, public+".old"); e != nil {
							t.Fatal(e)
						}
						if e = os.WriteFile(public, raw, 0600); e != nil {
							t.Fatal(e)
						}
					case "input-changed":
						if e := os.WriteFile(public, []byte("changed"), 0600); e != nil {
							t.Fatal(e)
						}
					case "ancestor-replaced":
						parent := filepath.Dir(plan)
						if e := os.Rename(parent, parent+".old"); e != nil {
							t.Fatal(e)
						}
						if e := os.Mkdir(parent, 0700); e != nil {
							t.Fatal(e)
						}
					case "cancel":
						cancel()
					}
				}
			case "output-final-bytes", "output-final-extra":
				observe = func(label string, f *os.File) {
					if label != "commit" {
						return
					}
					name := filepath.Join(output, "client-headless.json")
					if fault == "output-final-extra" {
						name = filepath.Join(output, "extra")
					}
					if e := os.WriteFile(name, []byte("changed"), 0600); e != nil {
						t.Fatal(e)
					}
				}
			case "output-close":
				observe = func(label string, f *os.File) {
					if label == "written" {
						if e := f.Close(); e != nil {
							t.Fatal(e)
						}
					}
				}
			}
			writeSourcePreparationPlan(t, plan, p)
			if result, err := PrepareSourceTrust(ctx, plan, output, observe); err == nil || result != nil {
				t.Fatal("causal refusal missing", fault, err)
			}
			if _, err := os.Lstat(output); err == nil {
				if sourcePrefixAdopted(t, plan, output) {
					t.Fatal("failed prefix adopted")
				}
			}
		})
	}
}

func TestSourcePreparationNativeOriginalProcessCuts(t *testing.T) {
	for _, phase := range []string{"directory-created", "file-created", "written", "closed", "directory-sync", "commit"} {
		t.Run(phase, func(t *testing.T) {
			_, plan, output := sourcePreparationArguments(t)
			command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestSourcePreparationNativeChild$", "-test.timeout=20s")
			command.Env = append(os.Environ(), "ARDENTS_SOURCE_PREP_CHILD="+phase, "ARDENTS_SOURCE_PREP_PLAN="+plan, "ARDENTS_SOURCE_PREP_OUTPUT="+output)
			stdout, e := command.StdoutPipe()
			if e != nil {
				t.Fatal(e)
			}
			if e = command.Start(); e != nil {
				t.Fatal(e)
			}
			joined := false
			t.Cleanup(func() {
				if !joined {
					_ = command.Process.Kill()
					_ = command.Wait()
				}
				_ = stdout.Close()
			})
			line, e := bufio.NewReader(stdout).ReadString('\n')
			if e != nil || line != phase+"\n" {
				t.Fatal("original child barrier", line, e)
			}
			if e = command.Process.Kill(); e != nil {
				t.Fatal(e)
			}
			e = command.Wait()
			joined = true
			if e == nil {
				t.Fatal("killed child reported success")
			}
			if _, e = os.Stat(output); e != nil {
				t.Fatal("prefix removed", e)
			}
			if sourcePrefixAdopted(t, plan, output) {
				t.Fatal("killed original prefix adopted")
			}
		})
	}
}

func TestSourcePreparationNativePartialWrite(t *testing.T) {
	_, plan, output := sourcePreparationArguments(t)
	command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestSourcePreparationNativeChild$", "-test.timeout=20s")
	command.Env = append(os.Environ(), "ARDENTS_SOURCE_PREP_CHILD=partial", "ARDENTS_SOURCE_PREP_PLAN="+plan, "ARDENTS_SOURCE_PREP_OUTPUT="+output)
	if raw, e := command.CombinedOutput(); e != nil {
		t.Fatal("real partial-write control", e, string(raw))
	}
	if sourcePrefixAdopted(t, plan, output) {
		t.Fatal("partial output adopted")
	}
}

func TestSourcePreparationNativeReferenceExpiry(t *testing.T) {
	for _, late := range []bool{false, true} {
		p, plan, output := sourcePreparationArguments(t)
		reference := time.Now().UTC().Truncate(time.Second).Add(-6 * time.Minute)
		if late {
			reference = time.Now().UTC().Truncate(time.Second).Add(-4*time.Minute - 58*time.Second)
		}
		p["reference_time"] = reference.Format(time.RFC3339)
		writeSourcePreparationPlan(t, plan, p)
		reached := false
		var observe func(string, *os.File)
		if late {
			observe = func(label string, _ *os.File) {
				if label == "inputs-read" {
					reached = true
					time.Sleep(2200 * time.Millisecond)
				}
			}
		}
		if _, e := PrepareSourceTrust(t.Context(), plan, output, observe); e == nil {
			t.Fatal("expired original reference accepted")
		}
		if late && !reached {
			t.Fatal("late original reference barrier not reached")
		}
		p["reference_time"] = time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
		writeSourcePreparationPlan(t, plan, p)
		if _, e := PrepareSourceTrust(t.Context(), plan, output+"-healthy", nil); e != nil {
			t.Fatal("fresh observation control", e)
		}
	}
}

func TestSourcePreparationNativeChild(t *testing.T) {
	phase := os.Getenv("ARDENTS_SOURCE_PREP_CHILD")
	if phase == "" {
		return
	}
	plan, output := os.Getenv("ARDENTS_SOURCE_PREP_PLAN"), os.Getenv("ARDENTS_SOURCE_PREP_OUTPUT")
	if phase == "partial" {
		if e := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &syscall.Rlimit{Cur: 32, Max: 32}); e != nil {
			t.Fatal("invalid environment", e)
		}
		_, e := PrepareSourceTrust(t.Context(), plan, output, nil)
		if !errors.Is(e, syscall.EFBIG) {
			t.Fatal("actual partial-write failure absent", e)
		}
		return
	}
	_, e := PrepareSourceTrust(t.Context(), plan, output, func(label string, _ *os.File) {
		if label == phase {
			if _, e := fmt.Fprintln(os.Stdout, label); e != nil {
				t.Fatal(e)
			}
			for {
				time.Sleep(time.Hour)
			}
		}
	})
	t.Fatal("original physical cut not reached", e)
}
