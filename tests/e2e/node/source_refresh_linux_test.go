//go:build linux

package node_test

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/endpoint"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
	sourcefixture "github.com/dianabuilds/ardents-network/tests/e2e/source"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
	networkinputs "github.com/dianabuilds/ardents-network/tests/networkfixture"
)

// These are genuine State servers and a real compiled profile intake consumer.
// All fixture infrastructure has one controller. No installed startup, root
// completion exchange, independent Source families or Service readiness is proved.
func TestSourceRefreshRetainsGenuineNetworkAuthority(t *testing.T) {
	spec, _, _ := networkinputs.PrepareNetworkGenesis(t, t.TempDir())
	bundle, err := networkfixture.BuildClosed(spec)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	clockPath, closeClock := networkinputs.StartObservedClock(t)
	t.Cleanup(closeClock)
	public := spec.Authority.Public().(ed25519.PublicKey)
	var authorityKey [32]byte
	copy(authorityKey[:], public)
	plan := &state.AuthorityDeclaration{Root: filepath.Join(dir, "participant-state"), NetworkID: spec.NetworkID,
		Authorities: [][32]byte{authorityKey}, Threshold: 1, ProfileAuthority: authorityKey, ClockObservationFile: clockPath}
	base, err := plan.Config()
	if err != nil {
		t.Fatal(err)
	}
	_, clientPEM, clientKey, clientPin := sourcefixture.SourceCertificate(t, "client", false)
	clientPath, keyPath := filepath.Join(dir, "client.pem"), filepath.Join(dir, "client.key")
	for path, raw := range map[string][]byte{clientPath: clientPEM, keyPath: clientKey} {
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	clear(clientKey)
	headless := endpoint.HeadlessPlan{NetworkStateRoot: plan.Root, LocalRoleStateRoot: filepath.Join(dir, "participant-roles"),
		TimeConfidenceFile: clockPath, NetworkID: hex.EncodeToString(plan.NetworkID[:]), NetworkAuthorities: []string{hex.EncodeToString(authorityKey[:])},
		NetworkThreshold: 1, ClosedProfileAuthority: hex.EncodeToString(authorityKey[:])}
	declared := endpoint.SourcePlan{ClockObservedAt: time.Now().UTC().Format(time.RFC3339), OrderSeed: hex.EncodeToString(spec.Seed[:]),
		RefreshIntervalMS: 1000, ClientCertificate: clientPath, ClientKey: keyPath}
	// Reserve both addresses together before opening either owner.
	var held [2]net.Listener
	var addresses [2]string
	for index := range held {
		held[index], err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		listener := held[index]
		t.Cleanup(func() {
			if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				t.Error(err)
			}
		})
		addresses[index] = held[index].Addr().String()
	}
	for _, listener := range held {
		if err := listener.Close(); err != nil {
			t.Fatal(err)
		}
	}
	var advance []func(networkfixture.Closed) error
	for index, address := range addresses {
		name := []string{"source-a.test", "source-b.test"}[index]
		certificate, rootPEM, keyPEM, pin := sourcefixture.SourceCertificate(t, name, true)
		clear(keyPEM)
		config := base
		config.Root = filepath.Join(dir, name+"-state")
		config.LocalRoleStateRoot = filepath.Join(dir, name+"-roles")
		seed, err := state.Open(config)
		if err != nil {
			t.Fatal(err)
		}
		_, acceptErr := seed.AcceptEpoch(t.Context(), bundle.Epoch.Raw, bundle.Epoch.Inputs, bundle.Epoch.Materials)
		closeErr := seed.Close()
		if acceptErr != nil || closeErr != nil {
			t.Fatal(acceptErr, closeErr)
		}
		config.Source.ServeAddress, config.Source.ServeCertificate = address, certificate
		config.Source.ServeClientRootPEM, config.Source.ServeClientKeyDigests = clientPEM, [][32]byte{clientPin}
		serving, err := state.Open(config)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := serving.Close(); err != nil {
				t.Error(err)
			}
		})
		advance = append(advance, func(next networkfixture.Closed) error {
			_, err := serving.AcceptEpoch(t.Context(), next.Epoch.Raw, next.Epoch.Inputs, next.Epoch.Materials)
			return err
		})
		rootPath := filepath.Join(dir, name+".pem")
		if err := os.WriteFile(rootPath, rootPEM, 0600); err != nil {
			t.Fatal(err)
		}
		identity := sha256.Sum256([]byte(name))
		// Populate the already-declared DTO without manufacturing RuntimeView.
		declared.Sources = append(declared.Sources, struct {
			Address        string `json:"address"`
			ServerName     string `json:"server_name"`
			Identity       string `json:"identity"`
			Family         string `json:"family"`
			EndpointHandle string `json:"endpoint_handle"`
			RootCA         string `json:"root_ca"`
			LeafKeyDigest  string `json:"leaf_key_digest"`
		}{address, name, hex.EncodeToString(identity[:]), name, address, rootPath, hex.EncodeToString(pin[:])})
	}
	config, err := endpoint.NetworkConfig(headless, declared, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	owner, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := owner.Close(); err != nil {
			t.Error(err)
		}
	})
	snapshot, err := owner.Refresh(t.Context())
	if err != nil || snapshot.Digest != bundle.Epoch.Digest || snapshot.ObservedDigests[0] != bundle.Epoch.Digest || snapshot.ObservedDigests[1] != bundle.Epoch.Digest {
		t.Fatalf("genuine TLS Source acquisition: %v, %+v", err, snapshot)
	}
	if _, err := owner.CurrentRuntime(); err == nil || err.Error() != "closed profile is unavailable" {
		t.Fatal("missing profile did not give its specific authority refusal", err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	input, err := json.Marshal(struct {
		Network *state.AuthorityDeclaration `json:"network"`
		Profile []byte                      `json:"profile"`
	}{plan, bundle.Profile})
	if err != nil {
		t.Fatal(err)
	}
	inputPath := filepath.Join(dir, "profile-input.json")
	if err := os.WriteFile(inputPath, input, 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(t.Context(), buildNodeCommand(t), "network", "accept-profile", "--config", inputPath)
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("genuine compiled profile intake: %v: %s", err, out)
	}
	reopened, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	owner = reopened
	view, err := owner.CurrentRuntime()
	if err != nil || view.Profile().EpochDigest != bundle.Epoch.Digest {
		t.Fatal("retained profile did not authorize current observation", err)
	}
	// Require a subsequent background wave through both real TLS servers. The
	// accepted old profile cannot authorize the signed successor Epoch.
	nextSpec := spec
	nextSpec.Number++
	nextSpec.Previous = bundle.Epoch.Digest
	nextSpec.Seed[0]++
	next, err := networkfixture.BuildClosed(nextSpec)
	if err != nil {
		t.Fatal(err)
	}
	for _, accept := range advance {
		if err := accept(next); err != nil {
			t.Fatal(err)
		}
	}
	wait, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		observed, err := owner.Current()
		if err != nil {
			t.Fatal(err)
		}
		if observed.Digest == next.Epoch.Digest && observed.ObservedDigests[0] == next.Epoch.Digest && observed.ObservedDigests[1] == next.Epoch.Digest {
			break
		}
		select {
		case <-wait.Done():
			t.Fatal("background Source wave did not accept the real signed successor", wait.Err())
		case <-ticker.C:
		}
	}
	if _, err := owner.CurrentRuntime(); err == nil || err.Error() != "closed profile is unavailable" {
		t.Fatal("old profile did not give its specific successor authority refusal", err)
	}
}
