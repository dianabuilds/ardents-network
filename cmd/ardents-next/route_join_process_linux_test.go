//go:build linux

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/allocation"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/token"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
	"github.com/dianabuilds/ardents-network/internal/successor/route/carrier"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// This scenario proves compiled holder/receiver composition and joined paired
// transport. It supplies no capsule ACK, Service authority or plaintext console
// workload. Actual encrypted traffic has its independent in-process oracle.
func TestRouteCompiledCommandJoinBothCarriers(t *testing.T) {
	binary := compiledCommand(t)
	for _, profile := range []carrier.CarrierProfile{carrier.ClosedCarrierTCP, carrier.ClosedCarrierQUIC} {
		t.Run(string(profile), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
			defer cancel()
			f, reservations, certificates := newJoinRouteFixture(t, profile)
			bundle, err := networkfixture.BuildClosed(f.spec)
			if err != nil {
				t.Fatal(err)
			}
			clockFile, stopClock := joinProcessClock(t)
			defer stopClock()
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
				_, err = owner.Accept(ctx, bundle.Epoch.Raw, bundle.Epoch.Inputs, bundle.Epoch.Materials)
				if err == nil {
					_, err = owner.AcceptClosedProfile(bundle.Profile)
				}
				if err := errors.Join(err, owner.Close()); err != nil {
					t.Fatal("independent signed State initialization", err)
				}
				return plan
			}
			var processes []*joinCommandProcess
			defer func() {
				// Interrupt every original process before waiting for any. This
				// also joins failed setup; timeout/kill can never pass the test.
				for _, process := range processes {
					process.interrupt(t)
				}
				for _, process := range processes {
					if !process.waited {
						_ = process.wait()
					}
				}
			}()
			var receivers []*joinCommandProcess
			var budgets []string
			for idByte := byte(12); idByte <= 21; idByte++ {
				id := [32]byte{idByte}
				certificate := certificates[id]
				directory := t.TempDir()
				certPath, keyPath := filepath.Join(directory, "certificate"), filepath.Join(directory, "private-key")
				if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Certificate[0]}), 0600); err != nil {
					t.Fatal(err)
				}
				key, err := x509.MarshalPKCS8PrivateKey(certificate.PrivateKey)
				if err != nil {
					t.Fatal(err)
				}
				keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key})
				err = os.WriteFile(keyPath, keyPEM, 0600)
				clear(key)
				clear(keyPEM)
				if err != nil {
					t.Fatal(err)
				}
				budget := routeProcessBudget(t)
				budgets = append(budgets, budget)
				plan := map[string]any{"network": seed(), "node_id": id, "spend_root": t.TempDir(), "hosting_root": budget, "certificate": certPath, "private_key": keyPath, "work": hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
				reservations[id]()
				process := startJoinCommandProcess(t, ctx, binary, "route", "receive", "--config", hostingConfig(t, plan))
				processes = append(processes, process)
				receivers = append(receivers, process)
				var ready map[string]string
				if err := process.decode.Decode(&ready); err != nil || ready["operation"] != "route.receive" || ready["phase"] != "listening" {
					t.Fatal("compiled JOIN receiver readiness", err)
				}
			}
			var holders [2]*joinCommandProcess
			var plans [2]routePrefixPlan
			for side, role := range []admission.AllocationRole{admission.AllocationUser, admission.AllocationPublisher} {
				root := t.TempDir()
				domain := uint8(1)
				if side == 1 {
					domain = 3
				}
				plan := routePrefixPlan{EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "interior"), HostingRoot: routeProcessBudget(t), Domain: domain, Deadline: time.Now().Add(time.Minute).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
				if side == 1 {
					plan.SourceInteriorRoot = filepath.Join(root, "source-interior")
				}
				plans[side] = plan
				budgets = append(budgets, plan.HostingRoot)
				config := map[string]any{"root": t.TempDir(), "network": seed(), "role": role, "route": plan}
				process := startJoinCommandProcess(t, ctx, binary, "admission", "holder", "--config", hostingConfig(t, config))
				processes = append(processes, process)
				holders[side] = process
				joinCommandStock(t, f, process, role)
				if reply, err := process.exchange(holderCommand{Operation: "join-prefix-open"}); err != nil || reply.Outcome != "completed" {
					t.Fatal("compiled Source/Responder prefix", side, reply.Outcome, err)
				}
			}
			chosen, err := holders[0].exchange(holderCommand{Operation: "rendezvous", Choice: 0})
			if err != nil || chosen.Outcome != "completed" || (chosen.Recipient.Node != [32]byte{20} && chosen.Recipient.Node != [32]byte{21}) || chosen.Recipient.Generation != 9 || chosen.Recipient.NotAfter.IsZero() {
				t.Fatal("compiled current Rendezvous selection", chosen.Outcome, err)
			}
			intent := routeJoinIntent{Node: chosen.Recipient.Node, Generation: chosen.Recipient.Generation, Deadline: minRouteDeadline(chosen.Recipient.NotAfter, plans[0].Deadline, plans[1].Deadline), SetupDeadline: time.Now().Add(8 * time.Second).UTC().Truncate(time.Second)}
			if _, err := rand.Read(intent.Secret[:]); err != nil {
				t.Fatal(err)
			}
			if _, err := rand.Read(intent.Context[:]); err != nil {
				t.Fatal(err)
			}
			defer func() { clear(intent.Secret[:]); clear(intent.Context[:]) }()
			type joinResult struct {
				side    int
				outcome string
				err     error
			}
			opened := make(chan joinResult, 2)
			for side, process := range holders {
				go func() {
					reply, err := process.exchange(holderCommand{Operation: "join-open", Join: intent})
					opened <- joinResult{side: side, outcome: reply.Outcome, err: err}
				}()
			}
			var failed bool
			for range holders {
				result := <-opened
				if result.err != nil || result.outcome != "completed" {
					t.Error("compiled opposite-side JOIN", result.side, result.outcome, result.err)
					failed = true
				}
			}
			if failed {
				t.FailNow()
			}
			for side, process := range holders {
				if reply, err := process.exchange(holderCommand{Operation: "join-close"}); err != nil || reply.Outcome != "completed" {
					t.Fatal("compiled JOIN did not join", side, reply.Outcome, err)
				}
			}
			for side, process := range holders {
				if reply, err := process.exchange(holderCommand{Operation: "prefix-close"}); err != nil || reply.Outcome != "completed" {
					t.Fatal("compiled original prefixes did not join", side, reply.Outcome, err)
				}
				if err := process.input.Close(); err != nil {
					t.Fatal(err)
				}
				if err := process.wait(); err != nil {
					t.Fatal("compiled holder terminal outcome", side, err)
				}
			}
			for _, process := range receivers {
				process.interrupt(t)
			}
			for _, process := range receivers {
				err := process.wait()
				var exit *exec.ExitError
				if !errors.As(err, &exit) || exit.ExitCode() != 130 {
					t.Error("compiled receiver failed joined cancellation", err, process.diagnostic.String())
				}
			}
			for _, root := range budgets {
				budget, err := hosting.Open(root)
				if err != nil {
					t.Fatal("joined process retained Hosting root", err)
				}
				observation, err := budget.Observe(ctx)
				closeErr := budget.Close()
				if err != nil || closeErr != nil || observation.ReservedBytes != 0 {
					t.Fatal("compiled JOIN retained physical reservation after exit", observation.ReservedBytes, err, closeErr)
				}
			}
		})
	}
}

type joinCommandReply struct {
	localAdmissionReply
	Recipient routeRecipient `json:"recipient"`
}

type joinCommandProcess struct {
	command    *exec.Cmd
	input      io.WriteCloser
	encode     *json.Encoder
	decode     *json.Decoder
	diagnostic bytes.Buffer
	once       sync.Once
	waited     bool
	result     error
}

func startJoinCommandProcess(t *testing.T, ctx context.Context, binary string, args ...string) *joinCommandProcess {
	t.Helper()
	p := &joinCommandProcess{command: exec.CommandContext(ctx, binary, args...)}
	input, err := p.command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := p.command.StdoutPipe()
	if err != nil {
		_ = input.Close()
		t.Fatal(err)
	}
	p.input, p.encode, p.decode = input, json.NewEncoder(input), json.NewDecoder(output)
	p.command.Stderr = &p.diagnostic
	if err := p.command.Start(); err != nil {
		_ = input.Close()
		_ = output.Close()
		t.Fatal(err)
	}
	return p
}

func (p *joinCommandProcess) exchange(command holderCommand) (joinCommandReply, error) {
	if err := p.encode.Encode(command); err != nil {
		return joinCommandReply{}, err
	}
	var reply joinCommandReply
	err := p.decode.Decode(&reply)
	return reply, err
}

func (p *joinCommandProcess) interrupt(t *testing.T) {
	t.Helper()
	if !p.waited {
		if err := p.command.Process.Signal(os.Interrupt); err != nil && !errors.Is(err, os.ErrProcessDone) {
			t.Error("compiled process interrupt", err)
		}
	}
}

func (p *joinCommandProcess) wait() error {
	p.once.Do(func() {
		p.result = p.command.Wait()
		p.waited = true
		_ = p.input.Close()
	})
	return p.result
}

func joinProcessClock(t *testing.T) (string, func()) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "clock-observation")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	stop, joined := make(chan struct{}), make(chan error, 1)
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				joined <- nil
				return
			case now := <-ticker.C:
				if err := os.Chtimes(path, now, now); err != nil {
					joined <- err
					return
				}
			}
		}
	}()
	return path, func() {
		close(stop)
		if err := <-joined; err != nil {
			t.Error("clock observation updater", err)
		}
	}
}

func joinCommandStock(t *testing.T, f *networkAdmissionFixture, process *joinCommandProcess, role admission.AllocationRole) {
	t.Helper()
	send := func(command holderCommand) joinCommandReply {
		t.Helper()
		reply, err := process.exchange(command)
		if err != nil || reply.Outcome != "completed" {
			t.Fatal("compiled JOIN stock", command.Operation, reply.Outcome, err)
		}
		return reply
	}
	request := send(holderCommand{Operation: "request", Maxima: [3]uint32{0, 10, 0}})
	prepared, err := allocation.Prepare(request.Request, f.profile.NetworkID, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	decision, err := prepared.Decide(nil, f.profile.IssuanceAuthorityKey, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	journal := filepath.Join(t.TempDir(), "allocation")
	if err := os.WriteFile(journal, decision.Journal(), 0600); err != nil {
		t.Fatal(err)
	}
	committed, err := os.ReadFile(journal)
	if err != nil || !bytes.Equal(committed, decision.Journal()) {
		t.Fatal("compiled JOIN allocation readback", err)
	}
	permission := decision.Permission()
	copy(permission.Signature[:], ed25519.Sign(f.spec.Authority, admission.PermissionTranscript(permission)))
	raw, err := admission.EncodePermission(permission)
	if err != nil {
		t.Fatal(err)
	}
	send(holderCommand{Operation: "import", Digest: request.Digest, Payload: raw})
	view, err := f.current()
	if err != nil {
		t.Fatal(err)
	}
	var challenges []token.ClosedTokenContext
	for idByte := byte(12); idByte <= 21; idByte++ {
		member, err := view.Member([32]byte{idByte}, view.ObservedAt())
		if err != nil {
			t.Fatal(err)
		}
		challenges = append(challenges, token.ClosedTokenContext{NetworkID: f.profile.NetworkID, ProfileDigest: f.profile.Digest, IssuerNodeID: f.profile.IssuerNodeID, ReceiverNodeID: member.NodeID, ReceiverDutyGeneration: member.DutyGeneration, Class: 2, WindowStart: time.Now().UTC().Truncate(time.Hour)})
	}
	batch := send(holderCommand{Operation: "begin", Intent: stock.IssuanceIntent{Challenges: challenges, Selection: stock.ExchangeBinding{ID: [32]byte{41, byte(role)}, ProfileDigest: f.profile.Digest}, Bootstrap: true, Deadline: f.profile.NotAfter}})
	decoded, err := admission.DecodeClosedTokenBatch(batch.Request)
	if err != nil || decoded.Class != 2 || len(decoded.BlindedRequests) != 10 {
		t.Fatal("compiled holder did not request exactly ten class-2 tokens", err)
	}
	issued := issuer.IssueCurrent(t.Context(), f.plan, batch.Request, quota.Bootstrap, func() (admission.AuthorityFacts, time.Time, error) {
		return f.authority.issuer(f.plan.KeyBinding.Signer)
	})
	if issued.Outcome != "issued-offline" {
		t.Fatal("genuine JOIN issuance", issued.Outcome)
	}
	result, err := admission.DecodeClosedTokenBatchResult(issued.Response)
	if err != nil || result.Status != admission.ClosedTokenIssued || len(result.Signatures) != 10 {
		t.Fatal("compiled holder batch response count or status", err)
	}
	send(holderCommand{Operation: "complete", Payload: issued.Response})
}
