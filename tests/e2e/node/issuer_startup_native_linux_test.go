//go:build linux && node_process_native

package node

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission/issuance"
	"github.com/dianabuilds/ardents-network/internal/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/processresource"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// Actual command refusals at distinct startup acquisitions. Prepared durable
// bytes remain unchanged and acquired roots/producer return only after join.
func TestNativeIssuerPartialStartupRefusalsTCP(t *testing.T) {
	binary := processfixture.PinnedCommand(t)
	for _, carrier := range []string{epoch.CarrierClosedTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			for _, fault := range []string{"profile", "role", "tls-key", "missing-results"} {
				t.Run(fault, func(t *testing.T) {
					spec, issuerPlan, inventory := processfixture.PrepareNetworkGenesis(t, t.TempDir())
					listener, err := net.Listen("tcp", "127.0.0.1:0")
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { _ = listener.Close() })
					spec.Nodes[0].Endpoint = listener.Addr().String()
					bundle, err := networkfixture.BuildClosed(spec)
					if err != nil {
						t.Fatal(err)
					}
					member, foreign := bundle.Nodes[0], bundle.Nodes[1]
					id := member.NodeID
					prepareNativeIssuer(t, &issuerPlan, inventory, spec, member, sha256.Sum256(bundle.Profile))
					clock, closeClock := processfixture.StartObservedClock(t)
					defer closeClock()
					network := processfixture.SeedNetworkState(t, spec.NetworkID, [32]byte(spec.Authority[32:]), bundle, clock)
					key := member.PrivateKey
					if fault == "tls-key" {
						key = foreign.PrivateKey
					}
					certPath, keyPath := processfixture.WriteNativeCredential(t, nativeRoleCertificate(t, key))
					participation, spend, budget := filepath.Join(t.TempDir(), "participation"), t.TempDir(), processfixture.ProcessBudget(t)
					input := IssuerDeclaration(issuerPlan)
					plan := map[string]any{"network": network, "node_id": id, "process_profile": processresource.IssuerProfile, "participation_root": participation, "spend_root": spend, "hosting_root": budget, "certificate": certPath, "private_key": keyPath, "issuance": input,
						"work": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "bootstrap_work": hosting.Traffic{Tx: 128 << 10, Rx: 128 << 10}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "retirement_wait_ms": 5000}
					expected := 1
					switch fault {
					case "profile":
						plan["process_profile"] = processresource.Profile
						expected = 2
					case "role":
						plan["node_id"] = foreign.NodeID
					case "missing-results":
						input["result_root"] = filepath.Join(t.TempDir(), "missing")
					}
					before, err := os.ReadFile(filepath.Join(issuerPlan.AdmissionRoot, "admission.journal"))
					if err != nil {
						t.Fatal(err)
					}
					if err := listener.Close(); err != nil {
						t.Fatal(err)
					}
					path := writeNodePlan(t, plan)
					p := processfixture.LaunchFreshRole(t, "issuer", binary, path, 0)
					var event map[string]string
					decodeErr := json.NewDecoder(p.Stdout).Decode(&event)
					err = p.Wait()
					var exit *exec.ExitError
					if !errors.As(err, &exit) || exit.ExitCode() != expected {
						t.Fatal("actual partial startup did not refuse", fault, err, p.Diagnostic.String())
					}
					if expected == 1 && (decodeErr != nil || len(event) != 4 || event["operation"] != "node.issuer" || event["phase"] != "refused") {
						t.Fatal("startup refusal lost finite projection", event, decodeErr)
					}
					if expected == 1 {
						processfixture.RequireStartupFailure(t, event["first_failure"])
					}
					if expected == 2 && (decodeErr == nil || len(event) != 0) {
						t.Fatal("wrong profile reached accepting composition", event)
					}
					after, err := os.ReadFile(filepath.Join(issuerPlan.AdmissionRoot, "admission.journal"))
					if err != nil || !bytes.Equal(before, after) {
						t.Fatal("rejected startup changed quota", err)
					}
					if _, err := os.Stat(filepath.Join(spend, "closed-token-spends")); !os.IsNotExist(err) {
						t.Fatal("early rejected acquisition reached Receiving", err)
					}
					RequireIssuerRootsReleased(t, issuerPlan)
					if _, err := os.Stat(participation); err == nil {
						journal, err := duty.OpenOperation(t.Context(), duty.Config{Root: participation, Clock: time.Now})
						if err != nil {
							t.Fatal(err)
						}
						conflict, err := journal.Conflict(id, sha256.Sum256([]byte(member.Family)))
						closeErr := journal.Close()
						if err != nil || closeErr != nil || conflict {
							t.Fatal("partial acquisition retained original producer", err, closeErr, conflict)
						}
					} else if !errors.Is(err, os.ErrNotExist) {
						t.Fatal("inspect participation after refusal", err)
					}
				})
			}
		})
	}
}

func prepareNativeIssuer(t testing.TB, plan *issuer.Plan, inventory issuance.Inventory, spec networkfixture.ClosedSpec, member networkfixture.ClosedNode, profile [32]byte) {
	t.Helper()
	plan.AdmissionBinding = quota.LedgerBinding{
		Network: spec.NetworkID, Issuer: member.NodeID, Authority: spec.IssuanceAuthority,
		Profile: profile, Duty: member.Generation, Start: spec.NotBefore, End: spec.NotAfter, Keys: inventory.Keys,
	}
	if err := quota.Initialize(plan.AdmissionRoot, plan.AdmissionBinding); err != nil {
		t.Fatal(err)
	}
	if result := issuer.Initialize(t.Context(), *plan); result.Outcome != "initialized-results" {
		t.Fatal(result)
	}
}
