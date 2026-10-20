//go:build linux && node_process_native

package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/network/duty"
	"github.com/dianabuilds/ardents-network/internal/successor/processresource"
	"github.com/dianabuilds/ardents-network/internal/successor/reachability"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

func nativeDescriptorCredential(t *testing.T, certificate tls.Certificate) (string, string) {
	t.Helper()
	root := t.TempDir()
	certPath, keyPath := filepath.Join(root, "certificate"), filepath.Join(root, "key")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Certificate[0]}), 0600); err != nil {
		t.Fatal(err)
	}
	key, err := x509.MarshalPKCS8PrivateKey(certificate.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(key)
	encoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key})
	defer clear(encoded)
	if err := os.WriteFile(keyPath, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	return certPath, keyPath
}

func launchNativeDescriptor(t *testing.T, binary, plan string, index byte) *nativeForwardingProcess {
	t.Helper()
	unit := fmt.Sprintf("ardents-descriptor-%d-%d.service", os.Getpid(), index)
	p := &nativeForwardingProcess{unit: unit}
	p.command = exec.CommandContext(t.Context(), "systemd-run", "--quiet", "--wait", "--pipe", "--collect", "--unit="+unit,
		"--property=CPUQuota=100%", "--property=CPUQuotaPeriodSec=100ms", "--property=MemoryMax=1G", "--property=MemoryHigh=768M", "--property=TasksMax=256", "--property=LimitNOFILE=1024", "--property=RuntimeMaxSec=120s",
		"--setenv=GOMAXPROCS=1", "--setenv=GOMEMLIMIT=512MiB", "--", binary, "node", "descriptor", "--config", plan)
	p.command.Stderr = &p.diagnostic
	var err error
	p.stdout, err = p.command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := p.command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if !p.joined {
			_ = exec.Command("systemctl", "stop", unit).Run()
			_ = p.command.Wait()
			_ = p.stdout.Close()
			p.joined = true
		}
		if t.Failed() {
			t.Log("original Descriptor retained diagnostic", p.diagnostic.String())
		}
	})
	return p
}

func startNativeDescriptor(t *testing.T, binary, plan string, index byte) *nativeForwardingProcess {
	t.Helper()
	p := launchNativeDescriptor(t, binary, plan, index)
	var event map[string]string
	if err := json.NewDecoder(p.stdout).Decode(&event); err != nil || event["operation"] != "node.descriptor" || event["phase"] != "listening" {
		t.Fatal("genuine original Descriptor startup refused", event, err, p.diagnostic.String())
	}
	raw, err := exec.Command("systemctl", "show", p.unit, "-p", "MainPID", "--value").Output()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Sscanf(string(raw), "%d", &p.pid); err != nil || p.pid < 2 {
		t.Fatal("original Descriptor MainPID unavailable", err)
	}
	return p
}

// Actual canonical commands run in separate sole-process native leaves. The
// signed public Descriptor is a named independent input fixture, not evidence
// of Publisher readiness. All State, Stock, receiving/Hosting, Store and Carrier
// successes come from real new owners. This is bounded development evidence.
func TestNativeDescriptorPublishLookupAndReopenTCP(t *testing.T) {
	nativeDescriptorCommandWorkload(t, "cancel")
}

func TestNativeDescriptorUndercoverageBeforeSpendTCP(t *testing.T) {
	nativeDescriptorCommandWorkload(t, "undercoverage")
}

// Refusals run only in private fixtures under the same bounded managed command.
// Restoring the declared inputs starts a separate original invocation after join;
// it cannot revive the rejected one or rewrite an accepted durable floor.
func TestNativeDescriptorPartialStartupRefusalsTCP(t *testing.T) {
	binary := nodeNativeCommand(t)
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			for _, fault := range []string{"profile", "unknown-field", "nested-roots", "role", "tls-key", "missing-hosting", "foreign-store", "foreign-spend"} {
				t.Run(fault, func(t *testing.T) {
					f, sockets, certificates := descriptorRouteFixture(t, carrier)
					bundle, err := networkfixture.BuildClosed(f.spec)
					if err != nil {
						t.Fatal(err)
					}
					clock, closeClock := joinProcessClock(t)
					defer closeClock()
					id := [32]byte{17}
					cert, key := nativeDescriptorCredential(t, certificates[id])
					participation, spend, budget, storeRoot := filepath.Join(t.TempDir(), "participation"), t.TempDir(), routeProcessBudget(t), t.TempDir()
					plan := map[string]any{"network": seedNativeNodeState(t, f, bundle, clock), "node_id": id, "process_profile": processresource.DescriptorProfile,
						"participation_root": participation, "spend_root": spend, "hosting_root": budget, "descriptor_root": storeRoot,
						"certificate": cert, "private_key": key, "work": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "retirement_wait_ms": 5000}
					expected := 1
					var foreign string
					switch fault {
					case "profile":
						plan["process_profile"], expected = processresource.Profile, 2
					case "unknown-field":
						plan["accepting"] = true
						expected = 2
					case "nested-roots":
						plan["descriptor_root"], expected = filepath.Join(spend, "nested"), 2
					case "role":
						plan["node_id"] = [32]byte{12}
					case "tls-key":
						plan["certificate"], plan["private_key"] = nativeDescriptorCredential(t, certificates[[32]byte{12}])
					case "missing-hosting":
						plan["hosting_root"] = filepath.Join(t.TempDir(), "missing")
					case "foreign-store":
						foreign = filepath.Join(storeRoot, ".ardents-reachability-store-v3")
					case "foreign-spend":
						foreign = filepath.Join(spend, "foreign-input")
					}
					foreignBytes := []byte("independent rejected input\n")
					if foreign != "" {
						if err := os.WriteFile(foreign, foreignBytes, 0600); err != nil {
							t.Fatal(err)
						}
					}
					sockets[id]()
					p := launchNativeDescriptor(t, binary, hostingConfig(t, plan), id[0])
					var event map[string]string
					decodeErr := json.NewDecoder(p.stdout).Decode(&event)
					err = p.command.Wait()
					p.joined = true
					_ = p.stdout.Close()
					var exit *exec.ExitError
					if !errors.As(err, &exit) || exit.ExitCode() != expected {
						t.Fatal("actual Descriptor startup did not refuse", err, p.diagnostic.String())
					}
					if expected == 1 && (decodeErr != nil || len(event) != 4 || event["operation"] != "node.descriptor" || event["phase"] != "refused" || event["outcome"] != "startup-unavailable") {
						t.Fatal("partial startup lost bounded refusal", event, decodeErr)
					}
					if expected == 1 {
						requireNativeStartupFailure(t, event["first_failure"])
					}
					if expected == 2 && (decodeErr == nil || len(event) != 0) {
						t.Fatal("invalid declaration reached runtime", event)
					}
					if _, err := os.Stat(filepath.Join(spend, "closed-token-spends")); !os.IsNotExist(err) {
						t.Fatal("refused startup created spend history", err)
					}
					if foreign != "" {
						raw, err := os.ReadFile(foreign)
						if err != nil || !bytes.Equal(raw, foreignBytes) {
							t.Fatal("rejected private input was rewritten", err)
						}
						// Remove only this test's unaccepted input, never Store records.
						if err := os.Remove(foreign); err != nil {
							t.Fatal(err)
						}
					}
					if _, err := os.Stat(participation); err == nil {
						view, err := f.current()
						if err != nil {
							t.Fatal(err)
						}
						member, err := view.Member(id, view.ObservedAt())
						if err != nil {
							t.Fatal(err)
						}
						journal, err := duty.OpenOperation(t.Context(), duty.Config{Root: participation, Clock: time.Now})
						if err != nil {
							t.Fatal("partial startup participation not released", err)
						}
						conflict, err := journal.Conflict(id, member.FamilyID)
						closeErr := journal.Close()
						if err != nil || closeErr != nil || conflict {
							t.Fatal("partial startup retained original producer", err, closeErr, conflict)
						}
					}
					delete(plan, "accepting")
					plan["process_profile"], plan["node_id"] = processresource.DescriptorProfile, id
					plan["certificate"], plan["private_key"] = cert, key
					plan["hosting_root"], plan["descriptor_root"] = budget, storeRoot
					fresh := startNativeDescriptor(t, binary, hostingConfig(t, plan), id[0])
					fresh.stop(t)
				})
			}
		})
	}
}

func TestNativeDescriptorOriginalCommandFailureTCP(t *testing.T) {
	for _, fault := range []string{"protection-loss", "clock-loss", "store-marker-loss", "cpu-pressure"} {
		t.Run(fault, func(t *testing.T) { nativeDescriptorCommandWorkload(t, fault) })
	}
}

func nativeDescriptorCommandWorkload(t *testing.T, fault string) {
	binary := nodeNativeCommand(t)
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			f, sockets, certificates := descriptorRouteFixture(t, carrier)
			bundle, err := networkfixture.BuildClosed(f.spec)
			if err != nil {
				t.Fatal(err)
			}
			clock, closeClock := joinProcessClock(t)
			defer closeClock()
			descriptorClock, closeDescriptorClock := joinProcessClock(t)
			var clockOnce sync.Once
			stopDescriptorClock := func() { clockOnce.Do(closeDescriptorClock) }
			defer stopDescriptorClock()
			// Fixture signing/issuance is input preparation, outside measured role lifetimes.
			holder := descriptorRouteStock(t, f)
			raw, target := routeSignedDescriptor(t, f, 3, [32]byte{18})
			var processes []*nativeForwardingProcess
			var budgets, participations, spends []string
			var descriptorConfig map[string]any
			descriptorRoot := t.TempDir()
			for _, id := range [][32]byte{{12}, {13}, {14}, {15}, {17}} {
				certPath, keyPath := nativeDescriptorCredential(t, certificates[id])
				budget := routeProcessBudget(t)
				participation := filepath.Join(t.TempDir(), "participation")
				spend := t.TempDir()
				profile := processresource.Profile
				work := hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20}
				if id == [32]byte{17} {
					profile = processresource.DescriptorProfile
					work = hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}
					if fault == "undercoverage" {
						// The fixture provider counts tx+rx: the shared total,
						// rather than each directional maximum, must be one byte short.
						work = hosting.Traffic{Tx: 32 << 10, Rx: (32 << 10) - 1}
					}
				}
				config := map[string]any{"network": seedNativeNodeState(t, f, bundle, clock), "node_id": id, "process_profile": profile, "participation_root": participation, "spend_root": spend, "hosting_root": budget, "certificate": certPath, "private_key": keyPath, "work": work, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "retirement_wait_ms": 5000}
				if id == [32]byte{17} {
					config["descriptor_root"] = descriptorRoot
					networkPlan := config["network"].(networkAuthorityPlan)
					networkPlan.ClockObservationFile = descriptorClock
					config["network"] = networkPlan
					descriptorConfig = config
				}
				sockets[id]()
				var p *nativeForwardingProcess
				if id == [32]byte{17} {
					p = startNativeDescriptor(t, binary, hostingConfig(t, config), id[0])
				} else {
					p = startNativeForwarding(t, binary, hostingConfig(t, config), id[0])
				}
				processes = append(processes, p)
				budgets = append(budgets, budget)
				participations = append(participations, participation)
				spends = append(spends, spend)
			}
			resolution := processes[len(processes)-1]
			journal := filepath.Join(spends[len(spends)-1], "closed-token-spends")
			before, err := os.ReadFile(journal)
			if err != nil || len(before) != 8+32+32+32+8 {
				t.Fatal("Descriptor receiving root not genuine empty header", err, len(before))
			}
			plan := routePrefixPlan{EntryRoot: filepath.Join(t.TempDir(), "entry"), InteriorRoot: filepath.Join(t.TempDir(), "interior"), HostingRoot: routeProcessBudget(t), Domain: 1, Deadline: time.Now().Add(30 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
			prefix, err := startRoutePrefix(t.Context(), plan, f.authority, holder)
			if err != nil {
				t.Fatal("genuine adjacent Forwarding unavailable", err)
			}
			prefixJoined := false
			defer func() {
				if !prefixJoined {
					if err := prefix.close(); err != nil {
						if fault == "cancel" {
							t.Error(err)
						} else {
							t.Log("retained original Source cleanup after forced failure", err)
						}
					}
				}
			}()
			history := &reachability.History{}
			defer history.Close()
			if fault == "undercoverage" {
				if err := prefix.publishDescriptor(t.Context(), raw); err == nil {
					t.Fatal("undersized Descriptor work admitted full Control")
				}
				after, err := os.ReadFile(journal)
				if err != nil || !bytes.Equal(before, after) {
					t.Fatal("physical undercoverage spent Control", err)
				}
				entries, err := os.ReadDir(filepath.Join(descriptorRoot, "records"))
				if err != nil || len(entries) != 0 {
					t.Fatal("physical undercoverage reached Store mutation", err)
				}
				if _, err := os.Stat(fmt.Sprintf("/proc/%d", resolution.pid)); err != nil {
					t.Fatal("request refusal retired the original command", err)
				}
				// A corrected declaration applies only to a fresh original command
				// after joined Stop, retaining the same unspent durable roots.
				resolution.stop(t, true)
				descriptorConfig["work"] = hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}
				resolution = startNativeDescriptor(t, binary, hostingConfig(t, descriptorConfig), 17)
				processes[len(processes)-1] = resolution
			}
			if err := prefix.publishDescriptor(t.Context(), raw); err != nil {
				t.Fatal("actual original Descriptor publish/ACK failed", err)
			}
			assertLookup := func() {
				t.Helper()
				got, err := prefix.lookupDescriptor(t.Context(), target, history)
				if err != nil || !bytes.Equal(got, raw) {
					t.Fatal("actual original Descriptor exact lookup failed", err)
				}
			}
			assertLookup()
			initial, err := os.ReadFile(journal)
			if err != nil || len(initial) <= len(before) {
				t.Fatal("publish/lookup did not irreversibly spend genuine Control", err)
			}
			if fault == "cancel" {
				connectOnly, _ := routeSignedDescriptor(t, f, 2, [32]byte{18})
				broken := bytes.Clone(raw)
				broken[len(broken)-1] ^= 1
				foreignIntroduction, _ := routeSignedDescriptor(t, f, 3, [32]byte{12})
				for _, input := range [][]byte{connectOnly, broken, foreignIntroduction} {
					if err := prefix.publishDescriptor(t.Context(), input); err == nil {
						t.Fatal("invalid signed input accepted")
					}
				}
				if _, err := prefix.lookupDescriptor(t.Context(), [32]byte{99}, history); err == nil {
					t.Fatal("foreign Target accepted")
				}
				assertLookup() // every healthy request refusal leaves the original usable
			}
			after, err := os.ReadFile(journal)
			if err != nil || len(after) < len(initial) || (fault == "cancel" && len(after) <= len(initial)) {
				t.Fatal("refused Descriptor requests refunded admission", err)
			}
			switch fault {
			case "cpu-pressure":
				nativeDescriptorCPUContention(t, resolution)
				resolution.failedJoin(t, "process")
				var event map[string]string
				var first struct{ Resource *processresource.FailureFacts }
				if err := json.Unmarshal([]byte(resolution.diagnostic.String()), &event); err != nil {
					t.Fatal("original CPU outcome framing unavailable", err)
				}
				if err := json.Unmarshal([]byte(event["first_failure"]), &first); err != nil || first.Resource == nil || first.Resource.Cause != "cpu-pressure" || first.Resource.Observation == nil || first.Resource.Observation.CPUPressure < 1 || first.Resource.Observation.CPUWindowStarted.IsZero() {
					t.Fatal("CPU negative lacked actual retained original overload", err, resolution.diagnostic.String())
				}
			case "protection-loss":
				if err := exec.Command("systemctl", "set-property", "--runtime", resolution.unit, "CPUQuota=50%").Run(); err != nil {
					t.Fatal(err)
				}
				resolution.failedJoin(t, "process")
			case "clock-loss":
				stopDescriptorClock()
				if err := os.Remove(descriptorClock); err != nil {
					t.Fatal(err)
				}
				resolution.failedJoin(t, "network")
			case "store-marker-loss":
				marker := filepath.Join(descriptorRoot, ".ardents-reachability-store-v3")
				original, err := os.ReadFile(marker)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(marker, []byte("foreign marker\n"), 0600); err != nil {
					t.Fatal(err)
				}
				resolution.failedJoin(t, "reachability")
				if err := os.WriteFile(marker, original, 0600); err != nil {
					t.Fatal(err)
				}
			case "cancel", "undercoverage":
			default:
				t.Fatal("unselected Descriptor command fault", fault)
			}
			prefixErr := prefix.close()
			prefixJoined = true
			if prefixErr != nil {
				if fault == "cancel" {
					t.Fatal("original Source/prefix did not physically join", prefixErr)
				}
				t.Log("retained original Source cleanup after forced failure", prefixErr)
			}
			// All original callers stop before any peer is waited, preserving the
			// distinction between deliberate cancellation and manufactured peer loss.
			signals := []string{"kill", "--kill-whom=main", "--signal=SIGINT"}
			for _, p := range processes {
				if !p.joined {
					signals = append(signals, p.unit)
				}
			}
			if output, err := exec.Command("systemctl", signals...).CombinedOutput(); err != nil {
				t.Fatal("original caller batch retirement failed", err, string(output))
			}
			for _, p := range processes {
				p.signaled = true
				p.stop(t, fault != "cancel")
			}
			if !resolution.joined {
				t.Fatal("Descriptor manager completion absent")
			}
			fresh, err := reachability.OpenStore(reachability.StoreConfig{Root: descriptorRoot, Network: f.profile.NetworkID})
			if err != nil {
				t.Fatal("actual Store lease/custody did not reopen after original join", err)
			}
			got, outcome, lookupErr := fresh.Lookup(target, f.profile.Digest, time.Now())
			if err := fresh.Close(); err != nil || lookupErr != nil || outcome != reachability.Accepted || !bytes.Equal(got, raw) {
				t.Fatal("ACK did not retain exact durable proof", outcome, lookupErr, err)
			}
			if retained, err := os.ReadFile(journal); err != nil || !bytes.Equal(retained, after) {
				t.Fatal("original Close changed irreversible spends", err)
			}
			for index, root := range participations {
				view, err := f.current()
				if err != nil {
					t.Fatal(err)
				}
				id := [][32]byte{{12}, {13}, {14}, {15}, {17}}[index]
				member, err := view.Member(id, view.ObservedAt())
				if err != nil {
					t.Fatal(err)
				}
				owner, err := duty.OpenOperation(t.Context(), duty.Config{Root: root, Clock: time.Now})
				if err != nil {
					t.Fatal(err)
				}
				conflict, err := owner.Conflict(member.NodeID, member.FamilyID)
				if closeErr := owner.Close(); err != nil || closeErr != nil || conflict {
					t.Fatal("original producer retained after actual join", err, closeErr, conflict)
				}
			}
			for _, root := range budgets {
				budget, err := hosting.Open(root)
				if err != nil {
					t.Fatal(err)
				}
				observed, err := budget.Observe(context.Background())
				closeErr := budget.Close()
				if err != nil || closeErr != nil || observed.ReservedBytes != 0 {
					t.Fatal("original Hosting returned before or failed after join", err, closeErr, observed.ReservedBytes)
				}
			}
		})
	}
}

// Create genuine scheduling pressure after successful work, without changing
// the selected cgroup/Go/FD envelope or placing another process in the role leaf.
func nativeDescriptorCPUContention(t *testing.T, original *nativeForwardingProcess) {
	t.Helper()
	if _, err := exec.LookPath("taskset"); err != nil {
		t.Fatal("selected native CPU-affinity prerequisite unavailable", err)
	}
	proc := filepath.Join("/proc", strconv.Itoa(original.pid))
	status, err := os.ReadFile(filepath.Join(proc, "status"))
	if err != nil {
		t.Fatal(err)
	}
	cpu := ""
	for _, line := range strings.Split(string(status), "\n") {
		if strings.HasPrefix(line, "Cpus_allowed_list:") {
			cpu = strings.Split(strings.Split(strings.TrimSpace(strings.TrimPrefix(line, "Cpus_allowed_list:")), ",")[0], "-")[0]
		}
	}
	if _, err := strconv.ParseUint(cpu, 10, 32); err != nil {
		t.Fatal("original allowed CPU unavailable", err)
	}
	groupRaw, err := os.ReadFile(filepath.Join(proc, "cgroup"))
	if err != nil {
		t.Fatal(err)
	}
	group := strings.TrimSpace(string(groupRaw))
	if !strings.HasPrefix(group, "0::/") || strings.Contains(group, "\n") {
		t.Fatal("original sole cgroup unavailable")
	}
	root := filepath.Join("/sys/fs/cgroup", strings.TrimPrefix(group, "0::/"))
	maximum, err := os.ReadFile(filepath.Join(root, "cpu.max"))
	if err != nil || string(maximum) != "100000 100000\n" {
		t.Fatal("original CPU envelope changed before pressure", string(maximum), err)
	}
	pressure := func() float64 {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(root, "cpu.pressure"))
		if err != nil {
			t.Fatal("original pressure unavailable", err)
		}
		for _, line := range strings.Split(string(raw), "\n") {
			fields := strings.Fields(line)
			if len(fields) == 5 && fields[0] == "some" && strings.HasPrefix(fields[1], "avg10=") {
				value, err := strconv.ParseFloat(strings.TrimPrefix(fields[1], "avg10="), 64)
				if err != nil || value < 0 || value > 100 {
					t.Fatal("actual pressure invalid", err)
				}
				return value
			}
		}
		t.Fatal("original pressure grammar unavailable")
		return 0
	}
	if pressure() != 0 {
		t.Fatal("pressure already present before causal control")
	}
	if output, err := exec.Command("taskset", "-apc", cpu, strconv.Itoa(original.pid)).CombinedOutput(); err != nil {
		t.Fatal("original allowed CPU affinity unavailable", err, string(output))
	}
	// Twelve bounded helper groups contend on ONE original allowed CPU. The
	// measured development control uses at most one busy CPU, 384 MiB declared
	// auxiliary memory and 24 helper tasks, outside the sole original leaf.
	// Positive PSI alone is not the selected 1% overload criterion.
	for index := range 12 {
		unit := fmt.Sprintf("ardents-descriptor-cpu-helper-%d-%d.service", os.Getpid(), index)
		// Independent manager limits survive a forced exit of the test driver.
		// These are test-helper limits, never the measured Node's envelope.
		helper := exec.CommandContext(t.Context(), "systemd-run", "--quiet", "--wait", "--pipe", "--collect", "--unit="+unit,
			"--property=RuntimeMaxSec=8s", "--property=CPUQuota=50%", "--property=MemoryMax=32M", "--property=TasksMax=2", "--property=LimitNOFILE=64",
			"--property=DynamicUser=yes", "--property=NoNewPrivileges=yes", "--property=PrivateTmp=yes",
			"--", "taskset", "-c", cpu, "sh", "-c", "while :; do :; done")
		if err := helper.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = exec.Command("systemctl", "stop", unit).Run(); _ = helper.Wait() })
		helperPID := 0
		readyUntil := time.Now().Add(time.Second)
		for helperPID == 0 {
			raw, err := exec.Command("systemctl", "show", unit, "-p", "MainPID", "--value").Output()
			if err == nil {
				helperPID, _ = strconv.Atoi(strings.TrimSpace(string(raw)))
			}
			if helperPID > 1 {
				break
			}
			if time.Now().After(readyUntil) {
				t.Fatal("bounded original CPU helper unavailable")
			}
			helperPID = 0
			time.Sleep(5 * time.Millisecond)
		}
		helperGroup, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(helperPID), "cgroup"))
		if err != nil || bytes.Equal(helperGroup, groupRaw) {
			t.Fatal("CPU helper entered original role leaf", err)
		}
	}
	deadline := time.Now().Add(6 * time.Second)
	for pressure() < 1 {
		if time.Now().After(deadline) {
			t.Fatal("causal original CPU pressure did not become measured")
		}
		time.Sleep(5 * time.Millisecond)
	}
	retained, err := os.ReadFile(filepath.Join(root, "cpu.max"))
	if err != nil || !bytes.Equal(retained, maximum) {
		t.Fatal("pressure control changed required CPU envelope", err)
	}
}
