package h3route_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/epoch/assignment"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/qualification/epochfixture"
	qualification "github.com/dianabuilds/ardents-network/internal/qualification/route"
	"github.com/dianabuilds/ardents-network/internal/route"
)

type processIdentity struct {
	private ed25519.PrivateKey
	public  [32]byte
	cert    string
	key     string
}

type processFixture struct {
	network, epochDigest, selectionSeed [32]byte
	now                                 time.Time
	authority                           ed25519.PrivateKey
	identities                          []processIdentity
	addresses                           []string
	stateRoot                           string
	snapshot                            state.Snapshot
	plan                                route.Plan
	publisherID                         [32]byte
	qualification                       qualification.Case
}

func newProcessFixture(t *testing.T) processFixture {
	t.Helper()
	root := t.TempDir()
	value := processFixture{network: sha256.Sum256([]byte("h3-route-process-network")),
		now: time.Now().UTC().Truncate(time.Second), selectionSeed: sha256.Sum256([]byte("h3-route-client-selection")),
		authority: ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0xa1}, ed25519.SeedSize)),
		stateRoot: filepath.Join(root, "state"), publisherID: sha256.Sum256([]byte("publisher-fixture"))}
	for index := range 6 {
		value.identities = append(value.identities, writeIdentity(t, root, byte(index+1), value.now))
	}
	for range 5 {
		value.addresses = append(value.addresses, unusedProcessAddress(t))
	}
	domains := []string{"initiator", "introduction", "rendezvous", "responder"}
	seed := sha256.Sum256([]byte("h3-route-domain-assignment"))
	inputs, accepted := make([][]byte, 0, 4), make([]epochfixture.Record, 0, 4)
	for index, domain := range domains {
		family := familyFor(t, value.network, seed, domain, domains)
		nodeID := sha256.Sum256([]byte("route-node-" + domain))
		record, err := epochfixture.BuildRecord(epochfixture.RecordSpec{NetworkID: value.network, NodeID: nodeID,
			Generation: 1, ValidFrom: value.now.Add(-time.Minute), ValidUntil: value.now.Add(time.Hour),
			Family: family, Endpoint: value.addresses[index], Capability: 2, Capacity: 1,
			PrivateKey: value.identities[index].private})
		if err != nil {
			t.Fatal(err)
		}
		inputs, accepted = append(inputs, record.Raw), append(accepted, record)
	}
	epoch, err := epochfixture.BuildEpoch(epochfixture.EpochSpec{NetworkID: value.network, Number: 1,
		ValidFrom: value.now.Add(-time.Minute), ValidUntil: value.now.Add(time.Hour), Inputs: inputs,
		Accepted: accepted, AssignmentSeed: seed, Profile: "h3-route-tracer-v1", Domains: domains,
		Authorities: []ed25519.PrivateKey{value.authority}})
	if err != nil {
		t.Fatal(err)
	}
	value.epochDigest = epoch.Digest
	authorityPublic := value.authority.Public().(ed25519.PublicKey)
	opened, err := state.Open(state.Config{Root: value.stateRoot, NetworkID: value.network,
		Authorities: map[[32]byte]ed25519.PublicKey{sha256.Sum256(authorityPublic): authorityPublic},
		Threshold:   1, AcceptedProfile: "h3-route-tracer-v1", Now: value.now})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := opened.Accept(context.Background(), epoch.Raw, epoch.Inputs, epoch.Materials); err != nil {
		t.Fatal(err)
	}
	view, err := opened.Current()
	if err != nil {
		t.Fatal(err)
	}
	value.snapshot = view
	value.plan, err = route.Select(view, route.Selection{Seed: value.selectionSeed, At: value.now})
	if err != nil {
		t.Fatal(err)
	}
	if err := opened.Close(); err != nil {
		t.Fatal(err)
	}
	value.qualification = qualification.Case{NetworkID: value.network, Generation: value.plan.Generation,
		Epoch: value.plan.Epoch, EpochDigest: value.epochDigest, Profile: value.plan.Profile, ViewRoot: value.plan.ViewRoot,
		SelectionSeed: value.selectionSeed, SelectionAt: value.now.Unix(), ClientPin: value.identities[5].public,
		PublisherID: value.publisherID}
	for index, position := range value.plan.Positions {
		candidate := value.snapshot.Candidates[index]
		value.qualification.Candidates = append(value.qualification.Candidates, qualification.Candidate{
			NodeID: candidate.NodeID, PublicKey: candidate.PublicKey, Family: candidate.Family,
			Endpoint: candidate.Endpoint, Domain: candidate.Domain, Capacity: candidate.Capacity,
			ValidFrom: candidate.ValidFrom.Unix(), ValidUntil: candidate.ValidUntil.Unix()})
		value.qualification.NodeIDs[index], value.qualification.PublicKeys[index] = position.NodeID, position.PublicKey
		value.qualification.Families[index], value.qualification.Endpoints[index] = position.Family, position.Endpoint
	}
	value.qualification.ManifestDigest = qualification.Commit(value.qualification)
	return value
}

func familyFor(t *testing.T, network, seed [32]byte, wanted string, domains []string) string {
	t.Helper()
	for index := range 10_000 {
		family := fmt.Sprintf("%s-family-%d", wanted, index)
		selected, err := assignment.Select(network, 1, seed, family, domains)
		if err != nil {
			t.Fatal(err)
		}
		if selected == wanted {
			return family
		}
	}
	t.Fatal("could not derive a family for the required Route domain")
	return ""
}

func writeIdentity(t *testing.T, root string, marker byte, now time.Time) processIdentity {
	t.Helper()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{marker}, ed25519.SeedSize))
	public := private.Public().(ed25519.PublicKey)
	template := &x509.Certificate{SerialNumber: big.NewInt(int64(marker)), Subject: pkix.Name{CommonName: "route.process"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}}
	certificate, err := x509.CreateCertificate(nil, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	encodedKey, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	certPath, keyPath := filepath.Join(root, fmt.Sprintf("identity-%d-cert.pem", marker)), filepath.Join(root, fmt.Sprintf("identity-%d-key.pem", marker))
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encodedKey}), 0o600); err != nil {
		t.Fatal(err)
	}
	var fixed [32]byte
	copy(fixed[:], public)
	return processIdentity{private: private, public: fixed, cert: certPath, key: keyPath}
}

func unusedProcessAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}

func hex32(value [32]byte) string { return hex.EncodeToString(value[:]) }
