//go:build linux && network_signing_native

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/forwarding"
)

func TestForwardingPreparationNativePlanInputs(t *testing.T) {
	parent := sourceArtifactNativeParent(t)
	command, _ := sourceNativePinnedProgram(t, "ARDENTS_SOURCE_CONTROL_PROGRAM")
	prefix := fmt.Sprintf("/var/lib/ardents-forwarding-plan-unopened-%d", os.Getpid())
	if _, err := os.Lstat(prefix); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("invalid environment: unopened declaration namespace required", err)
	}
	pin := func(first byte) string { value := [32]byte{first}; return hex.EncodeToString(value[:]) }
	for _, scenario := range []string{"healthy", "duplicate-option", "duplicate-authority", "zero-node", "wait-overflow", "wrong-output", "foreign-role", "input-loss"} {
		t.Run(scenario, func(t *testing.T) {
			base := filepath.Join(parent, scenario)
			if err := os.Mkdir(base, 0700); err != nil {
				t.Fatal(err)
			}
			requestOutput, planOutput := filepath.Join(base, "request"), filepath.Join(base, "plan")
			requestArgs := []string{"forwarding", "prepare-request", "--output", requestOutput}
			for _, option := range []string{"installation-root", "state-root", "participation-root", "spend-root", "hosting-root", "credential-directory", "certificate", "private-key", "clock"} {
				requestArgs = append(requestArgs, "--"+option, prefix+"/"+option)
			}
			requestArgs = append(requestArgs, "--plan", filepath.Join(planOutput, "forwarding-plan.json"))
			if output, err := exec.CommandContext(t.Context(), command, requestArgs...).CombinedOutput(); err != nil {
				t.Fatal("canonical request producer", string(output), err)
			}
			request := filepath.Join(requestOutput, "forwarding-request.json")
			args := []string{"prepare-plan", "--request", request, "--output", planOutput, "--network-id", pin(1), "--node-id", pin(4), "--profile-authority", pin(3), "--authority", pin(2), "--threshold", "1", "--work-tx", "40000000", "--work-rx", "40000000", "--termination-tx", "1024", "--termination-rx", "1024", "--retirement-wait-ms", "1000"}
			switch scenario {
			case "duplicate-option":
				args = append(args, "--threshold", "1")
			case "duplicate-authority":
				args = append(args, "--authority", pin(2))
			case "zero-node":
				args[8] = pin(0)
			case "wait-overflow":
				args[len(args)-1] = "4294967297"
			case "wrong-output":
				args[4] = filepath.Join(base, "foreign-output")
			case "foreign-role":
				args = append(args, "--role", "issuer")
			}
			if scenario == "input-loss" {
				seen := false
				raw, err := prepareForwardingArtifact(t.Context(), args, func(stage string, _ *os.File) {
					if stage == "before-file" && !seen {
						seen = true
						if err := os.WriteFile(request, []byte("changed original request\n"), 0600); err != nil {
							t.Error(err)
						}
					}
				})
				if !seen || err == nil || raw != nil {
					t.Fatal("lost original request published a plan", seen, err)
				}
				return
			}
			output, err := exec.CommandContext(t.Context(), command, append([]string{"forwarding"}, args...)...).CombinedOutput()
			if scenario != "healthy" {
				if err == nil {
					t.Fatal("foreign plan declaration accepted", string(output))
				}
				if _, err := os.Lstat(args[4]); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("refused plan created output", err)
				}
				return
			}
			if err != nil || !bytes.Contains(output, []byte(`"operation":"forwarding.prepare-plan"`)) {
				t.Fatal("canonical plan producer", string(output), err)
			}
			raw, err := os.ReadFile(filepath.Join(planOutput, "forwarding-plan.json"))
			if err != nil {
				t.Fatal(err)
			}
			p, err := forwarding.DecodePlan(raw)
			if err != nil || p.Network.NetworkID != [32]byte{1} || p.NodeID != [32]byte{4} || p.Network.ProfileAuthority != [32]byte{3} || len(p.Network.Authorities) != 1 || p.Network.Authorities[0] != [32]byte{2} || p.Network.Threshold != 1 || p.Work.Tx != 40000000 || p.Work.Rx != 40000000 || p.Termination.Tx != 1024 || p.Termination.Rx != 1024 || p.RetirementWaitMS != 1000 || p.Network.Root != prefix+"/state-root" || p.PrivateKey != prefix+"/credential-directory/private-key.pem" || !strings.HasSuffix(string(raw), "\n") {
				t.Fatal("public flags were not bound to the exact future inputs", err)
			}
		})
	}
	if _, err := os.Lstat(prefix); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("plan preparation opened a runtime namespace", err)
	}
}

func TestForwardingPreparationNativeRequestAndFixedTemplate(t *testing.T) {
	parent := sourceArtifactNativeParent(t)
	command, _ := sourceNativePinnedProgram(t, "ARDENTS_SOURCE_CONTROL_PROGRAM")
	prefix := fmt.Sprintf("/var/lib/forwarding-unopened-%d", os.Getpid())
	if _, err := os.Lstat(prefix); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("invalid environment: unopened declaration namespace required", err)
	}
	options := []string{"installation-root", "state-root", "participation-root", "spend-root", "hosting-root", "credential-directory", "plan", "certificate", "private-key", "clock"}
	args := []string{"forwarding", "prepare-request"}
	var paths []string
	for _, option := range options {
		path := prefix + "/" + option
		args = append(args, "--"+option, path)
		paths = append(paths, path)
	}
	destination := filepath.Join(parent, "request")
	args = append(args, "--output", destination)
	if output, err := exec.CommandContext(t.Context(), command, args...).CombinedOutput(); err != nil || !bytes.Contains(output, []byte(`"operation":"forwarding.prepare-request"`)) {
		t.Fatal("actual request preparation", string(output), err)
	}
	want := []byte(fmt.Sprintf("{\"schema\":\"ardents-forwarding-installation-v1\",\"invocation\":\"forwarding\",\"profile\":\"ardents-forwarding-process-v1\",\"installation_root\":%q,\"state_root\":%q,\"participation_root\":%q,\"spend_root\":%q,\"hosting_root\":%q,\"credential_directory\":%q,\"plan\":%q,\"certificate\":%q,\"private_key\":%q,\"clock\":%q}\n", paths[0], paths[1], paths[2], paths[3], paths[4], paths[5], paths[6], paths[7], paths[8], paths[9]))
	got, err := os.ReadFile(filepath.Join(destination, "forwarding-request.json"))
	if err != nil || !bytes.Equal(got, want) {
		t.Fatal("independent canonical request bytes", string(got), err)
	}
	if _, err := os.Lstat(prefix); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("preparation opened a future declared namespace", err)
	}
	bad := append([]string(nil), args...)
	bad[len(bad)-1] = filepath.Join(parent, "duplicate")
	bad = append(bad, "--clock", paths[9])
	if output, err := exec.CommandContext(t.Context(), command, bad...).CombinedOutput(); err == nil {
		t.Fatal("duplicate option accepted", string(output))
	}
	if _, err := os.Lstat(filepath.Join(parent, "duplicate")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("duplicate option created output", err)
	}
	templateOutput := filepath.Join(parent, "template")
	if output, err := exec.CommandContext(t.Context(), command, "forwarding", "prepare-template", templateOutput).CombinedOutput(); err != nil {
		t.Fatal("actual fixed template preparation", string(output), err)
	}
	template, err := os.ReadFile(filepath.Join(templateOutput, "ardents-forwarding.service"))
	if err != nil || !bytes.Equal(template, []byte(forwarding.Template)) {
		t.Fatal("fixed template bytes", err)
	}
}

// These offline controls prepare public bytes, not a signed/accepted candidate
// or installed execution. The existing checked native profile supplies root and
// original protected ext4 custody; absent prerequisites fail.
func TestForwardingPreparationNativeCanonicalCommand(t *testing.T) {
	parent := sourceArtifactNativeParent(t)
	command := os.Getenv("ARDENTS_SOURCE_CONTROL_PROGRAM")
	pin := os.Getenv("ARDENTS_SOURCE_CONTROL_PROGRAM_SHA256")
	compiled, err := os.ReadFile(command)
	sum := sha256.Sum256(compiled)
	if err != nil || !filepath.IsAbs(command) || len(pin) != 64 || hex.EncodeToString(sum[:]) != pin {
		t.Fatal("invalid environment: independently pinned canonical Control required", err)
	}
	program, template := []byte("public node program\n"), []byte("public forwarding template\n")
	for name, raw := range map[string][]byte{"program": program, "template": template} {
		if err := os.WriteFile(filepath.Join(parent, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	digest := func(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
	want := []byte("{\"schema\":\"ardents-infrastructure-artifact-v1\",\"platform\":\"linux-amd64\",\"release_identity\":\"one\",\"release_version\":1,\"invocation\":\"forwarding\",\"files\":{\"ardents-forwarding.service\":\"" + digest(template) + "\",\"ardents-node-linux-amd64\":\"" + digest(program) + "\"}}\n")
	outputDirectory := filepath.Join(parent, "descriptor")
	child := exec.CommandContext(t.Context(), command, "forwarding", "prepare-descriptor", "one", "1", filepath.Join(parent, "program"), filepath.Join(parent, "template"), outputDirectory)
	if output, err := child.CombinedOutput(); err != nil || !bytes.Contains(output, []byte(`"operation":"forwarding.prepare-descriptor"`)) {
		t.Fatal("canonical descriptor command", string(output), err)
	}
	got, err := os.ReadFile(filepath.Join(outputDirectory, "infrastructure-forwarding.json"))
	if err != nil || !bytes.Equal(got, want) {
		t.Fatal("independent descriptor bytes", string(got), err)
	}
	distribution := filepath.Join(parent, "distribution")
	if err := os.Mkdir(distribution, 0700); err != nil {
		t.Fatal(err)
	}
	// Deliberately unsigned metadata tests only immutable inventory preparation.
	files := map[string][]byte{"ardents-node-linux-amd64": program, "ardents-forwarding.service": template, "infrastructure-forwarding.json": want,
		"root.json": []byte("{}"), "timestamp.json": []byte("{}"), "snapshot.json": []byte("{}"), "targets.json": []byte("{}")}
	hashes := make(map[string]string, len(files))
	for name, raw := range files {
		if err := os.WriteFile(filepath.Join(distribution, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
		hashes[name] = digest(raw)
	}
	wantInventory, err := json.Marshal(struct {
		Schema     string            `json:"schema"`
		Platform   string            `json:"platform"`
		Release    string            `json:"release_identity"`
		Version    int               `json:"release_version"`
		Invocation string            `json:"invocation"`
		Files      map[string]string `json:"files"`
	}{"ardents-infrastructure-enrollment-v1", "linux-amd64", "one", 1, "forwarding", hashes})
	if err != nil {
		t.Fatal(err)
	}
	child = exec.CommandContext(t.Context(), command, "forwarding", "prepare-inventory", distribution, filepath.Join(parent, "inventory"))
	if output, err := child.CombinedOutput(); err != nil || !bytes.Contains(output, []byte(`"operation":"forwarding.prepare-inventory"`)) {
		t.Fatal("canonical inventory command", string(output), err)
	}
	got, err = os.ReadFile(filepath.Join(parent, "inventory", "infrastructure-enrollment.json"))
	if err != nil || !bytes.Equal(got, append(wantInventory, '\n')) {
		t.Fatal("independent inventory bytes", string(got), err)
	}
	// Source cannot consume these Forwarding resources; a foreign extra resource
	// also refuses without creating the requested output.
	for _, fault := range []string{"source-consumer", "extra-resource"} {
		args := []string{"forwarding", "prepare-inventory", distribution, filepath.Join(parent, fault)}
		if fault == "source-consumer" {
			args[0] = "source"
		} else if err := os.WriteFile(filepath.Join(distribution, "ardents-source.service"), []byte("foreign"), 0600); err != nil {
			t.Fatal(err)
		}
		if output, err := exec.CommandContext(t.Context(), command, args...).CombinedOutput(); err == nil {
			t.Fatal("foreign inventory accepted", fault, string(output))
		}
		if _, err := os.Lstat(args[3]); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("refusal created output", err)
		}
	}
}

func TestForwardingPreparationNativeRetainsOriginalFailures(t *testing.T) {
	for _, stage := range []string{"before-file", "written", "commit"} {
		t.Run(stage, func(t *testing.T) {
			parent := sourceArtifactNativeParent(t)
			program, template := filepath.Join(parent, "program"), filepath.Join(parent, "template")
			for _, path := range []string{program, template} {
				if err := os.WriteFile(path, []byte("public bytes"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			args := []string{"prepare-descriptor", "one", "1", program, template, filepath.Join(parent, "output")}
			seen := false
			raw, err := prepareForwardingArtifact(ctx, args, func(label string, file *os.File) {
				if label != stage || seen {
					return
				}
				seen = true
				if stage == "written" {
					cancel()
					if err := file.Close(); err != nil {
						t.Error(err)
					}
				} else if err := os.WriteFile(template, []byte("changed"), 0600); err != nil {
					t.Error(err)
				}
			})
			if !seen || err == nil || raw != nil {
				t.Fatal("original failure published", seen, err)
			}
			if stage == "written" && (!errors.Is(err, context.Canceled) || !errors.Is(err, os.ErrClosed)) {
				t.Fatal("original cancellation or physical failure lost", err)
			}
			if err := os.WriteFile(template, []byte("public bytes"), 0600); err != nil {
				t.Fatal(err)
			}
			if raw, err := prepareForwardingArtifact(t.Context(), args, nil); err == nil || raw != nil {
				t.Fatal("partial output adopted", err)
			}
		})
	}
}
