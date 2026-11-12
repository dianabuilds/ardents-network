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
	"strconv"
	"strings"
	"testing"
)

// This profile proves offline byte/output custody, not signatures, JOIN startup
// or runtime. Public fixture metadata intentionally supplies no Release proof.
func TestJoinPreparationNativeCanonicalBytesAndInventory(t *testing.T) {
	parent := sourceArtifactNativeParent(t)
	command := os.Getenv("ARDENTS_JOIN_CONTROL_PROGRAM")
	pin := os.Getenv("ARDENTS_JOIN_CONTROL_PROGRAM_SHA256")
	compiled, err := os.ReadFile(command)
	actualPin := sha256.Sum256(compiled)
	if err != nil || !filepath.IsAbs(command) || len(pin) != 64 || hex.EncodeToString(actualPin[:]) != pin {
		t.Fatal("invalid environment: independently pinned canonical Control required", err)
	}
	program, template := []byte("public JOIN program\n"), []byte("public JOIN template\n")
	for name, raw := range map[string][]byte{"program": program, "template": template} {
		if err := os.WriteFile(filepath.Join(parent, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	digest := func(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
	expected := []byte("{\"schema\":\"ardents-infrastructure-artifact-v1\",\"platform\":\"linux-amd64\",\"release_identity\":\"one\",\"release_version\":1,\"invocation\":\"join\",\"files\":{\"ardents-join.service\":\"" + digest(template) + "\",\"ardents-node-linux-amd64\":\"" + digest(program) + "\"}}\n")
	call := func(args ...string) []byte {
		t.Helper()
		output, err := exec.CommandContext(t.Context(), command, append([]string{"join"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatal("actual canonical JOIN preparation", string(output), err)
		}
		return output
	}
	out := filepath.Join(parent, "descriptor")
	if output := call("prepare-descriptor", "one", "1", filepath.Join(parent, "program"), filepath.Join(parent, "template"), out); !bytes.Contains(output, []byte(`"operation":"join.prepare-descriptor"`)) {
		t.Fatal("JOIN descriptor receipt", string(output))
	}
	onDisk, err := os.ReadFile(filepath.Join(out, "infrastructure-join.json"))
	if err != nil || !bytes.Equal(onDisk, expected) {
		t.Fatal("independent durable JOIN descriptor", string(onDisk), err)
	}
	distribution := filepath.Join(parent, "distribution")
	if err := os.Mkdir(distribution, 0700); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{"ardents-node-linux-amd64": program, "ardents-join.service": template, "infrastructure-join.json": expected,
		"root.json": []byte("{}"), "timestamp.json": []byte("{}"), "snapshot.json": []byte("{}"), "targets.json": []byte("{}")}
	hashes := make(map[string]string, len(files))
	for name, raw := range files {
		if err := os.WriteFile(filepath.Join(distribution, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
		hashes[name] = digest(raw)
	}
	expectedInventory, err := json.Marshal(struct {
		Schema     string            `json:"schema"`
		Platform   string            `json:"platform"`
		Release    string            `json:"release_identity"`
		Version    int               `json:"release_version"`
		Invocation string            `json:"invocation"`
		Files      map[string]string `json:"files"`
	}{"ardents-infrastructure-enrollment-v1", "linux-amd64", "one", 1, "join", hashes})
	if err != nil {
		t.Fatal(err)
	}
	if output := call("prepare-inventory", distribution, filepath.Join(parent, "inventory")); !bytes.Contains(output, []byte(`"operation":"join.prepare-inventory"`)) {
		t.Fatal("JOIN inventory receipt", string(output))
	}
	actual, err := os.ReadFile(filepath.Join(parent, "inventory", "infrastructure-enrollment.json"))
	if err != nil || !bytes.Equal(actual, append(expectedInventory, '\n')) {
		t.Fatal("independent durable JOIN inventory", string(actual), err)
	}
	for _, fault := range []string{"extra", "foreign-unit", "changed-program", "foreign-role"} {
		t.Run(fault, func(t *testing.T) {
			candidate := filepath.Join(parent, fault)
			if err := os.Mkdir(candidate, 0700); err != nil {
				t.Fatal(err)
			}
			for name, raw := range files {
				if fault == "foreign-unit" && name == "ardents-join.service" {
					name = "ardents-forwarding.service"
				}
				if fault == "changed-program" && name == "ardents-node-linux-amd64" {
					raw = []byte("changed")
				}
				if fault == "foreign-role" && name == "infrastructure-join.json" {
					raw = bytes.Replace(raw, []byte(`"join"`), []byte(`"forwarding"`), 1)
				}
				if err := os.WriteFile(filepath.Join(candidate, name), raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if fault == "extra" {
				if err := os.WriteFile(filepath.Join(candidate, "extra"), []byte("extra"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			destination := filepath.Join(parent, fault+"-output")
			if output, err := exec.CommandContext(t.Context(), command, "join", "prepare-inventory", candidate, destination).CombinedOutput(); err == nil || bytes.Contains(output, []byte(`"sha256"`)) {
				t.Fatal("invalid JOIN inventory published", string(output), err)
			}
			if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("refusal mutated output", err)
			}
		})
	}
}

func TestJoinPreparationNativeRetainsFailureAndRejectsAdoption(t *testing.T) {
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
			raw, err := prepareJoinArtifact(ctx, args, func(label string, file *os.File) {
				if label != stage || seen {
					return
				}
				seen = true
				if stage == "written" {
					cancel()
					if err := file.Close(); err != nil {
						t.Error(err)
					}
					return
				}
				if err := os.WriteFile(template, []byte("changed original"), 0600); err != nil {
					t.Error(err)
				}
			})
			if !seen || err == nil || raw != nil {
				t.Fatal("original failure published", seen, err)
			}
			if stage == "written" && (!errors.Is(err, context.Canceled) || !errors.Is(err, os.ErrClosed)) {
				t.Fatal("original cancellation/physical failure lost", err)
			}
			if err := os.WriteFile(template, []byte("public bytes"), 0600); err != nil {
				t.Fatal(err)
			}
			if raw, err := prepareJoinArtifact(t.Context(), args, nil); err == nil || raw != nil {
				t.Fatal("partial output adopted", err)
			}
			args[5] = filepath.Join(parent, "fresh")
			if raw, err := prepareJoinArtifact(t.Context(), args, nil); err != nil || len(raw) == 0 {
				t.Fatal("fresh positive counterpart", err)
			}
		})
	}
}

// Declaration preparation must neither open nor initialize the future runtime
// inputs. The real executable publishes only its separate public output.
func TestJoinPreparationNativeTemplateAndRequestLeaveRuntimeAbsent(t *testing.T) {
	parent := sourceArtifactNativeParent(t)
	command := os.Getenv("ARDENTS_JOIN_CONTROL_PROGRAM")
	pin := os.Getenv("ARDENTS_JOIN_CONTROL_PROGRAM_SHA256")
	compiled, err := os.ReadFile(command)
	digest := sha256.Sum256(compiled)
	if err != nil || !filepath.IsAbs(command) || len(pin) != 64 || hex.EncodeToString(digest[:]) != pin {
		t.Fatal("invalid environment: independently pinned canonical Control required", err)
	}
	// Short fixed declarations fit the complete finite Unix completion namespace.
	// No birth or deletion is performed at any of these declared locations.
	prefix := "/var/lib/ardents-join-declaration-" + strconv.Itoa(os.Getpid())
	names := []string{"installation", "state", "participation", "spend", "hosting", "credentials", "plan", "certificate", "private-key", "clock"}
	paths := make([]string, len(names))
	for i, name := range names {
		paths[i] = prefix + "-" + name
	}
	checkAbsent := func() {
		t.Helper()
		for _, path := range paths {
			if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("future runtime declaration was present or created", path, err)
			}
		}
	}
	planOutput := filepath.Join(parent, "plan-output")
	paths[6] = filepath.Join(planOutput, "join-plan.json")
	checkAbsent()
	templateOutput := filepath.Join(parent, "template-output")
	output, err := exec.CommandContext(t.Context(), command, "join", "prepare-template", templateOutput).CombinedOutput()
	if err != nil || !bytes.Contains(output, []byte(`"operation":"join.prepare-template"`)) {
		t.Fatal("actual canonical template preparation", string(output), err)
	}
	template, err := os.ReadFile(filepath.Join(templateOutput, "ardents-join.service"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{"Description=Ardents JOIN Node", "User=ardents-node", "Group=ardents-node", "ExecStart=:@ARDENTS_JOIN_PROGRAM@ join start-installed @ARDENTS_JOIN_BINDING@", "ReadWritePaths=@ARDENTS_JOIN_WRITE_PATHS@", "Restart=no", "KillMode=control-group", "NoNewPrivileges=yes", "CPUQuota=100%", "MemoryMax=1073741824", "TasksMax=256", "LimitNOFILE=1024"} {
		if bytes.Count(template, []byte("\n"+line+"\n")) != 1 {
			t.Fatal("fixed JOIN template property differs", line)
		}
	}
	if bytes.Contains(template, []byte("forwarding")) || bytes.Count(template, []byte("ExecStart=")) != 1 {
		t.Fatal("foreign or extra runtime command")
	}
	requestOutput := filepath.Join(parent, "request-output")
	args := []string{"join", "prepare-request"}
	for i, flag := range []string{"installation-root", "state-root", "participation-root", "spend-root", "hosting-root", "credential-directory", "plan", "certificate", "private-key", "clock"} {
		args = append(args, "--"+flag, paths[i])
	}
	args = append(args, "--output", requestOutput)
	output, err = exec.CommandContext(t.Context(), command, args...).CombinedOutput()
	if err != nil || !bytes.Contains(output, []byte(`"operation":"join.prepare-request"`)) {
		t.Fatal("actual canonical request preparation", string(output), err)
	}
	request, err := os.ReadFile(filepath.Join(requestOutput, "join-request.json"))
	expected := fmt.Sprintf("{\"schema\":\"ardents-join-installation-v1\",\"invocation\":\"join\",\"profile\":\"ardents-join-process-v1\",\"installation_root\":%q,\"state_root\":%q,\"participation_root\":%q,\"spend_root\":%q,\"hosting_root\":%q,\"credential_directory\":%q,\"plan\":%q,\"certificate\":%q,\"private_key\":%q,\"clock\":%q}\n", paths[0], paths[1], paths[2], paths[3], paths[4], paths[5], paths[6], paths[7], paths[8], paths[9])
	if err != nil || string(request) != expected {
		t.Fatal("independent canonical request bytes", string(request), err)
	}
	checkAbsent()
	// Duplicate/foreign declarations refuse before an output birth. Restoring
	// declarations after a failed output never grants adoption of that output.
	for i, extra := range [][]string{{"--state-root", paths[1]}, {"--invocation", "forwarding"}} {
		invalid := append([]string{}, args...)
		destination := filepath.Join(parent, "refused-"+strconv.Itoa(i))
		invalid[len(invalid)-1] = destination
		invalid = append(invalid, extra...)
		output, err := exec.CommandContext(t.Context(), command, invalid...).CombinedOutput()
		if err == nil || bytes.Contains(output, []byte(`"sha256"`)) {
			t.Fatal("foreign or duplicate declaration accepted", string(output), err)
		}
		if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("declaration refusal created output", err)
		}
	}
	if output, err := exec.CommandContext(t.Context(), command, args...).CombinedOutput(); err == nil || bytes.Contains(output, []byte(`"sha256"`)) {
		t.Fatal("existing preparation output adopted", string(output), err)
	}
	checkAbsent()
	planArgs := []string{"join", "prepare-plan", "--request", filepath.Join(requestOutput, "join-request.json"), "--output", planOutput,
		"--network-id", strings.Repeat("01", 32), "--node-id", strings.Repeat("02", 32), "--profile-authority", strings.Repeat("03", 32),
		"--authority", strings.Repeat("04", 32), "--threshold", "1", "--work-tx", "10", "--work-rx", "20",
		"--termination-tx", "30", "--termination-rx", "40", "--retirement-wait-ms", "5000"}
	output, err = exec.CommandContext(t.Context(), command, planArgs...).CombinedOutput()
	if err != nil || !bytes.Contains(output, []byte(`"operation":"join.prepare-plan"`)) {
		t.Fatal("actual canonical plan preparation", string(output), err)
	}
	plan, err := os.ReadFile(paths[6])
	var facts struct {
		Schema  string `json:"schema"`
		Network struct {
			Root             string     `json:"root"`
			NetworkID        [32]byte   `json:"network_id"`
			Authorities      [][32]byte `json:"authorities"`
			Threshold        int        `json:"threshold"`
			ProfileAuthority [32]byte   `json:"profile_authority"`
			Clock            string     `json:"clock_observation_file"`
		} `json:"network"`
		NodeID        [32]byte                `json:"node_id"`
		Profile       string                  `json:"process_profile"`
		Participation string                  `json:"participation_root"`
		Spend         string                  `json:"spend_root"`
		Hosting       string                  `json:"hosting_root"`
		Certificate   string                  `json:"certificate"`
		PrivateKey    string                  `json:"private_key"`
		Work          struct{ Tx, Rx uint64 } `json:"work"`
		Termination   struct{ Tx, Rx uint64 } `json:"termination"`
		Wait          uint32                  `json:"retirement_wait_ms"`
	}
	if err != nil || json.Unmarshal(plan, &facts) != nil || facts.Schema != "ardents-join-runtime-v1" || facts.Profile != "ardents-join-process-v1" || facts.Network.Root != paths[1] || facts.Network.Clock != paths[9] || facts.Network.Threshold != 1 || len(facts.Network.Authorities) != 1 || facts.Participation != paths[2] || facts.Spend != paths[3] || facts.Hosting != paths[4] || facts.Certificate != filepath.Join(paths[5], "certificate.pem") || facts.PrivateKey != filepath.Join(paths[5], "private-key.pem") || facts.Wait != 5000 || facts.Work.Tx != 10 || facts.Work.Rx != 20 || facts.Termination.Tx != 30 || facts.Termination.Rx != 40 {
		t.Fatal("independent public plan declarations differ", string(plan), err)
	}
	for i := range 32 {
		if facts.Network.NetworkID[i] != 1 || facts.NodeID[i] != 2 || facts.Network.ProfileAuthority[i] != 3 || facts.Network.Authorities[0][i] != 4 {
			t.Fatal("explicit public trust bytes differ")
		}
	}
	for i, path := range paths {
		if i == 6 {
			continue
		}
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("plan preparation created a declared runtime input", path, err)
		}
	}
	// Existing completed public output is never an adoption authority.
	if output, err := exec.CommandContext(t.Context(), command, planArgs...).CombinedOutput(); err == nil || bytes.Contains(output, []byte(`"sha256"`)) {
		t.Fatal("existing plan output adopted", string(output), err)
	}
}
