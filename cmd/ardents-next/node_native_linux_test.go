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
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/processresource"
	"github.com/dianabuilds/ardents-network/internal/route/transport"
	nodefixture "github.com/dianabuilds/ardents-network/tests/e2e/node"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

func launchNativeForwarding(t *testing.T, binary, plan string, index byte) *nodefixture.NativeProcess {
	t.Helper()
	unit := fmt.Sprintf("ardents-forwarding-%d-%d.service", os.Getpid(), index)
	return nodefixture.Launch(t, unit, "original Forwarding retained diagnostic after physical join", "",
		"--property=CPUQuota=100%", "--property=CPUQuotaPeriodSec=100ms", "--property=MemoryMax=1G", "--property=MemoryHigh=768M", "--property=TasksMax=256", "--property=LimitNOFILE=1024", "--property=RuntimeMaxSec=120s",
		"--setenv=GOMAXPROCS=1", "--setenv=GOMEMLIMIT=512MiB", "--", binary, "node", "forwarding", "--config", plan)
}

func startNativeForwarding(t *testing.T, binary, plan string, index byte) *nodefixture.NativeProcess {
	t.Helper()
	process := launchNativeForwarding(t, binary, plan, index)
	var event map[string]string
	if err := json.NewDecoder(process.Stdout).Decode(&event); err != nil || event["operation"] != "node.forwarding" || event["phase"] != "listening" {
		t.Fatal("genuine original Node startup refused", event, err, process.Diagnostic.String())
	}
	raw, err := exec.Command("systemctl", "show", process.Unit, "-p", "MainPID", "--value").Output()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Sscanf(string(raw), "%d", &process.PID); err != nil || process.PID < 2 {
		t.Fatal("actual Node MainPID unavailable", err)
	}
	return process
}

func TestNativeNodeForwardingTCP(t *testing.T) {
	for _, retirement := range []string{"cancel", "protection-loss", "clock-loss", "undercoverage"} {
		t.Run(retirement, func(t *testing.T) { testNativeNodeForwarding(t, retirement) })
	}
}

// Startup's new field is a closed diagnostic projection, not permission to add
// arbitrary fields or relax the original refusal/outcome/durability assertions.
func requireNativeStartupFailure(t *testing.T, raw string) {
	t.Helper()
	var fact struct {
		Kind, Phase, Cause string
		ElapsedMS          *int64 `json:"elapsed_ms"`
	}
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&fact); err != nil || fact.Kind != "logical" || fact.Phase != "startup" || fact.Cause != "owner-refusal" || fact.ElapsedMS == nil || *fact.ElapsedMS < 0 {
		t.Fatal("partial startup lost closed original first-cause projection", raw, err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		t.Fatal("startup diagnostic contains trailing data", err)
	}
}

func testNativeNodeForwarding(t *testing.T, retirement string) {
	binary := nodefixture.PinnedCommand(t)
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			var provisioned map[[32]byte][2]string
			f, sockets, certificates := newRoleRouteFixtureConfigured(t, carrier, 3, false, false, func(f *networkAdmissionFixture, _ map[[32]byte]func(), certificates map[[32]byte]tls.Certificate) {
				if retirement == "certificate" {
					provisioned = provisionNativeNodeCertificates(t, f, certificates, [32]byte{12}, [32]byte{13}, [32]byte{14}, [32]byte{15})
				}
			})
			bundle, err := networkfixture.BuildClosed(f.spec)
			if err != nil {
				t.Fatal(err)
			}
			clock, closeClock := joinProcessClock(t)
			var clockOnce sync.Once
			stopClock := func() { clockOnce.Do(closeClock) }
			defer stopClock()
			seed := func() state.AuthorityDeclaration {
				return nodefixture.SeedNetworkState(t, f.spec.NetworkID, [32]byte(f.spec.Authority[32:]), bundle, clock)
			}
			var processes []*nodefixture.NativeProcess
			var budgets []string
			var participation []string
			var spendRoots []string
			var initialSpends [][]byte
			for index := byte(12); index < 16; index++ {
				id := [32]byte{index}
				certificate := certificates[id]
				certPath, keyPath := filepath.Join(t.TempDir(), "certificate"), filepath.Join(t.TempDir(), "key")
				if pair, ok := provisioned[id]; ok {
					certPath, keyPath = pair[0], pair[1]
				} else {
					if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Certificate[0]}), 0600); err != nil {
						t.Fatal(err)
					}
					key, err := x509.MarshalPKCS8PrivateKey(certificate.PrivateKey)
					if err != nil {
						t.Fatal(err)
					}
					encoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key})
					if err := os.WriteFile(keyPath, encoded, 0600); err != nil {
						t.Fatal(err)
					}
					clear(key)
					clear(encoded)
				}
				budget := routeProcessBudget(t)
				budgets = append(budgets, budget)
				root := filepath.Join(t.TempDir(), "participation")
				participation = append(participation, root)
				spend := t.TempDir()
				spendRoots = append(spendRoots, spend)
				work := hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20}
				if retirement == "undercoverage" {
					work = hosting.Traffic{Tx: 1 << 20, Rx: 1 << 20}
				}
				plan := map[string]any{"network": seed(), "node_id": id, "process_profile": processresource.Profile, "participation_root": root, "spend_root": spend, "hosting_root": budget, "certificate": certPath, "private_key": keyPath, "work": work, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "retirement_wait_ms": 5000}
				sockets[id]()
				processes = append(processes, startNativeForwarding(t, binary, hostingConfig(t, plan), index))
				before, err := os.ReadFile(filepath.Join(spend, "closed-token-spends"))
				if err != nil || len(before) != 8+32+32+32+8 {
					t.Fatal("fresh real receiving ledger is not an empty canonical header", err)
				}
				initialSpends = append(initialSpends, before)
			}
			holder := routeStock(t, f)
			plan := routePrefixPlan{EntryRoot: filepath.Join(t.TempDir(), "entry"), InteriorRoot: filepath.Join(t.TempDir(), "interior"), HostingRoot: routeProcessBudget(t), Domain: 3, Deadline: time.Now().Add(30 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
			prefix, err := startRoutePrefix(context.Background(), plan, f.authority, holder)
			if retirement == "undercoverage" {
				if err == nil {
					_ = prefix.close()
					t.Fatal("undersized actual Hosting admitted the complete Forward allowance")
				}
			} else if err != nil {
				t.Fatal("actual Forwarding prefix failed", err)
			} else if err := prefix.close(); err != nil {
				t.Fatal("actual Forwarding physical join failed", err)
			}
			switch retirement {
			case "protection-loss":
				if err := exec.Command("systemctl", "set-property", "--runtime", processes[0].Unit, "CPUQuota=50%").Run(); err != nil {
					t.Fatal(err)
				}
				processes[0].FailedJoin(t, "process")
			case "clock-loss":
				stopClock()
				if err := os.Remove(clock); err != nil {
					t.Fatal(err)
				}
				for _, process := range processes {
					process.FailedJoin(t, "network")
				}
			}
			// Retire every original caller before waiting for any peer's join.
			// Sequential stop-and-wait would manufacture peer loss in still-live
			// invocations and mix that failure with the cancellation control.
			signals := []string{"kill", "--kill-whom=main", "--signal=SIGINT"}
			for _, process := range processes {
				if !process.Joined {
					signals = append(signals, process.Unit)
				}
			}
			if len(signals) > 3 {
				if raw, err := exec.Command("systemctl", signals...).CombinedOutput(); err != nil {
					for _, process := range processes {
						t.Log("original unit terminal evidence", process.Unit, process.Diagnostic.String())
					}
					t.Fatal("original caller batch cancellation unavailable", err, string(raw))
				}
				for _, process := range processes {
					process.Signaled = true
				}
			}
			for index, process := range processes {
				// A forced protection loss also closes its real peer channels;
				// adjacent invocations retain any resulting Route cleanup failure.
				process.Stop(t, retirement == "undercoverage" || retirement == "protection-loss")
				if retirement == "undercoverage" {
					after, err := os.ReadFile(filepath.Join(spendRoots[index], "closed-token-spends"))
					if err != nil || !bytes.Equal(initialSpends[index], after) {
						t.Fatal("pre-spend physical coverage refusal changed real durable spend", err)
					}
				}
				view, err := f.current()
				if err != nil {
					t.Fatal(err)
				}
				member, err := view.Member([32]byte{byte(12 + index)}, view.ObservedAt())
				if err != nil {
					t.Fatal(err)
				}
				journal, err := duty.OpenOperation(t.Context(), duty.Config{Root: participation[index], Clock: time.Now})
				if err != nil {
					t.Fatal("participation unavailable after known join", err)
				}
				conflict, err := journal.Conflict(member.NodeID, member.FamilyID)
				if closeErr := journal.Close(); err != nil || closeErr != nil || conflict {
					t.Fatal("joined original producer still conflicts", err, closeErr, conflict)
				}
			}
			for _, root := range budgets {
				budget, err := hosting.Open(root)
				if err != nil {
					t.Fatal(err)
				}
				observation, err := budget.Observe(t.Context())
				closeErr := budget.Close()
				if err != nil || closeErr != nil || observation.ReservedBytes != 0 {
					t.Fatal("joined original Node retained or invented capacity", err, closeErr, observation.ReservedBytes)
				}
			}
		})
	}
}

func TestNativeNodePartialStartupRefusalsTCP(t *testing.T) {
	binary := nodefixture.PinnedCommand(t)
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			f, sockets, certificates := newRouteFixture(t, carrier)
			bundle, err := networkfixture.BuildClosed(f.spec)
			if err != nil {
				t.Fatal(err)
			}
			clock, closeClock := joinProcessClock(t)
			defer closeClock()
			view, err := f.current()
			if err != nil {
				t.Fatal(err)
			}
			member, err := view.Member([32]byte{12}, view.ObservedAt())
			if err != nil {
				t.Fatal(err)
			}
			for index, fault := range []string{"wrong-key", "wrong-role", "shared-root", "wrong-profile", "hosting-open", "spend-open", "listener-open", "identity-conflict", "family-conflict", "state-conflict", "expired-member"} {
				t.Run(fault, func(t *testing.T) {
					certificate := certificates[[32]byte{12}]
					if fault == "wrong-key" {
						certificate = certificates[[32]byte{13}]
					}
					certPath, keyPath := filepath.Join(t.TempDir(), "certificate"), filepath.Join(t.TempDir(), "key")
					if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Certificate[0]}), 0600); err != nil {
						t.Fatal(err)
					}
					key, err := x509.MarshalPKCS8PrivateKey(certificate.PrivateKey)
					if err != nil {
						t.Fatal(err)
					}
					encoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key})
					if err := os.WriteFile(keyPath, encoded, 0600); err != nil {
						t.Fatal(err)
					}
					clear(key)
					clear(encoded)
					initial := bundle
					var expires time.Time
					if fault == "expired-member" {
						short := f.spec
						short.Nodes = append([]networkfixture.ClosedNode(nil), f.spec.Nodes...)
						expires = time.Now().Add(2 * time.Second).UTC().Truncate(time.Second)
						for index := range short.Nodes {
							if short.Nodes[index].NodeID == [32]byte{12} {
								short.Nodes[index].ValidUntil = expires
							}
						}
						initial, err = networkfixture.BuildClosed(short)
						if err != nil {
							t.Fatal(err)
						}
					}
					networkPlan := nodefixture.SeedNetworkState(t, f.spec.NetworkID, [32]byte(f.spec.Authority[32:]), initial, clock)
					if fault == "state-conflict" {
						changed := f.spec
						changed.IssuanceAuthority[0] ^= 1
						conflict, err := networkfixture.BuildClosed(changed)
						if err != nil || conflict.Epoch.Digest != initial.Epoch.Digest {
							t.Fatal("conflicting profile changed its original Epoch", err)
						}
						config, err := networkPlan.Config()
						if err != nil {
							t.Fatal(err)
						}
						stateOwner, err := state.Open(config)
						if err != nil {
							t.Fatal(err)
						}
						_, conflictErr := stateOwner.AcceptClosedProfile(conflict.Profile)
						if closeErr := stateOwner.Close(); conflictErr == nil || closeErr != nil {
							t.Fatal("genuine signed State conflict was not retained", conflictErr, closeErr)
						}
					}
					if fault == "expired-member" {
						timer := time.NewTimer(time.Until(expires) + 50*time.Millisecond)
						select {
						case <-timer.C:
						case <-t.Context().Done():
							t.Fatal(t.Context().Err())
						}
						timer.Stop()
					}
					participation := filepath.Join(t.TempDir(), "participation")
					spend := t.TempDir()
					budget := routeProcessBudget(t)
					configuredBudget := budget
					id := [32]byte{12}
					if fault == "wrong-role" {
						id = [32]byte{11}
					}
					if fault == "hosting-open" {
						configuredBudget = filepath.Join(t.TempDir(), "absent")
					}
					if fault == "spend-open" {
						spend = filepath.Join(t.TempDir(), "regular-file")
						if err := os.WriteFile(spend, []byte("preserve this unrelated file"), 0600); err != nil {
							t.Fatal(err)
						}
					}
					conflicted := fault == "identity-conflict" || fault == "family-conflict"
					var conflictPointer []byte
					if conflicted {
						journal, err := duty.OpenOperation(t.Context(), duty.Config{Root: participation, Clock: time.Now, Create: true})
						if err != nil {
							t.Fatal(err)
						}
						identity, family := member.NodeID, member.FamilyID
						if fault == "identity-conflict" {
							family = [32]byte{90}
						} else {
							identity = [32]byte{90}
						}
						if err := journal.Replace([32]byte{99}, []duty.Duty{{Identity: identity, Family: family, Class: "direct-source", State: "live", NotAfter: time.Now().Add(time.Hour)}}); err != nil {
							t.Fatal(err)
						}
						if err := journal.Close(); err != nil {
							t.Fatal(err)
						}
						conflictPointer, err = os.ReadFile(filepath.Join(participation, "current"))
						if err != nil {
							t.Fatal(err)
						}
					}
					plan := map[string]any{"network": networkPlan, "node_id": id, "process_profile": processresource.Profile, "participation_root": participation, "spend_root": spend, "hosting_root": configuredBudget, "certificate": certPath, "private_key": keyPath, "work": hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "retirement_wait_ms": 5000}
					invalidInput := fault == "shared-root" || fault == "wrong-profile"
					if fault == "shared-root" {
						plan["spend_root"] = participation
					}
					if fault == "wrong-profile" {
						plan["process_profile"] = "unselected-profile"
					}
					// The original reserved socket deliberately remains occupied. Only
					// listener-open reaches that late acquisition; earlier faults must
					// refuse without creating this invocation's accepting role.
					process := launchNativeForwarding(t, binary, hostingConfig(t, plan), byte(50+index))
					var event map[string]string
					decodeErr := json.NewDecoder(process.Stdout).Decode(&event)
					waitErr := process.Command.Wait()
					process.Joined = true
					_ = process.Stdout.Close()
					if conflicted {
						after, err := os.ReadFile(filepath.Join(participation, "current"))
						if err != nil || !bytes.Equal(conflictPointer, after) {
							t.Fatal("conflict refusal replaced original Source history", err)
						}
					}
					var exit *exec.ExitError
					if invalidInput {
						if decodeErr != io.EOF || len(event) != 0 || !errors.As(waitErr, &exit) || exit.ExitCode() != 2 {
							t.Fatal("invalid input reached runtime or reported acceptance", event, decodeErr, waitErr)
						}
					} else if decodeErr != nil || event["phase"] != "refused" || event["outcome"] != "startup-unavailable" || !errors.As(waitErr, &exit) || exit.ExitCode() != 1 {
						t.Fatal("partial startup published accepting work or lost refusal", event, decodeErr, waitErr, process.Diagnostic.String())
					}
					if _, err := os.Stat(filepath.Join(spend, "closed-token-spends")); fault == "listener-open" {
						raw, readErr := os.ReadFile(filepath.Join(spend, "closed-token-spends"))
						if readErr != nil || len(raw) != 112 {
							t.Fatal("late startup did not retain the empty genuine spend ledger", readErr)
						}
					} else if fault == "spend-open" {
						raw, readErr := os.ReadFile(spend)
						if readErr != nil || string(raw) != "preserve this unrelated file" {
							t.Fatal("partial startup changed the foreign regular file", readErr)
						}
					} else if !os.IsNotExist(err) {
						t.Fatal("early refusal created spend effects", err)
					}
					if _, err := os.Stat(participation); err == nil {
						journal, err := duty.OpenOperation(t.Context(), duty.Config{Root: participation, Clock: time.Now})
						if err != nil {
							t.Fatal(err)
						}
						collision, err := journal.Conflict(member.NodeID, member.FamilyID)
						if closeErr := journal.Close(); err != nil || closeErr != nil || collision != conflicted {
							t.Fatal("partial cleanup erased foreign Source or retained own producer", err, closeErr, collision)
						}
					} else if !os.IsNotExist(err) || conflicted || fault == "hosting-open" || fault == "spend-open" || fault == "listener-open" {
						t.Fatal("participation history unavailable", err)
					}
					owner, err := hosting.Open(budget)
					if err != nil {
						t.Fatal(err)
					}
					observed, err := owner.Observe(t.Context())
					if closeErr := owner.Close(); err != nil || closeErr != nil || observed.ReservedBytes != 0 {
						t.Fatal("partial startup retained physical capacity", err, closeErr)
					}
					config, err := networkPlan.Config()
					if err != nil {
						t.Fatal(err)
					}
					reopened, err := state.Open(config)
					if err != nil {
						t.Fatal("original State did not close after partial join", err)
					}
					current, currentErr := reopened.CurrentRuntime()
					if fault == "state-conflict" {
						if currentErr == nil {
							t.Fatal("startup State conflict vanished on reopen")
						}
					} else if currentErr != nil {
						t.Fatal(currentErr)
					} else if fault == "expired-member" {
						if _, err := current.Member(id, current.ObservedAt()); err == nil {
							t.Fatal("expired original member revived on reopen")
						}
					}
					if err := reopened.Close(); err != nil {
						t.Fatal(err)
					}
				})
			}
			// Release the actual reserved endpoint only after the entire refusal
			// matrix; no successful listener is fabricated for a negative case.
			sockets[[32]byte{12}]()
			certificate := certificates[[32]byte{12}]
			certPath, keyPath := filepath.Join(t.TempDir(), "certificate"), filepath.Join(t.TempDir(), "key")
			if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Certificate[0]}), 0600); err != nil {
				t.Fatal(err)
			}
			key, err := x509.MarshalPKCS8PrivateKey(certificate.PrivateKey)
			if err != nil {
				t.Fatal(err)
			}
			encoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key})
			if err := os.WriteFile(keyPath, encoded, 0600); err != nil {
				t.Fatal(err)
			}
			clear(key)
			clear(encoded)
			plan := map[string]any{"network": nodefixture.SeedNetworkState(t, f.spec.NetworkID, [32]byte(f.spec.Authority[32:]), bundle, clock), "node_id": [32]byte{12}, "process_profile": processresource.Profile, "participation_root": filepath.Join(t.TempDir(), "participation"), "spend_root": t.TempDir(), "hosting_root": routeProcessBudget(t), "certificate": certPath, "private_key": keyPath, "work": hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "retirement_wait_ms": 5000}
			// Genuine restored-input control: this same signed member, Carrier and
			// native envelope must start after removing the declared fault.
			control := startNativeForwarding(t, binary, hostingConfig(t, plan), 80)
			control.Stop(t)
		})
	}
}
