//go:build linux

package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
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
	testRouteCompiledCommand(t, false)
}

func TestRouteCompiledHolderBootstrapAndOrdinaryIssuerBothCarriers(t *testing.T) {
	_ = compiledCommand(t)
	for _, profile := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		t.Run(string(profile), func(t *testing.T) {
			f, sockets, certificates := newRoleRouteFixture(t, profile, 1, false, true)
			bundle, err := networkfixture.BuildClosed(f.spec)
			if err != nil {
				t.Fatal(err)
			}
			clock, closeClock := joinProcessClock(t)
			defer closeClock()
			seed := func() networkAuthorityPlan {
				var key [32]byte
				copy(key[:], f.spec.Authority.Public().(ed25519.PublicKey))
				plan := networkAuthorityPlan{Root: filepath.Join(t.TempDir(), "network"), NetworkID: f.spec.NetworkID, Authorities: [][32]byte{key}, Threshold: 1, ProfileAuthority: key, ClockObservationFile: clock}
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
			var receivers []*joinCommandProcess
			var budgets []string
			for _, id := range [][32]byte{{12}, {13}, {14}, {15}, f.profile.IssuerNodeID} {
				certificate := certificates[id]
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
				budget := routeProcessBudget(t)
				budgets = append(budgets, budget)
				root := t.TempDir()
				if err := os.Chmod(root, 0700); err != nil {
					t.Fatal(err)
				}
				config := map[string]any{"network": seed(), "node_id": id, "spend_root": root, "hosting_root": budget, "certificate": certPath, "private_key": keyPath, "work": hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
				if id == f.profile.IssuerNodeID {
					config["issuer"] = f.plan
				}
				sockets[id]()
				process := startJoinCommandProcess(t, t.Context(), compiledCommand(t), "route", "receive", "--config", hostingConfig(t, config))
				t.Cleanup(func() {
					if !process.waited {
						_ = process.command.Process.Kill()
						_ = process.wait()
					}
				})
				var ready map[string]string
				if err := process.decode.Decode(&ready); err != nil || ready["operation"] != "route.receive" || ready["phase"] != "listening" {
					t.Fatal("compiled issuer receiver not ready", id, err, process.diagnostic.String())
				}
				receivers = append(receivers, process)
			}
			holderRoot := t.TempDir()
			if err := os.Chmod(holderRoot, 0700); err != nil {
				t.Fatal(err)
			}
			plan := routePrefixPlan{EntryRoot: filepath.Join(t.TempDir(), "entry"), InteriorRoot: filepath.Join(t.TempDir(), "interior"), HostingRoot: routeProcessBudget(t), Domain: 1, Deadline: time.Now().Add(30 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
			budgets = append(budgets, plan.HostingRoot)
			send, closeHolder := admissionLocalConsole(t, "holder", map[string]any{"root": holderRoot, "network": seed(), "role": admission.AllocationUser, "route": plan})
			routeConsolePermissionStock(t, f, send, [3]uint32{2, 4, 0})
			for _, command := range []holderCommand{{Operation: "bootstrap-open"}, {Operation: "issuer-issue", Class: 2}, {Operation: "issuer-issue", Class: 1}} {
				if reply := send(command); reply.Outcome != "completed" {
					t.Fatal("compiled bootstrap sequence", command.Operation, command.Class, reply.Outcome)
				}
			}
			if reply := send(holderCommand{Operation: "issuer-issue", Class: 1}); reply.Outcome == "completed" {
				t.Fatal("compiled third bootstrap batch accepted")
			}
			for _, command := range []holderCommand{{Operation: "bootstrap-close"}, {Operation: "prefix-open"}, {Operation: "issuer-issue", Class: 2}, {Operation: "prefix-close"}} {
				if reply := send(command); reply.Outcome != "completed" {
					t.Fatal("compiled fresh admitted issuer sequence", command.Operation, command.Class, reply.Outcome)
				}
			}
			closeHolder()
			for _, process := range receivers {
				process.interrupt(t)
			}
			for _, process := range receivers {
				err := process.wait()
				var exit *exec.ExitError
				if !errors.As(err, &exit) || exit.ExitCode() != 130 {
					t.Fatal("compiled receiver did not join", err, process.diagnostic.String())
				}
			}
			for _, root := range budgets {
				budget, err := hosting.Open(root)
				if err != nil {
					t.Fatal(err)
				}
				observed, err := budget.Observe(t.Context())
				closeErr := budget.Close()
				if err != nil || closeErr != nil || observed.ReservedBytes != 0 {
					t.Fatal("compiled issuer retained Hosting after join", err, closeErr, observed.ReservedBytes)
				}
			}
		})
	}
}

func TestRouteCompiledCommandRegistrationBothCarriers(t *testing.T) {
	testRouteCompiledCommand(t, true)
}

func TestRouteCompiledCommandReplenishmentBothCarriers(t *testing.T) {
	testRouteCompiledCommand(t, true, true)
}

func testRouteCompiledCommand(t *testing.T, introduction bool, refill ...bool) {
	t.Helper()
	_ = compiledCommand(t)
	for _, profile := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		t.Run(string(profile), func(t *testing.T) {
			domain := byte(3)
			role := admission.AllocationUser
			last := byte(16)
			if introduction {
				domain, role, last = 4, admission.AllocationPublisher, 17
			}
			f, reservations, certificates := newRoleRouteFixture(t, profile, domain, introduction)
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
			for i := byte(12); i < last; i++ {
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
				if len(refill) != 0 && refill[0] && i != 16 {
					plan["work"] = hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20}
				}
				if i == 16 {
					root := t.TempDir()
					if err := os.Chmod(root, 0700); err != nil {
						t.Fatal(err)
					}
					plan["introduction_root"] = root
				}
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
						if introduction && exit != nil && exit.ExitCode() == 1 {
							var terminal map[string]string
							for _, line := range bytes.Split(diagnostic.Bytes(), []byte{'\n'}) {
								var event map[string]string
								if json.Unmarshal(line, &event) == nil && event["operation"] == "route.receive" && event["phase"] == "joined" {
									terminal = event
								}
							}
							if terminal["outcome"] == "failed" && terminal["stage"] == "peer-retired-write" {
								t.Log("receiver retained joined peer-retirement write failure", id)
								return
							}
						}
						t.Error("receiver failed joined cancellation", err, diagnostic.String())
					}
				})
			}
			holderRoot := t.TempDir()
			plan := routePrefixPlan{EntryRoot: filepath.Join(t.TempDir(), "entry"), InteriorRoot: filepath.Join(t.TempDir(), "interior"), HostingRoot: routeProcessBudget(t), Domain: domain, Deadline: time.Now().Add(30 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
			if len(refill) != 0 && refill[0] {
				plan.Work = hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20}
			}
			holderNetwork := seed()
			send, closeHolder := admissionLocalConsole(t, "holder", map[string]any{"root": holderRoot, "network": holderNetwork, "role": role, "route": plan})
			routeConsoleRoleStock(t, f, send, introduction, refill...)
			if reply := send(holderCommand{Operation: "prefix-open"}); reply.Outcome != "completed" {
				t.Fatal("compiled prefix not admitted", reply.Outcome)
			}
			if introduction {
				if len(refill) != 0 && refill[0] {
					if reply := send(holderCommand{Operation: "prefix-replenish"}); reply.Outcome != "completed" {
						t.Fatal("compiled genuine refill refused", reply)
					}
				}
				if reply := send(holderCommand{Operation: "registration-open", Revision: 1}); reply.Outcome != "completed" || reply.Slot == [32]byte{} {
					t.Fatal("compiled REGISTER", reply)
				}
				if reply := send(holderCommand{Operation: "registration-withdraw"}); reply.Outcome != "completed" {
					t.Fatal("compiled owning WITHDRAW", reply)
				}
				if reply := send(holderCommand{Operation: "registration-open", Revision: 2}); reply.Outcome == "completed" {
					t.Fatal("compiled Stock reused spent Registration token")
				}
			}
			if reply := send(holderCommand{Operation: "prefix-close"}); reply.Outcome != "completed" {
				t.Fatal("compiled prefix did not join", reply.Outcome, reply.Stage)
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
