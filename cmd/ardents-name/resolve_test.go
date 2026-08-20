package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/nameauthority"
	"github.com/dianabuilds/ardents-network/internal/namelease"
	"github.com/dianabuilds/ardents-network/internal/nameresolution"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func TestResolveCommandRunsPrivateResolution(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_800_000_000, 0).UTC()
	network := [32]byte{9}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	record := namelease.Record{Name: "alice", Generation: 1, Revision: 1,
		Lease: "active", Consistency: "current", Recovery: "stable",
		Authority: hex.EncodeToString(public), Target: "target-a",
		LeaseExpiresAt: now.Add(time.Hour).Unix(), GraceExpiresAt: now.Add(2 * time.Hour).Unix()}
	signed, err := nameauthority.SignRecord(network, record, private)
	if err != nil {
		t.Fatal(err)
	}
	gatewayPublic, gatewayPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	gateway, err := nameresolution.NewGateway(nameresolution.GatewayConfig{NetworkID: network,
		NodeID: [32]byte{2}, Family: "gateway-family", Domain: "rendezvous",
		AssignmentNotAfter: now.Add(time.Minute), MaximumPending: 8,
		SignedRecordChains: [][][]byte{{signed}}, IdentityKey: gatewayPrivate,
		Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	gatewayServer := httptest.NewUnstartedServer(gateway.Handler())
	gatewayServer.StartTLS()
	t.Cleanup(gatewayServer.Close)
	relay, err := nameresolution.NewRelay(gatewayServer.URL, gatewayServer.Client())
	if err != nil {
		t.Fatal(err)
	}
	relayServer := httptest.NewUnstartedServer(relay.Handler())
	relayServer.StartTLS()
	t.Cleanup(relayServer.Close)

	view := commandResolutionView(t, network, now, relayServer.URL, gatewayServer.URL, gatewayPublic)
	input := resolutionInput{Schema: resolutionInputSchema, StateRoot: t.TempDir(),
		NetworkID: hex.EncodeToString(network[:]), AuthorityPublic: []string{hex.EncodeToString(public)},
		AuthorityThreshold: 1, AcceptedProfile: "h3-route-tracer-v1",
		SelectionAt: now.Format(time.RFC3339Nano), Deadline: now.Add(15 * time.Second).Format(time.RFC3339Nano),
		RelayNodeID: fixedNodeID(1), GatewayNodeID: fixedNodeID(2),
		ConnectionRendezvousNodeID: fixedNodeID(3), GatewayProfile: gateway.Profile()}
	planRaw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(t.TempDir(), "resolution.json")
	if err := os.WriteFile(planPath, planRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	contextHex := hex.EncodeToString(append([]byte{1}, make([]byte, 31)...))
	var output bytes.Buffer
	load := func(state.Config) (state.Snapshot, error) { return view, nil }
	transport := relayServer.Client().Transport.(*http.Transport)
	if err := runWithRuntime([]string{"resolve", planPath, "alice", contextHex}, &output, transport, load); err != nil {
		t.Fatalf("resolve command: %v", err)
	}
	if output.String() != "resolved target-a\n" {
		t.Fatalf("output=%q", output.String())
	}
	input.StateRoot = t.TempDir()
	planRaw, err = json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(planPath, planRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runWithTransport([]string{"resolve", planPath, "alice", contextHex}, &bytes.Buffer{}, transport); err == nil {
		t.Fatal("resolve accepted an input without recovered authenticated Network State")
	}
}

func fixedNodeID(marker byte) string {
	return hex.EncodeToString(append([]byte{marker}, make([]byte, 31)...))
}

func commandResolutionView(t *testing.T, network [32]byte, now time.Time, relayURL, gatewayURL string,
	gatewayPublic ed25519.PublicKey) state.Snapshot {
	t.Helper()
	view := state.Snapshot{Generation: "generation-a", NetworkID: network, Epoch: 1,
		Digest: [32]byte{1}, ValidUntil: now.Add(time.Hour), Profile: "h3-route-tracer-v1",
		ViewRoot: [32]byte{2}, Freshness: "fresh", CandidateCount: 3}
	values := []struct {
		id                       byte
		family, domain, endpoint string
	}{
		{1, "relay-family", "initiator", commandEndpoint(t, relayURL)},
		{2, "gateway-family", "rendezvous", commandEndpoint(t, gatewayURL)},
		{3, "connection-family", "rendezvous", "127.0.0.1:7443"},
	}
	for index, value := range values {
		view.Candidates[index].NodeID = [32]byte{value.id}
		view.Candidates[index].PublicKey = [32]byte{value.id, 1}
		view.Candidates[index].KeyID = [32]byte{value.id, 2}
		view.Candidates[index].Family = value.family
		view.Candidates[index].Endpoint = value.endpoint
		view.Candidates[index].Capacity = 1
		view.Candidates[index].Domain = value.domain
		view.Candidates[index].ValidFrom = now.Add(-time.Minute)
		view.Candidates[index].ValidUntil = now.Add(time.Hour)
		view.Candidates[index].AssignmentNotAfter = now.Add(time.Minute)
	}
	copy(view.Candidates[1].PublicKey[:], gatewayPublic)
	return view
}

func commandEndpoint(t *testing.T, raw string) string {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return parsed.Host
}
