//go:build linux

package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
	"github.com/dianabuilds/ardents-network/internal/successor/route/carrier"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

func routeProcessBudget(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "budget")
	start := time.Now().UTC().Truncate(time.Hour)
	if err := hosting.Initialize(root, hosting.Policy{Provider: "Route process test", Start: start, End: start.Add(2 * time.Hour), Unit: "MiB", Quantity: 100, Directions: "tx+rx", Interfaces: []string{"lo"}, LowWatermarkBytes: 1000}); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRouteCompiledCommandPrefixBothCarriers(t *testing.T) {
	_ = compiledCommand(t)
	for _, profile := range []carrier.CarrierProfile{carrier.ClosedCarrierTCP, carrier.ClosedCarrierQUIC} {
		t.Run(string(profile), func(t *testing.T) {
			f, reservations, certificates := newRouteFixture(t, profile)
			bundle, err := networkfixture.BuildClosed(f.spec)
			if err != nil {
				t.Fatal(err)
			}
			clockFile := filepath.Join(t.TempDir(), "clock-observation")
			if err := os.WriteFile(clockFile, nil, 0600); err != nil {
				t.Fatal(err)
			}
			stopClock, clockJoined := make(chan struct{}), make(chan error, 1)
			go func() {
				ticker := time.NewTicker(100 * time.Millisecond)
				defer ticker.Stop()
				for {
					select {
					case <-stopClock:
						clockJoined <- nil
						return
					case now := <-ticker.C:
						if err := os.Chtimes(clockFile, now, now); err != nil {
							clockJoined <- err
							return
						}
					}
				}
			}()
			defer func() {
				close(stopClock)
				if err := <-clockJoined; err != nil {
					t.Error(err)
				}
			}()
			seed := func() networkAuthorityPlan {
				t.Helper()
				var key [32]byte
				copy(key[:], f.spec.Authority.Public().(ed25519.PublicKey))
				plan := networkAuthorityPlan{Root: filepath.Join(t.TempDir(), "network"), NetworkID: f.spec.NetworkID, Authorities: [][32]byte{key}, Threshold: 1, ProfileAuthority: key, ClockObservationFile: clockFile}
				config, err := networkStateConfig(&plan)
				if err != nil {
					t.Fatal(err)
				}
				owner, err := state.Open(config)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := owner.Accept(t.Context(), bundle.Epoch.Raw, bundle.Epoch.Inputs, bundle.Epoch.Materials); err != nil {
					t.Fatal(err)
				}
				if _, err := owner.AcceptClosedProfile(bundle.Profile); err != nil {
					t.Fatal(err)
				}
				if err := owner.Close(); err != nil {
					t.Fatal(err)
				}
				return plan
			}
			var stopReceivers []func()
			defer func() {
				for _, stop := range stopReceivers {
					stop()
				}
			}()
			var budgets []string
			for i := byte(12); i < 16; i++ {
				id := [32]byte{i}
				certificate := certificates[id]
				certPath, keyPath := filepath.Join(t.TempDir(), "cert"), filepath.Join(t.TempDir(), "key")
				if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Certificate[0]}), 0600); err != nil {
					t.Fatal(err)
				}
				key, err := x509.MarshalPKCS8PrivateKey(certificate.PrivateKey)
				if err != nil {
					t.Fatal(err)
				}
				keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key})
				if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
					t.Fatal(err)
				}
				clear(key)
				clear(keyPEM)
				budget := routeProcessBudget(t)
				budgets = append(budgets, budget)
				plan := map[string]any{"network": seed(), "node_id": id, "spend_root": t.TempDir(), "hosting_root": budget, "certificate": certPath, "private_key": keyPath, "work": hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
				reservations[id]()
				cmd := exec.CommandContext(t.Context(), compiledCommand(t), "route", "receive", "--config", hostingConfig(t, plan))
				out, err := cmd.StdoutPipe()
				if err != nil {
					t.Fatal(err)
				}
				var diagnostic bytes.Buffer
				cmd.Stderr = &diagnostic
				if err := cmd.Start(); err != nil {
					t.Fatal(err)
				}
				closed := false
				t.Cleanup(func() {
					if !closed {
						_ = cmd.Process.Kill()
						_ = cmd.Wait()
					}
				})
				var ready map[string]string
				if err := json.NewDecoder(out).Decode(&ready); err != nil {
					t.Fatal("receiver did not become ready", err)
				}
				if ready["operation"] != "route.receive" || ready["phase"] != "listening" {
					t.Fatal(ready)
				}
				stopReceivers = append(stopReceivers, func() {
					if closed {
						return
					}
					if err := cmd.Process.Signal(os.Interrupt); err != nil {
						t.Error(err)
					}
					err := cmd.Wait()
					closed = true
					var exit *exec.ExitError
					if !errors.As(err, &exit) || exit.ExitCode() != 130 {
						t.Error("receiver failed joined cancellation", err, diagnostic.String())
					}
				})
			}
			holderRoot := t.TempDir()
			plan := routePrefixPlan{EntryRoot: filepath.Join(t.TempDir(), "entry"), InteriorRoot: filepath.Join(t.TempDir(), "interior"), HostingRoot: routeProcessBudget(t), Domain: 3, Deadline: time.Now().Add(30 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
			holderNetwork := seed()
			send, closeHolder := admissionLocalConsole(t, "holder", map[string]any{"root": holderRoot, "network": holderNetwork, "role": admission.AllocationUser, "route": plan})
			routeConsoleStock(t, f, send)
			if reply := send(holderCommand{Operation: "prefix-open"}); reply.Outcome != "completed" {
				t.Fatal("compiled prefix not admitted", reply.Outcome)
			}
			if reply := send(holderCommand{Operation: "prefix-close"}); reply.Outcome != "completed" {
				t.Fatal("compiled prefix did not join", reply)
			}
			if reply := send(holderCommand{Operation: "prefix-open"}); reply.Outcome == "completed" {
				t.Fatal("compiled Stock reopened burnt token")
			}
			closeHolder()
			for _, stop := range stopReceivers {
				stop()
			}
			for _, root := range append(budgets, plan.HostingRoot) {
				budget, err := hosting.Open(root)
				if err != nil {
					t.Fatal(err)
				}
				observation, err := budget.Observe(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				if observation.ReservedBytes != 0 {
					t.Fatal("joined process leaked physical reserve", observation.ReservedBytes)
				}
				if err := budget.Close(); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
