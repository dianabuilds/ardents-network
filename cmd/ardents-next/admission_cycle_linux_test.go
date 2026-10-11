//go:build linux

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuance"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/token"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
)

type localAdmissionReply struct {
	Outcome, Stage                           string
	Link                                     string
	Request, Token, Response, Journal, Proof []byte
	Digest                                   [32]byte
	Slot                                     [32]byte
	Permission                               admission.Permission
	Repeated                                 bool
	Deadline                                 time.Time
	Bytes                                    uint64
}

func admissionLocalCommand(t *testing.T, operation string, config any) localAdmissionReply {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), compiledCommand(t), "admission", operation, "--config", hostingConfig(t, config))
	var output, log bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &log
	if err := cmd.Run(); err != nil {
		t.Fatalf("%s: %v %s %s", operation, err, &output, &log)
	}
	var reply localAdmissionReply
	if err := json.Unmarshal(output.Bytes(), &reply); err != nil {
		t.Fatal(err, &output)
	}
	return reply
}

func admissionLocalConsole(t *testing.T, operation string, config any) (func(any) localAdmissionReply, func()) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), compiledCommand(t), "admission", operation, "--config", hostingConfig(t, config))
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	cmd.Stderr = &log
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	closed := false
	t.Cleanup(func() {
		if !closed {
			_ = in.Close()
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
		if t.Failed() && log.Len() != 0 {
			t.Log("joined console diagnostics:", log.String())
		}
	})
	encode, decode := json.NewEncoder(in), json.NewDecoder(out)
	send := func(value any) localAdmissionReply {
		t.Helper()
		if err := encode.Encode(value); err != nil {
			t.Fatal(err)
		}
		var reply localAdmissionReply
		if err := decode.Decode(&reply); err != nil {
			t.Fatal(err)
		}
		return reply
	}
	close := func() {
		t.Helper()
		_ = in.Close()
		if err := cmd.Wait(); err != nil {
			t.Fatal(err, &log)
		}
		closed = true
	}
	return send, close
}

func TestAdmissionStandaloneCommandsIssuePresentReceiveAndReopen(t *testing.T) {
	// Finish compilation before selecting the real admission hour. The command
	// intentionally has no test-clock override; wait at most one minute to give
	// this multi-process handover a usable real window, never skip or retry it.
	_ = compiledCommand(t)
	if wait := time.Until(time.Now().UTC().Truncate(time.Hour).Add(time.Hour)); wait < time.Minute {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-t.Context().Done():
			t.Fatal(t.Context().Err())
		}
	}
	now := time.Now().UTC()
	start := now.Truncate(time.Hour)
	_, authority, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(authority)
	p := admission.AuthorityFacts{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3}, Digest: [32]byte{4}, IssuerNodeID: [32]byte{5}, IssuerDutyGeneration: 7, NotBefore: start, NotAfter: start.Add(time.Hour)}
	copy(p.IssuanceAuthorityKey[:], authority.Public().(ed25519.PublicKey))
	dir := t.TempDir()
	plan := issuer.Plan{AdmissionRoot: filepath.Join(dir, "quota"), KeyRoot: filepath.Join(dir, "keys"), ResultRoot: filepath.Join(dir, "results"), KeyBinding: issuance.Binding{Network: p.NetworkID, Issuer: p.IssuerNodeID, Signer: [32]byte{6}, Start: p.NotBefore, End: p.NotAfter}}
	if err := issuance.Initialize(t.Context(), plan.KeyRoot, plan.KeyBinding); err != nil {
		t.Fatal(err)
	}
	keys, err := issuance.Open(t.Context(), plan.KeyRoot, plan.KeyBinding)
	if err != nil {
		t.Fatal(err)
	}
	v, err := keys.Inventory()
	if err != nil {
		t.Fatal(err)
	}
	if err := keys.Close(); err != nil {
		t.Fatal(err)
	}
	plan.AdmissionBinding = quota.LedgerBinding{Network: p.NetworkID, Issuer: p.IssuerNodeID, Authority: p.IssuanceAuthorityKey, Profile: p.Digest, Duty: p.IssuerDutyGeneration, Start: p.NotBefore, End: p.NotAfter, Keys: v.Keys}
	if err := quota.Initialize(plan.AdmissionRoot, plan.AdmissionBinding); err != nil {
		t.Fatal(err)
	}
	if r := issuer.Initialize(t.Context(), plan); r.Outcome != "initialized-results" {
		t.Fatal(r)
	}
	p.TokenKeyCount = uint8(len(v.Keys))
	for i, k := range v.Keys {
		p.TokenKeys[i] = admission.TokenKey{WindowStart: time.Unix(int64(k.Window), 0).UTC(), Class: k.Class}
		copy(p.TokenKeys[i].SPKI[:], k.SPKI)
	}
	profile := hostingConfig(t, p)
	holderRoot := t.TempDir()
	holderConfig := map[string]any{"root": holderRoot, "profile": profile, "role": admission.AllocationUser}
	holder, closeHolder := admissionLocalConsole(t, "holder", holderConfig)
	request := holder(holderCommand{Operation: "request", Maxima: [3]uint32{0, 5, 0}})
	if request.Outcome != "completed" {
		t.Fatal(request)
	}
	allocationConfig := map[string]any{"request": request.Request, "journal": []byte{}, "network": p.NetworkID, "authority": p.IssuanceAuthorityKey}
	decision := admissionLocalCommand(t, "allocate", allocationConfig)
	if decision.Outcome != "unsigned-decision" || decision.Repeated || decision.Permission.Signature != [64]byte{} {
		t.Fatal("allocation signed or missing", decision)
	}
	// External Custody boundary: commit and read back the exact proposed public
	// journal before the external authority signs. This is not encrypted Custody.
	journal := filepath.Join(dir, "allocation")
	if err := os.WriteFile(journal, decision.Journal, 0600); err != nil {
		t.Fatal(err)
	}
	retained, err := os.ReadFile(journal)
	if err != nil || !bytes.Equal(retained, decision.Journal) {
		t.Fatal(err)
	}
	allocationConfig["journal"] = retained
	repeated := admissionLocalCommand(t, "allocate", allocationConfig)
	if !repeated.Repeated || repeated.Permission != decision.Permission {
		t.Fatal("allocation retry changed")
	}
	permission := decision.Permission
	copy(permission.Signature[:], ed25519.Sign(authority, admission.PermissionTranscript(permission)))
	signed, err := admission.EncodePermission(permission)
	if err != nil {
		t.Fatal(err)
	}
	if r := holder(holderCommand{Operation: "import", Digest: request.Digest, Payload: signed}); r.Outcome != "completed" {
		t.Fatal(r)
	}
	challenge := token.ClosedTokenContext{NetworkID: p.NetworkID, ProfileDigest: p.Digest, IssuerNodeID: p.IssuerNodeID, ReceiverNodeID: [32]byte{8}, ReceiverDutyGeneration: 9, Class: 2, WindowStart: start}
	intent := stock.IssuanceIntent{Challenges: []token.ClosedTokenContext{challenge, challenge}, Selection: stock.ExchangeBinding{ID: [32]byte{10}, ProfileDigest: p.Digest}, Bootstrap: true, Deadline: p.NotAfter}
	batch := holder(holderCommand{Operation: "begin", Intent: intent})
	issueConfig := map[string]any{"plan": plan, "profile": profile, "batch": batch.Request, "kind": quota.Bootstrap}
	issued := admissionLocalCommand(t, "issue-current", issueConfig)
	retry := admissionLocalCommand(t, "issue-current", issueConfig)
	if issued.Outcome != "issued-offline" || retry.Outcome != "already-issued" || !bytes.Equal(issued.Response, retry.Response) {
		t.Fatal("issuance retry failed", issued.Outcome, retry.Outcome)
	}
	if r := holder(holderCommand{Operation: "complete", Payload: issued.Response}); r.Outcome != "completed" {
		t.Fatal(r)
	}
	presentation := stock.Presentation{NetworkID: p.NetworkID, StateGeneration: p.StateGeneration, StateDigest: p.StateDigest, ProfileDigest: p.Digest, RecipientNodeID: challenge.ReceiverNodeID, RecipientDutyGeneration: challenge.ReceiverDutyGeneration, ChannelNonce: [32]byte{11}, Deadline: p.NotAfter}
	first := holder(holderCommand{Operation: "take", Presentation: presentation, Class: 2})
	second := holder(holderCommand{Operation: "take", Presentation: presentation, Class: 2})
	if first.Outcome != "completed" || second.Outcome != "completed" {
		t.Fatal(first.Outcome, second.Outcome)
	}
	intent.Challenges = append(intent.Challenges, challenge)
	batch = holder(holderCommand{Operation: "begin", Intent: intent})
	issueConfig["batch"] = batch.Request
	issued = admissionLocalCommand(t, "issue-current", issueConfig)
	if r := holder(holderCommand{Operation: "complete", Payload: issued.Response}); r.Outcome != "completed" {
		t.Fatal(r)
	}
	third := holder(holderCommand{Operation: "take", Presentation: presentation, Class: 2})
	if third.Outcome != "completed" {
		t.Fatal(third)
	}
	fourth := holder(holderCommand{Operation: "take", Presentation: presentation, Class: 2})
	if fourth.Outcome != "completed" {
		t.Fatal(fourth)
	}
	fifth := holder(holderCommand{Operation: "take", Presentation: presentation, Class: 2})
	if fifth.Outcome != "completed" {
		t.Fatal(fifth)
	}

	closeHolder()
	reopened, closeReopened := admissionLocalConsole(t, "holder", holderConfig)
	if r := reopened(holderCommand{Operation: "take", Presentation: presentation, Class: 2}); r.Outcome != "refused" {
		t.Fatal("volatile stock revived")
	}
	closeReopened()
	// Keep the actual inherited stdin writer open while interrupting. Closing
	// stdin in the parent would mask a leaked blocking read in the child.
	idle := exec.CommandContext(t.Context(), compiledCommand(t), "admission", "holder", "--config", hostingConfig(t, holderConfig))
	idleIn, err := idle.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer idleIn.Close()
	idleOut, err := idle.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := idle.Start(); err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(idleIn).Encode(holderCommand{Operation: "status"}); err != nil {
		t.Fatal(err)
	}
	var status localAdmissionReply
	if err := json.NewDecoder(idleOut).Decode(&status); err != nil {
		t.Fatal(err)
	}
	if err := idle.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	stopped := make(chan error, 1)
	go func() { stopped <- idle.Wait() }()
	select {
	case err := <-stopped:
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 130 {
			t.Fatal("interrupt result", err)
		}
	case <-time.After(5 * time.Second):
		_ = idle.Process.Kill()
		<-stopped
		t.Fatal("interrupted holder retained stdin/lease")
	}
	reopened, closeReopened = admissionLocalConsole(t, "holder", holderConfig)
	if r := reopened(holderCommand{Operation: "status"}); r.Outcome != "completed" {
		t.Fatal("interrupt retained lease")
	}
	closeReopened()
	receiver := receiving.Receiver{NetworkID: p.NetworkID, StateGeneration: p.StateGeneration, StateDigest: p.StateDigest, ProfileDigest: p.Digest, NodeID: challenge.ReceiverNodeID, DutyGeneration: challenge.ReceiverDutyGeneration}
	checkAdmissionHostingReservation(t, p, receiver, third.Token)
	checkHostingTransfer(t, p, receiver, profile, fourth.Token)
	checkCanceledAdmittedWork(t, p, receiver, fifth.Token)
	receiverConfig := map[string]any{"root": t.TempDir(), "profile": profile, "receiver": receiver, "not_after": p.NotAfter}
	accept := map[string]any{"operation": "accept", "token": first.Token, "class": 2, "deadline": p.NotAfter}
	wrong := receiver
	wrong.DutyGeneration++
	foreignConfig := map[string]any{"root": t.TempDir(), "profile": profile, "receiver": wrong, "not_after": p.NotAfter}
	foreign, closeForeign := admissionLocalConsole(t, "receiver", foreignConfig)
	if r := foreign(accept); r.Outcome != "refused" {
		t.Fatal("foreign duty accepted")
	}
	closeForeign()
	receive, closeReceive := admissionLocalConsole(t, "receiver", receiverConfig)
	grant := receive(accept)
	if grant.Outcome != "completed" {
		t.Fatal(grant)
	}
	refill := receive(map[string]any{"operation": "refill", "token": second.Token, "remaining": 1})
	if refill.Outcome != "completed" || refill.Deadline != grant.Deadline || refill.Bytes != admission.ForwardClass.ByteLimit() {
		t.Fatal("refill changed lifetime", refill)
	}
	closeReceive()
	receive, closeReceive = admissionLocalConsole(t, "receiver", receiverConfig)
	if r := receive(accept); r.Outcome != "refused" {
		t.Fatal("spend replay after reopen")
	}
	closeReceive()
}

func checkAdmissionHostingReservation(t *testing.T, p admission.AuthorityFacts, receiver receiving.Receiver, raw []byte) {
	t.Helper()
	plan := hostingInitializePlan(t)
	if err := hosting.Initialize(plan.Root, plan.Policy); err != nil {
		t.Fatal(err)
	}
	budget, err := hosting.Open(plan.Root)
	if err != nil {
		t.Fatal(err)
	}
	defer budget.Close()
	current := true
	owner, err := receiving.Open(t.TempDir(), receiver, func() (receiving.Observation, error) {
		if !current {
			return receiving.Observation{}, errors.New("authority withdrawn")
		}
		return receiving.Observation{Profile: p, Receiver: receiver, Now: time.Now().UTC(), NotAfter: p.NotAfter}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	tooLarge := true
	calls := 0
	reserve := func() (func() error, error) {
		calls++
		amount := uint64(100)
		if tooLarge {
			amount = plan.Policy.Quantity + 1
		}
		r, err := budget.Reserve(t.Context(), hosting.ReservationRequest{Work: hosting.Traffic{Tx: amount}, Termination: hosting.Traffic{Rx: 20}, WorkUntil: p.NotAfter, HoldUntil: p.NotAfter})
		if err != nil {
			return nil, err
		}
		return func() error { return r.Release(t.Context()) }, nil
	}
	current = false
	if _, err := owner.Accept(t.Context(), admission.ForwardClass, raw, p.NotAfter, reserve); err == nil || calls != 0 {
		t.Fatal("unavailable authority reached capacity")
	}
	current = true
	if _, err := owner.Accept(t.Context(), admission.ForwardClass, raw, p.NotAfter, reserve); err == nil {
		t.Fatal("exhausted budget accepted")
	}
	view, err := budget.Observe(t.Context())
	if err != nil || view.ReservedBytes != 0 {
		t.Fatal("refusal left reserve", view, err)
	}
	tooLarge = false
	grant, err := owner.Accept(t.Context(), admission.ForwardClass, raw, p.NotAfter, reserve)
	if err != nil {
		t.Fatal("capacity refusal burned token", err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	view, err = budget.Observe(t.Context())
	if err != nil || view.ReservedBytes != 120 {
		t.Fatal("receiver close released admitted work", view, err)
	}
	if err := grant.Release(); err != nil {
		t.Fatal(err)
	}
	if err := grant.Release(); err != nil {
		t.Fatal(err)
	}
	view, err = budget.Observe(t.Context())
	if err != nil || view.ReservedBytes != 0 {
		t.Fatal("grant release failed", view, err)
	}
}

func TestAdmissionConsoleCancellationJoinsInput(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	r, w := io.Pipe()
	defer w.Close()
	done := make(chan int, 1)
	go func() {
		done <- admissionConsole(ctx, r, io.Discard, func(context.Context, []byte) (any, bool, error) {
			t.Error("unexpected operation")
			return nil, true, nil
		})
	}()
	cancel()
	if code := <-done; code != 130 {
		t.Fatal(code)
	}
}

func checkHostingTransfer(t *testing.T, p admission.AuthorityFacts, receiver receiving.Receiver, profile string, raw []byte) {
	t.Helper()
	plan := hostingInitializePlan(t)
	if err := hosting.Initialize(plan.Root, plan.Policy); err != nil {
		t.Fatal(err)
	}
	config := hostingWorkFixture{Root: t.TempDir(), Observe: admissionObserver(profile), Budget: plan.Root, Receiver: receiver, NotAfter: p.NotAfter, Deadline: time.Now().Add(4 * time.Second), Token: raw, Bytes: 64 << 10}
	if err := runHostingFixture(t.Context(), config); err != nil {
		t.Fatal("actual work failed", err)
	}
	budget, err := hosting.Open(plan.Root)
	if err != nil {
		t.Fatal(err)
	}
	defer budget.Close()
	view, err := budget.Observe(t.Context())
	if err != nil || view.ReservedBytes != 0 || view.UsedBytes < config.Bytes {
		t.Fatal("work did not charge/join/release", view, err)
	}
	// Reopening with the same token must refuse, retaining no new reserve.
	config.Deadline = time.Now().Add(4 * time.Second)
	if err := runHostingFixture(t.Context(), config); err == nil {
		t.Fatal("replayed work token")
	}
	view, err = budget.Observe(t.Context())
	if err != nil || view.ReservedBytes != 0 {
		t.Fatal(view, err)
	}
}

func checkCanceledAdmittedWork(t *testing.T, facts admission.AuthorityFacts, recipient receiving.Receiver, raw []byte) {
	t.Helper()
	plan := hostingInitializePlan(t)
	if err := hosting.Initialize(plan.Root, plan.Policy); err != nil {
		t.Fatal(err)
	}
	budget, err := hosting.Open(plan.Root)
	if err != nil {
		t.Fatal(err)
	}
	defer budget.Close()
	root := t.TempDir()
	reached := make(chan struct{})
	resume := make(chan struct{})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	blocked := false
	observe := func() (admission.AuthorityFacts, time.Time, error) {
		// The durable spend is a semantic boundary, independent of observer call counts.
		info, err := os.Stat(filepath.Join(root, "closed-token-spends"))
		if !blocked && err == nil && info.Size() > 112 {
			blocked = true
			close(reached)
			<-resume
		}
		return facts, time.Now().UTC(), nil
	}
	work := hostingWorkFixture{Root: root, Budget: plan.Root, Receiver: recipient, NotAfter: facts.NotAfter, Deadline: time.Now().Add(4 * time.Second), Token: raw, Bytes: 64 << 10, Observe: observe}
	done := make(chan error, 1)
	go func() { done <- runHostingFixture(ctx, work) }()
	select {
	case <-reached:
	case err := <-done:
		t.Fatal("work failed before spend", err)
	case <-t.Context().Done():
		t.Fatal(t.Context().Err())
	}
	cancel()
	view, err := budget.Observe(t.Context())
	close(resume)
	outcome := <-done
	if err != nil || view.ReservedBytes == 0 {
		t.Fatal("cancellation released still-running work", view, err)
	}
	if !errors.Is(outcome, context.Canceled) {
		t.Fatal("lost cancellation", outcome)
	}
	view, err = budget.Observe(t.Context())
	if err != nil || view.ReservedBytes != 0 {
		t.Fatal("joined refusal retained reservation", view, err)
	}
	work.Observe = func() (admission.AuthorityFacts, time.Time, error) { return facts, time.Now().UTC(), nil }
	work.Deadline = time.Now().Add(4 * time.Second)
	if err := runHostingFixture(t.Context(), work); err == nil {
		t.Fatal("canceled spent token became reusable")
	}
}
