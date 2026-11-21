//go:build linux

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

type endpointConstructionSourceMemberFile struct {
	Address        string `json:"address"`
	ServerName     string `json:"server_name"`
	Identity       string `json:"identity"`
	Family         string `json:"family"`
	EndpointHandle string `json:"endpoint_handle"`
	RootCA         string `json:"root_ca"`
	LeafKeyDigest  string `json:"leaf_key_digest"`
}

type endpointConstructionSourceFile struct {
	Schema               string                                 `json:"schema"`
	ClockObservedAt      string                                 `json:"clock_observed_at"`
	OrderSeed            string                                 `json:"order_seed"`
	MaterializationIndex uint32                                 `json:"materialization_index"`
	RefreshIntervalMS    uint32                                 `json:"refresh_interval_ms"`
	ClientCertificate    string                                 `json:"client_certificate"`
	ClientKey            string                                 `json:"client_key"`
	Sources              []endpointConstructionSourceMemberFile `json:"sources"`
}

// constructionTokenKeys generates the genuine admission token-key cohorts the
// accepted closed profile's cohorts validator requires: three distinct
// classes per full hour window.
func constructionTokenKeys(t *testing.T, window time.Time, windows int) []networkfixture.ClosedTokenKey {
	t.Helper()
	type algorithm struct {
		OID    asn1.ObjectIdentifier
		Params asn1.RawValue
	}
	type parameters struct {
		Hash algorithm `asn1:"explicit,tag:0"`
		MGF  algorithm `asn1:"explicit,tag:1"`
		Salt int       `asn1:"explicit,tag:2"`
	}
	type subject struct {
		Algorithm algorithm
		Bits      asn1.BitString
	}
	hash := algorithm{asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 2}, asn1.RawValue{FullBytes: []byte{5, 0}}}
	hashBytes, err := asn1.Marshal(hash)
	if err != nil {
		t.Fatal(err)
	}
	params, err := asn1.Marshal(parameters{hash, algorithm{asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 8}, asn1.RawValue{FullBytes: hashBytes}}, 48})
	if err != nil {
		t.Fatal(err)
	}
	keys := make([]networkfixture.ClosedTokenKey, 0, 3*windows)
	for i := range 3 * windows {
		n := new(big.Int).Lsh(big.NewInt(1), 2047)
		n.Add(n, big.NewInt(int64(2*i+1)))
		raw := x509.MarshalPKCS1PublicKey(&rsa.PublicKey{N: n, E: 65537})
		encoded, err := asn1.Marshal(subject{algorithm{asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 10}, asn1.RawValue{FullBytes: params}}, asn1.BitString{Bytes: raw, BitLength: len(raw) * 8}})
		if err != nil {
			t.Fatal(err)
		}
		keys = append(keys, networkfixture.ClosedTokenKey{WindowStart: window.Add(time.Duration(i/3) * time.Hour), Class: uint8(i%3 + 1), SPKI: encoded})
	}
	return keys
}

// constructionCommandFixture prepares one independently signed accepted
// reference State, the distributed Source plan artifact and the operator's
// bounded declaration.
func constructionCommandFixture(t *testing.T) (ownerPath, sourcePath, declarationPath string, networkID, pins [32]byte) {
	t.Helper()
	base := t.TempDir()
	authority := ed25519.NewKeyFromSeed(make([]byte, 32))
	nodeSeed := [32]byte{1}
	nodeKey := ed25519.NewKeyFromSeed(nodeSeed[:])
	networkID = [32]byte{1}
	issuerAuthority := [32]byte{5}
	nodeID := [32]byte{6}
	window := time.Now().UTC().Truncate(time.Hour)
	spec := networkfixture.ClosedSpec{NetworkID: networkID, Seed: [32]byte{7},
		IssuanceAuthority: issuerAuthority, Number: 1, NotBefore: window, NotAfter: window.Add(2 * time.Hour),
		Authority: authority, Nodes: []networkfixture.ClosedNode{{RecordSpec: networkfixture.RecordSpec{NodeID: nodeID,
			Generation: 1, ValidFrom: window, ValidUntil: window.Add(2 * time.Hour), Endpoint: "127.0.0.1:4401",
			Carrier: "ardents-carrier-tcp-tls-v2", Capability: 2, Capacity: 3, PrivateKey: nodeKey}, RoleDomain: 2, Subrole: 6}}}
	spec.Keys = constructionTokenKeys(t, window, 2)
	signed, err := networkfixture.BuildClosed(spec)
	if err != nil {
		t.Fatal(err)
	}
	public := authority.Public().(ed25519.PublicKey)
	copy(pins[:], public)
	root := filepath.Join(base, "reference-state")
	clock := filepath.Join(base, "reference-clock")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(clock, nil, 0644); err != nil {
		t.Fatal(err)
	}
	config := state.Config{Root: root, NetworkID: networkID,
		Authorities: map[[32]byte]ed25519.PublicKey{sha256.Sum256(public): public}, Threshold: 1,
		ClosedProfileAuthority: public, AcceptedProfile: "ardents-route-v3", Clock: time.Now, ClockObservationFile: clock}
	owner, err := state.OpenFresh(config)
	if err != nil {
		t.Fatal(err)
	}
	_, err = owner.Accept(context.Background(), signed.Epoch.Raw, signed.Epoch.Inputs, signed.Epoch.Materials)
	if err == nil {
		_, err = owner.AcceptClosedProfile(signed.Profile)
	}
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	write := func(name string, body []byte) string {
		path := filepath.Join(base, name)
		if err := os.WriteFile(path, body, 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	ownerPlan := networkAuthorityPlan{Root: root, NetworkID: networkID, Authorities: [][32]byte{pins},
		Threshold: 1, ProfileAuthority: pins, ClockObservationFile: clock}
	ownerRaw, err := json.Marshal(ownerPlan)
	if err != nil {
		t.Fatal(err)
	}
	_, clientPEM, clientKey, _ := installedSourceCertificate(t, "client.construction.test", false)
	declared := endpointConstructionSourceFile{Schema: "ardents-source-plan-v1",
		ClockObservedAt: time.Now().UTC().Truncate(time.Second).Format(time.RFC3339),
		OrderSeed:       hex.EncodeToString(bytes.Repeat([]byte{5}, 32)), MaterializationIndex: 3, RefreshIntervalMS: 30000,
		ClientCertificate: write("client.pem", clientPEM), ClientKey: write("client.key", clientKey)}
	identity := func(n byte) string { value := [32]byte{n}; return hex.EncodeToString(value[:]) }
	for _, member := range []struct {
		name    string
		address string
		node    byte
		serial  string
	}{
		{"source-one.construction.test", "127.0.0.1:9401", 71, "one"},
		{"source-two.construction.test", "127.0.0.1:9402", 72, "two"},
	} {
		_, certificate, _, pin := installedSourceCertificate(t, member.name, true)
		rootPath := write("root-"+member.serial+".pem", certificate)
		declared.Sources = append(declared.Sources, endpointConstructionSourceMemberFile{Address: member.address,
			ServerName: member.name, Identity: identity(member.node), Family: "source-" + member.serial,
			EndpointHandle: "source-" + member.serial, RootCA: rootPath, LeafKeyDigest: hex.EncodeToString(pin[:])})
	}
	sourceRaw, err := json.Marshal(declared)
	if err != nil {
		t.Fatal(err)
	}
	declaration := endpointPlanDeclaration{
		NetworkStateRoot:        filepath.Join(base, "state"),
		LocalRoleStateRoot:      filepath.Join(base, "participation"),
		TextTokenRoot:           filepath.Join(base, "stock"),
		EntryStateRoot:          filepath.Join(base, "route"),
		PublicationRoot:         filepath.Join(base, "publication"),
		ServiceInstanceRoot:     filepath.Join(base, "instance"),
		TimeConfidenceFile:      filepath.Join(base, "clock"),
		ApplicationSocket:       filepath.Join(base, "application.sock"),
		AdministrationSocket:    filepath.Join(base, "administration.sock"),
		BrokerID:                hex.EncodeToString(bytes.Repeat([]byte{11}, 32)),
		ConnectionPrincipal:     hex.EncodeToString(bytes.Repeat([]byte{12}, 32)),
		AdministrationPrincipal: hex.EncodeToString(bytes.Repeat([]byte{13}, 32)),
		ReaderPermission:        endpointPermissionPlanInput{RequestPath: filepath.Join(base, "reader-request"), ResponsePath: filepath.Join(base, "reader-response"), Maxima: [3]uint32{1, 1, 1}},
		PublisherPermission:     endpointPermissionPlanInput{RequestPath: filepath.Join(base, "publisher-request"), ResponsePath: filepath.Join(base, "publisher-response"), Maxima: [3]uint32{1, 1, 1}},
	}
	declarationRaw, err := json.Marshal(declaration)
	if err != nil {
		t.Fatal(err)
	}
	return write("network-owner.json", ownerRaw), write("source-plan.json", sourceRaw), write("declaration.json", declarationRaw), networkID, pins
}

func TestEndpointConstructPlanCommandRendersFiles(t *testing.T) {
	ownerPath, sourcePath, declarationPath, networkID, pins := constructionCommandFixture(t)
	output := t.TempDir()
	var diagnostic bytes.Buffer
	code := runInstalledEndpoint(context.Background(), []string{"construct-plan", ownerPath, sourcePath, declarationPath, output}, &diagnostic)
	if code != 0 {
		t.Fatalf("construct-plan failed: %d %s", code, diagnostic.String())
	}
	raw, err := os.ReadFile(filepath.Join(output, "headless.json"))
	if err != nil {
		t.Fatal(err)
	}
	var plan endpoint.HeadlessPlan
	if err := json.Unmarshal(raw, &plan); err != nil || plan.Schema != "ardents-headless-runtime-v2" {
		t.Fatalf("rendered headless plan is unavailable: %v %+v", err, plan)
	}
	if plan.NetworkID != hex.EncodeToString(networkID[:]) || len(plan.NetworkAuthorities) != 1 ||
		plan.NetworkAuthorities[0] != hex.EncodeToString(pins[:]) || plan.ClosedProfileAuthority != hex.EncodeToString(pins[:]) ||
		plan.NetworkThreshold != 1 {
		t.Fatalf("rendered plan does not carry the authenticated pins: %+v", plan)
	}
	sourceRaw, err := os.ReadFile(filepath.Join(output, "source.json"))
	if err != nil {
		t.Fatal(err)
	}
	var sourcePlan endpoint.SourcePlan
	if err := json.Unmarshal(sourceRaw, &sourcePlan); err != nil || sourcePlan.Schema != "ardents-source-plan-v1" || len(sourcePlan.Sources) != 2 {
		t.Fatalf("rendered source plan is unavailable: %v", err)
	}
}

func TestEndpointConstructPlanCommandRefusals(t *testing.T) {
	ownerPath, sourcePath, declarationPath, _, _ := constructionCommandFixture(t)
	if code := runInstalledEndpoint(context.Background(), []string{"construct-plan", ownerPath, sourcePath}, &bytes.Buffer{}); code != 2 {
		t.Fatalf("usage refusal expected 2, got %d", code)
	}
	// A substituted authority set must not authenticate the accepted history.
	base := filepath.Dir(ownerPath)
	wrong := networkAuthorityPlan{Root: filepath.Join(base, "reference-state"), NetworkID: [32]byte{9},
		Authorities: [][32]byte{{8}}, Threshold: 1, ProfileAuthority: [32]byte{8},
		ClockObservationFile: filepath.Join(base, "reference-clock")}
	wrongRaw, err := json.Marshal(wrong)
	if err != nil {
		t.Fatal(err)
	}
	wrongPath := filepath.Join(base, "wrong-owner.json")
	if err := os.WriteFile(wrongPath, wrongRaw, 0600); err != nil {
		t.Fatal(err)
	}
	output := t.TempDir()
	if code := runInstalledEndpoint(context.Background(), []string{"construct-plan", wrongPath, sourcePath, declarationPath, output}, &bytes.Buffer{}); code != 1 {
		t.Fatalf("unauthenticated authority set accepted: %d", code)
	}
	if _, err := os.Stat(filepath.Join(output, "headless.json")); !os.IsNotExist(err) {
		t.Fatal("refused construction rendered plan files")
	}
	// An unknown declaration field refuses the bounded grammar.
	declarationRaw, err := os.ReadFile(declarationPath)
	if err != nil {
		t.Fatal(err)
	}
	foreign := append(declarationRaw[:len(declarationRaw)-1], []byte(`, "unknown": 1}`)...)
	foreignPath := filepath.Join(base, "foreign-declaration.json")
	if err := os.WriteFile(foreignPath, foreign, 0600); err != nil {
		t.Fatal(err)
	}
	if code := runInstalledEndpoint(context.Background(), []string{"construct-plan", ownerPath, sourcePath, foreignPath, output}, &bytes.Buffer{}); code != 1 {
		t.Fatalf("unknown declaration field accepted: %d", code)
	}
	// Existing output bytes are never adopted or overwritten.
	if err := os.WriteFile(filepath.Join(output, "headless.json"), []byte("earlier attempt"), 0600); err != nil {
		t.Fatal(err)
	}
	if code := runInstalledEndpoint(context.Background(), []string{"construct-plan", ownerPath, sourcePath, declarationPath, output}, &bytes.Buffer{}); code != 1 {
		t.Fatalf("existing plan output accepted: %d", code)
	}
	raw, err := os.ReadFile(filepath.Join(output, "headless.json"))
	if err != nil || string(raw) != "earlier attempt" {
		t.Fatalf("existing plan output was modified: %q %v", raw, err)
	}
}
