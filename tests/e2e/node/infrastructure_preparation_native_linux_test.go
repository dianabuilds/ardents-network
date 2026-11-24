//go:build linux && network_signing_native

package node_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	nodefixture "github.com/dianabuilds/ardents-network/tests/e2e/node"
	sourcefixture "github.com/dianabuilds/ardents-network/tests/e2e/source"
)

func TestNetworkSigningInfrastructurePreparation(t *testing.T) {
	parent := sourcefixture.SigningWorkspace(t)
	command := buildNodeCommand(t)
	binary, err := os.ReadFile(command)
	if err != nil {
		t.Fatal(err)
	}
	dir, _, files := nodefixture.InfrastructureDistribution(t, t.TempDir(), binary, nil, 1, nil, "source")
	template := filepath.Join(parent, "template")
	if err := os.WriteFile(template, files["ardents-source.service"], 0600); err != nil {
		t.Fatal(err)
	}
	descriptorOut := filepath.Join(parent, "descriptor")
	cmd := exec.CommandContext(t.Context(), command, "infrastructure", "prepare-runtime-descriptor", "source", "infrastructure-fixture", "1", command, template, descriptorOut)
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
	cmd = exec.CommandContext(t.Context(), command, "infrastructure", "prepare-runtime-inventory", "source", dir, inventoryOut)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("actual public inventory %s %v", out, err)
	}
	prepared, err := os.ReadFile(filepath.Join(inventoryOut, "infrastructure-enrollment.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, retired := range []struct {
		operation string
		input     []string
	}{
		{"prepare-descriptor", []string{"infrastructure-fixture", "1", command, template}},
		{"prepare-inventory", []string{dir}},
	} {
		destination := filepath.Join(parent, "retired-"+retired.operation)
		args := append(append([]string{"infrastructure", retired.operation}, retired.input...), destination)
		cmd := exec.CommandContext(t.Context(), command, args...)
		out, err := cmd.CombinedOutput()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 2 || len(out) != 0 {
			t.Fatalf("retired %s returned %q, %v", retired.operation, out, err)
		}
		if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("retired %s touched output: %v", retired.operation, err)
		}
	}
	oraclePin := nodefixture.WriteInfrastructureOracleInvocationInventory(t, dir, files, 1, "source")
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
	command := buildNodeCommand(t)
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
