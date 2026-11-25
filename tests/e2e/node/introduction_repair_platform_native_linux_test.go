//go:build linux && introduction_repair_native

package node_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/platform/systemd"
	"github.com/dianabuilds/ardents-network/internal/processresource"
	"github.com/dianabuilds/ardents-network/internal/release/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/release/infrastructure/launch"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
	networkinputs "github.com/dianabuilds/ardents-network/tests/networkfixture"
	"github.com/dianabuilds/ardents-network/tests/releasefixture"
)

const introductionRepairUnit = "ardents-introduction.service"

type installedIntroductionRepair struct {
	program, plan, address                 string
	network, participation, spend, hosting string
	history, release                       string
	nodeID, familyID                       [32]byte
	uid                                    int
}

func TestNativeIntroductionPlatformRepairsCrashedProducer(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("Introduction installed repair requires a root driver")
	}
	if _, err := exec.Command("/usr/bin/busctl", "--system", "--no-pager", "get-property", "org.freedesktop.systemd1", "/org/freedesktop/systemd1", "org.freedesktop.systemd1.Manager", "Version").Output(); err != nil {
		t.Fatal("real systemd manager unavailable", err)
	}
	unitPath := filepath.Join("/etc/systemd/system", introductionRepairUnit)
	if _, err := os.Lstat(unitPath); !os.IsNotExist(err) || exec.Command("systemctl", "is-active", "--quiet", introductionRepairUnit).Run() == nil {
		t.Fatal("Introduction unit already present or active", err)
	}

	stand := prepareInstalledIntroductionRepair(t, unitPath)
	introductionSystemctl(t, "start", introductionRepairUnit)
	waitInstalledIntroduction(t, stand.address)
	original := introductionProducer(t, stand.participation)
	repair := func() ([]byte, error) {
		return runAsInstalledNode(t, stand.program, "node", "introduction", "repair-retained", stand.plan)
	}
	if out, err := repair(); err == nil {
		t.Fatal("repair accepted the live Introduction unit", string(out))
	}

	introductionSystemctl(t, "kill", "--kill-whom=main", "--signal=SIGKILL", introductionRepairUnit)
	waitForIntroductionCrash(t)
	if retained := introductionProducer(t, stand.participation); retained != original {
		t.Fatal("crash changed the original producer")
	}
	before := snapshotIntroductionRepairRoots(t, stand.network, stand.spend, stand.hosting, stand.history, stand.release)
	if slotHistory := before[filepath.Join(stand.history, "closed-introduction-slots")]; !slotHistory.mode.IsRegular() || slotHistory.digest == ([32]byte{}) {
		t.Fatal("the original Introduction slot history was not retained at the crash")
	}
	if out, err := repair(); err != nil || !bytes.Equal(out, []byte("repaired\n")) {
		t.Fatal("repair of the failed Introduction unit", err, string(out))
	}
	after := snapshotIntroductionRepairRoots(t, stand.network, stand.spend, stand.hosting, stand.history, stand.release)
	if len(after) != len(before) {
		t.Fatalf("repair changed the retained directory inventory: %d entries became %d", len(before), len(after))
	}
	for path, original := range before {
		if current, exists := after[path]; !exists || current != original {
			t.Fatal("repair changed a retained file or root", path)
		}
	}
	conflict, err := duty.ReadConflict(stand.participation, time.Now, stand.nodeID, stand.familyID)
	if err != nil || conflict {
		t.Fatal("repair did not release the original participation fence", conflict, err)
	}
	if out, err := repair(); err == nil {
		t.Fatal("repair accepted an already clean participation root", string(out))
	}
	introductionSystemctl(t, "start", introductionRepairUnit)
	waitInstalledIntroduction(t, stand.address)
	if next := introductionProducer(t, stand.participation); next == original {
		t.Fatal("restart adopted the crashed producer")
	}
	introductionSystemctl(t, "stop", introductionRepairUnit)
	if conflict, err := duty.ReadConflict(stand.participation, time.Now, stand.nodeID, stand.familyID); err != nil || conflict {
		t.Fatal("joined stop retained a producer", conflict, err)
	}
}

func prepareInstalledIntroductionRepair(t *testing.T, unitPath string) installedIntroductionRepair {
	t.Helper()
	roles, etc, programDir := makeIntroductionRepairDirectories(t)
	stand := installedIntroductionRepair{
		program:       filepath.Join(programDir, "ardents-next"),
		plan:          filepath.Join(etc, "introduction.json"),
		network:       filepath.Join(roles, "state"),
		participation: filepath.Join(roles, "participation"),
		spend:         filepath.Join(roles, "spend"),
		hosting:       filepath.Join(roles, "hosting"),
		history:       filepath.Join(roles, "introduction"),
		nodeID:        [32]byte{25},
		uid:           createInstalledNodeAccount(t),
	}
	t.Cleanup(func() {
		if _, err := os.Lstat(unitPath); !os.IsNotExist(err) {
			if out, err := exec.Command("systemctl", "stop", introductionRepairUnit).CombinedOutput(); err != nil {
				t.Error("stop fixture unit", err, string(out))
			}
			if err := os.Remove(unitPath); err != nil {
				t.Error("remove fixture unit", err)
			}
			if out, err := exec.Command("systemctl", "daemon-reload").CombinedOutput(); err != nil {
				t.Error("reload after removing fixture unit", err, string(out))
			}
		}
		if out, err := exec.Command("userdel", "ardents-node").CombinedOutput(); err != nil {
			t.Error("remove fixture account", err, string(out))
		}
		if exec.Command("getent", "group", "ardents-node").Run() == nil {
			if out, err := exec.Command("groupdel", "ardents-node").CombinedOutput(); err != nil {
				t.Error("remove fixture group", err, string(out))
			}
		}
	})
	for _, root := range []string{stand.network, stand.participation, stand.spend, stand.history} {
		if err := os.Mkdir(root, 0700); err != nil {
			t.Fatal(err)
		}
	}

	clock := filepath.Join(roles, "clock")
	writeIntroductionRepairFile(t, clock, nil, 0644)
	t.Cleanup(networkinputs.RefreshObservedClock(t, clock))
	processfixture.WaitAdmissionHour(t)
	spec, _, _ := networkinputs.PrepareNetworkGenesis(t, t.TempDir())
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	stand.address = listener.Addr().String()
	spec.Nodes[1].NodeID = stand.nodeID
	spec.Nodes[1].RoleDomain, spec.Nodes[1].Subrole = 4, 3
	spec.Nodes[1].Endpoint = stand.address
	bundle, err := networkfixture.BuildClosed(spec)
	if err != nil {
		t.Fatal(err)
	}
	key := [32]byte(spec.Authority[32:])
	network := state.AuthorityDeclaration{
		Root: stand.network, NetworkID: spec.NetworkID, Authorities: [][32]byte{key},
		Threshold: 1, ProfileAuthority: key, ClockObservationFile: clock,
	}
	config, err := network.Config()
	if err != nil {
		t.Fatal(err)
	}
	owner, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := owner.AcceptEpoch(t.Context(), bundle.Epoch.Raw, bundle.Epoch.Inputs, bundle.Epoch.Materials); err != nil {
		t.Fatal(errors.Join(err, owner.Close()))
	}
	if _, err := owner.AcceptClosedProfile(bundle.Profile); err != nil {
		t.Fatal(errors.Join(err, owner.Close()))
	}
	view, viewErr := owner.CurrentRuntime()
	if err := errors.Join(viewErr, owner.Close()); err != nil {
		t.Fatal("read prepared State", err)
	}
	member, err := view.Member(stand.nodeID, view.ObservedAt())
	if err != nil {
		t.Fatal(err)
	}
	stand.familyID = member.FamilyID
	start := time.Now().UTC().Truncate(time.Hour)
	if err := hosting.Initialize(stand.hosting, hosting.Policy{
		Provider: "Introduction installed repair", Start: start, End: start.Add(2 * time.Hour),
		Unit: "MiB", Quantity: 100, Directions: "tx+rx", Interfaces: []string{"lo"}, LowWatermarkBytes: 1000,
	}); err != nil {
		t.Fatal(err)
	}
	roots := []string{stand.network, stand.participation, stand.spend, stand.hosting, stand.history}
	for _, root := range roots {
		if err := chownInstalledNodeTree(root, stand.uid); err != nil {
			t.Fatal(err)
		}
	}

	certPath, keyPath := filepath.Join(etc, "node.crt"), filepath.Join(etc, "node.key")
	cert := processfixture.RoleCertificate(t, spec.Nodes[1].PrivateKey)
	certBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]})
	keyDER, err := x509.MarshalPKCS8PrivateKey(cert.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(keyDER)
	keyBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	defer clear(keyBytes)
	writeIntroductionRepairFile(t, certPath, certBytes, 0644)
	writeIntroductionRepairFile(t, keyPath, keyBytes, 0600)
	if err := os.Chown(keyPath, stand.uid, stand.uid); err != nil {
		t.Fatal(err)
	}
	plan, err := json.Marshal(map[string]any{
		"network": network, "node_id": stand.nodeID, "process_profile": processresource.IntroductionProfile,
		"participation_root": stand.participation, "spend_root": stand.spend, "hosting_root": stand.hosting, "introduction_root": stand.history,
		"certificate": certPath, "private_key": keyPath,
		"work": hosting.Traffic{Tx: 1 << 20, Rx: 1 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "retirement_wait_ms": 5000,
	})
	if err != nil {
		t.Fatal(err)
	}
	writeIntroductionRepairFile(t, stand.plan, plan, 0644)

	binary, err := os.ReadFile(buildNodeCommand(t))
	if err != nil {
		t.Fatal(err)
	}
	writeIntroductionRepairFile(t, stand.program, binary, 0555)
	base := filepath.Join(roles, "software")
	if err := os.Mkdir(base, 0755); err != nil {
		t.Fatal(err)
	}
	stand.release = filepath.Join(base, "release-trust")
	if err := os.Mkdir(stand.release, 0700); err != nil {
		t.Fatal(err)
	}
	authority := releasefixture.NewReleaseAuthority(t, releasefixture.InfrastructureReference().Add(365*24*time.Hour))
	_, inventoryPin, _ := releasefixture.InfrastructureDistribution(t, base, binary, authority, 1, nil, "introduction", []byte(systemd.PlatformUnitTemplate))
	distribution := filepath.Join(base, "distribution")
	declaration, err := json.Marshal(launch.Declaration{
		Role: "introduction", Distribution: distribution, ReleaseRoot: stand.release, Environment: "h3-test",
		Network: "new-release-test", Program: stand.program, Configuration: stand.plan,
		ConfigSHA256: releasefixture.FixtureDigest(plan), WritableRoots: roots,
		Inputs: map[string]string{certPath: releasefixture.FixtureDigest(certBytes), keyPath: releasefixture.FixtureDigest(keyBytes)},
	})
	if err != nil {
		t.Fatal(err)
	}
	launchPath := filepath.Join(etc, "launch.json")
	writeIntroductionRepairFile(t, launchPath, append(declaration, '\n'), 0644)
	enrolled := filepath.Join(distribution, infrastructure.ProgramName)
	if out, err := exec.CommandContext(t.Context(), enrolled, "platform", "initialize-trust", launchPath, inventoryPin).CombinedOutput(); err != nil {
		t.Fatal("initialize Introduction Release trust", err, string(out))
	}
	unit, err := exec.CommandContext(t.Context(), enrolled, "platform", "unit", launchPath).CombinedOutput()
	if err != nil {
		t.Fatal("render Introduction platform unit", err, string(unit))
	}
	writeIntroductionRepairFile(t, unitPath, unit, 0644)
	introductionSystemctl(t, "daemon-reload")
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	initializeIntroductionHistories(t, stand)
	return stand
}

func makeIntroductionRepairDirectories(t *testing.T) (string, string, string) {
	t.Helper()
	dirs := make([]string, 0, 3)
	for _, parent := range []string{"/var/lib/ardents/roles/introduction", "/etc/ardents", "/usr/lib/ardents"} {
		if err := os.MkdirAll(parent, 0755); err != nil {
			t.Fatal(err)
		}
		dir, err := os.MkdirTemp(parent, "introduction-repair-")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := os.RemoveAll(dir); err != nil {
				t.Error("cleanup", dir, err)
			}
		})
		if err := os.Chmod(dir, 0755); err != nil {
			t.Fatal(err)
		}
		dirs = append(dirs, dir)
	}
	return dirs[0], dirs[1], dirs[2]
}

func writeIntroductionRepairFile(t *testing.T, path string, raw []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, raw, mode); err != nil {
		t.Fatal(err)
	}
}

func initializeIntroductionHistories(t *testing.T, stand installedIntroductionRepair) {
	t.Helper()
	unit := fmt.Sprintf("ardents-introduction-preinit-%d.service", os.Getpid())
	cmd := exec.CommandContext(t.Context(), "systemd-run", "--quiet", "--wait", "--pipe", "--collect", "--unit="+unit,
		"--property=CPUQuota=100%", "--property=CPUQuotaPeriodSec=100ms", "--property=MemoryMax=1G",
		"--property=MemoryHigh=768M", "--property=TasksMax=256", "--property=LimitNOFILE=1024", "--property=RuntimeMaxSec=120s",
		fmt.Sprintf("--property=User=%d", stand.uid), fmt.Sprintf("--property=Group=%d", stand.uid),
		"--setenv=GOMAXPROCS=1", "--setenv=GOMEMLIMIT=512MiB", "--", stand.program, "node", "introduction", "--config", stand.plan)
	var diagnostic bytes.Buffer
	cmd.Stderr = &diagnostic
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if cmd.ProcessState == nil {
			_ = exec.Command("systemctl", "stop", unit).Run()
			_ = cmd.Wait()
		}
	}()
	var event map[string]string
	if err := json.NewDecoder(stdout).Decode(&event); err != nil || event["operation"] != "node.introduction" || event["phase"] != "listening" {
		_ = exec.Command("systemctl", "stop", unit).Run()
		_ = cmd.Wait()
		t.Fatal("fresh Introduction did not initialize history", event, err, diagnostic.String())
	}
	introductionSystemctl(t, "kill", "--kill-whom=main", "--signal=SIGINT", unit)
	err = cmd.Wait()
	var exit *exec.ExitError
	var terminal struct {
		Operation string `json:"operation"`
		Phase     string `json:"phase"`
		Outcome   string `json:"outcome"`
	}
	decodeErr := json.Unmarshal(diagnostic.Bytes(), &terminal)
	if !errors.As(err, &exit) || exit.ExitCode() != 130 || decodeErr != nil ||
		terminal.Operation != "node.introduction" || terminal.Phase != "joined" || terminal.Outcome != "canceled" {
		t.Fatal("fresh Introduction did not join after SIGINT", err, diagnostic.String())
	}
}

func introductionSystemctl(t *testing.T, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "systemctl", args...).CombinedOutput(); err != nil {
		t.Fatalf("systemctl %v: %v\n%s\n%s", args, err, out, introductionRepairJournal())
	}
}

func introductionRepairJournal() string {
	out, _ := exec.Command("journalctl", "-u", introductionRepairUnit, "-n", "30", "-o", "cat", "--no-pager").CombinedOutput()
	return string(out)
}

func waitInstalledIntroduction(t *testing.T, address string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, time.Second)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("installed Introduction did not listen", introductionRepairJournal())
}

func waitForIntroductionCrash(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		out, err := exec.Command("systemctl", "show", introductionRepairUnit, "--property=ActiveState,ExecMainStatus").Output()
		if err == nil && bytes.Contains(out, []byte("ActiveState=failed\n")) && bytes.Contains(out, []byte("ExecMainStatus=9\n")) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("the original Introduction did not reach failed after SIGKILL", introductionRepairJournal())
}

func introductionProducer(t *testing.T, root string) [32]byte {
	t.Helper()
	owner, err := duty.OpenOperation(t.Context(), duty.Config{Root: root, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	id, _, checkErr := owner.CheckNodeProducer()
	if err := errors.Join(checkErr, owner.Close()); err != nil || id == ([32]byte{}) {
		t.Fatal("original Introduction producer is absent", err)
	}
	return id
}

type introductionRepairFile struct {
	mode       os.FileMode
	dev, inode uint64
	uid, gid   uint32
	digest     [32]byte
}

func snapshotIntroductionRepairRoots(t *testing.T, roots ...string) map[string]introductionRepairFile {
	t.Helper()
	snapshot := make(map[string]introductionRepairFile)
	for _, root := range roots {
		files := 0
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			native, ok := info.Sys().(*syscall.Stat_t)
			if !ok {
				return fmt.Errorf("native retained file identity unavailable: %s", path)
			}
			record := introductionRepairFile{mode: info.Mode(), dev: uint64(native.Dev), inode: native.Ino, uid: native.Uid, gid: native.Gid}
			switch {
			case info.Mode().IsRegular():
				raw, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				record.digest = sha256.Sum256(raw)
				files++
			case !info.IsDir():
				return fmt.Errorf("unexpected retained file type: %s", path)
			}
			snapshot[path] = record
			return nil
		})
		if err != nil || files == 0 {
			t.Fatal("retained history is unavailable", root, err)
		}
	}
	return snapshot
}
