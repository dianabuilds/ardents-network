//go:build linux && descriptor_installed_native

package descriptor_test

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
	"syscall"
	"testing"
	"time"

	descriptorinstallation "github.com/dianabuilds/ardents-network/internal/successor/installation/descriptor"
)

func TestDescriptorInstalledNativeResourceReplacement(t *testing.T) {
	runDescriptorReplacementStage(t, "resource", "2")
}
func TestDescriptorInstalledNativeProgramReplacement(t *testing.T) {
	runDescriptorReplacementStage(t, "program", "3")
}

// Each stage has its own unchanged checked three-minute test bound. Its
// prerequisite is independently verified committed domain/native retirement,
// never a QA marker or a reset deadline on a live predecessor operation.
func runDescriptorReplacementStage(t *testing.T, stage, version string) {
	descriptorNativeStand(t)
	const base = descriptorNativeParent
	read := func(path string) []byte {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	digest := func(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
	bindingPath := filepath.Join(base, "runtime/node/binding.json")
	previousBinding := read(bindingPath)
	var outer struct {
		Schema        string          `json:"schema"`
		Configuration json.RawMessage `json:"configuration"`
	}
	if json.Unmarshal(previousBinding, &outer) != nil {
		t.Fatal("original selected declaration unavailable")
	}
	configuration := previousBinding
	if outer.Schema == "ardents-descriptor-replacement-binding-v1" {
		configuration = outer.Configuration
	}
	var previous struct {
		Generation    string `json:"generation"`
		ProgramSHA256 string `json:"program_sha256"`
		UID, GID      uint32
		Request       descriptorinstallation.Request `json:"request"`
	}
	if json.Unmarshal(configuration, &previous) != nil || previous.Generation == "" {
		t.Fatal("original public configuration unavailable")
	}
	r := previous.Request
	control, controlBytes := descriptorNativePinnedProgram(t, "ARDENTS_DESCRIPTOR_CONTROL_PROGRAM")
	programVariable, buildVariable := "ARDENTS_DESCRIPTOR_NODE_PROGRAM", "ARDENTS_DESCRIPTOR_BUILD_ROOT"
	previousDistribution := filepath.Join(base, "delivery/distribution")
	if stage == "program" {
		programVariable, buildVariable = "ARDENTS_DESCRIPTOR_FINAL_NODE_PROGRAM", "ARDENTS_DESCRIPTOR_FINAL_BUILD_ROOT"
		previousDistribution = filepath.Join(base, "replacement-resource/distribution")
	}
	var previousArtifact struct {
		ReleaseVersion int64 `json:"release_version"`
	}
	previousArtifactRaw := read(filepath.Join(previousDistribution, "infrastructure-descriptor.json"))
	expectedVersion := int64(1)
	if stage == "program" {
		expectedVersion = 2
	}
	if json.Unmarshal(previousArtifactRaw, &previousArtifact) != nil || previousArtifact.ReleaseVersion != expectedVersion || digest(previousArtifactRaw) != previous.Generation {
		t.Fatal("selected original generation differs from required committed predecessor")
	}
	program, programBytes := descriptorNativePinnedProgram(t, programVariable)
	build := os.Getenv(buildVariable)
	if !filepath.IsAbs(build) || digest(read(filepath.Join(build, "READY"))) != os.Getenv(strings.TrimSuffix(buildVariable, "_ROOT")+"_READY_SHA256") {
		t.Fatal("invalid environment: exact matching build scope required")
	}
	for _, number := range []string{"build-1", "build-2"} {
		if !bytes.Equal(read(filepath.Join(build, number, "ardents-node")), programBytes) {
			t.Fatal("replacement program differs from genuine two-build outputs")
		}
	}
	if (digest(programBytes) != previous.ProgramSHA256) != (stage == "program") {
		t.Fatal("resource/program replacement kind differs from actual executable bytes")
	}
	history := filepath.Join(base, "release-history")
	invoke := func(args ...string) ([]byte, error) {
		return exec.CommandContext(t.Context(), program, append([]string{"descriptor"}, args...)...).CombinedOutput()
	}
	if raw, err := invoke("inspect-started-history", history, bindingPath); err != nil {
		t.Fatal("independent original committed history", err, string(raw))
	}
	// Re-establish the honest local clock writer for this new original native
	// test. The previous writer was joined; original file/clock floors persist.
	quit, joined := make(chan struct{}), make(chan struct{})
	tick := func() error { now := time.Now(); return os.Chtimes(r.Clock, now, now) }
	if err := tick(); err != nil {
		t.Fatal(err)
	}
	go func() {
		defer close(joined)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-quit:
				return
			case <-ticker.C:
				if tick() != nil {
					return
				}
			}
		}
	}()
	defer func() { close(quit); <-joined }()
	parent := filepath.Join(base, "replacement-"+stage)
	if err := os.Mkdir(parent, 0700); err != nil {
		t.Fatal(err)
	}
	write := func(path string, raw []byte, mode os.FileMode) {
		t.Helper()
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
		if err != nil {
			t.Fatal(err)
		}
		n, writeErr := f.Write(raw)
		syncErr, closeErr := f.Sync(), f.Close()
		if writeErr != nil || n != len(raw) || syncErr != nil || closeErr != nil {
			t.Fatal("original replacement output", writeErr, syncErr, closeErr)
		}
	}
	call := func(args ...string) []byte {
		t.Helper()
		raw, err := exec.CommandContext(t.Context(), control, append([]string{"descriptor"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatal("canonical Descriptor forward producer", args[0], err, string(raw))
		}
		return raw
	}
	preparer, _ := descriptorNativePinnedProgram(t, "ARDENTS_DESCRIPTOR_RELEASE_PREPARER")
	evidence, _ := descriptorNativePinnedProgram(t, "ARDENTS_DESCRIPTOR_BUILD_EVIDENCE")
	template := filepath.Join(base, "delivery/template/ardents-descriptor.service")
	artifactOutput := filepath.Join(parent, "artifact")
	call("prepare-artifact", "initial-stopped-descriptor", version, program, template, artifactOutput)
	artifact := filepath.Join(artifactOutput, "infrastructure-descriptor.json")
	planOutput := filepath.Join(parent, "public-plan")
	now := time.Now().UTC().Truncate(time.Second)
	args := []string{"-I", "-S", preparer, "--forward", "--build-root", build, "--descriptor", artifact, "--template", template, "--network", "initial-stopped-descriptor", "--expires", now.Add(time.Hour).Format(time.RFC3339), "--no-new-work-after", now.Add(30 * time.Minute).Format(time.RFC3339), "--terminate-after", now.Add(45 * time.Minute).Format(time.RFC3339), "--qualification-evidence", evidence, "--qualification-evidence-sha256", os.Getenv("ARDENTS_DESCRIPTOR_BUILD_EVIDENCE_SHA256"), "--output", planOutput}
	if raw, err := exec.CommandContext(t.Context(), "python3", args...).CombinedOutput(); err != nil {
		t.Fatal("actual build-bound forward plan", err, string(raw))
	}
	metadata := filepath.Join(parent, "metadata")
	call("sign-forward-release", filepath.Join(planOutput, "descriptor-release-plan.json"), filepath.Join(base, "delivery/release-keys"), metadata, filepath.Join(parent, "verification"), previousDistribution)
	distribution := filepath.Join(parent, "distribution")
	if err := os.Mkdir(distribution, 0700); err != nil {
		t.Fatal(err)
	}
	for name, raw := range map[string][]byte{"ardents-node-linux-amd64": programBytes, "ardents-descriptor.service": read(template), "infrastructure-descriptor.json": read(artifact)} {
		write(filepath.Join(distribution, name), raw, 0600)
	}
	for _, name := range []string{"root.json", "timestamp.json", "snapshot.json", "targets.json"} {
		write(filepath.Join(distribution, name), read(filepath.Join(metadata, name)), 0600)
	}
	inventory := filepath.Join(parent, "inventory")
	call("prepare-inventory", distribution, inventory)
	write(filepath.Join(distribution, "infrastructure-enrollment.json"), read(filepath.Join(inventory, "infrastructure-enrollment.json")), 0600)
	requestOutput, runtimePlan := filepath.Join(parent, "request"), filepath.Join(parent, "runtime-plan")
	call("prepare-request", "--installation-root", r.InstallationRoot, "--state-root", r.StateRoot, "--participation-root", r.ParticipationRoot, "--spend-root", r.SpendRoot, "--hosting-root", r.HostingRoot, "--descriptor-root", r.DescriptorRoot, "--credential-directory", r.CredentialDirectory, "--plan", filepath.Join(runtimePlan, "descriptor-plan.json"), "--certificate", r.Certificate, "--private-key", r.PrivateKey, "--clock", r.Clock, "--output", requestOutput)
	requestFile := filepath.Join(requestOutput, "descriptor-request.json")
	oldPlanRaw := read(filepath.Join(r.InstallationRoot, "generations", previous.Generation, "descriptor-plan.json"))
	oldPlan, err := descriptorinstallation.DecodePlan(oldPlanRaw)
	if err != nil {
		t.Fatal(err)
	}
	work := oldPlan.Work
	if stage == "resource" {
		work.Tx++
	}
	call("prepare-plan", "--request", requestFile, "--output", runtimePlan, "--network-id", hex.EncodeToString(oldPlan.Network.NetworkID[:]), "--node-id", hex.EncodeToString(oldPlan.NodeID[:]), "--profile-authority", hex.EncodeToString(oldPlan.Network.ProfileAuthority[:]), "--authority", hex.EncodeToString(oldPlan.Network.Authorities[0][:]), "--threshold", "1", "--work-tx", fmt.Sprint(work.Tx), "--work-rx", fmt.Sprint(work.Rx), "--termination-tx", fmt.Sprint(oldPlan.Termination.Tx), "--termination-rx", fmt.Sprint(oldPlan.Termination.Rx), "--retirement-wait-ms", fmt.Sprint(oldPlan.RetirementWaitMS))
	storedEntries, err := os.ReadDir(filepath.Join(r.DescriptorRoot, "records"))
	if err != nil || len(storedEntries) != 1 {
		t.Fatal("actual retained Store floor required", err)
	}
	stored := filepath.Join(r.DescriptorRoot, "records", storedEntries[0].Name())
	storeBefore := read(stored)
	keyPath := filepath.Join(r.CredentialDirectory, "private-key.pem")
	keyBefore := digest(read(keyPath))
	rootBefore := map[string]os.FileInfo{}
	for _, root := range []string{r.StateRoot, r.ParticipationRoot, r.SpendRoot, r.HostingRoot, r.DescriptorRoot} {
		info, err := os.Stat(root)
		if err != nil || info.Sys().(*syscall.Stat_t).Uid != previous.UID {
			t.Fatal("original semantic principal differs", err)
		}
		rootBefore[root] = info
	}
	// Unknown sibling history refuses before a new transition is born. This is
	// a deliberately created negative fixture, never adopted as product history.
	unknown := r.InstallationRoot + ".descriptor-replacement-unknown-journal"
	write(unknown, nil, 0600)
	if raw, err := invoke("replace", distribution, history, "alpha", "initial-stopped-descriptor", bindingPath, requestFile); err == nil {
		t.Fatal("unknown replacement journal accepted", string(raw))
	} else {
		write(filepath.Join(parent, "refusal-unknown-journal.log"), append(append([]byte(err.Error()+"\n"), raw...), '\n'), 0600)
	}
	if err := os.Remove(unknown); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(previousBinding, read(bindingPath)) || !bytes.Equal(storeBefore, read(stored)) {
		t.Fatal("refused transition changed selection or Store")
	}
	harness, _ := descriptorNativePinnedProgram(t, "ARDENTS_DESCRIPTOR_EXCHANGE_PROGRAM")
	// These are immutable public signed-proof inputs, not holder rights or a
	// readiness fact. The genuine helper independently matches them after fresh
	// issuance, and signals readiness only then. Its minute wait starts after
	// stopped replacement and independent bootstrap/history verification join.
	var originalProofs struct {
		Target, First, Second [32]byte
		Lifecycle             bool
	}
	if err := json.Unmarshal(read(filepath.Join(base, "exchange-prepared.json")), &originalProofs); err != nil || !originalProofs.Lifecycle || sha256.Sum256(read(filepath.Join(base, "descriptor-proof-1.bin"))) != originalProofs.First || sha256.Sum256(read(filepath.Join(base, "descriptor-proof-2.bin"))) != originalProofs.Second {
		t.Fatal("original signed proof commitments differ", err)
	}
	exchangeInputs, err := json.Marshal(struct {
		Stage                 string
		Target, First, Second [32]byte
	}{stage, originalProofs.Target, originalProofs.First, originalProofs.Second})
	if err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(parent, "exchange-prepared.json"), exchangeInputs, 0600)
	scopeRoot := os.Getenv("ARDENTS_DESCRIPTOR_REPLACEMENT_BOOTSTRAP_DIRECTORY")
	if !filepath.IsAbs(scopeRoot) {
		t.Fatal("invalid environment: original replacement bootstrap scope required")
	}
	scope := struct {
		Schema, Stage string
		Programs      map[string]string
		Inputs        map[string]string
		Limits        string
	}{"ardents-descriptor-replacement-bootstrap-v1", stage, map[string]string{"node": digest(programBytes), "control": digest(controlBytes), "exchange-harness": digest(read(harness))}, map[string]string{}, "one controlled VPS/cohort; explicit forward stopped replacement and fresh original startup; unchanged guards, bounds, quotas and absolute deadlines; no recovery/Service/public/load/privacy/independent-family or builder qualification"}
	for name, path := range map[string]string{"predecessor-binding": bindingPath, "inventory": filepath.Join(distribution, "infrastructure-enrollment.json"), "request": requestFile, "plan": filepath.Join(runtimePlan, "descriptor-plan.json"), "exchange": filepath.Join(parent, "exchange-prepared.json")} {
		scope.Inputs[name] = digest(read(path))
	}
	prepared, err := json.Marshal(scope)
	if err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(scopeRoot, "prepared.json"), prepared, 0600)
	until := time.Now().Add(time.Minute)
	for {
		raw, err := os.ReadFile(filepath.Join(scopeRoot, "accepted.json"))
		if err == nil {
			var grant struct {
				Prepared, Delegation string
			}
			if json.Unmarshal(raw, &grant) != nil || grant.Prepared != digest(prepared) || grant.Delegation != "product-owner-548-descriptor-no-new-agreements" {
				t.Fatal("exact replacement bootstrap delegation differs")
			}
			break
		}
		if !os.IsNotExist(err) || time.Now().After(until) {
			t.Fatal("exact delegated replacement scope unavailable", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if raw, err := invoke("replace", distribution, history, "alpha", "initial-stopped-descriptor", bindingPath, requestFile); err != nil || !bytes.Contains(raw, []byte("descriptor-replacement-stopped-completed")) {
		t.Fatal("explicit stopped forward replacement", err, string(raw))
	}
	if !bytes.Equal(storeBefore, read(stored)) || digest(read(keyPath)) != keyBefore {
		t.Fatal("stopped replacement changed Store or original Node key")
	}
	for root, original := range rootBefore {
		current, err := os.Stat(root)
		if err != nil || !os.SameFile(original, current) || current.Sys().(*syscall.Stat_t).Uid != previous.UID {
			t.Fatal("replacement changed original semantic root/account", err)
		}
	}
	exchangeLog, err := os.OpenFile(filepath.Join(parent, "exchange.log"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	exchangeCtx, cancelExchange := context.WithCancel(t.Context())
	exchange := exec.CommandContext(exchangeCtx, harness, "-test.run=^TestDescriptorInstalledNativeReplacementExchange$", "-test.count=1", "-test.v", "-test.timeout=3m")
	exchange.Env = append(os.Environ(), "ARDENTS_DESCRIPTOR_REPLACEMENT_STAGE="+stage)
	exchange.Stdout, exchange.Stderr = exchangeLog, exchangeLog
	if err := exchange.Start(); err != nil {
		cancelExchange()
		exchangeLog.Close()
		t.Fatal(err)
	}
	exchangeJoined := false
	defer func() {
		if !exchangeJoined {
			cancelExchange()
			waitErr := exchange.Wait()
			if err := errors.Join(waitErr, exchangeLog.Close()); err != nil {
				t.Error("original replacement exchange cleanup", err)
			}
		}
	}()
	until = time.Now().Add(30 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(parent, "exchange-ready")); err == nil {
			break
		}
		if time.Now().After(until) {
			t.Fatal("fresh genuine holder preparation unavailable", string(read(filepath.Join(parent, "exchange.log"))))
		}
		time.Sleep(20 * time.Millisecond)
	}
	if raw, err := invoke("restart", distribution, history, "alpha", "initial-stopped-descriptor", bindingPath); err != nil {
		t.Fatal("fresh independent replacement Startup", err, string(raw))
	}
	stopped := false
	defer func() {
		if !stopped {
			raw, err := exec.CommandContext(context.WithoutCancel(t.Context()), "systemctl", "stop", "ardents-descriptor.service").CombinedOutput()
			if err != nil {
				t.Error("QA native teardown, no accepted retirement proof", err, string(raw))
			}
		}
	}()
	rawID, err := exec.CommandContext(t.Context(), "systemctl", "show", "ardents-descriptor.service", "-p", "InvocationID", "--value").CombinedOutput()
	id := strings.TrimSpace(string(rawID))
	if err != nil || len(id) != 32 {
		t.Fatal("actual replacement invocation absent", err)
	}
	until = time.Now().Add(15 * time.Second)
	for {
		raw, err := exec.CommandContext(t.Context(), "journalctl", "_SYSTEMD_INVOCATION_ID="+id, "-o", "cat", "--no-pager").CombinedOutput()
		if err == nil && bytes.Contains(raw, []byte(`"phase":"listening"`)) {
			break
		}
		if time.Now().After(until) {
			t.Fatal("actual new invocation not ready", err, string(raw))
		}
		time.Sleep(20 * time.Millisecond)
	}
	write(filepath.Join(parent, "exchange-start"), []byte("original replacement Descriptor ready\n"), 0600)
	waitErr := exchange.Wait()
	exchangeJoined = true
	closeErr := exchangeLog.Close()
	cancelExchange()
	if err := errors.Join(waitErr, closeErr); err != nil {
		t.Fatal("actual preserved Store exchange", err, string(read(filepath.Join(parent, "exchange.log"))))
	}
	if raw, err := invoke("stop", history, bindingPath); err != nil {
		t.Fatal("replacement original domain/native joined Stop", err, string(raw))
	}
	stopped = true
	if raw, err := invoke("inspect-started-history", history, bindingPath); err != nil {
		t.Fatal("independent complete replacement retirement", err, string(raw))
	}
	if !bytes.Equal(storeBefore, read(stored)) || digest(read(keyPath)) != keyBefore {
		t.Fatal("replacement exchange changed original Store or private key")
	}
	t.Log("actual stopped replacement, independent startup, three retained Store operations and original domain/native retirement verified")
}
