//go:build linux && node_process_native

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/processresource"
	"github.com/dianabuilds/ardents-network/internal/reachability"
	"github.com/dianabuilds/ardents-network/internal/route/transport"
	nodefixture "github.com/dianabuilds/ardents-network/tests/e2e/node"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

func TestNativeDescriptorPublishLookupAndReopenTCP(t *testing.T) {
	nativeDescriptorCommandWorkload(t, "cancel")
}

func TestNativeDescriptorUndercoverageBeforeSpendTCP(t *testing.T) {
	nativeDescriptorCommandWorkload(t, "undercoverage")
}

func TestNativeDescriptorOriginalCommandFailureTCP(t *testing.T) {
	for _, fault := range []string{"protection-loss", "clock-loss", "store-marker-loss", "cpu-pressure"} {
		t.Run(fault, func(t *testing.T) { nativeDescriptorCommandWorkload(t, fault) })
	}
}

func nativeDescriptorCommandWorkload(t *testing.T, fault string) {
	binary := nodefixture.PinnedCommand(t)
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			f, sockets, certificates := descriptorRouteFixture(t, carrier)
			bundle, err := networkfixture.BuildClosed(f.spec)
			if err != nil {
				t.Fatal(err)
			}
			clock, closeClock := nodefixture.StartObservedClock(t)
			defer closeClock()
			descriptorClock, closeDescriptorClock := nodefixture.StartObservedClock(t)
			var clockOnce sync.Once
			stopDescriptorClock := func() { clockOnce.Do(closeDescriptorClock) }
			defer stopDescriptorClock()
			// Fixture signing/issuance is input preparation, outside measured role lifetimes.
			holder := descriptorRouteStock(t, f)
			raw, target := routeSignedDescriptor(t, f, 3, [32]byte{18})
			var processes []*nodefixture.NativeProcess
			var budgets, participations, spends []string
			var descriptorConfig map[string]any
			descriptorRoot := t.TempDir()
			for _, id := range [][32]byte{{12}, {13}, {14}, {15}, {17}} {
				certPath, keyPath := nodefixture.WriteNativeCredential(t, certificates[id])
				budget := nodefixture.ProcessBudget(t)
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
				config := map[string]any{"network": nodefixture.SeedNetworkState(t, f.spec.NetworkID, [32]byte(f.spec.Authority[32:]), bundle, clock), "node_id": id, "process_profile": profile, "participation_root": participation, "spend_root": spend, "hosting_root": budget, "certificate": certPath, "private_key": keyPath, "work": work, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "retirement_wait_ms": 5000}
				if id == [32]byte{17} {
					config["descriptor_root"] = descriptorRoot
					networkPlan := config["network"].(state.AuthorityDeclaration)
					networkPlan.ClockObservationFile = descriptorClock
					config["network"] = networkPlan
					descriptorConfig = config
				}
				sockets[id]()
				var p *nodefixture.NativeProcess
				if id == [32]byte{17} {
					p = nodefixture.StartFreshRole(t, "descriptor", binary, hostingConfig(t, config), id[0])
				} else {
					p = nodefixture.StartFreshRole(t, "forwarding", binary, hostingConfig(t, config), id[0])
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
			plan := routePrefixPlan{EntryRoot: filepath.Join(t.TempDir(), "entry"), InteriorRoot: filepath.Join(t.TempDir(), "interior"), HostingRoot: nodefixture.ProcessBudget(t), Domain: 1, Deadline: time.Now().Add(30 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
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
				if _, err := os.Stat(fmt.Sprintf("/proc/%d", resolution.PID)); err != nil {
					t.Fatal("request refusal retired the original command", err)
				}
				// A corrected declaration applies only to a fresh original command
				// after joined Stop, retaining the same unspent durable roots.
				resolution.Stop(t, true)
				descriptorConfig["work"] = hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}
				resolution = nodefixture.StartFreshRole(t, "descriptor", binary, hostingConfig(t, descriptorConfig), 17)
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
				nodefixture.ExerciseNativeCPUPressure(t, resolution)
				resolution.FailedJoin(t, "process")
				var event map[string]string
				var first struct{ Resource *processresource.FailureFacts }
				if err := json.Unmarshal([]byte(resolution.Diagnostic.String()), &event); err != nil {
					t.Fatal("original CPU outcome framing unavailable", err)
				}
				if err := json.Unmarshal([]byte(event["first_failure"]), &first); err != nil || first.Resource == nil || first.Resource.Cause != "cpu-pressure" || first.Resource.Observation == nil || first.Resource.Observation.CPUPressure < 1 || first.Resource.Observation.CPUWindowStarted.IsZero() {
					t.Fatal("CPU negative lacked actual retained original overload", err, resolution.Diagnostic.String())
				}
			case "protection-loss":
				if err := exec.Command("systemctl", "set-property", "--runtime", resolution.Unit, "CPUQuota=50%").Run(); err != nil {
					t.Fatal(err)
				}
				resolution.FailedJoin(t, "process")
			case "clock-loss":
				stopDescriptorClock()
				if err := os.Remove(descriptorClock); err != nil {
					t.Fatal(err)
				}
				resolution.FailedJoin(t, "network")
			case "store-marker-loss":
				marker := filepath.Join(descriptorRoot, ".ardents-reachability-store-v3")
				original, err := os.ReadFile(marker)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(marker, []byte("foreign marker\n"), 0600); err != nil {
					t.Fatal(err)
				}
				resolution.FailedJoin(t, "reachability")
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
				if !p.Joined {
					signals = append(signals, p.Unit)
				}
			}
			if output, err := exec.Command("systemctl", signals...).CombinedOutput(); err != nil {
				t.Fatal("original caller batch retirement failed", err, string(output))
			}
			for _, p := range processes {
				p.Signaled = true
				p.Stop(t, fault != "cancel")
			}
			if !resolution.Joined {
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
