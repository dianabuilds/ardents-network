//go:build linux || darwin

package networkfixture

import (
	"bytes"
	"crypto/ed25519"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/allocation"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/token"
)

// Signed Network and a genuinely allocated holder permission exercise the
// issuer's durable conflict boundary. Both requests have valid holder proofs;
// this is not malformed-input coverage or a Carrier/Stock consumer substitute.
func TestNetworkIssuerValidChangedDigestRefusesBeforeSigning(t *testing.T) {
	fixture := OpenSignedAdmission(t)
	spec, issuerPlan, observe := fixture.Spec, fixture.Plan, fixture.ObserveIssuer
	p, now, err := observe()
	if err != nil {
		t.Fatal(err)
	}
	prepared, holderKey, err := admission.PreparePermissionRequest(p.IssuanceAuthorityKey, p.NetworkID, p.IssuerNodeID, p.IssuerDutyGeneration, admission.AllocationUser, now.Truncate(time.Hour), [3]uint32{0, 2, 0})
	if err != nil {
		t.Fatal(err)
	}
	defer clear(holderKey)
	public, err := admission.EncodePermissionRequest(prepared)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(public)
	allocationRequest, err := allocation.Prepare(public, p.NetworkID, now)
	if err != nil {
		t.Fatal(err)
	}
	decision, err := allocationRequest.Decide(nil, p.IssuanceAuthorityKey, now)
	if err != nil {
		t.Fatal(err)
	}
	allocationPath := filepath.Join(t.TempDir(), "allocation")
	if err := os.WriteFile(allocationPath, decision.Journal(), 0600); err != nil {
		t.Fatal(err)
	}
	committed, err := os.ReadFile(allocationPath)
	if err != nil || !bytes.Equal(committed, decision.Journal()) {
		t.Fatal("allocation fixture journal readback", err)
	}
	defer clear(committed)
	permission := decision.Permission()
	copy(permission.Signature[:], ed25519.Sign(spec.Authority, admission.PermissionTranscript(permission)))
	challenge := token.ClosedTokenContext{NetworkID: p.NetworkID, ProfileDigest: p.Digest, IssuerNodeID: p.IssuerNodeID, ReceiverNodeID: spec.Nodes[1].NodeID, ReceiverDutyGeneration: spec.Nodes[1].Generation, Class: uint8(admission.ForwardClass), WindowStart: permission.NotBefore}
	var pending [2]*token.PendingClosedTokenBatch
	var raw [2][]byte
	for i := range pending {
		pending[i], err = token.PrepareClosedTokenBatch(token.ClosedTokenBatchConfig{Profile: p, Contexts: []token.ClosedTokenContext{challenge}, Permission: permission, HolderKey: holderKey, Now: now})
		if err != nil {
			t.Fatal(err)
		}
		defer pending[i].Discard()
		raw[i] = pending[i].Request()
		defer clear(raw[i])
	}
	first, err := admission.DecodeClosedTokenBatch(raw[0])
	if err != nil {
		t.Fatal(err)
	}
	changed, err := admission.DecodeClosedTokenBatch(raw[1])
	if err != nil {
		t.Fatal(err)
	}
	changed.RequestID = first.RequestID
	copy(changed.Signature[:], ed25519.Sign(holderKey, admission.TokenBatchTranscript(changed)))
	changedRaw, err := admission.EncodeClosedTokenBatch(changed)
	if err != nil {
		t.Fatal("changed request must retain a valid holder proof", err)
	}
	defer clear(changedRaw)
	if bytes.Equal(changedRaw, raw[0]) {
		t.Fatal("digest conflict oracle has identical inputs")
	}
	issued := issuer.IssueCurrent(t.Context(), issuerPlan, raw[0], quota.Bootstrap, observe)
	defer clear(issued.Response)
	if issued.Outcome != "issued-offline" {
		t.Fatal("original genuine issuance", issued.Phase, issued.Outcome)
	}
	tokens, err := pending[0].FinalizeEncoded(issued.Response)
	for _, raw := range tokens {
		clear(raw)
	}
	if err != nil || len(tokens) != 1 {
		t.Fatal("original signature did not verify", err)
	}
	readJournal := func(root, name string) []byte {
		raw, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	debitBefore, resultBefore := readJournal(issuerPlan.AdmissionRoot, "admission.journal"), readJournal(issuerPlan.ResultRoot, "results.journal")
	defer clear(debitBefore)
	defer clear(resultBefore)
	refused := issuer.IssueCurrent(t.Context(), issuerPlan, changedRaw, quota.Bootstrap, observe)
	defer clear(refused.Response)
	if refused.Phase != "debit" || refused.Outcome != "request-conflict" || refused.Response != nil {
		t.Fatal("valid changed digest reached signing or lost conflict category", refused.Phase, refused.Outcome)
	}
	retry := issuer.IssueCurrent(t.Context(), issuerPlan, raw[0], quota.Bootstrap, observe)
	defer clear(retry.Response)
	if retry.Outcome != "already-issued" || !bytes.Equal(retry.Response, issued.Response) {
		t.Fatal("conflict replaced original retained result", retry.Outcome)
	}
	debitAfter, resultAfter := readJournal(issuerPlan.AdmissionRoot, "admission.journal"), readJournal(issuerPlan.ResultRoot, "results.journal")
	defer clear(debitAfter)
	defer clear(resultAfter)
	if !bytes.Equal(debitBefore, debitAfter) || !bytes.Equal(resultBefore, resultAfter) {
		t.Fatal("valid conflict/retry changed durable histories")
	}
}
