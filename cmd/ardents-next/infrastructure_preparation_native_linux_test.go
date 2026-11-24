//go:build linux && network_signing_native

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	sourcefixture "github.com/dianabuilds/ardents-network/tests/e2e/source"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	nodefixture "github.com/dianabuilds/ardents-network/tests/e2e/node"
)

func TestNetworkSigningInfrastructurePreparation(t *testing.T) {
	parent := sourcefixture.SigningWorkspace(t)
	binary, err := os.ReadFile(compiledCommand(t))
	if err != nil {
		t.Fatal(err)
	}
	dir, _, files := infrastructureDistribution(t, binary, nil, 1, nil)
	template := filepath.Join(parent, "template")
	if err := os.WriteFile(template, files["ardents-source.service"], 0600); err != nil {
		t.Fatal(err)
	}
	descriptorOut := filepath.Join(parent, "descriptor")
	cmd := exec.CommandContext(t.Context(), compiledCommand(t), "infrastructure", "prepare-runtime-descriptor", "source", "infrastructure-fixture", "1", compiledCommand(t), template, descriptorOut)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("actual public descriptor %s %v", out, err)
	}
	raw, err := os.ReadFile(filepath.Join(descriptorOut, "infrastructure-source.json"))
	if err != nil || !bytes.Equal(raw, files["infrastructure-source.json"]) {
		t.Fatal("independent descriptor oracle", err)
	}
	if err := os.Remove(filepath.Join(dir, "infrastructure-enrollment.json")); err != nil {
		t.Fatal(err)
	}
	inventoryOut := filepath.Join(parent, "inventory")
	cmd = exec.CommandContext(t.Context(), compiledCommand(t), "infrastructure", "prepare-runtime-inventory", "source", dir, inventoryOut)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("actual public inventory %s %v", out, err)
	}
	prepared, err := os.ReadFile(filepath.Join(inventoryOut, "infrastructure-enrollment.json"))
	if err != nil {
		t.Fatal(err)
	}
	extra := filepath.Join(dir, "extra")
	raw, err = executeInfrastructurePreparation(t.Context(), []string{"prepare-runtime-inventory", "source", dir, filepath.Join(parent, "late-inventory")}, func(stage string, _ *os.File) {
		if stage == "before-directory-sync" {
			if err := os.WriteFile(extra, []byte("added after readback"), 0600); err != nil {
				t.Fatal(err)
			}
		}
	})
	if raw != nil || err == nil {
		t.Fatal("late inventory addition published", err)
	}
	if err := os.Remove(extra); err != nil {
		t.Fatal(err)
	}
	for _, retired := range []struct {
		operation string
		input     []string
	}{
		{"prepare-descriptor", []string{"infrastructure-fixture", "1", compiledCommand(t), template}},
		{"prepare-inventory", []string{dir}},
	} {
		destination := filepath.Join(parent, "retired-"+retired.operation)
		args := append(append([]string{"infrastructure", retired.operation}, retired.input...), destination)
		cmd := exec.CommandContext(t.Context(), compiledCommand(t), args...)
		out, err := cmd.CombinedOutput()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 2 || len(out) != 0 {
			t.Fatalf("retired %s returned %q, %v", retired.operation, out, err)
		}
		if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("retired %s touched output: %v", retired.operation, err)
		}
	}
	oraclePin := writeInfrastructureOracleInventory(t, dir, files, 1)
	if fmt.Sprintf("%x", sha256.Sum256(prepared)) != oraclePin {
		t.Fatal("independent inventory oracle")
	}
	// The independently prepared pin here is a controlled test input, not an
	// operational independent distribution or permission for first execution.
	cmd = exec.CommandContext(t.Context(), filepath.Join(dir, "ardents-node-linux-amd64"), "infrastructure", "authenticate-initial", dir, oraclePin, filepath.Join(parent, "history"), "h3-test", "new-release-test", nodefixture.InfrastructureReference().Format("2006-01-02T15:04:05Z07:00"))
	if out, err := cmd.CombinedOutput(); err != nil || !bytes.Contains(out, []byte("authenticated-source-artifact")) {
		t.Fatalf("prepared actual self and Release pair %s %v", out, err)
	}
}

func TestNetworkSigningInfrastructureRoleInventories(t *testing.T) {
	command := compiledCommand(t)
	digest := func(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }
	for _, role := range []string{"source", "forwarding", "issuer", "descriptor", "introduction", "join"} {
		t.Run(role, func(t *testing.T) {
			parent := sourcefixture.SigningWorkspace(t)
			write := func(path string, contents []byte) {
				t.Helper()
				if err := os.WriteFile(path, contents, 0600); err != nil {
					t.Fatal(err)
				}
			}
			run := func(args ...string) ([]byte, error) {
				return exec.CommandContext(t.Context(), command, append([]string{"infrastructure"}, args...)...).CombinedOutput()
			}
			program, template := []byte("public program\n"), []byte("public template\n")
			programPath, templatePath := filepath.Join(parent, "program"), filepath.Join(parent, "template")
			write(programPath, program)
			write(templatePath, template)
			destination := filepath.Join(parent, "descriptor")
			if out, err := run("prepare-runtime-descriptor", role, "one", "1", programPath, templatePath, destination); err != nil {
				t.Fatalf("public descriptor: %v %s", err, out)
			}
			unitName, descriptorName := "ardents-"+role+".service", "infrastructure-"+role+".json"
			resources, err := json.Marshal(map[string]string{"ardents-node-linux-amd64": digest(program), unitName: digest(template)})
			if err != nil {
				t.Fatal(err)
			}
			wantDescriptor := fmt.Sprintf("{\"schema\":\"ardents-infrastructure-artifact-v1\",\"platform\":\"linux-amd64\",\"release_identity\":\"one\",\"release_version\":1,\"invocation\":%q,\"files\":%s}\n", role, resources)
			descriptor, err := os.ReadFile(filepath.Join(destination, descriptorName))
			if err != nil || string(descriptor) != wantDescriptor {
				t.Fatalf("descriptor bytes: %v %q", err, descriptor)
			}
			distribution := filepath.Join(parent, "distribution")
			if err := os.Mkdir(distribution, 0700); err != nil {
				t.Fatal(err)
			}
			files := map[string][]byte{
				"ardents-node-linux-amd64": program, unitName: template, descriptorName: descriptor,
				"root.json": []byte("{}"), "timestamp.json": []byte("{}"), "snapshot.json": []byte("{}"), "targets.json": []byte("{}"),
			}
			digests := make(map[string]string, len(files))
			for name, raw := range files {
				write(filepath.Join(distribution, name), raw)
				digests[name] = digest(raw)
			}
			inventory := filepath.Join(parent, "inventory")
			if out, err := run("prepare-runtime-inventory", role, distribution, inventory); err != nil {
				t.Fatalf("public inventory: %v %s", err, out)
			}
			hashes, err := json.Marshal(digests)
			if err != nil {
				t.Fatal(err)
			}
			wantInventory := fmt.Sprintf("{\"schema\":\"ardents-infrastructure-enrollment-v1\",\"platform\":\"linux-amd64\",\"release_identity\":\"one\",\"release_version\":1,\"invocation\":%q,\"files\":%s}\n", role, hashes)
			got, err := os.ReadFile(filepath.Join(inventory, "infrastructure-enrollment.json"))
			if err != nil || string(got) != wantInventory {
				t.Fatalf("inventory bytes: %v %q", err, got)
			}
			for _, fault := range []struct {
				name, file string
				content    []byte
			}{
				{"extra", "unlisted", []byte("foreign")},
				{"program", "ardents-node-linux-amd64", []byte("different program")},
				{"unit", unitName, []byte("different template")},
				{"role", descriptorName, bytes.Replace(descriptor, []byte(fmt.Sprintf("\"invocation\":%q", role)), []byte(`"invocation":"foreign"`), 1)},
			} {
				t.Run(fault.name, func(t *testing.T) {
					path := filepath.Join(distribution, fault.file)
					write(path, fault.content)
					t.Cleanup(func() {
						if original, existed := files[fault.file]; existed {
							write(path, original)
						} else if err := os.Remove(path); err != nil {
							t.Error(err)
						}
					})
					destination := filepath.Join(parent, "refused-"+fault.name)
					if out, err := run("prepare-runtime-inventory", role, distribution, destination); err == nil || bytes.Contains(out, []byte(`"sha256"`)) {
						t.Fatalf("invalid inventory produced a receipt: %v %s", err, out)
					}
					if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("invalid inventory created output: %v", err)
					}
				})
			}
		})
	}
}

func TestNetworkSigningInfrastructureRefusesNonExt4Input(t *testing.T) {
	destination := filepath.Join(sourcefixture.SigningWorkspace(t), "output")
	parent, err := os.MkdirTemp("/run", "ardents-public-input-")
	if err != nil {
		t.Fatalf("invalid environment: protected /run fixture: %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(parent); err != nil {
			t.Error(err)
		}
	})
	path := filepath.Join(parent, "public")
	if err := os.WriteFile(path, []byte("public bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	raw, err := executeInfrastructurePreparation(t.Context(), []string{"prepare-runtime-descriptor", "source", "one", "1", path, path, destination}, nil)
	if !errors.Is(err, artifacts.ErrNotExt4) || raw != nil {
		t.Fatalf("expected native filesystem refusal, got %q %v", raw, err)
	}
	if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("filesystem refusal created output: %v", err)
	}
}

func TestNetworkSigningInfrastructureReobservesOutputMetadata(t *testing.T) {
	for _, fault := range []string{"group", "modification-time"} {
		t.Run(fault, func(t *testing.T) {
			parent := sourcefixture.SigningWorkspace(t)
			input := filepath.Join(parent, "public")
			if err := os.WriteFile(input, []byte("public bytes"), 0600); err != nil {
				t.Fatal(err)
			}
			destination := filepath.Join(parent, "descriptor")
			path := filepath.Join(destination, "infrastructure-source.json")
			changed := false
			raw, err := executeInfrastructurePreparation(t.Context(), []string{"prepare-runtime-descriptor", "source", "one", "1", input, input, destination}, func(stage string, _ *os.File) {
				if stage != "closed" {
					return
				}
				var err error
				if fault == "group" {
					err = os.Chown(path, 0, 1)
				} else {
					then := time.Unix(1_700_000_000, 0)
					err = os.Chtimes(path, then, then)
				}
				if err != nil {
					t.Fatal(err)
				}
				changed = true
			})
			if !changed || raw != nil || err == nil {
				t.Fatalf("changed output metadata published: changed=%v err=%v", changed, err)
			}
		})
	}
}

func TestNetworkSigningInfrastructureOriginalCustodyRefusesMutationAndLateClose(t *testing.T) {
	for _, stage := range []string{"before-file", "written", "commit"} {
		t.Run(stage, func(t *testing.T) {
			parent := sourcefixture.SigningWorkspace(t)
			program, template := filepath.Join(parent, "program"), filepath.Join(parent, "template")
			if err := os.WriteFile(program, []byte("public program"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(template, []byte("public template"), 0600); err != nil {
				t.Fatal(err)
			}
			descriptorArgs := []string{"prepare-runtime-descriptor", "source", "one", "1", program, template}
			args := append(descriptorArgs, filepath.Join(parent, "output"))
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			observed := false
			raw, err := executeInfrastructurePreparation(ctx, args, func(label string, f *os.File) {
				if label != stage || observed {
					return
				}
				observed = true
				if stage == "written" {
					cancel()
					if err := f.Close(); err != nil {
						t.Error(err)
					}
					return
				}
				if err := os.WriteFile(template, []byte("changed resource"), 0600); err != nil {
					t.Error(err)
				}
			})
			if !observed || err == nil || raw != nil {
				t.Fatal("original custody loss published", observed, err)
			}
			if stage == "written" && (!strings.Contains(err.Error(), "closed") || !strings.Contains(err.Error(), "canceled")) {
				t.Fatal("independent cancellation/physical Close lost", err)
			}
			// Restored bytes cannot adopt partial original output. A fresh output
			// with independently re-admitted inputs supplies the healthy counterpart.
			if err := os.WriteFile(template, []byte("public template"), 0600); err != nil {
				t.Fatal(err)
			}
			if raw, err := executeInfrastructurePreparation(t.Context(), args, nil); err == nil || raw != nil {
				t.Fatal("partial output adopted")
			}
			freshArgs := append(descriptorArgs, filepath.Join(parent, "fresh"))
			if raw, err := executeInfrastructurePreparation(t.Context(), freshArgs, nil); err != nil || len(raw) == 0 {
				t.Fatal("fresh healthy counterpart", err)
			}
		})
	}
}
