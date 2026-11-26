package node_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/admission/token"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	"github.com/dianabuilds/ardents-network/internal/domain/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
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

// TestRouteRefusesSignedQUICBeforeEffects verifies that the selected TCP-only
// release rejects an authenticated deferred QUIC duty before opening Admission.
func TestRouteRefusesSignedQUICBeforeEffects(t *testing.T) {
	binary := buildNodeCommand(t)
	processfixture.WaitAdmissionHour(t)
	spec, issuerPlan, inventory := networkinputs.PrepareNetworkGenesis(t, processfixture.TempDir(t))
	tcp, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer tcp.Close()
	udpReservation, err := net.ListenPacket("udp", tcp.Addr().String())
	if err != nil {
		t.Fatal("invalid environment: TCP/UDP endpoint unavailable", err)
	}
	spec.Nodes[1].Endpoint = tcp.Addr().String()
	spec.Nodes[1].Carrier = epoch.CarrierClosedQUIC
	for i := byte(13); i < 16; i++ {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = listener.Close() })
		_, key, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { clear(key) })
		subrole := byte(1)
		if i >= 14 {
			subrole = 2
		}
		spec.Nodes = append(spec.Nodes, networkfixture.ClosedNode{RecordSpec: networkfixture.RecordSpec{
			NodeID: [32]byte{i}, Generation: 9, ValidFrom: spec.NotBefore, ValidUntil: spec.NotAfter,
			Endpoint: listener.Addr().String(), Carrier: epoch.CarrierClosedQUIC, Capability: 2, Capacity: 1, PrivateKey: key,
		}, RoleDomain: 3, Subrole: subrole})
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
	networkPlan := networkinputs.SeedNetworkState(t, spec.NetworkID, authorityKey, bundle, clock)
	config, err := networkPlan.Config()
	if err != nil {
		t.Fatal(err)
	}
	owner, err := state.OpenRetained(config)
	if err != nil {
		t.Fatal(err)
	}
	view, viewErr := owner.CurrentRuntime()
	if viewErr != nil {
		t.Fatal(viewErr)
	}
	member, memberErr := view.Member(spec.Nodes[1].NodeID, view.ObservedAt())
	duty, dutyErr := view.RetainDuty(spec.Nodes[1].NodeID, view.ObservedAt())
	facts, now, factsErr := funding.NetworkFacts(view)
	closeErr := owner.Close()
	if memberErr != nil || dutyErr != nil || factsErr != nil || closeErr != nil || member.CarrierProfile != epoch.CarrierClosedQUIC {
		t.Fatal("signed QUIC duty", memberErr, dutyErr, factsErr, closeErr, member.CarrierProfile)
	}
	cert, key := processfixture.WriteNativeCredential(t, processfixture.RoleCertificate(t, spec.Nodes[1].PrivateKey))
	spendRoot := processfixture.TempDir(t)
	budgetRoot := processfixture.ProcessBudget(t)
	plan := map[string]any{"network": networkPlan, "node_id": member.NodeID, "spend_root": spendRoot, "hosting_root": budgetRoot,
		"certificate": cert, "private_key": key, "work": hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
	if err := udpReservation.Close(); err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(t.Context(), binary, "route", "receive", "--config", processfixture.WriteConfig(t, plan))
	output, err := command.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Fatal("QUIC receiver did not refuse", err, string(output))
	}
	entries, err := os.ReadDir(spendRoot)
	if err != nil || len(entries) != 0 {
		t.Fatal("QUIC refusal touched Admission root", err, len(entries))
	}
	udp, err := net.ListenPacket("udp", member.Endpoint)
	if err != nil {
		t.Fatal("QUIC refusal retained UDP listener", err)
	}
	defer udp.Close()
	seed := func() state.AuthorityDeclaration {
		t.Helper()
		return networkinputs.SeedNetworkState(t, spec.NetworkID, authorityKey, bundle, clock)
	}
	root := processfixture.TempDir(t)
	deadline := time.Now().Add(20 * time.Second).UTC().Truncate(time.Second)
	route := map[string]any{"entry_root": filepath.Join(root, "entry"), "interior_root": filepath.Join(root, "interior"),
		"hosting_root": budgetRoot, "domain": 3, "deadline": deadline, "work": hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20},
		"termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
	holder, closeHolder := admissionLocalConsole(t, binary, "holder", map[string]any{"root": processfixture.TempDir(t), "network": seed(), "role": admission.AllocationUser, "route": route})
	request := holder(holderCommand{Operation: "request", Maxima: [3]uint32{0, 4, 0}})
	if request.Outcome != "completed" {
		t.Fatal("holder request", request.Outcome)
	}
	permission := processfixture.FixturePermission(t, request.Request, facts, spec.Authority, now)
	if reply := holder(holderCommand{Operation: "import", Digest: request.Digest, Payload: permission}); reply.Outcome != "completed" {
		t.Fatal("holder import", reply.Outcome)
	}
	var challenges []token.ClosedTokenContext
	for i := byte(12); i < 16; i++ {
		challenges = append(challenges, token.ClosedTokenContext{NetworkID: facts.NetworkID, ProfileDigest: facts.Digest, IssuerNodeID: facts.IssuerNodeID,
			ReceiverNodeID: [32]byte{i}, ReceiverDutyGeneration: 9, Class: 2, WindowStart: time.Now().UTC().Truncate(time.Hour)})
	}
	intent := stock.IssuanceIntent{Challenges: challenges, Selection: stock.ExchangeBinding{ID: [32]byte{31}, ProfileDigest: facts.Digest}, Bootstrap: true, Deadline: facts.NotAfter}
	batch := holder(holderCommand{Operation: "begin", Intent: intent})
	if batch.Outcome != "completed" {
		t.Fatal("holder begin", batch.Outcome)
	}
	issued := admissionLocalCommand(t, binary, "issue-current", map[string]any{"plan": issuerPlan, "network": seed(), "batch": batch.Request, "kind": quota.Bootstrap})
	if issued.Outcome != "issued-offline" {
		t.Fatal("issuer response", issued.Outcome)
	}
	if reply := holder(holderCommand{Operation: "complete", Payload: issued.Response}); reply.Outcome != "completed" {
		t.Fatal("holder complete", reply.Outcome)
	}
	if reply := holder(holderCommand{Operation: "prefix-open"}); reply.Outcome == "completed" {
		t.Fatal("QUIC-only selection opened a Prefix")
	}
	for i := byte(12); i < 16; i++ {
		presentation := stock.Presentation{NetworkID: facts.NetworkID, StateGeneration: facts.StateGeneration, StateDigest: facts.StateDigest,
			ProfileDigest: facts.Digest, RecipientNodeID: [32]byte{i}, RecipientDutyGeneration: 9, ChannelNonce: [32]byte{i}, Deadline: deadline}
		reply := holder(holderCommand{Operation: "take", Presentation: presentation, Class: 2})
		if reply.Outcome != "completed" || len(reply.Token) == 0 {
			t.Fatal("QUIC refusal consumed a candidate token", i, reply.Outcome)
		}
		clear(reply.Token)
	}
	closeHolder()
	if err := tcp.(*net.TCPListener).SetDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	conn, err := tcp.Accept()
	if conn != nil {
		_ = conn.Close()
		t.Fatal("QUIC refusal attempted TCP fallback")
	}
	var timeout net.Error
	if !errors.As(err, &timeout) || !timeout.Timeout() {
		t.Fatal("TCP sentinel", err)
	}
	if err := udp.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := udp.ReadFrom(make([]byte, 1)); err == nil {
		t.Fatal("QUIC refusal emitted UDP")
	} else if !errors.As(err, &timeout) || !timeout.Timeout() {
		t.Fatal("UDP sentinel", err)
	}
	owner, err = state.OpenRetained(config)
	if err != nil {
		t.Fatal(err)
	}
	after, err := owner.CurrentRuntime()
	if err == nil {
		err = after.MatchDuty(duty, after.ObservedAt())
	}
	closeErr = owner.Close()
	if err != nil || closeErr != nil {
		t.Fatal("QUIC refusal changed Network duty", err, closeErr)
	}
}
