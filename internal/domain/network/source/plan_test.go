package source

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"net"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewRejectsPartiallyDeclaredHalves(t *testing.T) {
	cases := []struct {
		name   string
		config Config
	}{
		{"source identity", Config{Sources: [2]Source{{Identity: [32]byte{1}}}}},
		{"source family", Config{Sources: [2]Source{{Family: "declared"}}}},
		{"source server name", Config{Sources: [2]Source{{ServerName: "source.test"}}}},
		{"source endpoint handle", Config{Sources: [2]Source{{EndpointHandle: "declared"}}}},
		{"source leaf pin", Config{Sources: [2]Source{{LeafKeyDigest: [32]byte{1}}}}},
		{"second source trust root", Config{Sources: [2]Source{{}, {RootPEM: []byte("declared")}}}},
		{"client certificate", Config{ClientCertificate: tls.Certificate{Certificate: [][]byte{{1}}}}},
		{"source order seed", Config{OrderSeed: [32]byte{1}}},
		{"server certificate", Config{ServeCertificate: tls.Certificate{Certificate: [][]byte{{1}}}}},
		{"server client root", Config{ServeClientRootPEM: []byte("declared")}},
		{"server client pin", Config{ServeClientKeyDigests: [][32]byte{{1}}}},
		{"server header timeout", Config{ServeHeaderTimeout: time.Second}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if _, _, err := New(test.config, nil); err == nil {
				t.Fatal("incomplete declared Source half was silently treated as absent")
			}
		})
	}
	if _, details, err := New(Config{MaterialIndex: 3, VerificationClock: time.Now}, nil); err != nil || details.Configured || details.Serving {
		t.Fatalf("empty Source halves = %+v, %v", details, err)
	}
}

func TestSourceAddressRequiresUsableTCPPort(t *testing.T) {
	for _, address := range []string{"127.0.0.1:service", "127.0.0.1:65536", "[::1]:-1", "127.0.0.1:+80", "127.0.0.1:80x", "127.0.0.1:0", "[::1]:00000"} {
		if _, _, err := New(Config{ServeAddress: address, VerificationClock: time.Now}, nil); err == nil || !strings.Contains(err.Error(), "source address") {
			t.Errorf("Source plan accepted invalid TCP address %q: %v", address, err)
		}
	}
	for _, address := range []string{"127.0.0.1:1", "[::1]:65535"} {
		if err := validateAddress(address); err != nil {
			t.Errorf("valid TCP address %q: %v", address, err)
		}
	}
}

// A real authenticated request keeps its copied key until the request has
// joined. Closure denies new work and clears only buffers owned by the plan.
func TestPlanCloseJoinsTLSBorrowersBeforeClearingKeys(t *testing.T) {
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
	config := Config{Sources: [2]Source{
		{Address: address, ServerName: fixture.serverName, Identity: [32]byte{1}, Family: "one", EndpointHandle: "one", RootPEM: fixture.authority.rootPEM, LeafKeyDigest: fixture.serverPin},
		{Address: "127.0.0.1:9", ServerName: "unused.test", Identity: [32]byte{2}, Family: "two", EndpointHandle: "two", RootPEM: fixture.authority.rootPEM, LeafKeyDigest: [32]byte{3}},
	}, ClientCertificate: fixture.client, VerificationClock: clock}
	client, _, err := New(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	clientKey := client.clients[0].certificate.PrivateKey.(ed25519.PrivateKey)
	serverKey := server.server.certificate.PrivateKey.(ed25519.PrivateKey)
	originalClient := bytes.Clone(fixture.client.PrivateKey.(ed25519.PrivateKey))
	originalServer := bytes.Clone(fixture.server.PrivateKey.(ed25519.PrivateKey))
	ctx, cancel := context.WithCancel(t.Context())
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(func() { cancel(); unblock() })
	ready, served := make(chan error, 1), make(chan error, 1)
	go func() {
		served <- server.Serve(ctx, ready, nil, nil, func(context.Context, Message) Message {
			close(entered)
			<-release
			return Message{Status: "not-found"}
		})
	}()
	if err := <-ready; err != nil {
		t.Fatal(err)
	}
	fetched := make(chan error, 1)
	go func() { _, err := client.Fetch(ctx, 0, Message{Operation: "latest"}); fetched <- err }()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("authenticated Source request did not reach its handler")
	}
	closed := make(chan struct{})
	go func() { client.Close(); close(closed) }()
	deadline := time.After(3 * time.Second)
	for {
		client.mu.Lock()
		closing := client.closed
		client.mu.Unlock()
		if closing {
			break
		}
		select {
		case <-deadline:
			t.Fatal("plan did not deny new borrowers")
		default:
			runtime.Gosched()
		}
	}
	select {
	case <-closed:
		t.Fatal("plan closed while its original TLS request was held")
	default:
	}
	if !bytes.Equal(clientKey, originalClient) {
		t.Fatal("plan cleared a key still borrowed by TLS")
	}
	if _, err := client.Fetch(ctx, 0, Message{Operation: "latest"}); err == nil {
		t.Fatal("closed plan admitted another request")
	}
	serverClosed := make(chan struct{})
	go func() { server.Close(); close(serverClosed) }()
	deadline = time.After(3 * time.Second)
	for {
		server.mu.Lock()
		closing := server.closed
		server.mu.Unlock()
		if closing {
			break
		}
		select {
		case <-deadline:
			t.Fatal("serving plan did not begin closure")
		default:
			runtime.Gosched()
		}
	}
	select {
	case <-serverClosed:
		t.Fatal("serving plan closed before its listener joined")
	default:
	}
	if !bytes.Equal(serverKey, originalServer) {
		t.Fatal("plan cleared a key still borrowed by the serving listener")
	}
	unblock()
	if err := <-fetched; err != nil {
		t.Fatal("original authenticated request failed", err)
	}
	<-closed
	if !bytes.Equal(clientKey, make([]byte, len(clientKey))) || !bytes.Equal(fixture.client.PrivateKey.(ed25519.PrivateKey), originalClient) {
		t.Fatal("closing did not clear the owned client copy or changed the caller's key")
	}
	cancel()
	if err := <-served; !errors.Is(err, context.Canceled) {
		t.Fatal("server did not join on cancellation", err)
	}
	<-serverClosed
	if !bytes.Equal(serverKey, make([]byte, len(serverKey))) || !bytes.Equal(fixture.server.PrivateKey.(ed25519.PrivateKey), originalServer) {
		t.Fatal("closing did not clear the owned server copy or changed the caller's key")
	}
}

func TestFailedPlanConstructionPreservesCallerKey(t *testing.T) {
	now := time.Date(2032, time.February, 3, 4, 5, 6, 0, time.UTC)
	fixture := newSourceTLSFixture(t, now)
	original := bytes.Clone(fixture.client.PrivateKey.(ed25519.PrivateKey))
	declared := Source{Address: "127.0.0.1:12345", ServerName: fixture.serverName, Identity: [32]byte{1}, Family: "one", EndpointHandle: "one", RootPEM: fixture.authority.rootPEM, LeafKeyDigest: fixture.serverPin}
	config := Config{Sources: [2]Source{declared, declared}, ClientCertificate: fixture.client, VerificationClock: func() time.Time { return now }}
	if _, _, err := New(config, nil); err == nil {
		t.Fatal("duplicate source declaration accepted")
	}
	if !bytes.Equal(fixture.client.PrivateKey.(ed25519.PrivateKey), original) {
		t.Fatal("failed constructor cleared its caller's key")
	}
}
