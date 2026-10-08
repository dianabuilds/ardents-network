//go:build linux

package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// These are genuine State servers and a real compiled profile intake consumer.
// All fixture infrastructure has one controller. No installed startup, root
// completion exchange, independent Source families or Service readiness is proved.
func TestInstalledSourceInputsRetainGenuineNetworkAuthority(t *testing.T) {
	f := newNetworkAdmissionFixture(t)
	bundle, err := networkfixture.BuildClosed(f.spec)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	clockPath := filepath.Join(dir, "clock")
	if err := os.WriteFile(clockPath, nil, 0600); err != nil {
		t.Fatal(err)
	}
	stop, joined := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(joined)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case now := <-ticker.C:
				if err := os.Chtimes(clockPath, now, now); err != nil {
					t.Error(err)
					return
				}
			}
		}
	}()
	t.Cleanup(func() { close(stop); <-joined })
	public := f.spec.Authority.Public().(ed25519.PublicKey)
	var authorityKey [32]byte
	copy(authorityKey[:], public)
	plan := &networkAuthorityPlan{Root: filepath.Join(dir, "participant-state"), NetworkID: f.spec.NetworkID,
		Authorities: [][32]byte{authorityKey}, Threshold: 1, ProfileAuthority: authorityKey, ClockObservationFile: clockPath}
	base, err := networkStateConfig(plan)
	if err != nil {
		t.Fatal(err)
	}
	_, clientPEM, clientKey, clientPin := installedSourceCertificate(t, "client", false)
	clientPath, keyPath := filepath.Join(dir, "client.pem"), filepath.Join(dir, "client.key")
	for path, raw := range map[string][]byte{clientPath: clientPEM, keyPath: clientKey} {
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	clear(clientKey)
	headless := installedHeadless{NetworkStateRoot: plan.Root, LocalRoleStateRoot: filepath.Join(dir, "participant-roles"),
		TimeConfidenceFile: clockPath, NetworkID: hex.EncodeToString(plan.NetworkID[:]), NetworkAuthorities: []string{hex.EncodeToString(authorityKey[:])},
		NetworkThreshold: 1, ClosedProfileAuthority: hex.EncodeToString(authorityKey[:])}
	declared := installedSource{ClockObservedAt: time.Now().UTC().Format(time.RFC3339), OrderSeed: hex.EncodeToString(f.spec.Seed[:]),
		RefreshIntervalMS: 1000, ClientCertificate: clientPath, ClientKey: keyPath}
	// Reserve both addresses together before opening either owner.
	var held [2]net.Listener
	var addresses [2]string
	for index := range held {
		held[index], err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = held[index].Close() })
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
		certificate, rootPEM, keyPEM, pin := installedSourceCertificate(t, name, true)
		clear(keyPEM)
		config := base
		config.Root = filepath.Join(dir, name+"-state")
		config.LocalRoleStateRoot = filepath.Join(dir, name+"-roles")
		seed, err := state.Open(config)
		if err != nil {
			t.Fatal(err)
		}
		_, acceptErr := seed.Accept(t.Context(), bundle.Epoch.Raw, bundle.Epoch.Inputs, bundle.Epoch.Materials)
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
			_, err := serving.Accept(t.Context(), next.Epoch.Raw, next.Epoch.Inputs, next.Epoch.Materials)
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
	config, err := installedNetworkConfig(headless, declared, func() error { return nil })
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
		Network *networkAuthorityPlan `json:"network"`
		Profile []byte                `json:"profile"`
	}{plan, bundle.Profile})
	if err != nil {
		t.Fatal(err)
	}
	inputPath := filepath.Join(dir, "profile-input.json")
	if err := os.WriteFile(inputPath, input, 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(t.Context(), compiledCommand(t), "network", "accept-profile", "--config", inputPath)
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
	nextSpec := f.spec
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

func installedSourceCertificate(t *testing.T, name string, server bool) (tls.Certificate, []byte, []byte, [32]byte) {
	t.Helper()
	public, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	usage := x509.ExtKeyUsageClientAuth
	if server {
		usage = x509.ExtKeyUsageServerAuth
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), DNSNames: []string{name}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, public, key)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded})
	clear(encoded)
	certificate, err := tls.X509KeyPair(certificatePEM, keyPEM)
	clear(key)
	if err != nil {
		t.Fatal(err)
	}
	pin := sha256.Sum256(append([]byte("ardents-h3-source-transport-key-v1\x00"), public...))
	return certificate, certificatePEM, keyPEM, pin
}
