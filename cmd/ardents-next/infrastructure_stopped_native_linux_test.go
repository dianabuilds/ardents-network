//go:build linux && network_signing_native

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// A physical process cut never resumes the transaction. The outside original
// controller must additionally join the entire private manager cgroup before
// inspecting the retained prefix; Wait here proves only this original child.
func TestSourceStoppedNativeCompiledProcessCut(t *testing.T) {
	sourceStoppedStand(t)
	phase := os.Getenv("ARDENTS_SOURCE_STOPPED_CUT_PHASE")
	switch phase {
	case "account-intent", "account-ready", "directory-born", "file-born", "sealed", "selected", "reload-intent":
	default:
		t.Fatal("invalid environment: finite physical cut phase required")
	}
	r, request := sourcePreflightFixture(t, true)
	dir, pin, _ := stoppedDistribution(t, sourcePreflightCommand(t))
	history := filepath.Join(filepath.Dir(dir), "history")
	program := filepath.Join(dir, infrastructure.ProgramName)
	args := []string{"infrastructure", "install-initial-stopped", dir, pin, history, "h3-test", "new-release-test", infrastructureReference.Format(time.RFC3339), request}
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, program, args...)
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if e := cmd.Start(); e != nil {
		t.Fatal(e)
	}
	joined := make(chan error, 1)
	go func() { joined <- cmd.Wait() }()
	journal := r.InstallationRoot + ".source-initial-journal"
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case e := <-joined:
			t.Fatal("original consumer completed before selected physical cut", phase, e, output.String())
		case <-ctx.Done():
			_ = cmd.Process.Kill()
			<-joined
			t.Fatal("selected cut not observed", phase, ctx.Err())
		case <-ticker.C:
			raw, e := os.ReadFile(journal)
			if e != nil || !bytes.Contains(raw, []byte(`"phase":"`+phase+`"`)) {
				continue
			}
			if e := cmd.Process.Signal(syscall.SIGSTOP); e != nil {
				_ = cmd.Process.Kill()
				<-joined
				t.Fatal(e)
			}
			if e := cmd.Process.Kill(); e != nil {
				<-joined
				t.Fatal(e)
			}
			if e := <-joined; e == nil {
				t.Fatal("cut original reported success")
			}
			if bytes.Contains(output.Bytes(), []byte(`source-initial-stopped-selected`)) {
				t.Fatal("cut original already handed off success")
			}
			// This marker supplies no product authority. The read-only oracle
			// consumes it only after the outside cgroup has physically joined.
			marker, _ := json.Marshal(map[string]string{"journal": journal, "request": request, "distribution": dir, "history": history, "phase": phase, "control": "process-cut"})
			if e := os.WriteFile(filepath.Join(os.Getenv("ARDENTS_SOURCE_STOPPED_FUTURE_PARENT"), "cut-observation.json"), append(marker, '\n'), 0600); e != nil {
				t.Fatal(e)
			}
			before, e := os.ReadFile(journal)
			if e != nil {
				t.Fatal(e)
			}
			if retry, e := exec.CommandContext(t.Context(), program, args...).CombinedOutput(); e == nil {
				t.Fatal("cut prefix adopted or initial floors reset", string(retry))
			}
			after, e := os.ReadFile(journal)
			if e != nil || !bytes.Equal(before, after) {
				t.Fatal("retry changed retained cut prefix", e)
			}
			t.Log("original cut child joined; full private stand join and independent prefix classification still required", phase)
			return
		}
	}
}

// The owner-level lifetime case signs the actual executing test artifact with
// controlled authorities. It supplies genuine private initial provenance and
// exposes native failure causes locally; it does not qualify a product command.
func TestSourceStoppedNativeOriginalOwner(t *testing.T) {
	sourceStoppedStand(t)
	dir := os.Getenv("ARDENTS_SOURCE_STOPPED_OWNER_DISTRIBUTION")
	request := os.Getenv("ARDENTS_SOURCE_STOPPED_OWNER_REQUEST")
	pin := os.Getenv("ARDENTS_SOURCE_STOPPED_OWNER_PIN")
	if dir == "" {
		self, e := os.Executable()
		if e != nil {
			t.Fatal(e)
		}
		binary, e := os.ReadFile(self)
		if e != nil {
			t.Fatal(e)
		}
		_, request = sourcePreflightFixture(t, true)
		dir, pin, _ = stoppedDistribution(t, binary)
		child := exec.CommandContext(t.Context(), filepath.Join(dir, infrastructure.ProgramName), "-test.v", "-test.run=^TestSourceStoppedNativeOriginalOwner$", "-test.count=1", "-test.timeout=3m")
		child.Env = append(os.Environ(), "ARDENTS_SOURCE_STOPPED_OWNER_DISTRIBUTION="+dir, "ARDENTS_SOURCE_STOPPED_OWNER_REQUEST="+request, "ARDENTS_SOURCE_STOPPED_OWNER_PIN="+pin)
		out, e := child.CombinedOutput()
		t.Log(string(out))
		if e != nil {
			t.Fatal("original mapped test consumer", e)
		}
		return
	}
	binary, e := os.ReadFile(filepath.Join(dir, infrastructure.ProgramName))
	if e != nil {
		t.Fatal(e)
	}
	b, e := enrollment.VerifyInfrastructure(t.Context(), dir, pin)
	if e != nil {
		t.Fatal(e)
	}
	root, _ := b.File("root.json")
	ts, _ := b.File("timestamp.json")
	sn, _ := b.File("snapshot.json")
	targets, _ := b.File("targets.json")
	files, e := release.StaticMetadataFiles(root, ts, sn, targets)
	if e != nil {
		t.Fatal(e)
	}
	history := filepath.Join(filepath.Dir(dir), "owner-history")
	v, e := release.Open(history)
	if e != nil {
		t.Fatal(e)
	}
	defer v.Close()
	a, e := infrastructure.AuthenticateInitial(t.Context(), v, b, release.Inputs{RootBytes: root, Files: files, Artifact: binary, TargetPath: infrastructure.ProgramTarget, Local: release.LocalEnvironment{Platform: "linux-amd64", Architecture: "amd64", Environment: "h3-test", Network: "new-release-test", RefTime: infrastructureReference}})
	if e != nil {
		t.Fatal(e)
	}
	input, e := infrastructure.ReadSourceInputs(t.Context(), request)
	defer input.Close()
	if e != nil {
		t.Fatal(e)
	}
	if e = infrastructure.InstallInitialStoppedSource(v, a, input, dir, history); e != nil {
		facts, _ := exec.Command("/usr/bin/systemctl", "show", "ardents-source.service").CombinedOutput()
		t.Log("private actual manager diagnostic", string(facts))
		t.Fatal("original genuine owner", e)
	}
	t.Log("original owner retained fixture", input.Declaration().InstallationRoot)
}

// This profile runs in a fresh private rootfs for each case. No fixture removes
// an installed account/unit or retained partial prefix to make another case pass.
func sourceStoppedStand(t *testing.T) {
	t.Helper()
	marker, e := os.ReadFile("/etc/ardents-source-stopped-native-stand")
	container, ce := os.ReadFile("/run/systemd/container")
	if e != nil || ce != nil || string(marker) != "ardents-source-stopped-native-v1\n" || strings.TrimSpace(string(container)) != "ardents-stopped" {
		t.Fatal("invalid environment: original private stopped Source stand required", e, ce)
	}
	for _, ns := range []string{"mnt", "pid", "cgroup", "uts"} {
		a, e := os.Stat("/proc/1/ns/" + ns)
		b, be := os.Stat("/proc/self/ns/" + ns)
		if e != nil || be != nil || !os.SameFile(a, b) {
			t.Fatal("invalid environment: caller/manager namespace differs", ns, e, be)
		}
	}
}

// Keep signed inputs and mutable Release floors with the native prefix. Test
// completion and failure never remove history needed for independent inspection.
func stoppedDistribution(t *testing.T, binary []byte) (string, string, map[string][]byte) {
	t.Helper()
	parent, err := os.MkdirTemp(os.Getenv("ARDENTS_SOURCE_STOPPED_FUTURE_PARENT"), "retained-distribution-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("retained original distribution/history parent", parent)
	return infrastructureDistributionRoot(t, parent, binary, nil, 1, nil, []byte(infrastructure.SourceTemplate))
}

func TestSourceStoppedNativeCompiledPreEffectRefusals(t *testing.T) {
	sourceStoppedStand(t)
	binary := sourcePreflightCommand(t)
	for _, control := range []string{"mismatched-runtime-key", "foreign-journal", "existing-semantic-root"} {
		t.Run(control, func(t *testing.T) {
			r, request := sourcePreflightFixture(t, true)
			dir, pin, _ := stoppedDistribution(t, binary)
			journal := r.InstallationRoot + ".source-initial-journal"
			var preimage []byte
			switch control {
			case "mismatched-runtime-key":
				other, _ := sourcePreflightFixture(t, true)
				raw, e := os.ReadFile(other.PrivateKey)
				if e != nil {
					t.Fatal(e)
				}
				if e := os.WriteFile(r.PrivateKey, raw, 0600); e != nil {
					t.Fatal(e)
				}
				clear(raw)
			case "foreign-journal":
				preimage = []byte("unknown interrupted prefix\n")
				if e := os.WriteFile(journal, preimage, 0600); e != nil {
					t.Fatal(e)
				}
			case "existing-semantic-root":
				if e := os.Mkdir(r.StateRoot, 0700); e != nil {
					t.Fatal(e)
				}
			}
			history := filepath.Join(filepath.Dir(dir), "history")
			cmd := exec.CommandContext(t.Context(), filepath.Join(dir, infrastructure.ProgramName), "infrastructure", "install-initial-stopped", dir, pin, history, "h3-test", "new-release-test", infrastructureReference.Format(time.RFC3339), request)
			if out, e := cmd.CombinedOutput(); e == nil || bytes.Contains(out, []byte("source-initial-stopped-selected")) {
				t.Fatal("causal pre-effect refusal accepted", control, string(out), e)
			}
			for _, path := range []string{r.InstallationRoot, r.ParticipationRoot, r.CredentialDirectory, "/etc/systemd/system/ardents-source.service"} {
				if _, e := os.Lstat(path); !os.IsNotExist(e) {
					t.Fatal("refusal created/adopted native resource", path, e)
				}
			}
			if preimage != nil {
				if now, e := os.ReadFile(journal); e != nil || !bytes.Equal(now, preimage) {
					t.Fatal("foreign journal adopted or modified", e)
				}
			} else if _, e := os.Lstat(journal); !os.IsNotExist(e) {
				t.Fatal("pre-effect refusal created a journal", e)
			}
			passwd, e := os.ReadFile("/etc/passwd")
			if e != nil || bytes.Contains(passwd, []byte("ardents-source:")) {
				t.Fatal("refusal created a runtime account", e)
			}
		})
	}
}

func TestSourceStoppedNativeCompiledOriginalInputAndCancellation(t *testing.T) {
	sourceStoppedStand(t)
	control := os.Getenv("ARDENTS_SOURCE_STOPPED_CONTROL")
	if control != "input-substitution" && control != "caller-cancellation" && control != "semantic-content" && control != "hook-substitution" {
		t.Fatal("invalid environment: original input or caller control required")
	}
	r, request := sourcePreflightFixture(t, true)
	dir, pin, _ := stoppedDistribution(t, sourcePreflightCommand(t))
	history := filepath.Join(filepath.Dir(dir), "history")
	program := filepath.Join(dir, infrastructure.ProgramName)
	args := []string{"infrastructure", "install-initial-stopped", dir, pin, history, "h3-test", "new-release-test", infrastructureReference.Format(time.RFC3339), request}
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, program, args...)
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if e := cmd.Start(); e != nil {
		t.Fatal(e)
	}
	joined := make(chan error, 1)
	go func() { joined <- cmd.Wait() }()
	journal := r.InstallationRoot + ".source-initial-journal"
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case e := <-joined:
			t.Fatal("original consumer completed before causal control", e, output.String())
		case <-ctx.Done():
			_ = cmd.Process.Kill()
			<-joined
			t.Fatal("original admission not observed", ctx.Err())
		case <-ticker.C:
			raw, e := os.ReadFile(journal)
			if e != nil || !bytes.Contains(raw, []byte(`"phase":"admitted"`)) {
				continue
			}
			if control == "semantic-content" {
				if _, e := os.Lstat(r.StateRoot); os.IsNotExist(e) {
					continue
				}
			}
			if control == "caller-cancellation" {
				e = cmd.Process.Signal(syscall.SIGTERM)
			} else if control == "semantic-content" {
				e = os.WriteFile(filepath.Join(r.StateRoot, "foreign-history"), []byte("foreign semantic state\n"), 0600)
			} else if control == "hook-substitution" {
				e = os.MkdirAll("/etc/shadow-maint/useradd-pre.d", 0755)
				if e == nil {
					e = os.WriteFile("/etc/shadow-maint/useradd-pre.d/foreign-hook", []byte("#!/bin/sh\necho executed > /var/lib/foreign-hook-executed\n"), 0755)
				}
			} else {
				var original []byte
				original, e = os.ReadFile(r.Certificate)
				if e == nil {
					e = os.Rename(r.Certificate, r.Certificate+".original")
				}
				if e == nil {
					e = os.WriteFile(r.Certificate, original, 0444)
				}
			}
			if e != nil {
				_ = cmd.Process.Kill()
				<-joined
				t.Fatal("causal control failed", e)
			}
			if e := <-joined; e == nil || bytes.Contains(output.Bytes(), []byte("source-initial-stopped-selected")) {
				t.Fatal("original substitution/cancellation admitted", control, e, output.String())
			}
			if control == "hook-substitution" {
				if _, e := os.Lstat("/var/lib/foreign-hook-executed"); !os.IsNotExist(e) {
					t.Fatal("forbidden helper hook ran", e)
				}
			}
			marker, _ := json.Marshal(map[string]string{"journal": journal, "request": request, "distribution": dir, "history": history, "phase": "admitted", "control": control})
			if e := os.WriteFile(filepath.Join(os.Getenv("ARDENTS_SOURCE_STOPPED_FUTURE_PARENT"), "cut-observation.json"), append(marker, '\n'), 0600); e != nil {
				t.Fatal(e)
			}
			before, e := os.ReadFile(journal)
			if e != nil {
				t.Fatal(e)
			}
			if retry, e := exec.CommandContext(t.Context(), program, args...).CombinedOutput(); e == nil {
				t.Fatal("retained failed prefix adopted", string(retry))
			}
			after, e := os.ReadFile(journal)
			if e != nil || !bytes.Equal(before, after) {
				t.Fatal("retry changed original failed prefix", e)
			}
			t.Log("original refusal consumer joined; outside stand join remains required", control)
			return
		}
	}
}

func TestSourceStoppedNativeCompiledInitialAndRetainedRefusal(t *testing.T) {
	sourceStoppedStand(t)
	binary := sourcePreflightCommand(t)
	r, request := sourcePreflightFixture(t, true)
	dir, pin, original := stoppedDistribution(t, binary)
	history := filepath.Join(filepath.Dir(dir), "history")
	program := filepath.Join(dir, "ardents-node-linux-amd64")
	args := []string{"infrastructure", "install-initial-stopped", dir, pin, history, "h3-test", "new-release-test", infrastructureReference.Format(time.RFC3339), request}
	cmd := exec.CommandContext(t.Context(), program, args...)
	out, err := cmd.CombinedOutput()
	journal := r.InstallationRoot + ".source-initial-journal"
	if err != nil {
		if raw, e := os.ReadFile(journal); e == nil {
			t.Log("retained private native prefix", string(raw))
		}
		t.Fatalf("genuine initial stopped consumer: %s %v", out, err)
	}
	if !bytes.Equal(out, []byte("{\"outcome\":\"source-initial-stopped-selected\"}\n")) {
		t.Fatal("unbounded/foreign receipt", string(out))
	}
	g := fmt.Sprintf("%x", sha256.Sum256(original["infrastructure-source.json"]))
	generation := filepath.Join(r.InstallationRoot, "generations", g)
	wanted := []string{"ardents-node-linux-amd64", "ardents-source.service", "binding.json", "infrastructure-enrollment.json", "infrastructure-source.json", "rendered-source.service", "root.json", "snapshot.json", "source-plan.json", "source-request.json", "targets.json", "timestamp.json"}
	entries, e := os.ReadDir(generation)
	if e != nil || len(entries) != len(wanted) {
		t.Fatal("finite immutable inventory", e)
	}
	for i, entry := range entries {
		if entry.Name() != wanted[i] {
			t.Fatal("foreign/partial immutable inventory", entry.Name())
		}
		info, e := entry.Info()
		if e != nil {
			t.Fatal(e)
		}
		st := info.Sys().(*syscall.Stat_t)
		mode := os.FileMode(0444)
		if entry.Name() == "ardents-node-linux-amd64" {
			mode = 0555
		}
		if !info.Mode().IsRegular() || info.Mode().Perm() != mode || st.Uid != 0 || st.Gid != 0 || st.Nlink != 1 {
			t.Fatal("immutable access", entry.Name(), info.Mode())
		}
		if body, ok := original[entry.Name()]; ok {
			actual, e := os.ReadFile(filepath.Join(generation, entry.Name()))
			if e != nil || !bytes.Equal(body, actual) {
				t.Fatal("authenticated bytes changed", entry.Name(), e)
			}
		}
	}
	bound, e := os.ReadFile(filepath.Join(r.InstallationRoot, "binding.json"))
	if e != nil {
		t.Fatal(e)
	}
	var binding struct {
		Schema, Invocation, Generation, Account string
		UID, GID                                uint32
		RequestSHA256                           string `json:"request_sha256"`
		PlanSHA256                              string `json:"plan_sha256"`
		UnitSHA256                              string `json:"unit_sha256"`
	}
	if json.Unmarshal(bound, &binding) != nil || binding.Schema != "ardents-source-binding-v1" || binding.Invocation != "source" || binding.Generation != g || binding.Account != "ardents-source" || binding.UID == 0 || binding.GID == 0 {
		t.Fatal("original generation/account binding")
	}
	copy, e := os.ReadFile(filepath.Join(generation, "binding.json"))
	if e != nil || !bytes.Equal(bound, copy) {
		t.Fatal("selection differs from sealed binding", e)
	}
	for path, hash := range map[string]string{request: binding.RequestSHA256, r.Plan: binding.PlanSHA256, "/etc/systemd/system/ardents-source.service": binding.UnitSHA256} {
		raw, e := os.ReadFile(path)
		if e != nil || fmt.Sprintf("%x", sha256.Sum256(raw)) != hash {
			t.Fatal("independent rendered/request digest", path, e)
		}
	}
	for _, root := range []string{r.StateRoot, r.ParticipationRoot} {
		info, e := os.Lstat(root)
		if e != nil {
			t.Fatal(e)
		}
		st := info.Sys().(*syscall.Stat_t)
		entries, e := os.ReadDir(root)
		if e != nil || len(entries) != 0 || info.Mode().Perm() != 0700 || st.Uid != binding.UID || st.Gid != binding.GID {
			t.Fatal("Installation initialized domain history/authority", root, e)
		}
	}
	for name, path := range map[string]string{"certificate.pem": r.Certificate, "private-key.pem": r.PrivateKey, "client-ca.pem": r.ClientCA} {
		actualPath := filepath.Join(r.CredentialDirectory, name)
		actual, e := os.ReadFile(actualPath)
		expected, ee := os.ReadFile(path)
		if e != nil || ee != nil || !bytes.Equal(actual, expected) {
			t.Fatal("runtime credential bytes", name, e, ee)
		}
		info, e := os.Lstat(actualPath)
		if e != nil {
			t.Fatal(e)
		}
		st := info.Sys().(*syscall.Stat_t)
		if st.Nlink != 1 || !info.Mode().IsRegular() {
			t.Fatal("runtime credential link/type")
		}
		if name == "private-key.pem" {
			if info.Mode().Perm() != 0600 || st.Uid != binding.UID || st.Gid != binding.GID {
				t.Fatal("runtime secret access")
			}
		} else if info.Mode().Perm() != 0444 || st.Uid != 0 || st.Gid != 0 {
			t.Fatal("public credential access")
		}
	}
	// Independent real manager oracle, never Start to prove loading. Record the
	// full raw manager facts only in this private test output.
	observed, e := exec.Command("/usr/bin/systemctl", "show", "ardents-source.service", "-p", "LoadState", "-p", "ActiveState", "-p", "SubState", "-p", "MainPID", "-p", "ExecMainPID", "-p", "NRestarts", "-p", "Restart", "-p", "FragmentPath", "-p", "UnitFileState").CombinedOutput()
	if e != nil {
		t.Fatal(e)
	}
	t.Log("actual independent stopped manager", string(observed))
	for _, line := range []string{"LoadState=loaded\n", "ActiveState=inactive\n", "SubState=dead\n", "MainPID=0\n", "ExecMainPID=0\n", "NRestarts=0\n", "Restart=no\n", "UnitFileState=static\n"} {
		if !bytes.Contains(observed, []byte(line)) {
			t.Fatal("manager proves no stopped selection", line)
		}
	}
	before, e := os.ReadFile(journal)
	if e != nil {
		t.Fatal(e)
	}
	assertSourceStoppedPrefix(t, before, "stopped-verified")
	if retryOut, e := exec.CommandContext(t.Context(), program, args...).CombinedOutput(); e == nil {
		t.Fatal("retained floors/resources adopted", string(retryOut))
	}
	after, e := os.ReadFile(journal)
	if e != nil || !bytes.Equal(before, after) {
		t.Fatal("retry rewrote retained prefix", e)
	}
	if now, e := os.ReadFile(filepath.Join(r.InstallationRoot, "binding.json")); e != nil || !bytes.Equal(now, bound) {
		t.Fatal("retry reset selection", e)
	}
	t.Log("retained private stopped fixture", filepath.Dir(r.InstallationRoot))
}

func assertSourceStoppedPrefix(t *testing.T, raw []byte, last string) {
	t.Helper()
	if len(raw) == 0 || raw[len(raw)-1] != '\n' {
		t.Fatal("ambiguous/torn prefix: repair-required")
	}
	lines := bytes.Split(bytes.TrimSuffix(raw, []byte("\n")), []byte("\n"))
	var phase string
	for i, line := range lines {
		var r struct {
			Sequence       int    `json:"sequence"`
			Phase          string `json:"phase"`
			Path           string `json:"path"`
			Device, Inode  uint64
			SHA256         string `json:"sha256"`
			Mode, UID, GID uint32
		}
		if json.Unmarshal(line, &r) != nil || r.Sequence != i || r.Path == "" {
			t.Fatal("unrecognized prefix: repair-required")
		}
		switch r.Phase {
		case "initial", "admitted", "account-preimage", "account-auxiliary-preimage", "account-intent", "account-born", "account-auxiliary-result", "account-auxiliary-cleared", "account-ready", "directory-intent", "directory-born", "directory-ready", "file-intent", "file-born", "file-ready", "seal-intent", "sealed", "selected", "reload-intent", "stopped-verified":
		default:
			t.Fatal("unknown phase: repair-required", r.Phase)
		}
		if strings.Contains(r.Phase, "-born") && (r.Device == 0 || r.Inode == 0) {
			t.Fatal("birth lacks native provenance")
		}
		phase = r.Phase
	}
	if phase != last {
		t.Fatal("unexpected final prefix", phase, last)
	}
}
