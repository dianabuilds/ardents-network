//go:build linux && network_signing_native

package infrastructure

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

func TestNetworkSigningInfrastructureLateInventoryMutation(t *testing.T) {
	parent := infrastructureNativeWorkspace(t)
	dir := infrastructureInventoryFixture(t, parent)
	extra := filepath.Join(dir, "extra")
	raw, err := executeInfrastructurePreparation(t.Context(), []string{"prepare-runtime-inventory", "source", dir, filepath.Join(parent, "late-inventory")}, func(stage string, _ *os.File) {
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
}

func TestNetworkSigningInfrastructureRefusesNonExt4Input(t *testing.T) {
	destination := filepath.Join(infrastructureNativeWorkspace(t), "output")
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
			parent := infrastructureNativeWorkspace(t)
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
			parent := infrastructureNativeWorkspace(t)
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

func executeInfrastructurePreparation(ctx context.Context, args []string, observe func(string, *os.File)) ([]byte, error) {
	if len(args) < 2 {
		return nil, ErrInput
	}
	operation, role, values := args[0], args[1], args[2:]
	switch operation {
	case "prepare-runtime-descriptor":
		if len(values) != 5 {
			return nil, ErrInput
		}
		version, err := strconv.ParseInt(values[1], 10, 64)
		if err != nil || strconv.FormatInt(version, 10) != values[1] {
			return nil, ErrInput
		}
		return PrepareRuntimeDescriptorFile(ctx, role, values[0], version, values[2], values[3], values[4], observe)
	case "prepare-runtime-inventory":
		if len(values) != 2 {
			return nil, ErrInput
		}
		return PrepareRuntimeInventoryFile(ctx, role, values[0], values[1], observe)
	default:
		return nil, ErrInput
	}
}

func infrastructureNativeWorkspace(t *testing.T) string {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Fatal("invalid environment: Linux root required")
	}
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	chain, err := artifacts.OpenDirectoryChain(t.Context(), parent, nil)
	if err != nil {
		t.Fatal("invalid environment", err)
	}
	if err := errors.Join(chain.CheckRootAccess(), chain.Close()); err != nil {
		t.Fatal("invalid environment: root-controlled ancestors required", err)
	}
	var fs syscall.Statfs_t
	if err := syscall.Statfs(parent, &fs); err != nil || fs.Type != 0xef53 {
		t.Fatal("invalid environment: native ext4 required", err)
	}
	return parent
}

func infrastructureInventoryFixture(t *testing.T, parent string) string {
	t.Helper()
	dir := filepath.Join(parent, "distribution")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	program, unit := filepath.Join(dir, "ardents-node-linux-amd64"), filepath.Join(dir, "ardents-source.service")
	for path, value := range map[string][]byte{program: []byte("public program"), unit: []byte("public template")} {
		if err := os.WriteFile(path, value, 0600); err != nil {
			t.Fatal(err)
		}
	}
	output := filepath.Join(parent, "descriptor")
	if _, err := PrepareRuntimeDescriptorFile(t.Context(), "source", "one", 1, program, unit, output, nil); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(output, "infrastructure-source.json"))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{"infrastructure-source.json": raw, "root.json": []byte("{}"), "timestamp.json": []byte("{}"), "snapshot.json": []byte("{}"), "targets.json": []byte("{}")}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}
