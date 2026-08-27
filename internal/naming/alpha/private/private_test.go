package private

import (
	"crypto/ed25519"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
)

func TestPrivateAlphaResolutionUsesSeparateRelayAndGatewayRoles(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	authorityPublic, authorityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	link, err := alpha.ParseServiceLink("ardents-alpha://blog.alice")
	if err != nil {
		t.Fatal(err)
	}
	network := [32]byte{1}
	raw, err := alpha.IssueCorpus(alpha.CorpusInput{Cohort: "closed-alpha-1", Network: network, Serial: 1,
		Bindings:  []alpha.BindingInput{{Link: link, Target: [32]byte{9}}},
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
	gateway, err := NewGateway(GatewayConfig{Corpus: corpus, NodeID: [32]byte{2}, Family: "gateway-a",
		AssignmentNotAfter: now.Add(time.Hour), IdentityKey: gatewayPrivate, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	gatewayRequests := 0
	gatewayHandler := gateway.Handler()
	gatewayServer := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		gatewayRequests++
		gatewayHandler.ServeHTTP(writer, request)
	}))
	defer gatewayServer.Close()
	gatewayHTTP := gatewayServer.Client()
	relay, err := NewRelay(gatewayServer.URL, gatewayHTTP)
	if err != nil {
		t.Fatal(err)
	}
	relayOrigin := ""
	relayHandler := relay.Handler()
	relayServer := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		relayOrigin = request.RemoteAddr
		relayHandler.ServeHTTP(writer, request)
	}))
	defer relayServer.Close()
	relayTransport := relayServer.Client().Transport.(*http.Transport).Clone()
	floor, err := alpha.OpenPersistentFloor(alpha.PersistentFloorConfig{Root: persistentFloorTestRoot(t), Authority: authorityPublic,
		Cohort: "closed-alpha-1", Network: network})
	if err != nil {
		t.Fatal(err)
	}
	defer floor.Close()
	client, err := Open(ClientConfig{RelayURL: relayServer.URL, RelayNodeID: [32]byte{3}, RelayFamily: "relay-b",
		GatewayPublic: gatewayPublic, Gateway: gateway.Profile(), AuthorityPublic: authorityPublic,
		Cohort: "closed-alpha-1", Network: network, Floor: floor, Base: relayTransport}, now)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := client.Resolve(t.Context(), link, now)
	if err != nil {
		t.Fatal(err)
	}
	if binding.Target() != [32]byte{9} || binding.Network() != network {
		t.Fatalf("private alpha binding = %+v", binding)
	}
	if relayOrigin == "" || gatewayRequests != 1 {
		t.Fatalf("separate OHTTP role observations: relay origin=%q gateway requests=%d", relayOrigin, gatewayRequests)
	}
}

func TestPrivateAlphaResolutionRetainsOneBindingInSeparatePersistentFloors(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	authorityPublic, authorityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	link, err := alpha.ParseServiceLink("ardents-alpha://blog.alice")
	if err != nil {
		t.Fatal(err)
	}
	network, target := [32]byte{1}, [32]byte{9}
	raw, err := alpha.IssueCorpus(alpha.CorpusInput{Cohort: "closed-alpha-1", Network: network, Serial: 1,
		Bindings:  []alpha.BindingInput{{Link: link, Target: target}},
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
	gateway, err := NewGateway(GatewayConfig{Corpus: corpus, NodeID: [32]byte{2}, Family: "gateway-a",
		AssignmentNotAfter: now.Add(time.Hour), IdentityKey: gatewayPrivate, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	gatewayServer := httptest.NewTLSServer(gateway.Handler())
	defer gatewayServer.Close()
	relay, err := NewRelay(gatewayServer.URL, gatewayServer.Client())
	if err != nil {
		t.Fatal(err)
	}
	relayServer := httptest.NewTLSServer(relay.Handler())
	defer relayServer.Close()
	openFloor := func() *alpha.PersistentFloor {
		t.Helper()
		floor, openErr := alpha.OpenPersistentFloor(alpha.PersistentFloorConfig{Root: persistentFloorTestRoot(t), Authority: authorityPublic,
			Cohort: "closed-alpha-1", Network: network})
		if openErr != nil {
			t.Fatal(openErr)
		}
		return floor
	}
	openClient := func(floor *alpha.PersistentFloor) *Client {
		t.Helper()
		client, openErr := Open(ClientConfig{RelayURL: relayServer.URL, RelayNodeID: [32]byte{3}, RelayFamily: "relay-b",
			GatewayPublic: gatewayPublic, Gateway: gateway.Profile(), AuthorityPublic: authorityPublic,
			Cohort: "closed-alpha-1", Network: network, Floor: floor,
			Base: relayServer.Client().Transport.(*http.Transport).Clone()}, now)
		if openErr != nil {
			t.Fatal(openErr)
		}
		return client
	}
	first, second := openFloor(), openFloor()
	defer first.Close()
	defer second.Close()
	firstBinding, err := openClient(first).Resolve(t.Context(), link, now)
	if err != nil {
		t.Fatal(err)
	}
	secondBinding, err := openClient(second).Resolve(t.Context(), link, now)
	if err != nil {
		t.Fatal(err)
	}
	if firstBinding.Target() != target || secondBinding.Target() != target || firstBinding.Network() != network || secondBinding.Network() != network {
		t.Fatalf("independent alpha bindings = (%+v, %+v), want target %x on network %x", firstBinding, secondBinding, target, network)
	}
	for index, floor := range []*alpha.PersistentFloor{first, second} {
		retained, currentErr := floor.Current()
		if currentErr != nil || retained.Serial() != corpus.Serial() || retained.Digest() != corpus.Digest() {
			t.Fatalf("persistent floor %d retained = (%v, %v), want corpus serial %d", index, retained, currentErr, corpus.Serial())
		}
	}
}

func TestPrivateAlphaResolutionRejectsStaleAndConflictingSignedCorpus(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	authorityPublic, authorityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	link, err := alpha.ParseServiceLink("ardents-alpha://blog.alice")
	if err != nil {
		t.Fatal(err)
	}
	network := [32]byte{1}
	issue := func(serial uint64, target byte) *alpha.Corpus {
		t.Helper()
		raw, issueErr := alpha.IssueCorpus(alpha.CorpusInput{Cohort: "closed-alpha-1", Network: network, Serial: serial,
			Bindings:  []alpha.BindingInput{{Link: link, Target: [32]byte{target}}},
			NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour)}, authorityPrivate)
		if issueErr != nil {
			t.Fatal(issueErr)
		}
		corpus, openErr := alpha.OpenCorpus(authorityPublic, raw)
		if openErr != nil {
			t.Fatal(openErr)
		}
		return corpus
	}
	current, stale, conflict := issue(2, 9), issue(1, 9), issue(2, 10)
	gatewayPublic, gatewayPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	gateway, err := NewGateway(GatewayConfig{Corpus: current, NodeID: [32]byte{2}, Family: "gateway-a",
		AssignmentNotAfter: now.Add(time.Hour), IdentityKey: gatewayPrivate, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	gatewayServer := httptest.NewTLSServer(gateway.Handler())
	defer gatewayServer.Close()
	relay, err := NewRelay(gatewayServer.URL, gatewayServer.Client())
	if err != nil {
		t.Fatal(err)
	}
	relayServer := httptest.NewTLSServer(relay.Handler())
	defer relayServer.Close()
	floor, err := alpha.NewSessionFloor("closed-alpha-1")
	if err != nil {
		t.Fatal(err)
	}
	newClient := func() *Client {
		t.Helper()
		opened, openErr := Open(ClientConfig{RelayURL: relayServer.URL, RelayNodeID: [32]byte{3}, RelayFamily: "relay-b",
			GatewayPublic: gatewayPublic, Gateway: gateway.Profile(), AuthorityPublic: authorityPublic,
			Cohort: "closed-alpha-1", Network: network, Floor: floor,
			Base: relayServer.Client().Transport.(*http.Transport).Clone()}, now)
		if openErr != nil {
			t.Fatal(openErr)
		}
		return opened
	}
	if _, err := newClient().Resolve(t.Context(), link, now); err != nil {
		t.Fatal(err)
	}
	gateway.config.Corpus = stale
	if _, err := newClient().Resolve(t.Context(), link, now); !alpha.HasFailure(err, alpha.FailureStale) {
		t.Fatalf("stale alpha private response result = %v, want stale failure", err)
	}
	gateway.config.Corpus = conflict
	if _, err := newClient().Resolve(t.Context(), link, now); !alpha.HasFailure(err, alpha.FailureConflict) {
		t.Fatalf("conflicting alpha private response result = %v, want conflict failure", err)
	}
}

func TestPrivateAlphaResolutionDoesNotAdvancePersistentFloorOutsideCorpusValidity(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	authorityPublic, authorityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	link, err := alpha.ParseServiceLink("ardents-alpha://blog.alice")
	if err != nil {
		t.Fatal(err)
	}
	network := [32]byte{1}
	issue := func(serial uint64, target byte, notBefore, notAfter time.Time) *alpha.Corpus {
		t.Helper()
		raw, issueErr := alpha.IssueCorpus(alpha.CorpusInput{Cohort: "closed-alpha-1", Network: network, Serial: serial,
			Bindings: []alpha.BindingInput{{Link: link, Target: [32]byte{target}}}, NotBefore: notBefore, NotAfter: notAfter}, authorityPrivate)
		if issueErr != nil {
			t.Fatal(issueErr)
		}
		corpus, openErr := alpha.OpenCorpus(authorityPublic, raw)
		if openErr != nil {
			t.Fatal(openErr)
		}
		return corpus
	}
	current := issue(1, 9, now.Add(-time.Minute), now.Add(time.Hour))
	gatewayPublic, gatewayPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	gateway, err := NewGateway(GatewayConfig{Corpus: current, NodeID: [32]byte{2}, Family: "gateway-a",
		AssignmentNotAfter: now.Add(time.Hour), IdentityKey: gatewayPrivate, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	gatewayServer := httptest.NewTLSServer(gateway.Handler())
	defer gatewayServer.Close()
	relay, err := NewRelay(gatewayServer.URL, gatewayServer.Client())
	if err != nil {
		t.Fatal(err)
	}
	relayServer := httptest.NewTLSServer(relay.Handler())
	defer relayServer.Close()
	config := alpha.PersistentFloorConfig{Root: persistentFloorTestRoot(t), Authority: authorityPublic, Cohort: "closed-alpha-1", Network: network}
	for _, invalid := range []struct {
		name    string
		corpus  *alpha.Corpus
		failure alpha.Failure
	}{
		{name: "not yet valid", corpus: issue(2, 10, now.Add(time.Minute), now.Add(2*time.Hour)), failure: alpha.FailureNotYetValid},
		{name: "expired", corpus: issue(2, 11, now.Add(-2*time.Hour), now.Add(-time.Minute)), failure: alpha.FailureExpired},
	} {
		t.Run(invalid.name, func(t *testing.T) {
			floor, openErr := alpha.OpenPersistentFloor(config)
			if openErr != nil {
				t.Fatal(openErr)
			}
			newClient := func() *Client {
				t.Helper()
				client, clientErr := Open(ClientConfig{RelayURL: relayServer.URL, RelayNodeID: [32]byte{3}, RelayFamily: "relay-b",
					GatewayPublic: gatewayPublic, Gateway: gateway.Profile(), AuthorityPublic: authorityPublic,
					Cohort: "closed-alpha-1", Network: network, Floor: floor,
					Base: relayServer.Client().Transport.(*http.Transport).Clone()}, now)
				if clientErr != nil {
					t.Fatal(clientErr)
				}
				return client
			}
			gateway.config.Corpus = current
			if _, resolveErr := newClient().Resolve(t.Context(), link, now); resolveErr != nil {
				t.Fatal(resolveErr)
			}
			gateway.config.Corpus = invalid.corpus
			if _, resolveErr := newClient().Resolve(t.Context(), link, now); !alpha.HasFailure(resolveErr, invalid.failure) {
				t.Fatalf("outside-validity corpus result = %v, want %s", resolveErr, invalid.failure)
			}
			if err := floor.Close(); err != nil {
				t.Fatal(err)
			}
			floor, openErr = alpha.OpenPersistentFloor(config)
			if openErr != nil {
				t.Fatal(openErr)
			}
			defer floor.Close()
			retained, currentErr := floor.Current()
			if currentErr != nil || retained.Serial() != current.Serial() {
				t.Fatalf("persistent floor after outside-validity corpus = (%v, %v), want serial %d", retained, currentErr, current.Serial())
			}
		})
	}
}

func persistentFloorTestRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}
