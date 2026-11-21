//go:build linux && descriptor_platform_native

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/processresource"
	"github.com/dianabuilds/ardents-network/internal/reachability"
	"github.com/dianabuilds/ardents-network/internal/release/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/route/transport"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// Installed Descriptor operator acceptance (#604 FJD stage C) against the REAL
// platform systemd manager: the mandatory privileged platform check runs as the
// actual ExecStartPre of ardents-descriptor.service, node descriptor
// --retained-config runs as the shared fixed ardents-node main process, and
// stop/relaunch/forward program change/crash recovery follow the operator
// procedure. The crash leg uses the bounded node descriptor repair-retained
// (stage B) against real failed-unit manager facts and per-unit cgroup
// quiescence. This is the native installed evidence gate before the transitional
// Descriptor orchestration retires (stage D, ADR-0125).
//
// The first installation runs the fresh --config composition under a transient
// envelope to create the participation and receiving histories; the installed
// unit then opens them retained. The lifecycle/repair legs are single-unit: one
// Descriptor member (id 17, domain 2, subrole 5) owns its own participation root
// and sole node-duty fence plus its own Store root (the fifth writable root).
// Honest limits: one fixture Release authority and one operator, not independent
// participants or whole-host privacy qualification. The forward executable is a
// distinct build of the same checked source. The readiness oracle is the
// authenticated listener at the signed member Endpoint, NOT publication
// readiness; genuine publish/lookup is proven by
// TestNativeDescriptorPublishLookupAndReopenTCP. The transitional Descriptor
// orchestration is not exercised or retired here.
//
// The causal mid-Publish-crash leg simulates the uncertain Store stage honestly.
// A genuine signed publication is first committed through the real Store owner
// (reachability.OpenStore + Publish, verify-before-mutate/persist-before-expose)
// while the unit is cleanly stopped, then the installed retained runtime reopens
// that floor, is SIGKILLed, and the reserved .record-stage name is written
// directly into the records directory while the unit is failed. The stand cannot
// deterministically interrupt the serialized network writer mid-Publish (that
// path needs the full Route-prefix/Forwarding workload, proven separately by
// TestNativeDescriptorPublishLookupAndReopenTCP), so the staging file is a
// bounded fixture of exactly the durable shape a crash mid-Publish leaves:
// staging is never a committed proof. A plain retained start refuses the
// uncertain stage; the bounded repair validates the committed floor, removes only
// the staging under the exclusive Store lease and removes the producer; the
// post-repair launch opens the Store cleanly and listens, and the committed
// publication still resolves byte-exact.
type descriptorPlatformStand struct {
	ns                string
	binaryBytes       []byte
	base              string
	distribution      string
	releaseRoot       string
	programDir        string
	program           string
	etcDir            string
	rolesBase         string
	stateRoot         string
	participationRoot string
	spendRoot         string
	hostingRoot       string
	storeRoot         string
	clockPath         string
	planPath          string
	launchPath        string
	certPath          string
	keyPath           string
	launch            platformLaunch
	pin               string
	unit              []byte
	uid               int
	authority         *consumerReleaseAuthority
	networkPlan       networkAuthorityPlan
	address           string
	nodeID            [32]byte
	familyID          [32]byte
	fixture           *networkAdmissionFixture
	sockets           map[[32]byte]func()
	certificates      map[[32]byte]tls.Certificate
}

func TestNativeDescriptorPlatformInstalledLifecycle(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("Descriptor installed qualification requires a root driver")
	}
	if _, err := exec.Command("/usr/bin/busctl", "--system", "--no-pager", "call", "org.freedesktop.systemd1", "/org/freedesktop/systemd1", "org.freedesktop.DBus.Properties", "Get", "ss", "org.freedesktop.systemd1.Manager", "Version").Output(); err != nil {
		t.Fatal("real platform systemd manager unavailable:", err)
	}
	stand := provisionDescriptorPlatformStand(t)

	// First installed launch: the real ExecStartPre check accepts the genuine
	// provisioned delivery against the loaded manager, then the retained runtime
	// claims the original node-duty producer and listens.
	startDescriptorPlatformUnit(t)
	waitDescriptorPlatformReady(t, stand, "first launch")
	descriptorPlatformProducerPresent(t, stand, true, "first launch")
	if out, err := runAsDescriptorNode(t, stand.program, "node", "descriptor", "repair-retained", stand.planPath); err == nil {
		t.Fatal("repair accepted a live invocation", string(out))
	}
	stopDescriptorPlatformUnit(t, "first launch")
	descriptorPlatformProducerPresent(t, stand, false, "first completion")
	if out, err := runAsDescriptorNode(t, stand.program, "node", "descriptor", "repair-retained", stand.planPath); err == nil {
		t.Fatal("repair accepted a clean root", string(out))
	}

	// Retained relaunch over the same histories.
	startDescriptorPlatformUnit(t)
	waitDescriptorPlatformReady(t, stand, "relaunch")
	stopDescriptorPlatformUnit(t, "relaunch")
	descriptorPlatformProducerPresent(t, stand, false, "relaunch completion")

	// Forward program change: a distinct working executable authenticated against
	// the retained floors through the same mandatory pre-start check.
	forwardBytes := compiledDescriptorPlatformCommand(t)
	if err := os.RemoveAll(stand.distribution); err != nil {
		t.Fatal(err)
	}
	infrastructureDistributionInvocation(t, stand.base, forwardBytes, stand.authority, 2, nil, "descriptor", []byte(platformUnitTemplate))
	if err := os.WriteFile(stand.program, forwardBytes, 0555); err != nil {
		t.Fatal(err)
	}
	startDescriptorPlatformUnit(t)
	waitDescriptorPlatformReady(t, stand, "forward launch")
	stopDescriptorPlatformUnit(t, "forward launch")
	// Rollback to the committed predecessor must refuse before serving.
	if err := os.RemoveAll(stand.distribution); err != nil {
		t.Fatal(err)
	}
	infrastructureDistributionInvocation(t, stand.base, stand.binaryBytes, stand.authority, 1, nil, "descriptor", []byte(platformUnitTemplate))
	if err := os.WriteFile(stand.program, stand.binaryBytes, 0555); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.CommandContext(t.Context(), "systemctl", "start", "ardents-descriptor.service").CombinedOutput(); err == nil {
		t.Fatal("backward program change accepted", string(out))
	}
	if err := os.RemoveAll(stand.distribution); err != nil {
		t.Fatal(err)
	}
	infrastructureDistributionInvocation(t, stand.base, forwardBytes, stand.authority, 2, nil, "descriptor", []byte(platformUnitTemplate))
	if err := os.WriteFile(stand.program, forwardBytes, 0555); err != nil {
		t.Fatal(err)
	}

	// Crash recovery: a killed invocation supplies no completion evidence; the
	// live node-duty fence stays retained, a plain start refuses, and the bounded
	// repair against real manager facts removes exactly that producer.
	startDescriptorPlatformUnit(t)
	waitDescriptorPlatformReady(t, stand, "crash launch")
	descriptorPlatformProducerPresent(t, stand, true, "crash launch")
	if err := exec.Command("systemctl", "kill", "--kill-whom=main", "--signal=SIGKILL", "ardents-descriptor.service").Run(); err != nil {
		t.Fatal("original crash signal", err)
	}
	waitDescriptorPlatformFailed(t)
	descriptorPlatformProducerPresent(t, stand, true, "after crash")
	// A plain start must refuse the retained producer. Type=exec returns once the
	// main process forks, so the runtime's asynchronous refusal is observed as the
	// unit reaching failed state (an adopting runtime would stay active).
	_ = exec.CommandContext(t.Context(), "systemctl", "start", "ardents-descriptor.service").Run()
	waitDescriptorPlatformFailed(t)
	descriptorPlatformProducerPresent(t, stand, true, "after refused relaunch")
	out, err := runAsDescriptorNode(t, stand.program, "node", "descriptor", "repair-retained", stand.planPath)
	if err != nil || !bytes.Contains(out, []byte("repaired")) {
		t.Fatal("bounded repair against real manager facts", string(out), err)
	}
	descriptorPlatformProducerPresent(t, stand, false, "after repair")
	startDescriptorPlatformUnit(t)
	waitDescriptorPlatformReady(t, stand, "post-repair launch")
	stopDescriptorPlatformUnit(t, "post-repair launch")

	// Causal mid-Publish-crash stage recovery (option A). Commit a genuine signed
	// floor through the real Store owner while the unit is stopped, reopen it
	// through the installed retained runtime, kill the invocation and leave the
	// reserved uncertain staging name in the records directory exactly as a crash
	// mid-Publish would. A plain retained start refuses the uncertain stage; the
	// bounded repair validates the committed floor first, removes only the staging
	// under the exclusive Store lease and removes the producer; the post-repair
	// launch opens the Store cleanly and listens, and the committed publication is
	// preserved byte-exact.
	raw, target, profile := commitDescriptorPlatformFloor(t, stand)
	startDescriptorPlatformUnit(t)
	waitDescriptorPlatformReady(t, stand, "stage-crash launch")
	if err := exec.Command("systemctl", "kill", "--kill-whom=main", "--signal=SIGKILL", "ardents-descriptor.service").Run(); err != nil {
		t.Fatal("stage-crash signal", err)
	}
	waitDescriptorPlatformFailed(t)
	descriptorPlatformProducerPresent(t, stand, true, "after stage crash")
	stage := filepath.Join(stand.storeRoot, "records", ".record-stage")
	staged := []byte("uncertain unpublished write left by the killed invocation")
	if err := os.WriteFile(stage, staged, 0600); err != nil {
		t.Fatal("uncertain stage fixture", err)
	}
	if err := os.Chown(stage, stand.uid, stand.uid); err != nil {
		t.Fatal(err)
	}
	committed := filepath.Join(stand.storeRoot, "records", fmt.Sprintf("%x", target))
	committedBytes, err := os.ReadFile(committed)
	if err != nil {
		t.Fatal("committed publication floor unavailable", err)
	}
	_ = exec.CommandContext(t.Context(), "systemctl", "start", "ardents-descriptor.service").Run()
	waitDescriptorPlatformFailed(t)
	descriptorPlatformProducerPresent(t, stand, true, "after uncertain-stage refusal")
	if after, err := os.ReadFile(stage); err != nil || !bytes.Equal(staged, after) {
		t.Fatal("refused retained start repaired the uncertain stage", err)
	}
	out, err = runAsDescriptorNode(t, stand.program, "node", "descriptor", "repair-retained", stand.planPath)
	if err != nil || !bytes.Contains(out, []byte("repaired")) {
		t.Fatal("bounded stage repair against real manager facts", string(out), err)
	}
	descriptorPlatformProducerPresent(t, stand, false, "after stage repair")
	if _, err := os.Lstat(stage); !os.IsNotExist(err) {
		t.Fatal("uncertain stage retained after bounded repair", err)
	}
	if after, err := os.ReadFile(committed); err != nil || !bytes.Equal(committedBytes, after) {
		t.Fatal("stage repair changed the committed publication floor", err)
	}
	startDescriptorPlatformUnit(t)
	waitDescriptorPlatformReady(t, stand, "post-stage-repair launch")
	stopDescriptorPlatformUnit(t, "post-stage-repair launch")
	// With the unit stopped the exclusive Store lease is free: the committed
	// publication survived the stage repair and still resolves byte-exact.
	reopened, err := reachability.OpenRetainedStore(reachability.StoreConfig{Root: stand.storeRoot, Network: stand.networkPlan.NetworkID})
	if err != nil {
		t.Fatal("retained Store refused after stage repair", err)
	}
	got, outcome, lookupErr := reopened.Lookup(target, profile, time.Now())
	closeErr := reopened.Close()
	if lookupErr != nil || closeErr != nil || outcome != reachability.Accepted || !bytes.Equal(raw, got) {
		t.Fatal("committed publication lost across stage repair", outcome, lookupErr, closeErr)
	}

	// Causal pre-effect refusals against the committed delivery.
	original, err := os.ReadFile(stand.program)
	if err != nil {
		t.Fatal(err)
	}
	changed := bytes.Clone(original)
	changed[len(changed)-1] ^= 1
	if err := os.WriteFile(stand.program, changed, 0555); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.CommandContext(t.Context(), "systemctl", "start", "ardents-descriptor.service").CombinedOutput(); err == nil {
		t.Fatal("substituted program accepted", string(out))
	}
	if err := os.WriteFile(stand.program, original, 0555); err != nil {
		t.Fatal(err)
	}
	launchRaw, err := os.ReadFile(stand.launchPath)
	if err != nil {
		t.Fatal(err)
	}
	foreign := bytes.Replace(launchRaw, []byte(stand.launch.ConfigSHA256), []byte(strings.Repeat("0", 64)), 1)
	if err := os.WriteFile(stand.launchPath, foreign, 0644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.CommandContext(t.Context(), "systemctl", "start", "ardents-descriptor.service").CombinedOutput(); err == nil {
		t.Fatal("foreign launch pin accepted", string(out))
	}
	if err := os.WriteFile(stand.launchPath, launchRaw, 0644); err != nil {
		t.Fatal(err)
	}
	startDescriptorPlatformUnit(t)
	waitDescriptorPlatformReady(t, stand, "restored launch")
	stopDescriptorPlatformUnit(t, "restored launch")
}

func provisionDescriptorPlatformStand(t *testing.T) *descriptorPlatformStand {
	t.Helper()
	ctx := t.Context()
	binary := compiledCommand(t)
	binaryBytes, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	stand := &descriptorPlatformStand{ns: fmt.Sprintf("%d-%d", os.Getpid(), time.Now().UnixNano()), binaryBytes: binaryBytes}
	stand.base = filepath.Join("/var/lib/ardents", "descriptor-installed-"+stand.ns)
	stand.distribution = filepath.Join(stand.base, "distribution")
	stand.releaseRoot = filepath.Join(stand.base, "release-trust")
	stand.programDir = filepath.Join("/usr/lib/ardents", "descriptor-installed-"+stand.ns)
	stand.program = filepath.Join(stand.programDir, "ardents-next")
	stand.etcDir = filepath.Join("/etc/ardents", "descriptor-installed-"+stand.ns)
	stand.rolesBase = filepath.Join("/var/lib/ardents/roles/descriptor", "installed-"+stand.ns)
	stand.stateRoot = filepath.Join(stand.rolesBase, "state")
	stand.participationRoot = filepath.Join(stand.rolesBase, "participation")
	stand.spendRoot = filepath.Join(stand.rolesBase, "spend")
	stand.hostingRoot = filepath.Join(stand.rolesBase, "hosting")
	stand.storeRoot = filepath.Join(stand.rolesBase, "store")
	stand.clockPath = filepath.Join(stand.rolesBase, "clock")
	stand.planPath = filepath.Join(stand.etcDir, "descriptor.json")
	stand.launchPath = filepath.Join(stand.etcDir, "launch.json")
	stand.certPath = filepath.Join(stand.etcDir, "node.crt")
	stand.keyPath = filepath.Join(stand.etcDir, "node.key")
	stand.nodeID = [32]byte{17}

	stand.uid = descriptorPlatformAccount(t)
	t.Cleanup(func() { cleanupDescriptorPlatformStand(t, stand) })
	for _, dir := range []string{stand.base, stand.releaseRoot, stand.programDir, stand.etcDir, stand.rolesBase} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(stand.releaseRoot, 0700); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{stand.stateRoot, stand.participationRoot, stand.spendRoot} {
		if err := os.Mkdir(root, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chown(root, stand.uid, stand.uid); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(stand.clockPath, nil, 0644); err != nil {
		t.Fatal(err)
	}
	clockStop, clockJoined := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(clockJoined)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-clockStop:
				return
			case now := <-ticker.C:
				if err := os.Chtimes(stand.clockPath, now, now); err != nil {
					t.Error("clock observation updater", err)
					return
				}
			}
		}
	}()
	t.Cleanup(func() { close(clockStop); <-clockJoined })

	f, sockets, certificates := descriptorRouteFixtureConfigured(t, transport.ClosedCarrierTCP, nil)
	stand.fixture, stand.sockets, stand.certificates = f, sockets, certificates
	bundle, err := networkfixture.BuildClosed(f.spec)
	if err != nil {
		t.Fatal(err)
	}
	var key [32]byte
	copy(key[:], f.spec.Authority.Public().(ed25519.PublicKey))
	stand.networkPlan = networkAuthorityPlan{Root: stand.stateRoot, NetworkID: f.spec.NetworkID, Authorities: [][32]byte{key}, Threshold: 1, ProfileAuthority: key, ClockObservationFile: stand.clockPath}
	config, err := networkStateConfig(&stand.networkPlan)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Accept(ctx, bundle.Epoch.Raw, bundle.Epoch.Inputs, bundle.Epoch.Materials); err != nil {
		_ = owner.Close()
		t.Fatal(err)
	}
	if _, err := owner.AcceptClosedProfile(bundle.Profile); err != nil {
		_ = owner.Close()
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if err := chownTreeDescriptor(stand.stateRoot, stand.uid); err != nil {
		t.Fatal(err)
	}
	view, err := f.current()
	if err != nil {
		t.Fatal(err)
	}
	member, err := view.Member(stand.nodeID, view.ObservedAt())
	if err != nil {
		t.Fatal(err)
	}
	stand.address, stand.familyID = member.Endpoint, member.FamilyID

	certificate := certificates[stand.nodeID]
	certBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Certificate[0]})
	keyDER, err := x509.MarshalPKCS8PrivateKey(certificate.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	keyBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	clear(keyDER)
	if err := os.WriteFile(stand.certPath, certBytes, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stand.keyPath, keyBytes, 0600); err != nil {
		t.Fatal(err)
	}
	certDigest := platformDigest(certBytes)
	keyDigest := platformDigest(keyBytes)
	clear(keyBytes)
	if err := os.Chown(stand.keyPath, stand.uid, stand.uid); err != nil {
		t.Fatal(err)
	}

	start := time.Now().UTC().Truncate(time.Hour)
	if err := hosting.Initialize(stand.hostingRoot, hosting.Policy{Provider: "Descriptor installed test", Start: start, End: start.Add(2 * time.Hour), Unit: "MiB", Quantity: 100, Directions: "tx+rx", Interfaces: []string{"lo"}, LowWatermarkBytes: 1000}); err != nil {
		t.Fatal(err)
	}
	if err := chownTreeDescriptor(stand.hostingRoot, stand.uid); err != nil {
		t.Fatal(err)
	}
	// Provision the fifth writable root through the genuine fresh Store opener,
	// then release it; the installed retained runtime reopens this same root.
	provisioned, err := reachability.OpenStore(reachability.StoreConfig{Root: stand.storeRoot, Network: stand.networkPlan.NetworkID})
	if err != nil {
		t.Fatal("Store root provisioning refused", err)
	}
	if err := provisioned.Close(); err != nil {
		t.Fatal(err)
	}
	if err := chownTreeDescriptor(stand.storeRoot, stand.uid); err != nil {
		t.Fatal(err)
	}

	plan := map[string]any{"network": stand.networkPlan, "node_id": stand.nodeID, "process_profile": processresource.DescriptorProfile,
		"participation_root": stand.participationRoot, "spend_root": stand.spendRoot, "hosting_root": stand.hostingRoot, "descriptor_root": stand.storeRoot,
		"certificate": stand.certPath, "private_key": stand.keyPath,
		"work": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "retirement_wait_ms": 5000}
	planRaw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stand.planPath, planRaw, 0644); err != nil {
		t.Fatal(err)
	}

	stand.authority = newConsumerReleaseAuthority(t, infrastructureReference.Add(365*24*time.Hour))
	_, inventoryPin, _ := infrastructureDistributionInvocation(t, stand.base, binaryBytes, stand.authority, 1, nil, "descriptor", []byte(platformUnitTemplate))
	if err := os.WriteFile(stand.program, binaryBytes, 0555); err != nil {
		t.Fatal(err)
	}
	stand.launch = platformLaunch{
		Role:          "descriptor",
		Distribution:  stand.distribution,
		ReleaseRoot:   stand.releaseRoot,
		Environment:   "h3-test",
		Network:       "new-release-test",
		Program:       stand.program,
		Configuration: stand.planPath,
		ConfigSHA256:  platformDigest(planRaw),
		WritableRoots: []string{stand.stateRoot, stand.participationRoot, stand.spendRoot, stand.hostingRoot, stand.storeRoot},
		Inputs: map[string]string{
			stand.certPath: certDigest,
			stand.keyPath:  keyDigest,
		},
	}
	launchRaw, err := json.Marshal(stand.launch)
	if err != nil {
		t.Fatal(err)
	}
	launchRaw = append(launchRaw, '\n')
	stand.pin = platformDigest(launchRaw)
	if err := os.WriteFile(stand.launchPath, launchRaw, 0644); err != nil {
		t.Fatal(err)
	}
	enrolled := filepath.Join(stand.distribution, infrastructure.ProgramName)
	if out, err := exec.CommandContext(ctx, enrolled, "platform", "initialize-trust", stand.launchPath, inventoryPin).CombinedOutput(); err != nil {
		t.Fatal("explicit installed first pin", string(out), err)
	}
	unitOut, err := exec.CommandContext(ctx, enrolled, "platform", "unit", stand.launchPath).CombinedOutput()
	if err != nil {
		t.Fatal("platform unit render", string(unitOut), err)
	}
	stand.unit = unitOut
	if !bytes.Contains(stand.unit, []byte("ExecStartPre=+:")) || !bytes.Contains(stand.unit, []byte("node descriptor --retained-config "+stand.planPath)) {
		t.Fatal("rendered unit lacks the mandatory precheck or retained runtime", string(stand.unit))
	}
	if err := os.WriteFile("/etc/systemd/system/ardents-descriptor.service", stand.unit, 0644); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		t.Fatal("daemon-reload", err)
	}

	// First installation: the fresh --config composition creates the participation
	// and receiving histories under a transient envelope, then joins cleanly. The
	// installed unit subsequently opens those histories retained.
	stand.sockets[stand.nodeID]()
	preinitDescriptorHistories(t, stand, stand.planPath)
	return stand
}

// preinitDescriptorHistories runs the fresh --config composition under a
// transient envelope to create the participation and receiving histories, then
// joins cleanly. The installed unit subsequently opens those histories retained.
func preinitDescriptorHistories(t *testing.T, stand *descriptorPlatformStand, planPath string) {
	t.Helper()
	unit := fmt.Sprintf("ardents-descriptor-preinit-%d-%d.service", os.Getpid(), time.Now().UnixNano())
	cmd := exec.CommandContext(t.Context(), "systemd-run", "--quiet", "--wait", "--pipe", "--collect", "--unit="+unit,
		"--property=CPUQuota=100%", "--property=CPUQuotaPeriodSec=100ms", "--property=MemoryMax=1G", "--property=MemoryHigh=768M", "--property=TasksMax=256", "--property=LimitNOFILE=1024", "--property=RuntimeMaxSec=120s",
		fmt.Sprintf("--property=User=%d", stand.uid), fmt.Sprintf("--property=Group=%d", stand.uid),
		"--setenv=GOMAXPROCS=1", "--setenv=GOMEMLIMIT=512MiB", "--", stand.program, "node", "descriptor", "--config", planPath)
	var diagnostic bytes.Buffer
	cmd.Stderr = &diagnostic
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	var event map[string]string
	if err := json.NewDecoder(stdout).Decode(&event); err != nil || event["operation"] != "node.descriptor" || event["phase"] != "listening" {
		_ = exec.Command("systemctl", "stop", unit).Run()
		_ = cmd.Wait()
		t.Fatal("preinit fresh Descriptor startup refused", event, err, diagnostic.String())
	}
	if err := exec.Command("systemctl", "kill", "--kill-whom=main", "--signal=SIGINT", unit).Run(); err != nil {
		t.Fatal("preinit stop signal", err)
	}
	_ = cmd.Wait()
	_ = stdout.Close()
}

// commitDescriptorPlatformFloor commits one genuine signed publication through
// the real Store owner while the installed unit is cleanly stopped (the exclusive
// flock lease is free). It uses the actual verify-before-mutate /
// persist-before-expose Publish mechanism, then hands the committed record to the
// service principal so the installed retained runtime reopens it. The signed
// input is a public fixture transcript, not evidence of Publisher readiness; the
// network publish/ACK path through the Route prefix is proven separately by
// TestNativeDescriptorPublishLookupAndReopenTCP. The fresh 60-second validity
// window is why this runs immediately before the bounded stage-crash leg.
func commitDescriptorPlatformFloor(t *testing.T, stand *descriptorPlatformStand) ([]byte, [32]byte, [32]byte) {
	t.Helper()
	if exec.Command("systemctl", "is-active", "ardents-descriptor.service").Run() == nil {
		t.Fatal("floor commit requires the stopped unit's released Store lease")
	}
	raw, target := routeSignedDescriptor(t, stand.fixture, 3, [32]byte{18})
	profile := stand.fixture.profile.Digest
	store, err := reachability.OpenStore(reachability.StoreConfig{Root: stand.storeRoot, Network: stand.networkPlan.NetworkID})
	if err != nil {
		t.Fatal("Store floor commit opening refused", err)
	}
	if _, err := store.Publish(raw, profile, time.Now(), func() error { return nil }); err != nil {
		_ = store.Close()
		t.Fatal("genuine Store floor commit refused", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(filepath.Join(stand.storeRoot, "records", fmt.Sprintf("%x", target)), stand.uid, stand.uid); err != nil {
		t.Fatal(err)
	}
	return raw, target, profile
}

// compiledDescriptorPlatformCommand builds a distinct working executable of the
// same checked source (canonical flags without -w), a genuine forward artifact.
func compiledDescriptorPlatformCommand(t *testing.T) []byte {
	t.Helper()
	dir, err := os.MkdirTemp("", "ardents-descriptor-platform-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, "ardents-next")
	cmd := exec.CommandContext(t.Context(), "go", "build", "-trimpath", "-buildvcs=false", "-o", path, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatal("forward command build", string(out), err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func chownTreeDescriptor(root string, uid int) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(path, uid, uid)
	})
}

func descriptorPlatformAccount(t *testing.T) int {
	t.Helper()
	if exec.Command("getent", "passwd", "ardents-node").Run() == nil || exec.Command("getent", "group", "ardents-node").Run() == nil {
		t.Fatal("ardents-node principal already present; invalid environment")
	}
	uid := 0
	for candidate := 990; candidate >= 100; candidate-- {
		name := fmt.Sprintf("%d", candidate)
		if exec.Command("getent", "passwd", name).Run() != nil && exec.Command("getent", "group", name).Run() != nil {
			uid = candidate
			break
		}
	}
	if uid == 0 {
		t.Fatal("no free unprivileged system UID/GID available")
	}
	if err := exec.Command("groupadd", "-g", fmt.Sprintf("%d", uid), "ardents-node").Run(); err != nil {
		t.Fatal("groupadd", err)
	}
	if err := exec.Command("useradd", "-u", fmt.Sprintf("%d", uid), "-g", fmt.Sprintf("%d", uid), "-d", "/nonexistent", "-s", "/usr/sbin/nologin", "-M", "ardents-node").Run(); err != nil {
		t.Fatal("useradd", err)
	}
	return uid
}

func runAsDescriptorNode(t *testing.T, name string, args ...string) ([]byte, error) {
	t.Helper()
	full := append([]string{"-u", "ardents-node", "--", name}, args...)
	cmd := exec.CommandContext(t.Context(), "runuser", full...)
	cmd.Dir = "/"
	return cmd.CombinedOutput()
}

func startDescriptorPlatformUnit(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "systemctl", "start", "ardents-descriptor.service").CombinedOutput(); err != nil {
		t.Fatalf("installed Descriptor start: %v\n%s\njournal:\n%s", err, out, descriptorPlatformJournal(t))
	}
}

func stopDescriptorPlatformUnit(t *testing.T, phase string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "systemctl", "stop", "ardents-descriptor.service").CombinedOutput(); err != nil {
		t.Fatalf(phase, "installed Descriptor stop", string(out), err)
	}
	result, err := exec.Command("systemctl", "show", "ardents-descriptor.service", "-p", "Result", "--value").Output()
	if err != nil || strings.TrimSpace(string(result)) != "success" {
		t.Fatal(phase, "clean stop manager result", string(result), err)
	}
	var terminal struct{ Phase, Outcome string }
	found := false
	for _, line := range strings.Split(descriptorPlatformJournal(t), "\n") {
		if json.Unmarshal([]byte(line), &terminal) == nil && terminal.Phase != "" {
			found = true
		}
	}
	if !found || terminal.Phase != "joined" || terminal.Outcome != "canceled" {
		t.Fatal(phase, "original joined cancellation report", terminal, descriptorPlatformJournal(t))
	}
}

func waitDescriptorPlatformFailed(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if exec.Command("systemctl", "is-failed", "ardents-descriptor.service").Run() == nil {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("killed unit did not reach failed state", descriptorPlatformJournal(t))
}

func descriptorPlatformJournal(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("journalctl", "-u", "ardents-descriptor.service", "-o", "cat", "-n", "60", "--no-pager").Output()
	if err != nil {
		return "journal unavailable: " + err.Error()
	}
	return string(out)
}

// waitDescriptorPlatformReady proves the retained runtime opened its shared
// carrier listener at the signed member Endpoint (receiver.Listen admission).
func waitDescriptorPlatformReady(t *testing.T, stand *descriptorPlatformStand, phase string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", stand.address, 2*time.Second)
		if err == nil {
			_ = conn.Close()
			return
		}
		if exec.Command("systemctl", "is-failed", "ardents-descriptor.service").Run() == nil {
			t.Fatal(phase, "unit failed before listening", descriptorPlatformJournal(t))
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal(phase, "installed Descriptor never listened", descriptorPlatformJournal(t))
}

func descriptorPlatformProducerPresent(t *testing.T, stand *descriptorPlatformStand, want bool, phase string) {
	t.Helper()
	conflict, err := duty.ReadConflict(stand.participationRoot, time.Now, stand.nodeID, stand.familyID)
	if err != nil || conflict != want {
		t.Fatal(phase, "node-duty producer presence", conflict, want, err)
	}
}

func cleanupDescriptorPlatformStand(t *testing.T, stand *descriptorPlatformStand) {
	t.Helper()
	_ = exec.Command("systemctl", "stop", "ardents-descriptor.service").Run()
	_ = os.Remove("/etc/systemd/system/ardents-descriptor.service")
	_ = exec.Command("systemctl", "daemon-reload").Run()
	_ = exec.Command("userdel", "ardents-node").Run()
	_ = exec.Command("groupdel", "ardents-node").Run()
	_ = os.RemoveAll(stand.etcDir)
	_ = os.RemoveAll(stand.programDir)
	_ = os.RemoveAll(stand.base)
	_ = os.RemoveAll(stand.rolesBase)
}
