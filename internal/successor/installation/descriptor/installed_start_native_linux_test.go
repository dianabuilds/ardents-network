//go:build linux && descriptor_installed_native

package descriptor_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	descriptorinstallation "github.com/dianabuilds/ardents-network/internal/successor/installation/descriptor"
)

// Genuine canonical Release, signed offline State, original Hosting and first
// protected non-root Descriptor startup. Public declarations are not authority;
// runtime owners independently verify all supplied material and native custody.
func TestDescriptorInstalledNativePublishLookup(t *testing.T) {
	runDescriptorInstalledJourney(t, false)
}
func TestDescriptorInstalledNativeStopAndRestart(t *testing.T) {
	runDescriptorInstalledJourney(t, true)
}

func TestDescriptorInstalledNativeReplacementPrerequisite(t *testing.T) {
	runDescriptorInstalledPreparation(t, true, true)
}

func runDescriptorInstalledJourney(t *testing.T, lifecycle bool) {
	runDescriptorInstalledPreparation(t, lifecycle, false)
}

// Replacement starts from one genuine retired invocation. The separate retained
// restart profile owns its own complete two-invocation acceptance chain.
func runDescriptorInstalledPreparation(t *testing.T, lifecycle, replacement bool) {
	const exchange = true
	marker, err := os.ReadFile("/etc/ardents-descriptor-initial-native-stand")
	if err != nil || string(marker) != "ardents-descriptor-initial-native-v1\n" || os.Geteuid() != 0 {
		t.Fatal("invalid environment: isolated initial Descriptor stand required", err)
	}
	const parent = "/var/lib/descriptor-initial"
	if err := os.Mkdir(parent, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(parent, 0755); err != nil {
		t.Fatal(err)
	}
	delivery := filepath.Join(parent, "delivery")
	if err := os.Mkdir(delivery, 0700); err != nil {
		t.Fatal(err)
	}
	command, controlBytes := descriptorNativePinnedProgram(t, "ARDENTS_DESCRIPTOR_CONTROL_PROGRAM")
	_, nodeBytes := descriptorNativePinnedProgram(t, "ARDENTS_DESCRIPTOR_NODE_PROGRAM")
	preparer, _ := descriptorNativePinnedProgram(t, "ARDENTS_DESCRIPTOR_RELEASE_PREPARER")
	evidence, _ := descriptorNativePinnedProgram(t, "ARDENTS_DESCRIPTOR_BUILD_EVIDENCE")
	buildRoot := os.Getenv("ARDENTS_DESCRIPTOR_BUILD_ROOT")
	read := func(path string) []byte {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	digest := func(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
	if !filepath.IsAbs(buildRoot) || digest(read(filepath.Join(buildRoot, "READY"))) != os.Getenv("ARDENTS_DESCRIPTOR_BUILD_READY_SHA256") {
		t.Fatal("invalid environment: actual two-build commitment required")
	}
	for _, number := range []string{"build-1", "build-2"} {
		if !bytes.Equal(read(filepath.Join(buildRoot, number, "ardents-node")), nodeBytes) || !bytes.Equal(read(filepath.Join(buildRoot, number, "ardents-control")), controlBytes) {
			t.Fatal("canonical commands differ from actual build records")
		}
	}
	call := func(args ...string) []byte {
		t.Helper()
		raw, err := exec.CommandContext(t.Context(), command, append([]string{"descriptor"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatal("canonical Descriptor producer", args[0], err, string(raw))
		}
		return raw
	}
	templateRoot, descriptorRoot := filepath.Join(delivery, "template"), filepath.Join(delivery, "descriptor")
	call("prepare-template", templateRoot)
	template := filepath.Join(templateRoot, "ardents-descriptor.service")
	call("prepare-artifact", "initial-stopped-descriptor", "1", filepath.Join(buildRoot, "build-1", "ardents-node"), template, descriptorRoot)
	descriptor := filepath.Join(descriptorRoot, "infrastructure-descriptor.json")
	now := time.Now().UTC().Truncate(time.Second)
	planOutput := filepath.Join(delivery, "public-plan")
	args := []string{"-I", "-S", preparer, "--build-root", buildRoot, "--descriptor", descriptor, "--template", template, "--network", "initial-stopped-descriptor", "--expires", now.Add(time.Hour).Format(time.RFC3339), "--no-new-work-after", now.Add(30 * time.Minute).Format(time.RFC3339), "--terminate-after", now.Add(45 * time.Minute).Format(time.RFC3339), "--qualification-evidence", evidence, "--qualification-evidence-sha256", os.Getenv("ARDENTS_DESCRIPTOR_BUILD_EVIDENCE_SHA256"), "--output", planOutput}
	if raw, err := exec.CommandContext(t.Context(), "python3", args...).CombinedOutput(); err != nil {
		t.Fatal("actual build-bound Descriptor plan", err, string(raw))
	}
	keys, metadataRoot := filepath.Join(delivery, "release-keys"), filepath.Join(delivery, "metadata")
	call("prepare-release-keys", keys)
	call("sign-initial-release", filepath.Join(planOutput, "descriptor-release-plan.json"), keys, metadataRoot, filepath.Join(delivery, "verification"))
	distribution := filepath.Join(delivery, "distribution")
	if err := os.Mkdir(distribution, 0700); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{"ardents-node-linux-amd64": nodeBytes, "ardents-descriptor.service": read(template), "infrastructure-descriptor.json": read(descriptor)}
	for _, name := range []string{"root.json", "timestamp.json", "snapshot.json", "targets.json"} {
		files[name] = read(filepath.Join(metadataRoot, name))
	}
	for name, raw := range files {
		mode := os.FileMode(0600)
		if name == "ardents-node-linux-amd64" {
			mode = 0700
		}
		if err := os.WriteFile(filepath.Join(distribution, name), raw, mode); err != nil {
			t.Fatal(err)
		}
	}
	inventoryOutput := filepath.Join(delivery, "inventory")
	call("prepare-inventory", distribution, inventoryOutput)
	inventory := read(filepath.Join(inventoryOutput, "infrastructure-enrollment.json"))
	if err := os.WriteFile(filepath.Join(distribution, "infrastructure-enrollment.json"), inventory, 0600); err != nil {
		t.Fatal(err)
	}
	var permissionPin [32]byte
	var harness string
	if exchange {
		harness, _ = descriptorNativePinnedProgram(t, "ARDENTS_DESCRIPTOR_EXCHANGE_PROGRAM")
		raw, err := exec.CommandContext(t.Context(), harness, "-test.run=^TestDescriptorInstalledNativeAuthority$", "-test.count=1", "-test.v", "-test.timeout=30s").CombinedOutput()
		if err != nil {
			t.Fatal("genuine public Permission authority preparation", err, string(raw))
		}
		var receipt struct {
			Public [32]byte `json:"public"`
		}
		if json.Unmarshal(read(filepath.Join(parent, "permission-public.json")), &receipt) != nil || receipt.Public == ([32]byte{}) {
			t.Fatal("genuine Permission public receipt absent")
		}
		permissionPin = receipt.Public
	} else {
		permission, key, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		clear(key)
		copy(permissionPin[:], permission)
	}
	bundle, profile, authority, peer := prepareDescriptorNativeCohort(t, command, parent, permissionPin, exchange)

	preparation := filepath.Join(parent, "canonical-preparation")
	recordPlan := filepath.Join(preparation, "record-plan-03", "node-record-plan.json")
	record := filepath.Join(preparation, "record-03", "node-record.bin")
	certificateOutput := filepath.Join(delivery, "node-certificate")
	call("prepare-node-certificate", recordPlan, record, digest(read(record)), filepath.Join(preparation, "node-key-03", "node-key.pem"), certificateOutput)
	certPath, keyPath := filepath.Join(certificateOutput, "node-certificate.pem"), filepath.Join(preparation, "node-key-03", "node-key.pem")
	if exchange {
		peerText := hex.EncodeToString(peer[:])[:2]
		peerRecord := filepath.Join(preparation, "record-"+peerText, "node-record.bin")
		call("prepare-node-certificate", filepath.Join(preparation, "record-plan-"+peerText, "node-record-plan.json"), peerRecord, digest(read(peerRecord)), filepath.Join(preparation, "node-key-"+peerText, "node-key.pem"), filepath.Join(delivery, "peer-certificate"))
	}

	runtime := filepath.Join(parent, "runtime")
	if err := os.Mkdir(runtime, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(runtime, 0755); err != nil {
		t.Fatal(err)
	}
	clockParent := filepath.Join(parent, "clock")
	if err := os.Mkdir(clockParent, 0755); err != nil {
		t.Fatal(err)
	}
	clock := filepath.Join(clockParent, "observation")
	if err := os.WriteFile(clock, []byte("live clock\n"), 0644); err != nil {
		t.Fatal(err)
	}
	requestOutput, runtimePlan := filepath.Join(delivery, "request"), filepath.Join(delivery, "runtime-plan")
	call("prepare-request", "--installation-root", filepath.Join(runtime, "node"), "--state-root", filepath.Join(runtime, "state"), "--participation-root", filepath.Join(runtime, "participation"), "--spend-root", filepath.Join(runtime, "spend"), "--hosting-root", filepath.Join(runtime, "hosting"), "--descriptor-root", filepath.Join(runtime, "store"), "--credential-directory", filepath.Join(runtime, "credentials"), "--plan", filepath.Join(runtimePlan, "descriptor-plan.json"), "--certificate", certPath, "--private-key", keyPath, "--clock", clock, "--output", requestOutput)
	requestPath := filepath.Join(requestOutput, "descriptor-request.json")
	request, err := descriptorinstallation.DecodeRequest(read(requestPath))
	if err != nil {
		t.Fatal(err)
	}
	network, nodeID := [32]byte{1}, [32]byte{3}
	call("prepare-plan", "--request", requestPath, "--output", runtimePlan, "--network-id", hex.EncodeToString(network[:]), "--node-id", hex.EncodeToString(nodeID[:]), "--profile-authority", hex.EncodeToString(authority[:]), "--authority", hex.EncodeToString(authority[:]), "--threshold", "1", "--work-tx", "134217728", "--work-rx", "134217728", "--termination-tx", "1048576", "--termination-rx", "1048576", "--retirement-wait-ms", "5000")
	node := filepath.Join(distribution, "ardents-node-linux-amd64")
	invoke := func(args ...string) ([]byte, error) {
		return exec.CommandContext(t.Context(), node, append([]string{"descriptor"}, args...)...).CombinedOutput()
	}
	history := filepath.Join(parent, "release-history")
	// Wrong independently supplied pin refuses before Release history or births.
	wrong := digest([]byte("foreign independent inventory"))
	if raw, err := invoke("install-initial-stopped", distribution, wrong, history, "alpha", "initial-stopped-descriptor", requestPath); err == nil {
		t.Fatal("foreign pin accepted", string(raw))
	}
	for _, path := range []string{history, request.InstallationRoot, request.StateRoot, request.ParticipationRoot, request.SpendRoot, request.HostingRoot, request.DescriptorRoot, request.CredentialDirectory} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("pin refusal created resource", path, err)
		}
	}
	if raw, err := invoke("inspect-inputs", requestPath); err != nil || !bytes.Contains(raw, []byte("descriptor-inputs-preflight-completed")) {
		t.Fatal("original read-only preflight", err, string(raw))
	}
	if raw, err := invoke("install-initial-stopped", distribution, digest(inventory), history, "alpha", "initial-stopped-descriptor", requestPath); err != nil || !bytes.Equal(raw, []byte("{\"outcome\":\"descriptor-initial-stopped-completed\",\"started\":false}\n")) {
		t.Fatal("actual Descriptor initial stopped installation", err, string(raw))
	}
	binding := filepath.Join(request.InstallationRoot, "binding.json")
	for range 2 {
		if raw, err := invoke("inspect-initial-history", distribution, history, "alpha", "initial-stopped-descriptor", binding); err != nil || !bytes.Contains(raw, []byte("descriptor-initial-history-verified")) {
			t.Fatal("independent original history", err, string(raw))
		}
	}
	for _, root := range []string{request.StateRoot, request.ParticipationRoot, request.SpendRoot, request.HostingRoot, request.DescriptorRoot} {
		entries, err := os.ReadDir(root)
		if err != nil || len(entries) != 0 {
			t.Fatal("initial installation initialized semantic state", root, err)
		}
	}
	journal := request.InstallationRoot + ".descriptor-initial-journal"
	before := read(journal)
	if raw, err := invoke("install-initial-stopped", distribution, digest(inventory), history, "alpha", "initial-stopped-descriptor", requestPath); err == nil {
		t.Fatal("completed original adopted as fresh installation", string(raw))
	}
	if !bytes.Equal(before, read(journal)) {
		t.Fatal("fresh retry rewrote original journal")
	}
	unit, err := exec.CommandContext(t.Context(), "systemctl", "show", "ardents-descriptor.service", "-p", "MainPID", "-p", "ActiveState", "-p", "SubState").CombinedOutput()
	if err != nil || !bytes.Contains(unit, []byte("MainPID=0\n")) || !bytes.Contains(unit, []byte("ActiveState=inactive\n")) || !bytes.Contains(unit, []byte("SubState=dead\n")) {
		t.Fatal("initial installation started a runtime", err, string(unit))
	}

	// A copied empty Store directory cannot replace the original recorded birth.
	// No Store data or floors have been initialized in this stopped boundary.
	storeInfo, err := os.Lstat(request.DescriptorRoot)
	if err != nil {
		t.Fatal(err)
	}
	retainedStore := filepath.Join(runtime, "original-empty-store")
	if err := os.Rename(request.DescriptorRoot, retainedStore); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(request.DescriptorRoot, 0700); err != nil {
		t.Fatal(err)
	}
	uid := storeInfo.Sys().(*syscall.Stat_t).Uid
	if err := os.Chown(request.DescriptorRoot, int(uid), int(uid)); err != nil {
		t.Fatal(err)
	}
	if raw, err := invoke("inspect-initial-history", distribution, history, "alpha", "initial-stopped-descriptor", binding); err == nil {
		t.Fatal("copied empty Descriptor Store accepted as the original birth", string(raw))
	}
	if err := os.Remove(request.DescriptorRoot); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(retainedStore, request.DescriptorRoot); err != nil {
		t.Fatal(err)
	}
	if raw, err := invoke("inspect-initial-history", distribution, history, "alpha", "initial-stopped-descriptor", binding); err != nil {
		t.Fatal("independent original Store history unavailable", err, string(raw))
	}
	if !bytes.Equal(before, read(journal)) {
		t.Fatal("Store refusal changed original installation history")
	}

	// A copied empty directory never supplies original Hosting initialization.
	hostingArgs := []string{"prepare-hosting", "--binding", binding, "--provider", "descriptor-native-provider", "--start", now.Add(-time.Minute).Format(time.RFC3339), "--end", now.Add(time.Hour).Format(time.RFC3339), "--unit", "GiB", "--quantity", "1", "--directions", "tx+rx", "--interfaces", "lo", "--initial-used-bytes", "0", "--low-watermark-bytes", "1024"}
	call(hostingArgs...)
	hostingJournal := request.InstallationRoot + ".descriptor-hosting-journal"
	originalHosting := read(hostingJournal)
	if raw, err := exec.CommandContext(t.Context(), command, append([]string{"descriptor"}, hostingArgs...)...).CombinedOutput(); err == nil {
		t.Fatal("second allowance initialization accepted", string(raw))
	}
	if !bytes.Equal(originalHosting, read(hostingJournal)) {
		t.Fatal("second initialization changed history")
	}
	var selected struct {
		Generation string `json:"generation"`
	}
	if json.Unmarshal(read(binding), &selected) != nil || selected.Generation == "" {
		t.Fatal("original selection")
	}
	planPath := filepath.Join(request.InstallationRoot, "generations", selected.Generation, "descriptor-plan.json")
	principal, err := os.Stat(request.StateRoot)
	if err != nil {
		t.Fatal(err)
	}
	actualUID := principal.Sys().(*syscall.Stat_t).Uid
	client := filepath.Join(parent, "ardents-control")
	if err := os.WriteFile(client, controlBytes, 0555); err != nil {
		t.Fatal(err)
	}
	bundlePath, profilePath := filepath.Join(parent, "bundle.bin"), filepath.Join(parent, "profile.bin")
	for path, raw := range map[string][]byte{bundlePath: bundle, profilePath: profile} {
		if err := os.WriteFile(path, raw, 0444); err != nil {
			t.Fatal(err)
		}
	}
	tick := func() error { at := time.Now(); return os.Chtimes(clock, at, at) }
	if err := tick(); err != nil {
		t.Fatal(err)
	}
	done, quit := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		timer := time.NewTicker(100 * time.Millisecond)
		defer timer.Stop()
		for {
			select {
			case <-quit:
				return
			case <-timer.C:
				if tick() != nil {
					return
				}
			}
		}
	}()
	t.Cleanup(func() { close(quit); <-done })
	prepareState := exec.CommandContext(t.Context(), client, "descriptor", "prepare-state", planPath, bundlePath, profilePath)
	prepareState.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: actualUID, Gid: actualUID, Groups: []uint32{}}, Pdeathsig: syscall.SIGKILL}
	if raw, err := prepareState.CombinedOutput(); err != nil {
		t.Fatal("actual non-root offline State", err, string(raw))
	}
	var exchangeCmd *exec.Cmd
	var exchangeCancel context.CancelFunc
	var exchangeLog *os.File
	exchangeJoined := false
	joinExchange := func() {
		if exchangeCmd == nil || exchangeJoined {
			return
		}
		exchangeCancel()

		err := exchangeCmd.Wait()
		exchangeJoined = true
		closeErr := exchangeLog.Close()
		if err != nil || closeErr != nil {
			t.Error("original exchange helper joined with failure", err, closeErr, string(read(filepath.Join(parent, "exchange-diagnostic.log"))))
		}
	}
	if exchange {
		raw, _ := json.Marshal(struct {
			Authority, Peer        [32]byte
			Lifecycle, Replacement bool
		}{authority, peer, lifecycle, replacement})
		if err := os.WriteFile(filepath.Join(parent, "exchange-inputs.json"), append(raw, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
		exchangeCtx, cancel := context.WithCancel(t.Context())
		exchangeCancel = cancel
		exchangeCmd = exec.CommandContext(exchangeCtx, harness, "-test.run=^TestDescriptorInstalledNativeExchange$", "-test.count=1", "-test.v", "-test.timeout=3m")
		exchangeLog, err = os.OpenFile(filepath.Join(parent, "exchange-diagnostic.log"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			t.Fatal(err)
		}
		exchangeCmd.Stdout, exchangeCmd.Stderr = exchangeLog, exchangeLog
		if err := exchangeCmd.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(joinExchange)
		until := time.Now().Add(30 * time.Second)
		for {
			if _, err := os.Stat(filepath.Join(parent, "exchange-prepared.json")); err == nil {
				break
			} else if !errors.Is(err, os.ErrNotExist) || time.Now().After(until) {
				t.Fatal("exchange preparation unavailable", err, string(read(filepath.Join(parent, "exchange-diagnostic.log"))))
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	// Direct execution cannot manufacture a systemd invocation or root-peer ACK.
	directProgram := filepath.Join(request.InstallationRoot, "generations", selected.Generation, "ardents-node-linux-amd64")
	direct := exec.CommandContext(t.Context(), directProgram, "descriptor", "start-installed", binding)
	direct.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: actualUID, Gid: actualUID, Groups: []uint32{}}, Pdeathsig: syscall.SIGKILL}
	if raw, err := direct.CombinedOutput(); err == nil {
		t.Fatal("uninstalled process accepted", string(raw))
	}
	// Freeze public commitments before any installed execution. The runner
	// records the Product Owner's delegation against these actual inputs.
	scopeRoot := os.Getenv("ARDENTS_DESCRIPTOR_BOOTSTRAP_DIRECTORY")
	if !filepath.IsAbs(scopeRoot) {
		t.Fatal("invalid environment: original bootstrap evidence directory")
	}
	scope := struct {
		Schema   string            `json:"schema"`
		Targets  []string          `json:"targets"`
		Programs map[string]string `json:"programs"`
		Inputs   map[string]string `json:"inputs"`
		Limits   string            `json:"limits"`
	}{"ardents-descriptor-installed-bootstrap-v1", []string{"ardents/linux-amd64/node", "ardents/linux-amd64/infrastructure/descriptor"}, map[string]string{"node": digest(nodeBytes), "control": digest(controlBytes)}, map[string]string{}, "one controlled VPS/cohort; TCP/TLS only; protected first startup only; no Store ACK, full lifecycle, independent family/operator/builder, public, load, privacy or platform qualification"}
	for name, path := range map[string]string{"binding": binding, "plan": planPath, "inventory": filepath.Join(distribution, "infrastructure-enrollment.json"), "bundle": bundlePath, "profile": profilePath, "hosting": hostingJournal} {
		scope.Inputs[name] = digest(read(path))
	}
	if exchange {
		scope.Programs["exchange-harness"] = digest(read(harness))
		scope.Inputs["exchange"] = digest(read(filepath.Join(parent, "exchange-prepared.json")))
		scope.Limits = "one controlled VPS/cohort; TCP/TLS only; actual Descriptor Store publish/ACK/lookup and conflict floors; public signed input is not registered Introduction, live Publisher or Service readiness; four independent refusal requests, at most two concurrent, join before dependent operations; unchanged runtime guards and per-channel absolute deadlines; no retained lifecycle, independent operator/builder, public/load/privacy/platform qualification"
	}
	if lifecycle {
		scope.Limits = "one controlled VPS/cohort; TCP/TLS only; genuine Descriptor exchange, explicit joined Stop and same-generation retained restart; actual preserved histories and fresh original Startup; unchanged protections and deadlines; no replacement/recovery/Service/public/load/privacy qualification"
	}
	if replacement {
		scope.Limits = "one controlled VPS/cohort; TCP/TLS only; genuine Descriptor replacement prerequisite with fresh signed proof inputs bounded to 600 seconds and explicit joined Stop; actual preserved histories and original Startup; unchanged protections and operation deadlines; no replacement/recovery/Service/public/load/privacy qualification"
	}
	prepared, err := json.Marshal(scope)
	if err != nil {
		t.Fatal(err)
	}
	prepared = append(prepared, '\n')
	scopeFile := filepath.Join(scopeRoot, "prepared.json")
	f, err := os.OpenFile(scopeFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(prepared); err != nil {
		t.Fatal(err)
	}
	if err := f.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	acceptanceDeadline := time.Now().Add(time.Minute)
	for {
		raw, err := os.ReadFile(filepath.Join(scopeRoot, "accepted.json"))
		if err == nil {
			var receipt struct {
				Prepared   string `json:"prepared_sha256"`
				Delegation string `json:"delegation"`
			}
			if json.Unmarshal(raw, &receipt) != nil || receipt.Prepared != digest(prepared) || receipt.Delegation != "product-owner-548-descriptor-startup-no-new-agreements" {
				t.Fatal("exact bootstrap delegation mismatch")
			}
			break
		}
		if !errors.Is(err, os.ErrNotExist) || time.Now().After(acceptanceDeadline) {
			t.Fatal("completed delegated bootstrap receipt unavailable", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if raw, err := invoke("start-initial", distribution, history, "alpha", "initial-stopped-descriptor", binding); err != nil {
		t.Fatal("actual protected Start", err, string(raw))
	}
	nativeStopped := false
	defer func() {
		joinExchange()
		if nativeStopped {
			return
		}
		raw, err := exec.CommandContext(context.WithoutCancel(t.Context()), "systemctl", "stop", "ardents-descriptor.service").CombinedOutput()
		if err != nil {
			t.Error("native stop", err, string(raw))
		}
	}()
	invocationRaw, err := exec.CommandContext(t.Context(), "systemctl", "show", "ardents-descriptor.service", "-p", "InvocationID", "--value").CombinedOutput()
	invocation := strings.TrimSpace(string(invocationRaw))
	if err != nil || len(invocation) != 32 {
		t.Fatal("original invocation missing", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		journal, err := exec.CommandContext(t.Context(), "journalctl", "_SYSTEMD_INVOCATION_ID="+invocation, "--no-pager", "-o", "cat").CombinedOutput()
		if err == nil && bytes.Contains(journal, []byte(`"phase":"listening"`)) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("actual Descriptor runtime did not become ready", err, string(journal))
		}
		time.Sleep(20 * time.Millisecond)
	}
	facts, err := exec.CommandContext(t.Context(), "systemctl", "show", "ardents-descriptor.service", "-p", "User", "-p", "MainPID", "-p", "ActiveState", "-p", "SubState").CombinedOutput()
	if err != nil || !bytes.Contains(facts, []byte("User=ardents-node\n")) || !bytes.Contains(facts, []byte("ActiveState=active\n")) || bytes.Contains(facts, []byte("MainPID=0\n")) {
		t.Fatal("non-root installed invocation absent", err, string(facts))
	}
	for _, path := range []string{request.ParticipationRoot, request.SpendRoot, request.DescriptorRoot} {
		entries, err := os.ReadDir(path)
		if err != nil || len(entries) == 0 {
			t.Fatal("genuine owner was not initialized", path, err)
		}
	}
	if raw, err := invoke("start-initial", distribution, history, "alpha", "initial-stopped-descriptor", binding); err == nil {
		t.Fatal("second first Start accepted", string(raw))
	}
	if exchange {
		if err := os.WriteFile(filepath.Join(parent, "exchange-start"), []byte("original installed Descriptor ready\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if lifecycle && !replacement {
			until := time.Now().Add(time.Minute)
			for {
				if _, err := os.Stat(filepath.Join(parent, "exchange-phase-one")); err == nil {
					break
				}
				if time.Now().After(until) {
					t.Fatal("initial exchange unavailable", string(read(filepath.Join(parent, "exchange-diagnostic.log"))))
				}
				time.Sleep(20 * time.Millisecond)
			}
			if raw, err := invoke("restart", distribution, history, "alpha", "initial-stopped-descriptor", binding); err == nil {
				t.Fatal("restart before original domain/native retirement accepted", string(raw))
			}
			if _, err := os.Lstat(request.InstallationRoot + ".descriptor-invocation-00000001-journal"); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("refused restart created an invocation", err)
			}
			storePath := filepath.Join(request.DescriptorRoot, "records")
			entries, err := os.ReadDir(storePath)
			if err != nil || len(entries) != 1 {
				t.Fatal("one actual stored floor required", err)
			}
			stored := filepath.Join(storePath, entries[0].Name())
			before := read(stored)
			firstUID := actualUID
			credential := read(request.PrivateKey)
			if raw, err := invoke("stop", history, binding); err != nil || !bytes.Contains(raw, []byte("descriptor-original-stop-completed")) {
				t.Fatal("explicit original joined Stop", err, string(raw))
			}
			if !bytes.Equal(before, read(stored)) {
				t.Fatal("Stop altered original Store bytes")
			}
			if raw, err := invoke("stop", history, binding); err == nil {
				t.Fatal("retirement report replay accepted", string(raw))
			}
			for range 2 {
				if raw, err := invoke("inspect-started-history", history, binding); err != nil {
					t.Fatal("independent retirement readback", err, string(raw))
				}
			}
			if raw, err := invoke("restart", distribution, history, "alpha", "initial-stopped-descriptor", binding); err != nil {
				t.Fatal("fresh same-generation restart", err, string(raw))
			}
			if !bytes.Equal(before, read(stored)) || !bytes.Equal(credential, read(request.PrivateKey)) {
				t.Fatal("restart rewrote Store or private Node key")
			}
			clear(credential)
			info, err := os.Stat(request.StateRoot)
			if err != nil || info.Sys().(*syscall.Stat_t).Uid != firstUID {
				t.Fatal("restart replaced original account", err)
			}
			newIDRaw, err := exec.CommandContext(t.Context(), "systemctl", "show", "ardents-descriptor.service", "-p", "InvocationID", "--value").CombinedOutput()
			newID := strings.TrimSpace(string(newIDRaw))
			if err != nil || len(newID) != 32 || newID == invocation {
				t.Fatal("independent new invocation absent", err)
			}
			restarted := time.Now().Add(15 * time.Second)
			for {
				raw, err := exec.CommandContext(t.Context(), "journalctl", "_SYSTEMD_INVOCATION_ID="+newID, "-o", "cat", "--no-pager").CombinedOutput()
				if err == nil && bytes.Contains(raw, []byte(`"phase":"listening"`)) {
					break
				}
				if time.Now().After(restarted) {
					t.Fatal("retained invocation not ready", err, string(raw))
				}
				time.Sleep(20 * time.Millisecond)
			}
			if err := os.WriteFile(filepath.Join(parent, "exchange-restarted"), []byte("original retained Descriptor ready\n"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		err := exchangeCmd.Wait()
		exchangeJoined = true
		closeErr := exchangeLog.Close()
		exchangeCancel()
		raw := read(filepath.Join(parent, "exchange-diagnostic.log"))
		t.Log(string(raw))
		if err != nil || closeErr != nil {
			t.Fatal("actual installed Descriptor publish/ACK/lookup", err, closeErr)
		}
	}
	if exchange {
		// This is native test teardown only; it cannot supply a product Stop or
		// retirement transcript. The independent Store reader needs actual exit.
		if lifecycle {
			if raw, err := invoke("stop", history, binding); err != nil {
				t.Fatal("retained invocation original Stop", err, string(raw))
			}
			if raw, err := invoke("inspect-started-history", history, binding); err != nil {
				t.Fatal("retained invocation readback", err, string(raw))
			}
		}
		if raw, err := exec.CommandContext(t.Context(), "systemctl", "stop", "ardents-descriptor.service").CombinedOutput(); err != nil {
			t.Fatal("native teardown before independent Store reopen", err, string(raw))
		}
		nativeStopped = true
		program := filepath.Join(parent, "descriptor-exchange.test")
		if err := os.WriteFile(program, read(harness), 0555); err != nil {
			t.Fatal(err)
		}
		reopen := exec.CommandContext(t.Context(), program, "-test.run=^TestDescriptorInstalledNativeStoreReopen$", "-test.count=1", "-test.v", "-test.timeout=30s")
		reopen.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: actualUID, Gid: actualUID, Groups: []uint32{}}, Pdeathsig: syscall.SIGKILL}
		if raw, err := reopen.CombinedOutput(); err != nil {
			t.Fatal("independent non-root Store reopen", err, string(raw))
		} else {
			t.Log(string(raw))
		}
		t.Log("actual installed signed publication/Store ACK/exact lookup and retained Store reopen; product Stop/restart remain separate")
	} else {
		t.Log("actual protected first Start, independent non-root Startup, genuine Descriptor owners initialized; no Store ACK or lifecycle completion claimed")
	}

}
