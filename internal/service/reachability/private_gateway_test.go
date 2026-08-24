package reachability_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

func TestPrivateLookupPassesOnlyThroughRelayAndGateway(t *testing.T) {
	t.Parallel()
	now := time.Unix(2_000_300_000, 0).UTC()
	network, node := [32]byte{41}, [32]byte{42}
	identityPublic, identityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var gatewayPublic [32]byte
	copy(gatewayPublic[:], identityPublic)
	store, err := reachability.OpenStore(reachability.StoreConfig{Root: t.TempDir(), NetworkID: network})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	gateway, err := reachability.NewGateway(reachability.GatewayConfig{NetworkID: network, NodeID: node, IdentityKey: identityPrivate,
		AssignmentNotAfter: now.Add(time.Minute), Store: store, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	gatewayServer := httptest.NewTLSServer(gateway.Handler())
	defer gatewayServer.Close()
	relay, err := reachability.NewRelay(gatewayServer.URL, gatewayServer.Client())
	if err != nil {
		t.Fatal(err)
	}
	relayServer := httptest.NewTLSServer(relay.Handler())
	defer relayServer.Close()
	base, ok := relayServer.Client().Transport.(*http.Transport)
	if !ok {
		t.Fatal("TLS relay client does not expose HTTP transport")
	}
	client, err := reachability.OpenClient(reachability.ClientConfig{NetworkID: network, GatewayPublic: gatewayPublic,
		Profile: gateway.Profile(), RelayURL: relayServer.URL + "/ohttp", BaseTransport: base, At: now, Deadline: now.Add(5 * time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	if descriptor, class, resolveErr := client.Resolve(context.Background(), [32]byte{43}); resolveErr == nil || class != reachability.StoreStale || len(descriptor) != 0 {
		t.Fatalf("Resolve = %x, %q, %v", descriptor, class, resolveErr)
	}
}

func TestPrivateLookupReturnsCurrentSignedDescriptor(t *testing.T) {
	t.Parallel()
	fixture := newStoreFixture(t)
	store, err := reachability.OpenStore(reachability.StoreConfig{Root: t.TempDir(), NetworkID: fixture.network})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	raw := fixture.issue(t, fixture.current, fixture.now.Add(30*time.Second), "OHTTP")
	if result, err := store.Publish(raw, fixture.now); err != nil || result.Class != reachability.StoreAccepted {
		t.Fatalf("Store Publish = %+v, %v", result, err)
	}
	identityPublic, identityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var gatewayPublic, node [32]byte
	copy(gatewayPublic[:], identityPublic)
	node[0] = 44
	gateway, err := reachability.NewGateway(reachability.GatewayConfig{NetworkID: fixture.network, NodeID: node,
		IdentityKey: identityPrivate, AssignmentNotAfter: fixture.now.Add(time.Minute), Store: store, Clock: func() time.Time { return fixture.now }})
	if err != nil {
		t.Fatal(err)
	}
	gatewayServer := httptest.NewTLSServer(gateway.Handler())
	defer gatewayServer.Close()
	relay, err := reachability.NewRelay(gatewayServer.URL, gatewayServer.Client())
	if err != nil {
		t.Fatal(err)
	}
	relayServer := httptest.NewTLSServer(relay.Handler())
	defer relayServer.Close()
	base := relayServer.Client().Transport.(*http.Transport)
	client, err := reachability.OpenClient(reachability.ClientConfig{NetworkID: fixture.network, GatewayPublic: gatewayPublic,
		Profile: gateway.Profile(), RelayURL: relayServer.URL + "/ohttp", BaseTransport: base, At: fixture.now, Deadline: fixture.now.Add(5 * time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	descriptor, class, err := client.Resolve(context.Background(), fixture.current.Credential.Target)
	if err != nil || class != reachability.StoreAlreadyCurrent || string(descriptor) != string(raw) {
		t.Fatalf("Resolve = %x, %q, %v", descriptor, class, err)
	}
}

func TestPrivateLookupClientUsesOnlyItsOpaqueExchangePort(t *testing.T) {
	t.Parallel()
	fixture := newStoreFixture(t)
	store, err := reachability.OpenStore(reachability.StoreConfig{Root: t.TempDir(), NetworkID: fixture.network})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	raw := fixture.issue(t, fixture.current, fixture.now.Add(30*time.Second), "carrier")
	if result, err := store.Publish(raw, fixture.now); err != nil || result.Class != reachability.StoreAccepted {
		t.Fatalf("Store Publish = %+v, %v", result, err)
	}
	identityPublic, identityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var gatewayPublic, node [32]byte
	copy(gatewayPublic[:], identityPublic)
	node[0] = 47
	gateway, err := reachability.NewGateway(reachability.GatewayConfig{NetworkID: fixture.network, NodeID: node,
		IdentityKey: identityPrivate, AssignmentNotAfter: fixture.now.Add(time.Minute), Store: store, Clock: func() time.Time { return fixture.now }})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(gateway.Handler())
	defer server.Close()
	client, err := reachability.OpenClient(reachability.ClientConfig{NetworkID: fixture.network, GatewayPublic: gatewayPublic,
		Profile: gateway.Profile(), At: fixture.now, Deadline: fixture.now.Add(5 * time.Second),
		Exchange: func(ctx context.Context, envelope []byte) (reachability.OHTTPResponse, error) {
			return reachability.ForwardOHTTP(ctx, server.URL, server.Client(), envelope)
		}})
	if err != nil {
		t.Fatal(err)
	}
	descriptor, class, err := client.Resolve(context.Background(), fixture.current.Credential.Target)
	if err != nil || class != reachability.StoreAlreadyCurrent || string(descriptor) != string(raw) {
		t.Fatalf("opaque Exchange Resolve = %x, %q, %v", descriptor, class, err)
	}
}
