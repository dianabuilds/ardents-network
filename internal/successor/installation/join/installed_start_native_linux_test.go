//go:build linux && join_installed_native

package join_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/custody"
	joininstallation "github.com/dianabuilds/ardents-network/internal/successor/installation/join"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Original installed first-start composition; no development Node, fixture
// authority, foreign role runtime or fabricated successful State observation.
func TestJoinInstalledNativeProtectedStartupAndPair(t *testing.T) {
	marker, err := os.ReadFile("/etc/ardents-join-initial-native-stand")
	if err != nil || string(marker) != "ardents-join-initial-native-v1\n" || os.Geteuid() != 0 {
		t.Fatal("invalid environment: isolated initial JOIN stand required", err)
	}
	const parent = "/var/lib/join-installed"
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
	command, controlBytes := joinNativePinnedProgram(t, "ARDENTS_JOIN_CONTROL_PROGRAM")
	_, nodeBytes := joinNativePinnedProgram(t, "ARDENTS_JOIN_NODE_PROGRAM")
	preparer, _ := joinNativePinnedProgram(t, "ARDENTS_JOIN_RELEASE_PREPARER")
	evidence, _ := joinNativePinnedProgram(t, "ARDENTS_JOIN_BUILD_EVIDENCE")
	buildRoot := os.Getenv("ARDENTS_JOIN_BUILD_ROOT")
	read := func(path string) []byte {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	digest := func(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
	if !filepath.IsAbs(buildRoot) || digest(read(filepath.Join(buildRoot, "READY"))) != os.Getenv("ARDENTS_JOIN_BUILD_READY_SHA256") {
		t.Fatal("invalid environment: actual two-build commitment required")
	}
	for _, number := range []string{"build-1", "build-2"} {
		if !bytes.Equal(read(filepath.Join(buildRoot, number, "ardents-node")), nodeBytes) || !bytes.Equal(read(filepath.Join(buildRoot, number, "ardents-control")), controlBytes) {
			t.Fatal("canonical commands differ from actual build records")
		}
	}
	call := func(args ...string) []byte {
		t.Helper()
		raw, err := exec.CommandContext(t.Context(), command, append([]string{"join"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatal("canonical JOIN producer", args[0], err, string(raw))
		}
		return raw
	}
	templateRoot, descriptorRoot := filepath.Join(delivery, "template"), filepath.Join(delivery, "descriptor")
	call("prepare-template", templateRoot)
	template := filepath.Join(templateRoot, "ardents-join.service")
	call("prepare-descriptor", "initial-stopped-join", "1", filepath.Join(buildRoot, "build-1", "ardents-node"), template, descriptorRoot)
	descriptor := filepath.Join(descriptorRoot, "infrastructure-join.json")
	now := time.Now().UTC().Truncate(time.Second)
	planOutput := filepath.Join(delivery, "public-plan")
	args := []string{"-I", "-S", preparer, "--build-root", buildRoot, "--descriptor", descriptor, "--template", template, "--network", "initial-stopped-join", "--expires", now.Add(time.Hour).Format(time.RFC3339), "--no-new-work-after", now.Add(30 * time.Minute).Format(time.RFC3339), "--terminate-after", now.Add(45 * time.Minute).Format(time.RFC3339), "--qualification-evidence", evidence, "--qualification-evidence-sha256", os.Getenv("ARDENTS_JOIN_BUILD_EVIDENCE_SHA256"), "--output", planOutput}
	if raw, err := exec.CommandContext(t.Context(), "python3", args...).CombinedOutput(); err != nil {
		t.Fatal("actual build-bound JOIN plan", err, string(raw))
	}
	keys, metadataRoot := filepath.Join(delivery, "release-keys"), filepath.Join(delivery, "metadata")
	call("prepare-release-keys", keys)
	call("sign-initial-release", filepath.Join(planOutput, "join-release-plan.json"), keys, metadataRoot, filepath.Join(delivery, "verification"))
	distribution := filepath.Join(delivery, "distribution")
	if err := os.Mkdir(distribution, 0700); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{"ardents-node-linux-amd64": nodeBytes, "ardents-join.service": read(template), "infrastructure-join.json": read(descriptor)}
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
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		t.Fatal(err)
	}
	password := []byte(hex.EncodeToString(secret))
	clear(secret)
	defer clear(password)
	secrets := joinTokenTestSecrets{password}
	vault, err := custody.Open(custody.VaultConfig{Root: filepath.Join(parent, "permission-vault")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := vault.Close(); err != nil {
			t.Error(err)
		}
	})
	created, err := vault.Execute(t.Context(), custody.Operation{Kind: custody.OperationCreateAdmissionAuthority, Binding: custody.AuthorityBinding{Environment: sha256.Sum256([]byte("bounded JOIN native pair")), Network: [32]byte{1}, Root: sha256.Sum256([]byte("separate permission authority")), Kind: custody.AuthorityAdmission}}, secrets)
	if err != nil {
		t.Fatal(err)
	}
	bundle, profile, authority, peer := prepareJoinStateNativeCohort(t, command, parent, created.AdmissionAuthority.Public)
	preparation := filepath.Join(parent, "canonical-preparation")
	recordPlan := filepath.Join(preparation, "record-plan-03", "node-record-plan.json")
	record := filepath.Join(preparation, "record-03", "node-record.bin")
	certificateOutput := filepath.Join(delivery, "node-certificate")
	call("prepare-node-certificate", recordPlan, record, digest(read(record)), filepath.Join(preparation, "node-key-03", "node-key.pem"), certificateOutput)
	certPath, keyPath := filepath.Join(certificateOutput, "node-certificate.pem"), filepath.Join(preparation, "node-key-03", "node-key.pem")
	peerText := hex.EncodeToString(peer[:])
	peerRecord := filepath.Join(preparation, "record-"+peerText[:2], "node-record.bin")
	peerCertificateOutput := filepath.Join(delivery, "peer-certificate")
	call("prepare-node-certificate", filepath.Join(preparation, "record-plan-"+peerText[:2], "node-record-plan.json"), peerRecord, digest(read(peerRecord)), filepath.Join(preparation, "node-key-"+peerText[:2], "node-key.pem"), peerCertificateOutput)
	peerCertificate, err := tls.LoadX509KeyPair(filepath.Join(peerCertificateOutput, "node-certificate.pem"), filepath.Join(preparation, "node-key-"+peerText[:2], "node-key.pem"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if key, ok := peerCertificate.PrivateKey.(ed25519.PrivateKey); ok {
			clear(key)
		}
		peerCertificate.PrivateKey = nil
	}()

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
	call("prepare-request", "--installation-root", filepath.Join(runtime, "node"), "--state-root", filepath.Join(runtime, "state"), "--participation-root", filepath.Join(runtime, "participation"), "--spend-root", filepath.Join(runtime, "spend"), "--hosting-root", filepath.Join(runtime, "hosting"), "--credential-directory", filepath.Join(runtime, "credentials"), "--plan", filepath.Join(runtimePlan, "join-plan.json"), "--certificate", certPath, "--private-key", keyPath, "--clock", clock, "--output", requestOutput)
	requestPath := filepath.Join(requestOutput, "join-request.json")
	request, err := joininstallation.DecodeRequest(read(requestPath))
	if err != nil {
		t.Fatal(err)
	}
	network, nodeID := [32]byte{1}, [32]byte{3}
	call("prepare-plan", "--request", requestPath, "--output", runtimePlan, "--network-id", hex.EncodeToString(network[:]), "--node-id", hex.EncodeToString(nodeID[:]), "--profile-authority", hex.EncodeToString(authority[:]), "--authority", hex.EncodeToString(authority[:]), "--threshold", "1", "--work-tx", "134217728", "--work-rx", "134217728", "--termination-tx", "1048576", "--termination-rx", "1048576", "--retirement-wait-ms", "5000")
	node := filepath.Join(distribution, "ardents-node-linux-amd64")
	invoke := func(args ...string) ([]byte, error) {
		return exec.CommandContext(t.Context(), node, append([]string{"join"}, args...)...).CombinedOutput()
	}
	history := filepath.Join(parent, "release-history")
	// Wrong independently supplied pin refuses before Release history or births.
	wrong := digest([]byte("foreign independent inventory"))
	if raw, err := invoke("install-initial-stopped", distribution, wrong, history, "alpha", "initial-stopped-join", requestPath); err == nil {
		t.Fatal("foreign pin accepted", string(raw))
	}
	for _, path := range []string{history, request.InstallationRoot, request.StateRoot, request.ParticipationRoot, request.SpendRoot, request.HostingRoot, request.CredentialDirectory} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("pin refusal created resource", path, err)
		}
	}
	if raw, err := invoke("inspect-inputs", requestPath); err != nil || !bytes.Contains(raw, []byte("join-inputs-preflight-completed")) {
		t.Fatal("original read-only preflight", err, string(raw))
	}
	if raw, err := invoke("install-initial-stopped", distribution, digest(inventory), history, "alpha", "initial-stopped-join", requestPath); err != nil || !bytes.Equal(raw, []byte("{\"outcome\":\"join-initial-stopped-completed\",\"started\":false}\n")) {
		t.Fatal("actual JOIN initial stopped installation", err, string(raw))
	}
	binding := filepath.Join(request.InstallationRoot, "binding.json")
	hostingArgs := []string{"prepare-hosting", "--binding", binding, "--provider", "join-native-provider", "--start", now.Add(-time.Minute).Format(time.RFC3339), "--end", now.Add(time.Hour).Format(time.RFC3339), "--unit", "GiB", "--quantity", "1", "--directions", "tx+rx", "--interfaces", "lo", "--initial-used-bytes", "0", "--low-watermark-bytes", "1024"}
	if raw := call(hostingArgs...); !bytes.Contains(raw, []byte("join.prepare-hosting")) {
		t.Fatal("missing hosting preparation result")
	}

	var selected struct {
		Generation string `json:"generation"`
	}
	if err := json.Unmarshal(read(binding), &selected); err != nil || selected.Generation == "" {
		t.Fatal("selected generation", err)
	}
	planPath := filepath.Join(request.InstallationRoot, "generations", selected.Generation, "join-plan.json")
	principal, err := os.Stat(request.StateRoot)
	if err != nil {
		t.Fatal(err)
	}
	uid := principal.Sys().(*syscall.Stat_t).Uid
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
	// Root-controlled actual wall-clock observations. No supplied confidence or
	// rewritten time floor; both producer and runtime independently read mtime.
	tick := func() error { at := time.Now(); return os.Chtimes(clock, at, at) }
	if err := tick(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	quit := make(chan struct{})
	go func() {
		defer close(done)
		timer := time.NewTicker(100 * time.Millisecond)
		defer timer.Stop()
		for {
			select {
			case <-quit:
				return
			case <-timer.C:
				if err := tick(); err != nil {
					return
				}
			}
		}
	}()
	t.Cleanup(func() { close(quit); <-done })
	stateProducer := exec.CommandContext(t.Context(), client, "join", "prepare-state", planPath, bundlePath, profilePath)
	stateProducer.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: uid, Gid: uid, Groups: []uint32{}}, Pdeathsig: syscall.SIGKILL}
	if raw, err := stateProducer.CombinedOutput(); err != nil {
		t.Fatal("canonical actual non-root State", err, string(raw))
	}
	observer, _ := joinNativePinnedProgram(t, "ARDENTS_JOIN_SYSTEMD_OBSERVER")
	if raw, err := exec.CommandContext(t.Context(), observer, "-test.run=^TestJoinSlotsNativeConcurrentQueriesMatchSequential$", "-test.count=1", "-test.v", "-test.timeout=30s").CombinedOutput(); err != nil {
		t.Fatal("actual sequential/concurrent manager canary comparison", err, string(raw))
	} else {
		t.Log(string(raw))
	}
	stock := prepareJoinIssuedStock(t, parent, vault, created, secrets, bundle, profile, authority, 2, peer)
	// The external retained supervisor commits exact source/build/artifact/input
	// hashes under the Product Owner's bounded campaign delegation before Start.
	inputHashes := map[string]string{}
	for _, root := range []string{delivery, runtime, preparation, bundlePath, profilePath} {
		if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.Type().IsRegular() {
				inputHashes[path] = digest(read(path))
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	prepared, err := json.Marshal(struct {
		Schema                         string            `json:"schema"`
		Inputs                         map[string]string `json:"inputs"`
		Binding, Distribution, History string
	}{"ardents-join-installed-prepared-v1", inputHashes, binding, distribution, history})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "prepared.json"), prepared, 0600); err != nil {
		t.Fatal(err)
	}
	until := time.Now().Add(30 * time.Second)
	for {
		receipt, err := os.ReadFile(filepath.Join(parent, "bootstrap-scope.json"))
		if err == nil {
			var grant struct {
				Schema         string `json:"schema"`
				PreparedSHA256 string `json:"prepared_sha256"`
				Delegation     string `json:"delegation"`
			}
			if json.Unmarshal(receipt, &grant) != nil || grant.Schema != "ardents-join-installed-bootstrap-v1" || grant.PreparedSHA256 != digest(prepared) || grant.Delegation != "selected-545-startup-pair" {
				t.Fatal("exact bootstrap scope mismatch")
			}
			break
		}
		if !errors.Is(err, os.ErrNotExist) || time.Now().After(until) {
			t.Fatal("completed bootstrap receipt unavailable", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if raw, err := invoke("start-initial", distribution, history, "alpha", "initial-stopped-join", binding); err != nil {
		t.Fatal("actual protected first Start", err, string(raw))
	}
	t.Log("root first Start and independent non-root root-peer Startup completed")
	// Failed root launch and command cleanup never supply retirement permission.
	// This bounded stage joins the actual native invocation explicitly, without
	// claiming the still-unimplemented persisted domain retirement/restart proof.
	defer func() {
		raw, err := exec.CommandContext(context.WithoutCancel(t.Context()), "systemctl", "stop", "ardents-join.service").CombinedOutput()
		if err != nil {
			t.Error("actual native stop", err, string(raw))
		}
		raw, err = exec.CommandContext(context.WithoutCancel(t.Context()), "systemctl", "show", "ardents-join.service", "-p", "MainPID", "-p", "ActiveState", "-p", "SubState").CombinedOutput()
		if err != nil || !bytes.Contains(raw, []byte("MainPID=0\n")) {
			t.Error("native process not joined", err, string(raw))
		}
	}()
	// Native listening-socket observation creates no speculative TLS client
	// and sends no unauthenticated probe into the original role.
	until = time.Now().Add(10 * time.Second)
	for {
		listening := false
		for _, line := range strings.Split(string(read("/proc/net/tcp")), "\n") {
			fields := strings.Fields(line)
			if len(fields) > 3 && fields[1] == "0100007F:1132" && fields[3] == "0A" {
				listening = true
				break
			}
		}
		if listening {
			break
		}
		if time.Now().After(until) {
			t.Fatal("installed listener absent")
		}
		time.Sleep(20 * time.Millisecond)
	}
	runJoinInstalledPair(t, stock, peerCertificate)
	t.Log("actual installed pair: two admissions, both RESULTs, duplex bytes, directional EOF and CLOSE0")
}

func joinNativePinnedProgram(t *testing.T, variable string) (string, []byte) {
	t.Helper()
	path, pin := os.Getenv(variable), os.Getenv(variable+"_SHA256")
	raw, err := os.ReadFile(path)
	digest := sha256.Sum256(raw)
	if err != nil || !filepath.IsAbs(path) || len(pin) != 64 || hex.EncodeToString(digest[:]) != pin {
		t.Fatal("invalid environment: original independently pinned program required", variable, err)
	}
	return path, raw
}
