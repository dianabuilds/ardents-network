package node_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/token"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/session"
	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	"github.com/dianabuilds/ardents-network/internal/domain/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
	"github.com/dianabuilds/ardents-network/internal/domain/reachability"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
	networkinputs "github.com/dianabuilds/ardents-network/tests/networkfixture"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestRouteDescriptorSurvivesReceiverRestart verifies published proof and
// refusal behavior through actual holder and receiving command processes.
func TestRouteDescriptorSurvivesReceiverRestart(t *testing.T) {
	binary := buildNodeCommand(t)
	processfixture.WaitAdmissionHour(t)
	spec, issuerPlan, inventory := networkinputs.PrepareNetworkGenesis(t, processfixture.TempDir(t))
	spec.Nodes = spec.Nodes[:1]
	listeners := make(map[[32]byte]net.Listener)
	certificates := make(map[[32]byte]tls.Certificate)
	for _, id := range [][32]byte{{12}, {13}, {14}, {15}, {17}, {18}} {
		_, key, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { clear(key) })
		endpoint := "127.0.0.1:1"
		if id[0] != 18 {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = listener.Close() })
			listeners[id], endpoint = listener, listener.Addr().String()
		}
		domain, subrole := byte(1), byte(1)
		if id[0] >= 14 {
			subrole = 2
		}
		if id[0] == 17 {
			domain, subrole = 2, 5
		}
		if id[0] == 18 {
			domain, subrole = 4, 3
		}
		spec.Nodes = append(spec.Nodes, networkfixture.ClosedNode{RecordSpec: networkfixture.RecordSpec{
			NodeID: id, Generation: 9, ValidFrom: spec.NotBefore, ValidUntil: spec.NotAfter, Endpoint: endpoint,
			Carrier: epoch.CarrierClosedTCP, Capability: 2, Capacity: 1, PrivateKey: key,
		}, RoleDomain: domain, Subrole: subrole})
		certificates[id] = processfixture.RoleCertificate(t, key)
	}
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
	var peers []*routeBootstrapPeer
	var budgets []string
	descriptorRoot := processfixture.TempDir(t)
	var descriptorConfig, descriptorSpendRoot string
	for _, id := range [][32]byte{{12}, {13}, {14}, {15}, {17}} {
		cert, key := processfixture.WriteNativeCredential(t, certificates[id])
		budget := processfixture.ProcessBudget(t)
		budgets = append(budgets, budget)
		spendRoot := processfixture.TempDir(t)
		plan := map[string]any{"network": seed(), "node_id": id, "spend_root": spendRoot, "hosting_root": budget,
			"certificate": cert, "private_key": key, "work": hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
		if id[0] == 17 {
			plan["descriptor_root"] = descriptorRoot
			descriptorSpendRoot = spendRoot
		}
		if err := listeners[id].Close(); err != nil {
			t.Fatal(err)
		}
		path := processfixture.WriteConfig(t, plan)
		if id[0] == 17 {
			descriptorConfig = path
		}
		peers = append(peers, startRouteBootstrapPeer(t, binary, path))
	}
	networkPlan := seed()
	config, err := networkPlan.Config()
	if err != nil {
		t.Fatal(err)
	}
	authority, err := state.OpenRetained(config)
	if err != nil {
		t.Fatal(err)
	}
	view, viewErr := authority.CurrentRuntime()
	closeErr := authority.Close()
	if viewErr != nil || closeErr != nil {
		t.Fatal("signed holder authority", viewErr, closeErr)
	}
	facts, now, err := funding.NetworkFacts(view)
	if err != nil {
		t.Fatal(err)
	}
	budget := processfixture.ProcessBudget(t)
	budgets = append(budgets, budget)
	route := map[string]any{"entry_root": filepath.Join(processfixture.TempDir(t), "entry"), "interior_root": filepath.Join(processfixture.TempDir(t), "interior"),
		"hosting_root": budget, "domain": 1, "deadline": time.Now().Add(30 * time.Second).UTC().Truncate(time.Second),
		"work": hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
	send, closeHolder := admissionLocalConsole(t, binary, "holder", map[string]any{"root": processfixture.TempDir(t), "network": networkPlan, "role": admission.AllocationUser, "route": route})
	request := send(holderCommand{Operation: "request", Maxima: [3]uint32{10, 4, 0}})
	if request.Outcome != "completed" {
		t.Fatal("holder request", request.Outcome)
	}
	permission := processfixture.FixturePermission(t, request.Request, facts, spec.Authority, now)
	if reply := send(holderCommand{Operation: "import", Digest: request.Digest, Payload: permission}); reply.Outcome != "completed" {
		t.Fatal("holder import", reply.Outcome)
	}
	var forwards, controls []token.ClosedTokenContext
	for i := byte(12); i < 16; i++ {
		forwards = append(forwards, token.ClosedTokenContext{NetworkID: facts.NetworkID, ProfileDigest: facts.Digest, IssuerNodeID: facts.IssuerNodeID,
			ReceiverNodeID: [32]byte{i}, ReceiverDutyGeneration: 9, Class: 2, WindowStart: time.Now().UTC().Truncate(time.Hour)})
	}
	for range 10 {
		controls = append(controls, token.ClosedTokenContext{NetworkID: facts.NetworkID, ProfileDigest: facts.Digest, IssuerNodeID: facts.IssuerNodeID,
			ReceiverNodeID: [32]byte{17}, ReceiverDutyGeneration: 9, Class: 1, WindowStart: time.Now().UTC().Truncate(time.Hour)})
	}
	for index, contexts := range [][]token.ClosedTokenContext{forwards, controls} {
		intent := stock.IssuanceIntent{Challenges: contexts, Selection: stock.ExchangeBinding{ID: [32]byte{byte(41 + index)}, ProfileDigest: facts.Digest}, Bootstrap: true, Deadline: facts.NotAfter}
		batch := send(holderCommand{Operation: "begin", Intent: intent})
		if batch.Outcome != "completed" {
			t.Fatal("holder begin", batch.Outcome)
		}
		issued := admissionLocalCommand(t, binary, "issue-current", map[string]any{"plan": issuerPlan, "network": seed(), "batch": batch.Request, "kind": quota.Bootstrap})
		if issued.Outcome != "issued-offline" {
			t.Fatal("issuer response", issued.Outcome)
		}
		if reply := send(holderCommand{Operation: "complete", Payload: issued.Response}); reply.Outcome != "completed" {
			t.Fatal("holder complete", reply.Outcome)
		}
	}
	raw, target := processfixture.SignedDescriptor(t, facts, 3, [32]byte{18}, facts.Digest)
	if reply := send(holderCommand{Operation: "prefix-open"}); reply.Outcome != "completed" {
		t.Fatal("descriptor prefix", reply.Outcome)
	}
	spendPath := filepath.Join(descriptorSpendRoot, "closed-token-spends")
	descriptorCommand := func(command holderCommand) localAdmissionReply {
		t.Helper()
		before, err := os.ReadFile(spendPath)
		if err != nil {
			t.Fatal("read Descriptor spending journal", err)
		}
		reply := send(command)
		after, err := os.ReadFile(spendPath)
		if err != nil || bytes.Equal(before, after) {
			t.Fatal("Descriptor operation did not spend its Control right", command.Operation, err)
		}
		return reply
	}
	if reply := descriptorCommand(holderCommand{Operation: "descriptor-publish", Payload: raw}); reply.Outcome != "completed" {
		t.Fatal("descriptor publication", reply.Outcome)
	}
	assertLookup := func() {
		t.Helper()
		reply := descriptorCommand(holderCommand{Operation: "descriptor-lookup", Target: target})
		if reply.Outcome != "completed" || !bytes.Equal(reply.Proof, raw) {
			t.Fatal("lookup changed exact proof", reply.Outcome)
		}
	}
	assertLookup()
	connectOnly, _ := processfixture.SignedDescriptor(t, facts, 2, [32]byte{18}, facts.Digest)
	broken := bytes.Clone(raw)
	broken[len(broken)-1] ^= 1
	for _, command := range []holderCommand{{Operation: "descriptor-publish", Payload: connectOnly}, {Operation: "descriptor-publish", Payload: broken}, {Operation: "descriptor-lookup", Target: [32]byte{99}}} {
		if reply := descriptorCommand(command); reply.Outcome == "completed" {
			t.Fatal("invalid Descriptor accepted", command.Operation)
		}
	}
	for _, introduction := range [][32]byte{{12}, {99}} {
		invalidDuty, _ := processfixture.SignedDescriptor(t, facts, 3, introduction, facts.Digest)
		if reply := descriptorCommand(holderCommand{Operation: "descriptor-publish", Payload: invalidDuty}); reply.Outcome == "completed" {
			t.Fatal("Descriptor bypassed current Introduction duty", introduction[0])
		}
	}
	foreignProfile := facts.Digest
	foreignProfile[0] ^= 1
	foreign, _ := processfixture.SignedDescriptor(t, facts, 3, [32]byte{18}, foreignProfile)
	if _, err := reachability.VerifyPublish(foreign, facts.NetworkID, foreignProfile, time.Now()); err != nil {
		t.Fatal("foreign-profile control has invalid signatures", err)
	}
	if reply := descriptorCommand(holderCommand{Operation: "descriptor-publish", Payload: foreign}); reply.Outcome == "completed" {
		t.Fatal("foreign-profile Descriptor accepted")
	}
	assertLookup()
	resolution := peers[len(peers)-1]
	if err := processfixture.Interrupt(resolution.command); err != nil {
		t.Fatal(err)
	}
	stopErr := resolution.command.Wait()
	resolution.waited = true
	var exit *exec.ExitError
	if !errors.As(stopErr, &exit) || exit.ExitCode() != 130 {
		t.Fatal("resolution did not join", stopErr, resolution.diagnostic.String())
	}
	peers[len(peers)-1] = startRouteBootstrapPeer(t, binary, descriptorConfig)
	assertLookup()
	if reply := send(holderCommand{Operation: "prefix-close"}); reply.Outcome != "completed" {
		t.Fatal("prefix close", reply.Outcome)
	}
	closeHolder()
	// The console intentionally projects only a generic refusal. Exercise the
	// same actual client interface against the same real receiving processes to
	// check the complete refusal cause without adding a test-only command.
	clientPlan := seed()
	clientAuthority, err := funding.OpenAdmissionAuthority("", &clientPlan)
	if err != nil {
		t.Fatal("Descriptor client authority", err)
	}
	defer clientAuthority.Close()
	clientStock, err := stock.Open(processfixture.TempDir(t), admission.AllocationUser, clientAuthority.ObserveFacts)
	if err != nil {
		t.Fatal("Descriptor client Stock", err)
	}
	defer clientStock.Close()
	clientRequest, clientDigest, err := clientStock.Request([3]uint32{2, 4, 0})
	if err != nil {
		t.Fatal("Descriptor client permission request", err)
	}
	clientPermission := processfixture.FixturePermission(t, clientRequest, facts, spec.Authority, time.Now())
	if err := clientStock.Import(clientDigest, clientPermission); err != nil {
		t.Fatal("Descriptor client permission", err)
	}
	for index, contexts := range [][]token.ClosedTokenContext{forwards, {controls[0], controls[1]}} {
		intent := stock.IssuanceIntent{Challenges: contexts, Selection: stock.ExchangeBinding{ID: [32]byte{62, byte(index)}, ProfileDigest: facts.Digest}, Bootstrap: true, Deadline: facts.NotAfter}
		attempt, err := clientStock.Begin(intent)
		if err != nil {
			t.Fatal("Descriptor client issuance", err)
		}
		batch, _, err := attempt.Request()
		if err != nil {
			t.Fatal("Descriptor client batch", err)
		}
		issued := admissionLocalCommand(t, binary, "issue-current", map[string]any{"plan": issuerPlan, "network": seed(), "batch": batch, "kind": quota.Bootstrap})
		if issued.Outcome != "issued-offline" {
			t.Fatal("Descriptor client issuer", issued.Outcome)
		}
		if err := attempt.Complete(issued.Response, nil); err != nil {
			t.Fatal("Descriptor client issuance result", err)
		}
	}
	clientRoot := processfixture.TempDir(t)
	clientBudget := processfixture.ProcessBudget(t)
	budgets = append(budgets, clientBudget)
	clientRoute := funding.Plan{EntryRoot: filepath.Join(clientRoot, "entry"), InteriorRoot: filepath.Join(clientRoot, "interior"), HostingRoot: clientBudget,
		Domain: 1, Deadline: route["deadline"].(time.Time), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
	client, err := session.OpenPrefix(t.Context(), clientRoute, funding.Observe(clientAuthority.Current, clientAuthority.Witness, clientAuthority.Presentation), clientStock)
	if err != nil {
		t.Fatal("Descriptor client Prefix", err)
	}
	for _, input := range [][]byte{connectOnly, foreign} {
		if err := client.PublishDescriptor(t.Context(), input); !descriptorRefusalOnly(err) {
			t.Fatal("Descriptor refusal lost exact status or included unrelated failure", err)
		}
	}
	if err := errors.Join(client.Close(), clientStock.Close(), clientAuthority.Close()); err != nil {
		t.Fatal("Descriptor client joined result", err)
	}
	for _, peer := range peers {
		if err := processfixture.Interrupt(peer.command); err != nil {
			t.Fatal(err)
		}
	}
	for _, peer := range peers {
		err := peer.command.Wait()
		peer.waited = true
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 130 {
			t.Fatal("receiver did not join", err, peer.diagnostic.String())
		}
	}
	stored, err := reachability.OpenStore(reachability.StoreConfig{Root: descriptorRoot, Network: facts.NetworkID})
	if err != nil {
		t.Fatal("Descriptor Store reopen", err)
	}
	proof, outcome, lookupErr := stored.Lookup(target, facts.Digest, time.Now())
	closeErr = stored.Close()
	if lookupErr != nil || closeErr != nil || outcome != reachability.Accepted || !bytes.Equal(proof, raw) {
		t.Fatal("Descriptor durability", outcome, lookupErr, closeErr)
	}
	for _, root := range budgets {
		owner, err := hosting.Open(root)
		if err != nil {
			t.Fatal(err)
		}
		observation, observeErr := owner.Observe(t.Context())
		closeErr := owner.Close()
		if observeErr != nil || closeErr != nil || observation.ReservedBytes != 0 {
			t.Fatal("joined receiver retained Hosting", observeErr, closeErr, observation.ReservedBytes)
		}
	}
}

// descriptorRefusalOnly accepts only the fixed refusal, including through
// wrappers; a joined unrelated failure cannot be concealed by one match.
func descriptorRefusalOnly(err error) bool {
	if err == nil {
		return false
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		children := joined.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if !descriptorRefusalOnly(child) {
				return false
			}
		}
		return true
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return descriptorRefusalOnly(wrapped.Unwrap())
	}
	return err.Error() == "route Descriptor refused (1)"
}
