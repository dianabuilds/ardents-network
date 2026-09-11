package source

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

// TestSourceObservationBindsTransportRequestToPublicPlanFacts exercises the
// real mutually authenticated Source transport. Its oracle deliberately keeps
// only public plan facts and the bounded request class; it does not retain the
// request payload or TLS key material.
func TestSourceObservationBindsTransportRequestToPublicPlanFacts(t *testing.T) {
	now := time.Date(2032, time.February, 3, 4, 5, 6, 0, time.UTC)
	fixture := newSourceTLSFixture(t, now)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	clock := func() time.Time { return now }
	server, _, err := New(Config{ServeAddress: address, ServeCertificate: fixture.server,
		ServeClientRootPEM: fixture.authority.rootPEM, ServeClientKeyDigests: [][32]byte{fixture.clientPin}, VerificationClock: clock}, nil)
	if err != nil {
		t.Fatal(err)
	}
	identity := [32]byte{7}
	client, details, err := New(Config{Sources: [2]Source{
		{Address: address, ServerName: fixture.serverName, Identity: identity, Family: "observed-source-family", EndpointHandle: "observed-source", RootPEM: fixture.authority.rootPEM, LeafKeyDigest: fixture.serverPin},
		{Address: "127.0.0.1:9", ServerName: "unused-source.test", Identity: [32]byte{8}, Family: "unused-source-family", EndpointHandle: "unused-source", RootPEM: fixture.authority.rootPEM, LeafKeyDigest: [32]byte{9}},
	}, ClientCertificate: fixture.client, MaterialIndex: 3, VerificationClock: clock}, nil)
	if err != nil {
		t.Fatal(err)
	}
	type observation struct {
		identity  [32]byte
		family    string
		operation string
		material  uint32
		status    string
	}
	observed := make(chan observation, 1)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	ready, served := make(chan error, 1), make(chan error, 1)
	go func() {
		served <- server.Serve(ctx, ready, nil, nil, func(_ context.Context, request Message) Message {
			observed <- observation{identity: details.Identities[0], family: details.Families[0], operation: request.Operation, material: request.MaterialIndex, status: "not-found"}
			return Message{Status: "not-found"}
		})
	}()
	if err := <-ready; err != nil {
		t.Fatal(err)
	}
	response, err := client.Fetch(ctx, 0, Message{Operation: "latest", NetworkDigest: [32]byte{1}, MaterialIndex: 3})
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != "not-found" {
		t.Fatalf("response status = %q", response.Status)
	}
	select {
	case got := <-observed:
		if got.identity != identity || got.family != "observed-source-family" || got.operation != "latest" || got.material != 3 || got.status != "not-found" {
			t.Fatalf("observation = %#v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("Source resolver observation did not arrive")
	}
	cancel()
	if err := <-served; !errors.Is(err, context.Canceled) {
		t.Fatalf("Source server stop = %v", err)
	}
}
