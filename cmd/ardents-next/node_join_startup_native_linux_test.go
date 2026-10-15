//go:build linux && node_process_native

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/network/duty"
	"github.com/dianabuilds/ardents-network/internal/successor/processresource"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestNativeJoinPartialStartupRefusalsBothCarriers(t *testing.T) {
	binary := nodeNativeCommand(t)
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			for _, fault := range []string{"profile", "unknown-field", "nested-roots", "role", "tls-key", "missing-hosting", "foreign-spend", "listener"} {
				t.Run(fault, func(t *testing.T) {
					f, sockets, certificates := newJoinRouteFixture(t, carrier)
					bundle, err := networkfixture.BuildClosed(f.spec)
					if err != nil {
						t.Fatal(err)
					}
					clock, closeClock := joinProcessClock(t)
					defer closeClock()
					id := [32]byte{20}
					cert, key := nativeDescriptorCredential(t, certificates[id])
					participation, spend, budget := filepath.Join(t.TempDir(), "participation"), t.TempDir(), routeProcessBudget(t)
					plan := map[string]any{"network": seedNativeNodeState(t, f, bundle, clock), "node_id": id, "process_profile": processresource.JoinProfile,
						"participation_root": participation, "spend_root": spend, "hosting_root": budget,
						"certificate": cert, "private_key": key, "work": hosting.Traffic{Tx: 32 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "retirement_wait_ms": 5000}
					expected := 1
					var foreign string
					switch fault {
					case "profile":
						plan["process_profile"], expected = processresource.Profile, 2
					case "unknown-field":
						plan["accepting"] = true
						expected = 2
					case "nested-roots":
						plan["spend_root"], expected = filepath.Join(participation, "nested"), 2
					case "role":
						plan["node_id"] = [32]byte{12}
					case "tls-key":
						plan["certificate"], plan["private_key"] = nativeDescriptorCredential(t, certificates[[32]byte{12}])
					case "missing-hosting":
						plan["hosting_root"] = filepath.Join(t.TempDir(), "missing")
					case "foreign-spend":
						foreign = filepath.Join(spend, "foreign-input")
					}
					foreignBytes := []byte("independent rejected input\n")
					if foreign != "" {
						if err := os.WriteFile(foreign, foreignBytes, 0600); err != nil {
							t.Fatal(err)
						}
					}
					if fault != "listener" {
						sockets[id]()
					}
					p := launchNativeJoin(t, binary, hostingConfig(t, plan), id[0])
					var event map[string]string
					decodeErr := json.NewDecoder(p.stdout).Decode(&event)
					err = p.command.Wait()
					p.joined = true
					_ = p.stdout.Close()
					var exit *exec.ExitError
					if !errors.As(err, &exit) || exit.ExitCode() != expected {
						t.Fatal("actual Join startup did not refuse", err, p.diagnostic.String())
					}
					if expected == 1 && (decodeErr != nil || len(event) != 3 || event["operation"] != "node.join" || event["phase"] != "refused" || event["outcome"] != "startup-unavailable") {
						t.Fatal("partial startup lost bounded refusal", event, decodeErr)
					}
					if expected == 2 && (decodeErr == nil || len(event) != 0) {
						t.Fatal("invalid declaration reached runtime", event)
					}
					spendBytes, spendErr := os.ReadFile(filepath.Join(spend, "closed-token-spends"))
					if fault == "listener" {
						if spendErr != nil || len(spendBytes) != 112 || string(spendBytes[:8]) != "ARDSPN01" {
							t.Fatal("failed listener changed spend history", spendErr)
						}
					} else if !os.IsNotExist(spendErr) {
						t.Fatal("refused startup created receiving history", len(spendBytes), spendErr)
					}

					if foreign != "" {
						raw, err := os.ReadFile(foreign)
						if err != nil || !bytes.Equal(raw, foreignBytes) {
							t.Fatal("rejected private input was rewritten", err)
						}
						// Remove only this test's unaccepted input, never History records.
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
					plan["process_profile"], plan["node_id"] = processresource.JoinProfile, id
					plan["certificate"], plan["private_key"] = cert, key
					plan["hosting_root"], plan["spend_root"] = budget, spend
					sockets[id]()
					fresh := startNativeJoin(t, binary, hostingConfig(t, plan), id[0])
					fresh.stop(t)
				})
			}
		})
	}
}
