//go:build linux && network_signing_native

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func sourcePreparationArguments(t *testing.T) (map[string]any, string, string) {
	t.Helper()
	parent := signingNativeParent(t)
	p, _ := sourcePreparationFixture(t, parent, installedHeadless{})
	plan, output := filepath.Join(parent, "plan.json"), filepath.Join(parent, "output")
	if e := os.WriteFile(plan, sourcePreparationJSON(t, p), 0600); e != nil {
		t.Fatal(e)
	}
	return p, plan, output
}

func TestSourcePreparationNativePublicOnlyCommand(t *testing.T) {
	p, plan, output := sourcePreparationArguments(t)
	// None of the three private declarations exists. Successful preparation
	// cannot rely on even opening them, let alone generating or copying keys.
	cmd := exec.CommandContext(t.Context(), compiledCommand(t), "source", "prepare-trust", plan, output)
	raw, err := cmd.Output()
	if err != nil {
		t.Fatal("genuine preparation command", err)
	}
	var receipt struct {
		Schema, Validation string
		Files              map[string]string
	}
	if json.Unmarshal(raw, &receipt) != nil || receipt.Schema != "ardents-source-trust-prepared-v1" || len(receipt.Files) != 10 {
		t.Fatal("public receipt inventory", string(raw))
	}
	entries, err := os.ReadDir(output)
	if err != nil || len(entries) != 10 {
		t.Fatal("finite inventory", err)
	}
	for _, path := range []string{p["client_private_key"].(string), p["sources"].([]map[string]any)[0]["private_key"].(string), p["sources"].([]map[string]any)[1]["private_key"].(string)} {
		if bytes.Contains(raw, []byte(path)) {
			t.Fatal("private path in receipt")
		}
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("private path effect", err)
		}
	}
	client := p["client"].(installedHeadless)
	roots := []string{client.NetworkStateRoot, client.LocalRoleStateRoot}
	for _, member := range p["sources"].([]map[string]any) {
		roots = append(roots, member["state_root"].(string), member["participation_root"].(string))
	}
	for _, root := range roots {
		if _, e := os.Lstat(root); !errors.Is(e, os.ErrNotExist) {
			t.Fatal("offline State/participation root effect", e)
		}
	}
	if runSourcePreparation(t.Context(), []string{"prepare-trust", plan, output}, io.Discard) == 0 {
		t.Fatal("adopted output")
	}
}

func TestSourcePreparationNativeCommitmentsAndCustody(t *testing.T) {
	for _, fault := range []string{"certificate-commitment", "root-commitment", "pin", "source-duplicate", "private-collision", "state-output-collision", "public-private-collision", "symlink", "hardlink", "access", "input-replaced", "input-changed", "ancestor-replaced", "cancel", "output-final-bytes", "output-final-extra", "output-close", "receipt"} {
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
			if e := os.WriteFile(plan, sourcePreparationJSON(t, p), 0600); e != nil {
				t.Fatal(e)
			}
			if fault == "receipt" {
				if runSourcePreparation(ctx, []string{"prepare-trust", plan, output}, &sourcePreparationFailWriter{}) == 0 {
					t.Fatal("receipt failure accepted")
				}
			} else if result, err := executeSourcePreparation(ctx, plan, output, observe); err == nil || result != nil {
				t.Fatal("causal refusal missing", fault, err)
			}
			// Every created prefix stays non-adoptable, including receipt failure.
			if _, e := os.Lstat(output); e == nil && runSourcePreparation(t.Context(), []string{"prepare-trust", plan, output}, io.Discard) == 0 {
				t.Fatal("failed prefix adopted")
			}
		})
	}
}

type sourcePreparationFailWriter struct{}

func (*sourcePreparationFailWriter) Write([]byte) (int, error) { return 0, syscall.EPIPE }

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
			if runSourcePreparation(t.Context(), []string{"prepare-trust", plan, output}, io.Discard) == 0 {
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
	if runSourcePreparation(t.Context(), []string{"prepare-trust", plan, output}, io.Discard) == 0 {
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
		if e := os.WriteFile(plan, sourcePreparationJSON(t, p), 0600); e != nil {
			t.Fatal(e)
		}
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
		if _, e := executeSourcePreparation(t.Context(), plan, output, observe); e == nil {
			t.Fatal("expired original reference accepted")
		}
		if late && !reached {
			t.Fatal("late original reference barrier not reached")
		}
		p["reference_time"] = time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
		if e := os.WriteFile(plan, sourcePreparationJSON(t, p), 0600); e != nil {
			t.Fatal(e)
		}
		if _, e := executeSourcePreparation(t.Context(), plan, output+"-healthy", nil); e != nil {
			t.Fatal("fresh observation control", e)
		}
	}
}

func TestSourcePreparationNativePublicMountRefusal(t *testing.T) {
	// Isolate the actual original-descriptor mount mechanism. The complete
	// command's separate ancestor checks must not turn a writable /dev/shm
	// ancestor into fabricated evidence that Fstatfs rejected this file.
	directory, e := os.MkdirTemp("/dev/shm", "ardents-source-preparation-")
	if e != nil {
		t.Fatal("invalid environment: real tmpfs control unavailable", e)
	}
	t.Cleanup(func() {
		if e := os.RemoveAll(directory); e != nil {
			t.Error(e)
		}
	})
	path := filepath.Join(directory, "public.pem")
	if e = os.WriteFile(path, []byte("public fixture"), 0600); e != nil {
		t.Fatal(e)
	}
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	var fs syscall.Statfs_t
	if e = syscall.Fstatfs(int(f.Fd()), &fs); e != nil || fs.Type != 0x01021994 {
		t.Fatal("invalid environment: original control is not tmpfs", e)
	}
	if sourcePreparationMount(&signingInput{file: f}) == nil {
		t.Fatal("actual non-ext4 public descriptor accepted")
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
		_, e := executeSourcePreparation(t.Context(), plan, output, nil)
		if !errors.Is(e, syscall.EFBIG) {
			t.Fatal("actual partial-write failure absent", e)
		}
		return
	}
	_, e := executeSourcePreparation(t.Context(), plan, output, func(label string, _ *os.File) {
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
