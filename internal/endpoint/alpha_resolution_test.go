package endpoint

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
	alphaprivate "github.com/dianabuilds/ardents-network/internal/naming/alpha/private"
)

func TestEndpointResolveAlphaBindsTheVerifiedPrivateResultToItsNetwork(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	network := targetLinkBytes(1)
	authorityPublic, authorityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	link, err := alpha.ParseServiceLink("ardents-alpha://blog.alice")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := alpha.IssueCorpus(alpha.CorpusInput{Cohort: "closed-alpha-1", Network: network, Serial: 1,
		Bindings:  []alpha.BindingInput{{Link: link, Target: targetLinkBytes(33)}},
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour)}, authorityPrivate)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := alpha.OpenCorpus(authorityPublic, raw)
	if err != nil {
		t.Fatal(err)
	}
	gatewayPublic, gatewayPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	gateway, err := alphaprivate.NewGateway(alphaprivate.GatewayConfig{Corpus: corpus, NodeID: [32]byte{2}, Family: "gateway-a",
		AssignmentNotAfter: now.Add(time.Hour), IdentityKey: gatewayPrivate, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	gatewayServer := httptest.NewTLSServer(gateway.Handler())
	defer gatewayServer.Close()
	relay, err := alphaprivate.NewRelay(gatewayServer.URL, gatewayServer.Client())
	if err != nil {
		t.Fatal(err)
	}
	relayServer := httptest.NewTLSServer(relay.Handler())
	defer relayServer.Close()
	floor, err := alpha.NewSessionFloor("closed-alpha-1")
	if err != nil {
		t.Fatal(err)
	}
	resolver, err := alphaprivate.Open(alphaprivate.ClientConfig{RelayURL: relayServer.URL, RelayNodeID: [32]byte{3}, RelayFamily: "relay-b",
		GatewayPublic: gatewayPublic, Gateway: gateway.Profile(), AuthorityPublic: authorityPublic,
		Cohort: "closed-alpha-1", Network: network, Floor: floor,
		Base: relayServer.Client().Transport.(*http.Transport).Clone()}, now)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := (&endpoint{network: network}).ResolveAlpha(t.Context(), resolver, link.String(), now)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Network() != network || resolved.Target() != targetLinkBytes(33) {
		t.Fatalf("Endpoint alpha result = %+v", resolved)
	}
}

func TestEndpointResolveAcceptedAlphaUsesOnlyItsPersistentFloor(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	network := targetLinkBytes(1)
	authorityPublic, authorityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	link, err := alpha.ParseServiceLink("ardents-alpha://blog.alice")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := alpha.IssueCorpus(alpha.CorpusInput{Cohort: "closed-alpha-1", Network: network, Serial: 1,
		Bindings:  []alpha.BindingInput{{Link: link, Target: targetLinkBytes(33)}},
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour)}, authorityPrivate)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := alpha.OpenCorpus(authorityPublic, raw)
	if err != nil {
		t.Fatal(err)
	}
	floor, err := alpha.OpenPersistentFloor(alpha.PersistentFloorConfig{Root: t.TempDir(), Authority: authorityPublic,
		Cohort: "closed-alpha-1", Network: network})
	if err != nil {
		t.Fatal(err)
	}
	defer floor.Close()
	endpoint := &endpoint{network: network}
	if _, err := endpoint.ResolveAcceptedAlpha(floor, link.String(), now); err == nil {
		t.Fatal("ResolveAcceptedAlpha accepted an empty floor")
	}
	if err := floor.Observe(corpus); err != nil {
		t.Fatal(err)
	}
	resolved, err := endpoint.ResolveAcceptedAlpha(floor, link.String(), now)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Network() != network || resolved.Target() != targetLinkBytes(33) {
		t.Fatalf("Endpoint accepted alpha result = %+v", resolved)
	}
	wrongNetwork := targetLinkBytes(2)
	wrongFloor, err := alpha.OpenPersistentFloor(alpha.PersistentFloorConfig{Root: t.TempDir(), Authority: authorityPublic,
		Cohort: "closed-alpha-1", Network: wrongNetwork})
	if err != nil {
		t.Fatal(err)
	}
	defer wrongFloor.Close()
	wrongRaw, err := alpha.IssueCorpus(alpha.CorpusInput{Cohort: "closed-alpha-1", Network: wrongNetwork, Serial: 1,
		Bindings:  []alpha.BindingInput{{Link: link, Target: targetLinkBytes(34)}},
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour)}, authorityPrivate)
	if err != nil {
		t.Fatal(err)
	}
	wrongCorpus, err := alpha.OpenCorpus(authorityPublic, wrongRaw)
	if err != nil {
		t.Fatal(err)
	}
	if err := wrongFloor.Observe(wrongCorpus); err != nil {
		t.Fatal(err)
	}
	if _, err := endpoint.ResolveAcceptedAlpha(wrongFloor, link.String(), now); !errors.Is(err, ErrAlphaBindingNetwork) {
		t.Fatalf("ResolveAcceptedAlpha wrong network error = %v, want %v", err, ErrAlphaBindingNetwork)
	}
}
