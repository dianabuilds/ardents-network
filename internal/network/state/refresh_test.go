package state_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	localroles "github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/source"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

type testCertificate struct {
	certificate tls.Certificate
	rootPEM     []byte
	pin         [32]byte
}

func TestRefreshWaitsForTwoAuthenticatedSourcesAndRestarts(t *testing.T) {
	genesis := newFixture(t)
	successor := nextFixture(t, genesis)
	now := time.Unix(genesis.now, 0).UTC()
	clientAuthority := makeTestAuthority(t, 0x61, "endpoint-client-root")
	client := makeTestLeaf(t, clientAuthority, 0x62, "endpoint.test", false)
	firstAuthority := makeTestAuthority(t, 0x71, "source-one-root")
	secondAuthority := makeTestAuthority(t, 0x72, "source-two-root")
	firstServer := makeTestLeaf(t, firstAuthority, 0x73, "source-one.test", true)
	secondServer := makeTestLeaf(t, secondAuthority, 0x74, "source-two.test", true)
	reserved := availableAddresses(t, 2)
	addresses := [2]string{reserved[0], reserved[1]}

	first := openTestSource(t, successor, addresses[0], firstServer, clientAuthority.rootPEM, client.pin)
	second := openTestSource(t, successor, addresses[1], secondServer, clientAuthority.rootPEM, client.pin)
	defer first.Close()
	defer second.Close()

	endpointRoot := t.TempDir()
	endpointConfig := fixtureConfig(genesis, endpointRoot, now)
	endpoint, err := state.Open(endpointConfig)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := endpoint.Accept(context.Background(), genesis.epoch, genesis.inputs, genesis.materializations); err != nil {
		t.Fatalf("accept installed genesis: %v", err)
	}
	if err := endpoint.Close(); err != nil {
		t.Fatal(err)
	}

	endpointConfig.Clock = func() time.Time { return now }
	endpointConfig.Now = time.Time{}
	endpointConfig.ClockObservation = now
	endpointConfig.Source.Sources = [2]source.Source{
		{Address: addresses[0], ServerName: "source-one.test", Identity: sha256.Sum256([]byte("source-one")),
			Family: "source-family-one", EndpointHandle: "source-handle-one", RootPEM: firstAuthority.rootPEM, LeafKeyDigest: firstServer.pin},
		{Address: addresses[1], ServerName: "source-two.test", Identity: sha256.Sum256([]byte("source-two")),
			Family: "source-family-two", EndpointHandle: "source-handle-two", RootPEM: secondAuthority.rootPEM, LeafKeyDigest: secondServer.pin},
	}
	endpointConfig.Source.ClientCertificate = client.certificate
	endpointConfig.Source.OrderSeed = sha256.Sum256([]byte("fixed-source-order"))

	endpoint, err = state.Open(endpointConfig)
	if err != nil {
		t.Fatalf("open endpoint source mode: %v", err)
	}
	refreshed, err := endpoint.Refresh(context.Background())
	if err != nil {
		t.Fatalf("refresh through finite sources: %v", err)
	}
	if refreshed.Epoch != 2 || refreshed.Digest != successor.epochDigest || refreshed.SourceAttempts != 2 {
		t.Fatalf("unexpected refreshed snapshot: %+v", refreshed)
	}
	roles, err := localroles.Open(localroles.Config{Root: endpointConfig.LocalRoleStateRoot, Clock: endpointConfig.Clock})
	if err != nil {
		t.Fatal(err)
	}
	if conflict, err := roles.Conflict(endpointConfig.Source.Sources[0].Identity,
		sha256.Sum256([]byte(endpointConfig.Source.Sources[0].Family))); err != nil || !conflict {
		t.Fatalf("Direct Source local duty = %v, %v", conflict, err)
	}
	if err := roles.Close(); err != nil {
		t.Fatal(err)
	}
	if err := endpoint.Close(); err != nil {
		t.Fatal(err)
	}

	restarted, err := state.Open(endpointConfig)
	if err != nil {
		t.Fatalf("restart endpoint: %v", err)
	}
	defer restarted.Close()
	recovered, err := restarted.Current()
	if err != nil || recovered.Epoch != 2 || recovered.SourceAttempts != 2 || recovered.TrustedTime != now {
		t.Fatalf("recovered source state=%+v err=%v", recovered, err)
	}
	if err := restarted.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(endpointRoot, "distribution", "current")); err != nil {
		t.Fatal(err)
	}
	if _, err := state.Open(endpointConfig); err == nil || !strings.Contains(err.Error(), "lacks its current pointer") {
		t.Fatalf("missing distribution pointer returned %v", err)
	}
}

func TestRefreshPersistsObservedConflictAcrossRestart(t *testing.T) {
	genesis := newFixture(t)
	first := nextFixtureWithSeed(t, genesis, "conflict-first")
	second := nextFixtureWithSeed(t, genesis, "conflict-second")
	config, closeSources := sourceEnvironment(t, genesis, first, second)
	defer closeSources()
	endpoint, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := endpoint.Refresh(context.Background()); err == nil || !strings.Contains(err.Error(), "conflicting") {
		t.Fatalf("conflicting wave returned %v", err)
	}
	conflicting, err := endpoint.Current()
	if err != nil || !conflicting.Conflicting || conflicting.Epoch != 1 {
		t.Fatalf("conflict snapshot=%+v err=%v", conflicting, err)
	}
	if err := endpoint.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	if _, err := restarted.Refresh(context.Background()); err == nil || !strings.Contains(err.Error(), "persistent source conflict") {
		t.Fatalf("restart did not preserve conflict: %v", err)
	}
}

func TestRefreshPersistsTLSFailureBackoff(t *testing.T) {
	genesis := newFixture(t)
	successor := nextFixture(t, genesis)
	config, closeSources := sourceEnvironment(t, genesis, successor, successor)
	defer closeSources()
	config.Source.Sources[0].LeafKeyDigest = sha256.Sum256([]byte("wrong-source-one-pin"))
	config.Source.Sources[1].LeafKeyDigest = sha256.Sum256([]byte("wrong-source-two-pin"))
	endpoint, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := endpoint.Refresh(context.Background()); err == nil || !strings.Contains(err.Error(), "authentication failed") {
		t.Fatalf("TLS-pin failure returned %v", err)
	}
	failed, err := endpoint.Current()
	if err != nil || failed.SourceAttempts != 2 || failed.NextAutomatic.IsZero() {
		t.Fatalf("failure state=%+v err=%v", failed, err)
	}
	if err := endpoint.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	if _, err := restarted.Refresh(context.Background()); err == nil || !strings.Contains(err.Error(), "durable backoff") {
		t.Fatalf("restart ignored durable backoff: %v", err)
	}
}

func TestRefreshRecordsPartialWaveWithoutCompletenessClaim(t *testing.T) {
	genesis := newFixture(t)
	successor := nextFixture(t, genesis)
	config, closeSources := sourceEnvironment(t, genesis, successor, successor)
	defer closeSources()
	config.Source.Sources[0].LeafKeyDigest = sha256.Sum256([]byte("wrong-source-one-pin"))
	endpoint, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer endpoint.Close()
	refreshed, err := endpoint.Refresh(context.Background())
	if err != nil {
		t.Fatalf("partial finite wave: %v", err)
	}
	if refreshed.Epoch != 2 || refreshed.SourceOutcomes != [4]string{"authentication-failed", "valid", "not-attempted", "not-attempted"} ||
		refreshed.NextAutomatic.IsZero() || refreshed.LatestCompleteness != "latest completeness unproven" {
		t.Fatalf("partial wave evidence=%+v", refreshed)
	}
}

func TestSourceClientTransportKeyCannotReuseEpochSigner(t *testing.T) {
	genesis := newFixture(t)
	successor := nextFixture(t, genesis)
	config, closeSources := sourceEnvironment(t, genesis, successor, successor)
	defer closeSources()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(909), Subject: pkix.Name{CommonName: "reused-signer.test"},
		NotBefore: time.Unix(1_600_000_000, 0), NotAfter: time.Unix(2_200_000_000, 0),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	raw, err := x509.CreateCertificate(nil, template, template, genesis.authorityPublic, genesis.authorityPrivate)
	if err != nil {
		t.Fatal(err)
	}
	config.Source.ClientCertificate = tls.Certificate{Certificate: [][]byte{raw}, PrivateKey: genesis.authorityPrivate}
	if _, err := state.Open(config); err == nil || !strings.Contains(err.Error(), "separate from Epoch signer") {
		t.Fatalf("reused signer transport key returned %v", err)
	}
}

func TestRefreshRejectsUncertainClockBeforeContact(t *testing.T) {
	genesis := newFixture(t)
	successor := nextFixture(t, genesis)
	config, closeSources := sourceEnvironment(t, genesis, successor, successor)
	defer closeSources()
	config.ClockObservation = config.ClockObservation.Add(-3 * time.Second)
	endpoint, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer endpoint.Close()
	if _, err := endpoint.Refresh(context.Background()); err == nil || !strings.Contains(err.Error(), "clock confidence") {
		t.Fatalf("uncertain clock returned %v", err)
	}
	current, err := endpoint.Current()
	if err != nil || current.SourceAttempts != 0 || current.Freshness != "clock-uncertain" {
		t.Fatalf("clock-uncertain snapshot=%+v err=%v", current, err)
	}
}

func TestRefreshStagesFutureEpochAndActivatesAfterRestart(t *testing.T) {
	genesis := newFixture(t)
	future := futureFixture(t, genesis, genesis.now+20)
	config, closeSources := sourceEnvironment(t, genesis, future, future)
	defer closeSources()
	endpoint, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	staged, err := endpoint.Refresh(context.Background())
	if err != nil {
		t.Fatalf("stage future Epoch: %v", err)
	}
	if staged.Epoch != 1 || staged.PendingEpoch != 2 || staged.PendingDigest != future.epochDigest {
		t.Fatalf("staged snapshot=%+v", staged)
	}
	if err := endpoint.Close(); err != nil {
		t.Fatal(err)
	}
	later := time.Unix(genesis.now+21, 0).UTC()
	config.Clock = func() time.Time { return later }
	config.ClockObservation = later
	restarted, err := state.Open(config)
	if err != nil {
		t.Fatalf("restart with pending Epoch: %v", err)
	}
	defer restarted.Close()
	activated, err := restarted.Refresh(context.Background())
	if err != nil {
		t.Fatalf("activate pending Epoch: %v", err)
	}
	if activated.Epoch != 2 || activated.Digest != future.epochDigest || activated.PendingEpoch != 0 {
		t.Fatalf("activated snapshot=%+v", activated)
	}
}

func sourceEnvironment(t *testing.T, genesis, firstValue, secondValue fixture) (state.Config, func()) {
	t.Helper()
	now := time.Unix(genesis.now, 0).UTC()
	clientAuthority := makeTestAuthority(t, 0x51, "shared-client-root")
	client := makeTestLeaf(t, clientAuthority, 0x52, "endpoint.test", false)
	firstAuthority := makeTestAuthority(t, 0x53, "first-source-root")
	secondAuthority := makeTestAuthority(t, 0x54, "second-source-root")
	firstServer := makeTestLeaf(t, firstAuthority, 0x55, "first-source.test", true)
	secondServer := makeTestLeaf(t, secondAuthority, 0x56, "second-source.test", true)
	reserved := availableAddresses(t, 2)
	var addresses [2]string
	addresses[0] = reserved[0]
	first := openTestSource(t, firstValue, addresses[0], firstServer, clientAuthority.rootPEM, client.pin)
	addresses[1] = reserved[1]
	second := openTestSource(t, secondValue, addresses[1], secondServer, clientAuthority.rootPEM, client.pin)
	config := fixtureConfig(genesis, t.TempDir(), now)
	installed, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := installed.Accept(context.Background(), genesis.epoch, genesis.inputs, genesis.materializations); err != nil {
		t.Fatal(err)
	}
	if err := installed.Close(); err != nil {
		t.Fatal(err)
	}
	config.Now = time.Time{}
	config.Clock = func() time.Time { return now }
	config.ClockObservation = now
	config.Source.Sources = [2]source.Source{
		{Address: addresses[0], ServerName: "first-source.test", Identity: sha256.Sum256([]byte("first-source")),
			Family: "first-source-family", EndpointHandle: "first-source-handle", RootPEM: firstAuthority.rootPEM, LeafKeyDigest: firstServer.pin},
		{Address: addresses[1], ServerName: "second-source.test", Identity: sha256.Sum256([]byte("second-source")),
			Family: "second-source-family", EndpointHandle: "second-source-handle", RootPEM: secondAuthority.rootPEM, LeafKeyDigest: secondServer.pin},
	}
	config.Source.ClientCertificate = client.certificate
	config.Source.OrderSeed = sha256.Sum256([]byte("source-environment-order"))
	return config, func() { _ = first.Close(); _ = second.Close() }
}

func fixtureConfig(value fixture, root string, now time.Time) state.Config {
	return state.Config{
		Root: root, LocalRoleStateRoot: root + "-local-roles", NetworkID: value.networkID,
		Authorities: map[[32]byte]ed25519.PublicKey{value.authorityID: value.authorityPublic},
		Threshold:   1, Now: now,
	}
}

func openTestSource(t *testing.T, value fixture, address string, server testCertificate, clientRoot []byte, clientPin [32]byte) interface{ Close() error } {
	return openTestSourceAtIndex(t, value, address, server, clientRoot, clientPin, 0)
}

func openTestSourceAtIndex(t *testing.T, value fixture, address string, server testCertificate, clientRoot []byte, clientPin [32]byte, materialIndex uint32) interface{ Close() error } {
	t.Helper()
	config := fixtureConfig(value, t.TempDir(), time.Unix(value.now, 0))
	store, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	genesis := newFixture(t)
	if _, err := store.Accept(context.Background(), genesis.epoch, genesis.inputs, genesis.materializations); err != nil {
		t.Fatal(err)
	}
	if value.epochDigest != genesis.epochDigest {
		if _, err := store.Accept(context.Background(), value.epoch, value.inputs, value.materializations); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	config.Source.ServeAddress = address
	config.Source.ServeCertificate = server.certificate
	config.Source.ServeClientRootPEM = clientRoot
	config.Source.ServeClientKeyDigests = [][32]byte{clientPin}
	config.Source.MaterialIndex = materialIndex
	served, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	return served
}

// availableAddresses reserves a batch before releasing it. Allocating and
// closing one ephemeral listener at a time can return the same port to the
// second State-source role before either source begins listening.
func availableAddresses(t *testing.T, count int) []string {
	t.Helper()
	listeners := make([]net.Listener, 0, count)
	addresses := make([]string, 0, count)
	for index := 0; index < count; index++ {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			for _, held := range listeners {
				_ = held.Close()
			}
			t.Fatal(err)
		}
		listeners = append(listeners, listener)
		addresses = append(addresses, listener.Addr().String())
	}
	for _, listener := range listeners {
		if err := listener.Close(); err != nil {
			t.Fatal(err)
		}
	}
	return addresses
}

func TestAvailableAddressesAreDistinctWithinOneFixture(t *testing.T) {
	addresses := availableAddresses(t, 2)
	if addresses[0] == addresses[1] {
		t.Fatalf("State fixture reused endpoint address %q", addresses[0])
	}
}

func makeTestAuthority(t *testing.T, marker byte, commonName string) testCertificate {
	t.Helper()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{marker}, ed25519.SeedSize))
	template := &x509.Certificate{
		SerialNumber: big.NewInt(int64(marker)), Subject: pkix.Name{CommonName: commonName},
		NotBefore: time.Unix(1_600_000_000, 0), NotAfter: time.Unix(2_200_000_000, 0),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	raw, err := x509.CreateCertificate(nil, template, template, private.Public(), private)
	if err != nil {
		t.Fatal(err)
	}
	rootPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: raw})
	return testCertificate{certificate: tls.Certificate{Certificate: [][]byte{raw}, PrivateKey: private}, rootPEM: rootPEM}
}

func makeTestLeaf(t *testing.T, authority testCertificate, marker byte, name string, server bool) testCertificate {
	t.Helper()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{marker}, ed25519.SeedSize))
	parent, err := x509.ParseCertificate(authority.certificate.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	usage := x509.ExtKeyUsageClientAuth
	if server {
		usage = x509.ExtKeyUsageServerAuth
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(int64(marker)), Subject: pkix.Name{CommonName: name}, DNSNames: []string{name},
		NotBefore: time.Unix(1_600_000_000, 0), NotAfter: time.Unix(2_200_000_000, 0),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage},
	}
	raw, err := x509.CreateCertificate(nil, template, parent, private.Public(), authority.certificate.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	prefix := []byte("ardents-h3-source-transport-key-v1\x00")
	pin := sha256.Sum256(append(prefix, private.Public().(ed25519.PublicKey)...))
	return testCertificate{certificate: tls.Certificate{Certificate: [][]byte{raw}, PrivateKey: private}, pin: pin}
}
