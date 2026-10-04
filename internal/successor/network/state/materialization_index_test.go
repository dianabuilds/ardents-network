package state_test

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
	source2 "github.com/dianabuilds/ardents-network/internal/successor/network/source"
	state2 "github.com/dianabuilds/ardents-network/internal/successor/network/state"
)

func TestRefreshRejectsWrongIndexAndMalformedMaterialization(t *testing.T) {
	genesis := newFixture(t)
	successor := nextFixture(t, genesis)
	now := time.Unix(genesis.now, 0).UTC()
	policy := epoch.Policy{NetworkID: genesis.networkID,
		Authorities: map[[32]byte]ed25519.PublicKey{genesis.authorityID: genesis.authorityPublic},
		Threshold:   1, Profile: "ardents-route-v3", Now: now}
	first, err := epoch.Verify(policy, genesis.epoch, genesis.inputs, genesis.materializations, true)
	if err != nil {
		t.Fatal(err)
	}
	policy.Previous = &first.Snapshot
	second, err := epoch.Verify(policy, successor.epoch, successor.inputs, successor.materializations, true)
	if err != nil {
		t.Fatal(err)
	}
	wrongIndex, err := second.Materialization(1)
	if err != nil {
		t.Fatal(err)
	}
	trailing := append(append([]byte(nil), successor.materializations[0]...), 0)

	for _, test := range []struct {
		name, wantError string
		material        []byte
	}{
		{name: "wrong valid index", material: wrongIndex, wantError: "source withheld the requested materialization index"},
		{name: "short prefix", material: []byte{1}, wantError: "source withheld the requested materialization index"},
		{name: "trailing proof byte", material: trailing, wantError: "materialization has trailing bytes"},
	} {
		t.Run(test.name, func(t *testing.T) {
			config := sourceEnvironmentWithMaterial(t, genesis, successor, test.material)
			store, err := state2.Open(config)
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			if test.name == "short prefix" {
				time.Sleep(2100 * time.Millisecond)
			}
			if _, err := store.Refresh(context.Background()); err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("Refresh error = %v, want %q", err, test.wantError)
			}
			current, err := store.Current()
			if err != nil || current.Epoch != 1 || current.SourceOutcomes != [4]string{
				"invalid-state", "invalid-state", "not-attempted", "not-attempted",
			} {
				t.Fatalf("retained current after refused sources = %+v, %v", current, err)
			}
		})
	}
}

func sourceEnvironmentWithMaterial(t *testing.T, genesis, successor fixture, material []byte) state2.Config {
	t.Helper()
	now := time.Unix(genesis.now, 0).UTC()
	clientAuthority := makeTestAuthority(t, 0x91, "material-client-root")
	client := makeTestLeaf(t, clientAuthority, 0x92, "material-client.test", false)
	addresses := availableAddresses(t, 2)
	serverAuthorities := [2]testCertificate{
		makeTestAuthority(t, 0x93, "material-source-one-root"),
		makeTestAuthority(t, 0x94, "material-source-two-root"),
	}
	servers := [2]testCertificate{
		makeTestLeaf(t, serverAuthorities[0], 0x95, "material-source-one.test", true),
		makeTestLeaf(t, serverAuthorities[1], 0x96, "material-source-two.test", true),
	}
	payload, err := source2.EncodeBundle(source2.Bundle{Epoch: successor.epoch, Inputs: successor.inputs, Materials: [][]byte{material}})
	if err != nil {
		t.Fatal(err)
	}
	config := fixtureConfig(genesis, t.TempDir(), now)
	installed, err := state2.Open(config)
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
	config.Clock = advancingVerificationClock(now)
	config.ObserveClock = config.Clock
	config.Source.ClientCertificate = client.certificate
	config.Source.OrderSeed = sha256.Sum256([]byte("material-source-order"))
	var cancels [2]context.CancelFunc
	var done [2]chan error
	t.Cleanup(func() {
		for _, cancel := range cancels {
			if cancel != nil {
				cancel()
			}
		}
		for _, stopped := range done {
			if stopped != nil {
				<-stopped
			}
		}
	})
	for index := range servers {
		name := "material-source-one.test"
		if index == 1 {
			name = "material-source-two.test"
		}
		config.Source.Sources[index] = source2.Source{Address: addresses[index], ServerName: name,
			Identity: sha256.Sum256([]byte(name)), Family: name, EndpointHandle: name,
			RootPEM: serverAuthorities[index].rootPEM, LeafKeyDigest: servers[index].pin}
		plan, _, err := source2.New(source2.Config{ServeAddress: addresses[index],
			ServeCertificate: servers[index].certificate, ServeClientRootPEM: clientAuthority.rootPEM,
			ServeClientKeyDigests: [][32]byte{client.pin}, VerificationClock: func() time.Time { return now }},
			config.Authorities)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancels[index] = cancel
		ready := make(chan error, 1)
		done[index] = make(chan error, 1)
		go func(slot int) {
			done[slot] <- plan.Serve(ctx, ready, nil, nil, func(context.Context, source2.Message) source2.Message {
				return source2.Message{Status: "ok", ObjectDigest: successor.epochDigest, Payload: payload}
			})
		}(index)
		if err := <-ready; err != nil {
			t.Fatal(err)
		}
	}
	return config
}
