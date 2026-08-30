package endpoint_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
	alphaprivate "github.com/dianabuilds/ardents-network/internal/naming/alpha/private"
)

// c2PrivateAlphaBinding resolves the C2 alpha name through the maintained
// alpha OHTTP roles and retains the response before the C2 test uses it.
func c2PrivateAlphaBinding(t *testing.T, network, target [32]byte, now time.Time) alpha.Binding {
	t.Helper()
	authorityPublic, authorityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	link, err := alpha.ParseServiceLink("ardents-alpha://reference")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := alpha.IssueCorpus(alpha.CorpusInput{Cohort: "c2-test", Network: network, Serial: 1,
		NotBefore: now.Add(-time.Second), NotAfter: now.Add(time.Minute), Bindings: []alpha.BindingInput{{Link: link, Target: target}}}, authorityPrivate)
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
	gateway, err := alphaprivate.NewGateway(alphaprivate.GatewayConfig{Corpus: corpus, NodeID: c2Identifier(110), Family: "alpha-gateway",
		AssignmentNotAfter: now.Add(time.Minute), IdentityKey: gatewayPrivate, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	gatewayServer := httptest.NewTLSServer(gateway.Handler())
	t.Cleanup(gatewayServer.Close)
	relay, err := alphaprivate.NewRelay(gatewayServer.URL, gatewayServer.Client())
	if err != nil {
		t.Fatal(err)
	}
	relayServer := httptest.NewTLSServer(relay.Handler())
	t.Cleanup(relayServer.Close)
	floorRoot := t.TempDir()
	if err := os.Chmod(floorRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	floor, err := alpha.OpenPersistentFloor(alpha.PersistentFloorConfig{Root: floorRoot, Authority: authorityPublic, Cohort: "c2-test", Network: network})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = floor.Close() })
	client, err := alphaprivate.Open(alphaprivate.ClientConfig{RelayURL: relayServer.URL, RelayNodeID: c2Identifier(111), RelayFamily: "alpha-relay",
		GatewayPublic: gatewayPublic, Gateway: gateway.Profile(), AuthorityPublic: authorityPublic, Cohort: "c2-test", Network: network,
		Floor: floor, Base: relayServer.Client().Transport.(*http.Transport).Clone()}, now)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := client.Resolve(t.Context(), link, now)
	if err != nil {
		t.Fatal(err)
	}
	retained, err := floor.Current()
	if err != nil || retained.Serial() != corpus.Serial() || retained.Digest() != corpus.Digest() {
		t.Fatalf("C2 alpha persistent floor = (%v, %v), want serial %d", retained, err, corpus.Serial())
	}
	return binding
}
