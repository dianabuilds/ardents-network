package node_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuance"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/token"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
)

func TestAdmissionStandaloneCommandsIssuePresentReceiveAndReopen(t *testing.T) {
	runAdmissionCycle(t, nil)
}

func runAdmissionCycle(t *testing.T, check func(*testing.T, admission.AuthorityFacts, receiving.Receiver, string, []byte, []byte, []byte)) {
	t.Helper()
	// Finish compilation before selecting the real admission hour. The command
	// intentionally has no test-clock override; wait at most one minute to give
	// this multi-process handover a usable real window, never skip or retry it.
	binary := buildNodeCommand(t)
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
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
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
	profile := processfixture.WriteConfig(t, p)
	holderRoot := t.TempDir()
	holderConfig := map[string]any{"root": holderRoot, "profile": profile, "role": admission.AllocationUser}
	holder, closeHolder := processfixture.OpenConfiguredAdmissionConsole(t, binary, "holder", holderConfig)
	request := holder(processfixture.HolderCommand{Operation: "request", Maxima: [3]uint32{0, 5, 0}})
	if request.Outcome != "completed" {
		t.Fatal(request)
	}
	allocationConfig := map[string]any{"request": request.Request, "journal": []byte{}, "network": p.NetworkID, "authority": p.IssuanceAuthorityKey}
	decision := processfixture.RunAdmissionCommand(t, binary, "allocate", allocationConfig)
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
	repeated := processfixture.RunAdmissionCommand(t, binary, "allocate", allocationConfig)
	if !repeated.Repeated || repeated.Permission != decision.Permission {
		t.Fatal("allocation retry changed")
	}
	permission := decision.Permission
	copy(permission.Signature[:], ed25519.Sign(authority, admission.PermissionTranscript(permission)))
	signed, err := admission.EncodePermission(permission)
	if err != nil {
		t.Fatal(err)
	}
	if r := holder(processfixture.HolderCommand{Operation: "import", Digest: request.Digest, Payload: signed}); r.Outcome != "completed" {
		t.Fatal(r)
	}
	challenge := token.ClosedTokenContext{NetworkID: p.NetworkID, ProfileDigest: p.Digest, IssuerNodeID: p.IssuerNodeID, ReceiverNodeID: [32]byte{8}, ReceiverDutyGeneration: 9, Class: 2, WindowStart: start}
	intent := stock.IssuanceIntent{Challenges: []token.ClosedTokenContext{challenge, challenge}, Selection: stock.ExchangeBinding{ID: [32]byte{10}, ProfileDigest: p.Digest}, Bootstrap: true, Deadline: p.NotAfter}
	batch := holder(processfixture.HolderCommand{Operation: "begin", Intent: intent})
	issueConfig := map[string]any{"plan": plan, "profile": profile, "batch": batch.Request, "kind": quota.Bootstrap}
	issued := processfixture.RunAdmissionCommand(t, binary, "issue-current", issueConfig)
	retry := processfixture.RunAdmissionCommand(t, binary, "issue-current", issueConfig)
	if issued.Outcome != "issued-offline" || retry.Outcome != "already-issued" || !bytes.Equal(issued.Response, retry.Response) {
		t.Fatal("issuance retry failed", issued.Outcome, retry.Outcome)
	}
	if r := holder(processfixture.HolderCommand{Operation: "complete", Payload: issued.Response}); r.Outcome != "completed" {
		t.Fatal(r)
	}
	presentation := stock.Presentation{NetworkID: p.NetworkID, StateGeneration: p.StateGeneration, StateDigest: p.StateDigest, ProfileDigest: p.Digest, RecipientNodeID: challenge.ReceiverNodeID, RecipientDutyGeneration: challenge.ReceiverDutyGeneration, ChannelNonce: [32]byte{11}, Deadline: p.NotAfter}
	first := holder(processfixture.HolderCommand{Operation: "take", Presentation: presentation, Class: 2})
	second := holder(processfixture.HolderCommand{Operation: "take", Presentation: presentation, Class: 2})
	if first.Outcome != "completed" || second.Outcome != "completed" {
		t.Fatal(first.Outcome, second.Outcome)
	}
	intent.Challenges = append(intent.Challenges, challenge)
	batch = holder(processfixture.HolderCommand{Operation: "begin", Intent: intent})
	issueConfig["batch"] = batch.Request
	issued = processfixture.RunAdmissionCommand(t, binary, "issue-current", issueConfig)
	if r := holder(processfixture.HolderCommand{Operation: "complete", Payload: issued.Response}); r.Outcome != "completed" {
		t.Fatal(r)
	}
	third := holder(processfixture.HolderCommand{Operation: "take", Presentation: presentation, Class: 2})
	if third.Outcome != "completed" {
		t.Fatal(third)
	}
	fourth := holder(processfixture.HolderCommand{Operation: "take", Presentation: presentation, Class: 2})
	if fourth.Outcome != "completed" {
		t.Fatal(fourth)
	}
	fifth := holder(processfixture.HolderCommand{Operation: "take", Presentation: presentation, Class: 2})
	if fifth.Outcome != "completed" {
		t.Fatal(fifth)
	}

	closeHolder()
	reopened, closeReopened := processfixture.OpenConfiguredAdmissionConsole(t, binary, "holder", holderConfig)
	if r := reopened(processfixture.HolderCommand{Operation: "take", Presentation: presentation, Class: 2}); r.Outcome != "refused" {
		t.Fatal("volatile stock revived")
	}
	closeReopened()
	receiver := receiving.Receiver{NetworkID: p.NetworkID, StateGeneration: p.StateGeneration, StateDigest: p.StateDigest, ProfileDigest: p.Digest, NodeID: challenge.ReceiverNodeID, DutyGeneration: challenge.ReceiverDutyGeneration}
	if check != nil {
		check(t, p, receiver, profile, third.Token, fourth.Token, fifth.Token)
	}
	receiverConfig := map[string]any{"root": t.TempDir(), "profile": profile, "receiver": receiver, "not_after": p.NotAfter}
	accept := map[string]any{"operation": "accept", "token": first.Token, "class": 2, "deadline": p.NotAfter}
	wrong := receiver
	wrong.DutyGeneration++
	foreignConfig := map[string]any{"root": t.TempDir(), "profile": profile, "receiver": wrong, "not_after": p.NotAfter}
	foreign, closeForeign := processfixture.OpenConfiguredAdmissionConsole(t, binary, "receiver", foreignConfig)
	if r := foreign(accept); r.Outcome != "refused" {
		t.Fatal("foreign duty accepted")
	}
	closeForeign()
	receive, closeReceive := processfixture.OpenConfiguredAdmissionConsole(t, binary, "receiver", receiverConfig)
	grant := receive(accept)
	if grant.Outcome != "completed" {
		t.Fatal(grant)
	}
	refill := receive(map[string]any{"operation": "refill", "token": second.Token, "remaining": 1})
	if refill.Outcome != "completed" || refill.Deadline != grant.Deadline || refill.Bytes != admission.ForwardClass.ByteLimit() {
		t.Fatal("refill changed lifetime", refill)
	}
	closeReceive()
	receive, closeReceive = processfixture.OpenConfiguredAdmissionConsole(t, binary, "receiver", receiverConfig)
	if r := receive(accept); r.Outcome != "refused" {
		t.Fatal("spend replay after reopen")
	}
	closeReceive()
}
