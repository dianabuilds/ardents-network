package state_test

import (
	"context"
	"crypto/sha256"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/source"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func TestOfflineAcceptRejectsCandidateConflictingWithPendingEpoch(t *testing.T) {
	genesis := newFixture(t)
	pending := futureFixture(t, genesis, genesis.now+20)
	conflicting := nextFixtureWithSeed(t, genesis, "offline-pending-conflict")
	config, closeSources := sourceEnvironment(t, genesis, pending, pending)
	defer closeSources()

	endpoint, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = endpoint.Close() })
	staged, err := endpoint.Refresh(context.Background())
	if err != nil {
		t.Fatalf("stage pending Epoch: %v", err)
	}
	if staged.PendingEpoch != pendingEpoch || staged.PendingDigest != pending.epochDigest {
		t.Fatalf("staged snapshot=%+v", staged)
	}
	if err := endpoint.Close(); err != nil {
		t.Fatal(err)
	}

	offline := fixtureConfig(genesis, config.Root, time.Unix(genesis.now, 0).UTC())
	opened, err := state.Open(offline)
	if err != nil {
		t.Fatalf("open pending root offline: %v", err)
	}
	t.Cleanup(func() { _ = opened.Close() })
	if _, err := opened.Accept(context.Background(), conflicting.epoch, conflicting.inputs, conflicting.materializations); err == nil || !strings.Contains(err.Error(), "pending") {
		t.Fatalf("conflicting offline accept returned %v", err)
	}
	if err := opened.Close(); err != nil {
		t.Fatal(err)
	}

	restarted, err := state.Open(offline)
	if err != nil {
		t.Fatalf("reopen conflict root: %v", err)
	}
	defer restarted.Close()
	current, err := restarted.Current()
	if err != nil || !current.Conflicting || current.Epoch != genesisEpoch {
		t.Fatalf("reopened conflict snapshot=%+v err=%v", current, err)
	}
	if _, err := restarted.Accept(context.Background(), pending.epoch, pending.inputs, pending.materializations); err == nil || !strings.Contains(err.Error(), "persistent") {
		t.Fatalf("accept after durable conflict returned %v", err)
	}
}

func TestOfflineAcceptActivatesExactPendingEpoch(t *testing.T) {
	genesis := newFixture(t)
	pending := futureFixture(t, genesis, genesis.now+20)
	config, closeSources := sourceEnvironment(t, genesis, pending, pending)
	defer closeSources()

	endpoint, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = endpoint.Close() })
	if _, err := endpoint.Refresh(context.Background()); err != nil {
		t.Fatalf("stage pending Epoch: %v", err)
	}
	if err := endpoint.Close(); err != nil {
		t.Fatal(err)
	}

	later := time.Unix(genesis.now+21, 0).UTC()
	offline := fixtureConfig(genesis, config.Root, later)
	opened, err := state.Open(offline)
	if err != nil {
		t.Fatalf("open pending root offline: %v", err)
	}
	t.Cleanup(func() { _ = opened.Close() })
	accepted, err := opened.Accept(context.Background(), pending.epoch, pending.inputs, pending.materializations)
	if err != nil {
		t.Fatalf("accept exact pending Epoch: %v", err)
	}
	if accepted.Epoch != pendingEpoch || accepted.Digest != pending.epochDigest || accepted.PendingEpoch != 0 {
		t.Fatalf("accepted snapshot=%+v", accepted)
	}
	if err := opened.Close(); err != nil {
		t.Fatal(err)
	}

	restarted, err := state.Open(offline)
	if err != nil {
		t.Fatalf("reopen activated pending root: %v", err)
	}
	defer restarted.Close()
	current, err := restarted.Current()
	if err != nil || current.Epoch != pendingEpoch || current.Digest != pending.epochDigest || current.PendingEpoch != 0 {
		t.Fatalf("reopened snapshot=%+v err=%v", current, err)
	}
}

func TestSourceRefreshRefusesExpiredPendingEpoch(t *testing.T) {
	genesis := newFixture(t)
	pending := buildFixtureEpoch(t, genesis, pendingEpoch, genesis.epochDigest,
		sha256.Sum256([]byte("expired-pending-assignment-seed")), time.Unix(genesis.now+20, 0).UTC(), time.Unix(genesis.now+22, 0).UTC())
	pending.now = genesis.now + 20
	config, closeSources := sourceEnvironment(t, genesis, pending, pending)
	defer closeSources()

	endpoint, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := endpoint.Refresh(context.Background()); err != nil {
		_ = endpoint.Close()
		t.Fatalf("stage pending Epoch: %v", err)
	}
	if err := endpoint.Close(); err != nil {
		t.Fatal(err)
	}

	expired := time.Unix(genesis.now+23, 0).UTC()
	config.Clock = func() time.Time { return expired }
	config.ClockObservation = expired
	restarted, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	if _, err := restarted.Refresh(context.Background()); err == nil || !strings.Contains(err.Error(), "strictly current") {
		t.Fatalf("expired pending refresh returned %v", err)
	}
	current, err := restarted.Current()
	if err != nil || current.Epoch != genesisEpoch || current.PendingDigest != pending.epochDigest || current.Conflicting {
		t.Fatalf("expired pending snapshot=%+v err=%v", current, err)
	}
}

func TestSourceRefreshRejectsCandidateConflictingWithPendingEpoch(t *testing.T) {
	genesis := newFixture(t)
	pending := futureFixture(t, genesis, genesis.now+20)
	conflicting := nextFixtureWithSeed(t, genesis, "source-pending-conflict")
	now := time.Unix(genesis.now, 0).UTC()
	clientAuthority := makeTestAuthority(t, 0x81, "pending-client-root")
	client := makeTestLeaf(t, clientAuthority, 0x82, "pending-endpoint.test", false)
	firstAuthority := makeTestAuthority(t, 0x83, "pending-first-source-root")
	secondAuthority := makeTestAuthority(t, 0x84, "pending-second-source-root")
	firstServer := makeTestLeaf(t, firstAuthority, 0x85, "pending-first-source.test", true)
	secondServer := makeTestLeaf(t, secondAuthority, 0x86, "pending-second-source.test", true)
	addresses := availableAddresses(t, 4)
	first := openTestSource(t, pending, addresses[0], firstServer, clientAuthority.rootPEM, client.pin)
	second := openTestSource(t, pending, addresses[1], secondServer, clientAuthority.rootPEM, client.pin)
	t.Cleanup(func() { _ = first.Close() })
	t.Cleanup(func() { _ = second.Close() })

	config := fixtureConfig(genesis, t.TempDir(), now)
	installGenesis(t, config, genesis)
	config.Now = time.Time{}
	config.Clock = func() time.Time { return now }
	config.ClockObservation = now
	config.Source.ClientCertificate = client.certificate
	config.Source.OrderSeed = sha256.Sum256([]byte("pending-conflict-source-order"))
	config.Source.Sources = sourcePair(addresses[:2], firstAuthority, secondAuthority, firstServer, secondServer)
	endpoint, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = endpoint.Close() })
	if _, err := endpoint.Refresh(context.Background()); err != nil {
		t.Fatalf("stage pending Epoch: %v", err)
	}
	if err := endpoint.Close(); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	now = time.Unix(genesis.now+21, 0).UTC()
	config.ClockObservation = now

	replacementFirst := openTestSource(t, conflicting, addresses[2], firstServer, clientAuthority.rootPEM, client.pin)
	replacementSecond := openTestSource(t, conflicting, addresses[3], secondServer, clientAuthority.rootPEM, client.pin)
	t.Cleanup(func() { _ = replacementFirst.Close() })
	t.Cleanup(func() { _ = replacementSecond.Close() })
	config.Source.Sources = sourcePair(addresses[2:], firstAuthority, secondAuthority, firstServer, secondServer)
	restarted, err := state.Open(config)
	if err != nil {
		t.Fatalf("reopen pending root: %v", err)
	}
	t.Cleanup(func() { _ = restarted.Close() })
	if _, err := restarted.Refresh(context.Background()); err == nil || !strings.Contains(err.Error(), "pending") {
		t.Fatalf("conflicting Source refresh returned %v", err)
	}
	if err := restarted.Close(); err != nil {
		t.Fatal(err)
	}

	conflicted, err := state.Open(config)
	if err != nil {
		t.Fatalf("reopen conflict root: %v", err)
	}
	defer conflicted.Close()
	current, err := conflicted.Current()
	if err != nil || !current.Conflicting || current.Epoch != genesisEpoch || current.PendingDigest != pending.epochDigest {
		t.Fatalf("reopened conflict snapshot=%+v err=%v", current, err)
	}
	if _, err := conflicted.Refresh(context.Background()); err == nil || !strings.Contains(err.Error(), "persistent source conflict") {
		t.Fatalf("refresh after durable conflict returned %v", err)
	}
}

func installGenesis(t *testing.T, config state.Config, genesis fixture) {
	t.Helper()
	installed, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := installed.Accept(context.Background(), genesis.epoch, genesis.inputs, genesis.materializations); err != nil {
		_ = installed.Close()
		t.Fatal(err)
	}
	if err := installed.Close(); err != nil {
		t.Fatal(err)
	}
}

func sourcePair(addresses []string, firstAuthority, secondAuthority, firstServer, secondServer testCertificate) [2]source.Source {
	return [2]source.Source{
		{Address: addresses[0], ServerName: "pending-first-source.test", Identity: sha256.Sum256([]byte("pending-first-source")),
			Family: "pending-first-source-family", EndpointHandle: "pending-first-source-handle", RootPEM: firstAuthority.rootPEM, LeafKeyDigest: firstServer.pin},
		{Address: addresses[1], ServerName: "pending-second-source.test", Identity: sha256.Sum256([]byte("pending-second-source")),
			Family: "pending-second-source-family", EndpointHandle: "pending-second-source-handle", RootPEM: secondAuthority.rootPEM, LeafKeyDigest: secondServer.pin},
	}
}

const (
	genesisEpoch = 1
	pendingEpoch = 2
)
