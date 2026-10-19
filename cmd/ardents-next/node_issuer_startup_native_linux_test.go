//go:build linux && node_process_native

package main

import (
	"bytes"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/network/duty"
	"github.com/dianabuilds/ardents-network/internal/successor/processresource"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// Actual command refusals at distinct startup acquisitions. Prepared durable
// bytes remain unchanged and acquired roots/producer return only after join.
func TestNativeIssuerPartialStartupRefusalsTCP(t *testing.T) {
	binary := nodeNativeCommand(t)
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			for _, fault := range []string{"profile", "role", "tls-key", "missing-results"} {
				t.Run(fault, func(t *testing.T) {
					f, sockets, certificates := newRoleRouteFixture(t, carrier, 1, false, true)
					bundle, err := networkfixture.BuildClosed(f.spec)
					if err != nil {
						t.Fatal(err)
					}
					clock, closeClock := joinProcessClock(t)
					defer closeClock()
					network := seedNativeNodeState(t, f, bundle, clock)
					id := f.profile.IssuerNodeID
					certificate := certificates[id]
					if fault == "tls-key" {
						certificate = certificates[[32]byte{12}]
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
					participation, spend, budget := filepath.Join(t.TempDir(), "participation"), t.TempDir(), routeProcessBudget(t)
					input := nativeIssuerIssuanceInput(f.plan)
					plan := map[string]any{"network": network, "node_id": id, "process_profile": processresource.IssuerProfile, "participation_root": participation, "spend_root": spend, "hosting_root": budget, "certificate": certPath, "private_key": keyPath, "issuance": input,
						"work": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "bootstrap_work": hosting.Traffic{Tx: 128 << 10, Rx: 128 << 10}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "retirement_wait_ms": 5000}
					expected := 1
					switch fault {
					case "profile":
						plan["process_profile"] = processresource.Profile
						expected = 2
					case "role":
						plan["node_id"] = [32]byte{12}
					case "missing-results":
						input["result_root"] = filepath.Join(t.TempDir(), "missing")
					}
					before, err := os.ReadFile(filepath.Join(f.plan.AdmissionRoot, "admission.journal"))
					if err != nil {
						t.Fatal(err)
					}
					sockets[id]()
					p := launchNativeIssuer(t, binary, hostingConfig(t, plan))
					var event map[string]string
					decodeErr := json.NewDecoder(p.stdout).Decode(&event)
					err = p.command.Wait()
					p.joined = true
					_ = p.stdout.Close()
					var exit *exec.ExitError
					if !errors.As(err, &exit) || exit.ExitCode() != expected {
						t.Fatal("actual partial startup did not refuse", fault, err, p.diagnostic.String())
					}
					if expected == 1 && (decodeErr != nil || len(event) != 3 || event["operation"] != "node.issuer" || event["phase"] != "refused") {
						t.Fatal("startup refusal lost finite projection", event, decodeErr)
					}
					if expected == 2 && (decodeErr == nil || len(event) != 0) {
						t.Fatal("wrong profile reached accepting composition", event)
					}
					after, err := os.ReadFile(filepath.Join(f.plan.AdmissionRoot, "admission.journal"))
					if err != nil || !bytes.Equal(before, after) {
						t.Fatal("rejected startup changed quota", err)
					}
					if _, err := os.Stat(filepath.Join(spend, "closed-token-spends")); !os.IsNotExist(err) {
						t.Fatal("early rejected acquisition reached Receiving", err)
					}
					checkNativeIssuerRootsReleased(t, f.plan)
					if _, err := os.Stat(participation); err == nil {
						view, err := f.current()
						if err != nil {
							t.Fatal(err)
						}
						m, err := view.Member(id, view.ObservedAt())
						if err != nil {
							t.Fatal(err)
						}
						journal, err := duty.OpenOperation(t.Context(), duty.Config{Root: participation, Clock: time.Now})
						if err != nil {
							t.Fatal(err)
						}
						conflict, err := journal.Conflict(id, m.FamilyID)
						closeErr := journal.Close()
						if err != nil || closeErr != nil || conflict {
							t.Fatal("partial acquisition retained original producer", err, closeErr, conflict)
						}
					}
				})
			}
		})
	}
}
