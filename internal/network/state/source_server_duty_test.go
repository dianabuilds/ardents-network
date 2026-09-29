package state_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/source"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func TestServingSourceDutyTracksAcceptedSuccessor(t *testing.T) {
	genesis := newFixture(t)
	successor := nextFixture(t, genesis)
	config := installedServingSourceConfig(t, genesis)
	serving, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := serving.Close(); err != nil {
			t.Error(err)
		}
	}()
	original, err := serving.Current()
	if err != nil {
		t.Fatal(err)
	}
	afterOriginal := original.ValidUntil.Add(time.Minute)
	family := sha256.Sum256([]byte(original.DeclaredFamily))
	if conflict, err := duty.ReadConflict(config.LocalRoleStateRoot, func() time.Time { return afterOriginal }, original.NodeID, family); err != nil || !conflict {
		t.Fatalf("initial Source duty after its Epoch = %t, %v; want held until Close", conflict, err)
	}

	advanced, err := serving.Accept(context.Background(), successor.epoch, successor.inputs, successor.materializations)
	if err != nil {
		t.Fatal(err)
	}
	if advanced.NodeID != original.NodeID || advanced.DeclaredFamily != original.DeclaredFamily || !afterOriginal.Before(advanced.ValidUntil) {
		t.Fatalf("successor did not retain and extend the serving Source: before=%+v after=%+v", original, advanced)
	}
	if conflict, err := duty.ReadConflict(config.LocalRoleStateRoot, func() time.Time { return afterOriginal }, advanced.NodeID, family); err != nil || !conflict {
		t.Fatalf("serving Source duty after accepted successor = %t, %v; want retained", conflict, err)
	}
}

func TestServingSourceDutyProtectsRealResponseAfterEpochExpiry(t *testing.T) {
	genesis := newFixture(t)
	now := time.Unix(genesis.now, 0).UTC()
	config := fixtureConfig(genesis, t.TempDir(), now)
	installed, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := installed.Accept(t.Context(), genesis.epoch, genesis.inputs, genesis.materializations); err != nil {
		_ = installed.Close()
		t.Fatal(err)
	}
	if err := installed.Close(); err != nil {
		t.Fatal(err)
	}

	clientAuthority := makeTestAuthority(t, 0x81, "expiry-client-root")
	client := makeTestLeaf(t, clientAuthority, 0x82, "expiry-client.test", false)
	serverAuthority := makeTestAuthority(t, 0x83, "expiry-server-root")
	server := makeTestLeaf(t, serverAuthority, 0x84, "expiry-server.test", true)
	otherAuthority := makeTestAuthority(t, 0x85, "other-source-root")
	other := makeTestLeaf(t, otherAuthority, 0x86, "other-source.test", true)
	var unixClock atomic.Int64
	unixClock.Store(now.Unix())
	clock := func() time.Time { return time.Unix(unixClock.Load(), 0).UTC() }
	config.Now = time.Time{}
	config.Clock = clock
	config.Source.ServeAddress = availableAddresses(t, 1)[0]
	config.Source.ServeCertificate = server.certificate
	config.Source.ServeClientRootPEM = clientAuthority.rootPEM
	config.Source.ServeClientKeyDigests = [][32]byte{client.pin}
	serving, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := serving.Close(); err != nil {
			t.Error(err)
		}
	}()
	current, err := serving.Current()
	if err != nil {
		t.Fatal(err)
	}
	clientPlan, _, err := source.New(source.Config{Sources: [2]source.Source{
		{Address: config.Source.ServeAddress, ServerName: "expiry-server.test", Identity: current.NodeID,
			Family: current.DeclaredFamily, EndpointHandle: "expiry-server", RootPEM: serverAuthority.rootPEM, LeafKeyDigest: server.pin},
		{Address: "127.0.0.1:4109", ServerName: "other-source.test", Identity: sha256.Sum256([]byte("other-source")),
			Family: "other-source-family", EndpointHandle: "other-source", RootPEM: otherAuthority.rootPEM, LeafKeyDigest: other.pin},
	}, ClientCertificate: client.certificate, VerificationClock: clock}, nil)
	if err != nil {
		t.Fatal(err)
	}
	unixClock.Store(current.ValidUntil.Add(time.Minute).Unix())
	response, err := clientPlan.Fetch(t.Context(), 0, source.Message{Operation: "latest", NetworkDigest: source.NetworkDigest(config.NetworkID)})
	if err != nil || response.Status != "ok" || response.ObjectDigest != current.Digest || len(response.Payload) == 0 {
		t.Fatalf("real Source response after Epoch expiry: status=%q digest=%x payload=%d err=%v", response.Status, response.ObjectDigest, len(response.Payload), err)
	}
	family := sha256.Sum256([]byte(current.DeclaredFamily))
	if protected, err := duty.ReadConflict(config.LocalRoleStateRoot, clock, current.NodeID, family); err != nil || !protected {
		t.Fatalf("Source served while its identity/family guard was unavailable: protected=%t err=%v", protected, err)
	}
}

// A successor whose serving identity or family conflicts with another local
// role must not publish its new State decision.
func TestServingSourceDutyRejectsConflictingSuccessor(t *testing.T) {
	genesis := newFixture(t)
	config := installedServingSourceConfig(t, genesis)
	serving, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := serving.Close(); err != nil {
			t.Error(err)
		}
	}()
	original, err := serving.Current()
	if err != nil {
		t.Fatal(err)
	}
	marker, endpoint, capacity := byte(0x11), "127.0.0.1:4101", uint16(5)
	if original.NodeID == sha256.Sum256([]byte{0x4e, 0x21}) {
		marker, endpoint, capacity = 0x21, "127.0.0.1:4102", 3
	} else if original.NodeID != sha256.Sum256([]byte{0x4e, 0x11}) {
		t.Fatalf("unexpected fixture Source identity %x", original.NodeID)
	}
	const nextFamily = "source-successor-family"
	replacement := makeRecord(t, genesis.networkID, marker, nextFamily, endpoint, capacity)
	successor := genesis
	successor.inputs = append([][]byte(nil), genesis.inputs...)
	successor.accepted = append([]fixtureRecord(nil), genesis.accepted...)
	for index, record := range successor.accepted {
		if record.nodeID != original.NodeID {
			continue
		}
		for inputIndex, input := range successor.inputs {
			if bytes.Equal(input, record.bytes) {
				successor.inputs[inputIndex] = replacement.bytes
			}
		}
		successor.accepted[index] = replacement
	}
	now := time.Unix(genesis.now, 0).UTC()
	successor = buildFixtureEpoch(t, successor, 2, genesis.epochDigest, sha256.Sum256([]byte("source-duty-conflict")),
		now.Add(-30*time.Second), now.Add(time.Hour))
	roles, err := duty.Open(duty.Config{Root: config.LocalRoleStateRoot, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	conflict := duty.Duty{Identity: [32]byte{99}, Family: sha256.Sum256([]byte(nextFamily)),
		Class: "node-duty", State: "live", NotAfter: now.Add(2 * time.Hour)}
	if err := roles.Replace([32]byte{88}, []duty.Duty{conflict}); err != nil {
		_ = roles.Close()
		t.Fatal(err)
	}
	if err := roles.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := serving.Accept(context.Background(), successor.epoch, successor.inputs, successor.materializations); !errors.Is(err, duty.ErrLocalRoleConflict) {
		t.Fatalf("conflicting Source successor accepted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(config.Root, "generations", hex.EncodeToString(successor.epochDigest[:]))); !os.IsNotExist(err) {
		t.Fatalf("conflicting Source successor staged a durable generation: %v", err)
	}
	current, err := serving.Current()
	if err != nil || current.Epoch != original.Epoch || current.Digest != original.Digest {
		t.Fatalf("conflicting Source successor changed current State: %+v, %v", current, err)
	}
	if protected, err := duty.ReadConflict(config.LocalRoleStateRoot, func() time.Time { return now }, original.NodeID,
		sha256.Sum256([]byte(original.DeclaredFamily))); err != nil || !protected {
		t.Fatalf("original Source duty lost after refusal: %t, %v", protected, err)
	}
}

func TestServingSourceDutyTracksSourceWaveSuccessor(t *testing.T) {
	genesis := newFixture(t)
	successor := nextFixture(t, genesis)
	config, closeSources := sourceEnvironment(t, genesis, successor, successor)
	defer closeSources()
	config = configureSourceServer(t, config)
	serving, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := serving.Close(); err != nil {
			t.Error(err)
		}
	}()
	original, err := serving.Current()
	if err != nil {
		t.Fatal(err)
	}
	advanced, err := serving.Refresh(context.Background())
	if err != nil || advanced.Epoch != 2 {
		t.Fatalf("Source wave successor = %+v, %v", advanced, err)
	}
	afterOriginal := original.ValidUntil.Add(time.Minute)
	if protected, err := duty.ReadConflict(config.LocalRoleStateRoot, func() time.Time { return afterOriginal },
		advanced.NodeID, sha256.Sum256([]byte(advanced.DeclaredFamily))); err != nil || !protected {
		t.Fatalf("Source wave lost the serving duty after predecessor expiry: %t, %v", protected, err)
	}
}

func TestServingSourceRetiresWhenDutyRootDisappears(t *testing.T) {
	genesis := newFixture(t)
	successor := nextFixture(t, genesis)
	config := installedServingSourceConfig(t, genesis)
	serving, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := serving.Close(); err != nil {
			t.Error(err)
		}
	}()
	held := config.LocalRoleStateRoot + "-held"
	if err := os.Rename(config.LocalRoleStateRoot, held); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Rename(held, config.LocalRoleStateRoot); err != nil {
			t.Error(err)
		}
	}()
	if _, err := serving.Accept(context.Background(), successor.epoch, successor.inputs, successor.materializations); err == nil {
		t.Fatal("successor accepted without local Source duty root")
	}
	if _, err := serving.Current(); err == nil {
		t.Fatal("Source owner remained usable without its local collision guard")
	}
}

func installedServingSourceConfig(t *testing.T, genesis fixture) state.Config {
	t.Helper()
	now := time.Unix(genesis.now, 0).UTC()
	config := fixtureConfig(genesis, t.TempDir(), now)
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
	return configureSourceServer(t, config)
}

func configureSourceServer(t *testing.T, config state.Config) state.Config {
	t.Helper()
	clientAuthority := makeTestAuthority(t, 0x61, "source-duty-client-root")
	client := makeTestLeaf(t, clientAuthority, 0x62, "source-duty-client.test", false)
	serverAuthority := makeTestAuthority(t, 0x71, "source-duty-server-root")
	server := makeTestLeaf(t, serverAuthority, 0x72, "source-duty-server.test", true)
	config.Source.ServeAddress = availableAddresses(t, 1)[0]
	config.Source.ServeCertificate = server.certificate
	config.Source.ServeClientRootPEM = clientAuthority.rootPEM
	config.Source.ServeClientKeyDigests = [][32]byte{client.pin}
	return config
}

func TestServingSourceDutyKeepsOpeningRoleRootAfterChdir(t *testing.T) {
	genesis := newFixture(t)
	successor := nextFixture(t, genesis)
	config := installedServingSourceConfig(t, genesis)
	openingDirectory := t.TempDir()
	laterDirectory := t.TempDir()
	t.Chdir(openingDirectory)
	config.LocalRoleStateRoot = "roles"

	serving, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	current, err := serving.Current()
	if err != nil {
		_ = serving.Close()
		t.Fatal(err)
	}
	roleRoot := filepath.Join(openingDirectory, "roles")
	clock := func() time.Time { return time.Unix(genesis.now, 0).UTC() }
	family := sha256.Sum256([]byte(current.DeclaredFamily))
	if conflict, err := duty.ReadConflict(roleRoot, clock, current.NodeID, family); err != nil || !conflict {
		_ = serving.Close()
		t.Fatalf("serving Source duty at opening root = %t, %v; want retained", conflict, err)
	}

	t.Chdir(laterDirectory)
	advanced, err := serving.Accept(context.Background(), successor.epoch, successor.inputs, successor.materializations)
	if err != nil {
		_ = serving.Close()
		t.Fatalf("accept after working directory changed: %v", err)
	}
	afterOriginal := current.ValidUntil.Add(time.Minute)
	if conflict, err := duty.ReadConflict(roleRoot, func() time.Time { return afterOriginal }, advanced.NodeID,
		sha256.Sum256([]byte(advanced.DeclaredFamily))); err != nil || !conflict {
		_ = serving.Close()
		t.Fatalf("successor Source duty at opening root = %t, %v; want retained", conflict, err)
	}
	if err := serving.Close(); err != nil {
		t.Fatalf("close after working directory changed: %v", err)
	}
	if conflict, err := duty.ReadConflict(roleRoot, clock, advanced.NodeID,
		sha256.Sum256([]byte(advanced.DeclaredFamily))); err != nil || conflict {
		t.Fatalf("serving Source duty after close = %t, %v; want released", conflict, err)
	}
}
