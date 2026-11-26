//go:build linux && node_process_native

package node

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/processresource"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
	networkinputs "github.com/dianabuilds/ardents-network/tests/networkfixture"
)

func TestNativeNodePartialStartupRefusalsTCP(t *testing.T) {
	binary := processfixture.PinnedCommand(t)
	for _, carrier := range []string{epoch.CarrierClosedTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			spec, _, _ := networkinputs.PrepareNetworkGenesis(t, t.TempDir())
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = listener.Close() })
			spec.Nodes[1].Endpoint = listener.Addr().String()
			bundle, err := networkfixture.BuildClosed(spec)
			if err != nil {
				t.Fatal(err)
			}
			member, foreign := bundle.Nodes[1], bundle.Nodes[0]
			family := sha256.Sum256([]byte(member.Family))
			clock, closeClock := networkinputs.StartObservedClock(t)
			defer closeClock()
			for index, fault := range []string{"wrong-key", "wrong-role", "shared-root", "wrong-profile", "hosting-open", "spend-open", "listener-open", "identity-conflict", "family-conflict", "state-conflict", "expired-member"} {
				t.Run(fault, func(t *testing.T) {
					key := member.PrivateKey
					if fault == "wrong-key" {
						key = foreign.PrivateKey
					}
					certPath, keyPath := processfixture.WriteNativeCredential(t, processfixture.RoleCertificate(t, key))
					initial := bundle
					var expires time.Time
					if fault == "expired-member" {
						short := spec
						short.Nodes = append([]networkfixture.ClosedNode(nil), spec.Nodes...)
						expires = time.Now().Add(2 * time.Second).UTC().Truncate(time.Second)
						for index := range short.Nodes {
							if short.Nodes[index].NodeID == member.NodeID {
								short.Nodes[index].ValidUntil = expires
							}
						}
						initial, err = networkfixture.BuildClosed(short)
						if err != nil {
							t.Fatal(err)
						}
					}
					networkPlan := networkinputs.SeedNetworkState(t, spec.NetworkID, [32]byte(spec.Authority[32:]), initial, clock)
					if fault == "state-conflict" {
						changed := spec
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
					budget := processfixture.ProcessBudget(t)
					configuredBudget := budget
					id := member.NodeID
					if fault == "wrong-role" {
						id = foreign.NodeID
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
						identity, conflictFamily := member.NodeID, family
						if fault == "identity-conflict" {
							conflictFamily = [32]byte{90}
						} else {
							identity = [32]byte{90}
						}
						if err := journal.Replace([32]byte{99}, []duty.Duty{{Identity: identity, Family: conflictFamily, Class: "direct-source", State: "live", NotAfter: time.Now().Add(time.Hour)}}); err != nil {
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
					process := processfixture.LaunchFreshRole(t, "forwarding", binary, writeNodePlan(t, plan), byte(50+index))
					var event map[string]string
					decodeErr := json.NewDecoder(process.Stdout).Decode(&event)
					waitErr := process.Wait()
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
						collision, err := journal.Conflict(member.NodeID, family)
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
			if err := listener.Close(); err != nil {
				t.Fatal(err)
			}
			certPath, keyPath := processfixture.WriteNativeCredential(t, processfixture.RoleCertificate(t, member.PrivateKey))
			plan := map[string]any{"network": networkinputs.SeedNetworkState(t, spec.NetworkID, [32]byte(spec.Authority[32:]), bundle, clock), "node_id": member.NodeID, "process_profile": processresource.Profile, "participation_root": filepath.Join(t.TempDir(), "participation"), "spend_root": t.TempDir(), "hosting_root": processfixture.ProcessBudget(t), "certificate": certPath, "private_key": keyPath, "work": hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "retirement_wait_ms": 5000}
			// Genuine restored-input control: this same signed member, Carrier and
			// native envelope must start after removing the declared fault.
			control := processfixture.StartFreshRole(t, "forwarding", binary, writeNodePlan(t, plan), 80)
			control.Stop(t)
		})
	}
}
