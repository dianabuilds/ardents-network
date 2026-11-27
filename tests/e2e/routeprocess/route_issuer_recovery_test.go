package routeprocess_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/token"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
	networkinputs "github.com/dianabuilds/ardents-network/tests/networkfixture"
)

// TestIssuerLostResultDoesNotReconstructHolder uses real holder and offline
// issuer commands. It does not claim that a network response was interrupted.
func TestIssuerLostResultDoesNotReconstructHolder(t *testing.T) {
	binary := buildNodeCommand(t)
	processfixture.WaitAdmissionHour(t)
	spec, issuerPlan, inventory := networkinputs.PrepareNetworkGenesis(t, processfixture.TempDir(t))
	bundle, err := networkfixture.BuildClosed(spec)
	if err != nil {
		t.Fatal(err)
	}
	networkinputs.PrepareIssuer(t, &issuerPlan, inventory, spec, bundle.Nodes[0], sha256.Sum256(bundle.Profile))
	clock, stopClock := networkinputs.StartObservedClock(t)
	defer stopClock()
	var authorityKey [32]byte
	copy(authorityKey[:], spec.Authority.Public().(ed25519.PublicKey))
	seed := func() state.AuthorityDeclaration {
		t.Helper()
		return networkinputs.SeedNetworkState(t, spec.NetworkID, authorityKey, bundle, clock)
	}
	networkPlan := seed()
	config, err := networkPlan.Config()
	if err != nil {
		t.Fatal(err)
	}
	owner, err := state.OpenRetained(config)
	if err != nil {
		t.Fatal(err)
	}
	view, viewErr := owner.CurrentRuntime()
	closeErr := owner.Close()
	if viewErr != nil || closeErr != nil {
		t.Fatal("holder authority", viewErr, closeErr)
	}
	facts, now, err := funding.NetworkFacts(view)
	if err != nil {
		t.Fatal(err)
	}
	holderPlan := map[string]any{"root": processfixture.TempDir(t), "network": networkPlan, "role": admission.AllocationUser}
	holder, closeHolder := processfixture.OpenConfiguredAdmissionConsole(t, binary, "holder", holderPlan)
	request := holder(processfixture.HolderCommand{Operation: "request", Maxima: [3]uint32{0, 1, 0}})
	if request.Outcome != "completed" {
		t.Fatal("original holder request", request.Outcome)
	}
	permission := processfixture.FixturePermission(t, request.Request, facts, spec.Authority, now)
	if reply := holder(processfixture.HolderCommand{Operation: "import", Digest: request.Digest, Payload: permission}); reply.Outcome != "completed" {
		t.Fatal("original holder permission", reply.Outcome)
	}
	challenge := token.ClosedTokenContext{NetworkID: facts.NetworkID, ProfileDigest: facts.Digest, IssuerNodeID: facts.IssuerNodeID,
		ReceiverNodeID: spec.Nodes[1].NodeID, ReceiverDutyGeneration: spec.Nodes[1].Generation, Class: 2, WindowStart: now.UTC().Truncate(time.Hour)}
	intent := stock.IssuanceIntent{Challenges: []token.ClosedTokenContext{challenge}, Selection: stock.ExchangeBinding{ID: [32]byte{72}, ProfileDigest: facts.Digest}, Deadline: facts.NotAfter}
	batch := holder(processfixture.HolderCommand{Operation: "begin", Intent: intent})
	if batch.Outcome != "completed" || len(batch.Request) == 0 {
		t.Fatal("original issuance request", batch.Outcome)
	}
	issuerConfig := map[string]any{"plan": issuerPlan, "network": seed(), "batch": batch.Request, "kind": quota.Admitted}
	issued := processfixture.RunAdmissionCommand(t, binary, "issue-current", issuerConfig)
	if issued.Outcome != "issued-offline" || len(issued.Response) == 0 {
		t.Fatal("actual issuer result", issued.Outcome)
	}
	debitPath := filepath.Join(issuerPlan.AdmissionRoot, "admission.journal")
	resultPath := filepath.Join(issuerPlan.ResultRoot, "results.journal")
	debits, err := os.ReadFile(debitPath)
	if err != nil {
		t.Fatal("issuer debit journal", err)
	}
	results, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatal("issuer result journal", err)
	}
	retry := processfixture.RunAdmissionCommand(t, binary, "issue-current", issuerConfig)
	if retry.Outcome != "already-issued" || !bytes.Equal(retry.Response, issued.Response) {
		t.Fatal("issuer retry changed retained result", retry.Outcome)
	}
	refuse := func(input map[string]any) processfixture.AdmissionReply {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), binary, "admission", "issue-current", "--config", processfixture.WriteConfig(t, input))
		var diagnostic bytes.Buffer
		cmd.Stderr = &diagnostic
		output, err := cmd.Output()
		if diagnostic.String() != `{"operation":"admission.issue-current","phase":"command","outcome":"failed"}`+"\n" {
			t.Fatalf("issuer refusal observation: %q", &diagnostic)
		}
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 1 {
			t.Fatal("issuer refusal did not execute", err, string(output))
		}
		var reply processfixture.AdmissionReply
		if err := json.Unmarshal(output, &reply); err != nil {
			t.Fatal("issuer refusal report", err)
		}
		return reply
	}
	wrongKind := refuse(map[string]any{"plan": issuerPlan, "network": seed(), "batch": batch.Request, "kind": quota.Bootstrap})
	if wrongKind.Outcome != "request-conflict" || len(wrongKind.Response) != 0 {
		t.Fatal("issuer accepted a changed issuance kind", wrongKind.Outcome)
	}
	changed := bytes.Clone(batch.Request)
	changed[len(changed)-1] ^= 1
	wrongRequest := refuse(map[string]any{"plan": issuerPlan, "network": seed(), "batch": changed, "kind": quota.Admitted})
	if len(wrongRequest.Response) != 0 || wrongRequest.Outcome == "issued-offline" || wrongRequest.Outcome == "already-issued" {
		t.Fatal("issuer accepted a changed request", wrongRequest.Outcome)
	}
	afterDebits, debitErr := os.ReadFile(debitPath)
	afterResults, resultErr := os.ReadFile(resultPath)
	if debitErr != nil || resultErr != nil || !bytes.Equal(debits, afterDebits) || !bytes.Equal(results, afterResults) {
		t.Fatal("retry changed retained issuer histories", debitErr, resultErr)
	}
	if status := holder(processfixture.HolderCommand{Operation: "status"}); status.Outcome != "completed" || !status.Stock.Pending {
		t.Fatal("original pending issuance absent", status.Outcome)
	}
	closeHolder() // The issued response is deliberately never delivered.
	reopened, closeReopened := processfixture.OpenConfiguredAdmissionConsole(t, binary, "holder", holderPlan)
	if status := reopened(processfixture.HolderCommand{Operation: "status"}); status.Outcome != "completed" || status.Stock.Accepted || status.Stock.Pending || status.Stock.Busy {
		t.Fatal("reopened holder reconstructed volatile state", status.Outcome)
	}
	for _, payload := range [][]byte{permission, issued.Response} {
		if reply := reopened(processfixture.HolderCommand{Operation: "import", Digest: request.Digest, Payload: payload}); reply.Outcome != "refused" {
			t.Fatal("reopened holder adopted old material", reply.Outcome)
		}
	}
	fresh := reopened(processfixture.HolderCommand{Operation: "request", Maxima: [3]uint32{0, 1, 0}})
	oldRequest, oldErr := admission.DecodePermissionRequest(request.Request)
	newRequest, newErr := admission.DecodePermissionRequest(fresh.Request)
	if fresh.Outcome != "completed" || oldErr != nil || newErr != nil || fresh.Digest == request.Digest || oldRequest.Permission.HolderKey == newRequest.Permission.HolderKey {
		t.Fatal("fresh holder reused lost identity", fresh.Outcome, oldErr, newErr)
	}
	if reply := reopened(processfixture.HolderCommand{Operation: "import", Digest: fresh.Digest, Payload: permission}); reply.Outcome != "refused" {
		t.Fatal("new request adopted an old permission", reply.Outcome)
	}
	closeReopened()
}
