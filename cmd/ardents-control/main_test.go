package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/alphacontrol"
	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

func TestRetiredPlanningCampaignRoutesAreNotCommandSurface(t *testing.T) {
	const usage = "usage: ardents-control inspect-bundle, inspect-transitions, inspect-alpha-corpus, accept-alpha-corpus, prepare-closed-profile, sign-closed-profile, or inspect-closed-profile"
	for _, route := range []string{
		"inspect",
		"inspect-public-control",
		"simulate-public-control",
		"simulate-public-control-transitions",
		"simulate-namespace-lifecycle",
		"simulate-root-claims",
	} {
		t.Run(route, func(t *testing.T) {
			var output bytes.Buffer
			if err := run([]string{route}, &output); err == nil || err.Error() != usage {
				t.Fatalf("retired route error = %v", err)
			}
			if output.Len() != 0 {
				t.Fatalf("retired route output = %q", output.String())
			}
		})
	}
}

func TestInspectTransitionsNamesItsInvalidArguments(t *testing.T) {
	var output bytes.Buffer
	err := run([]string{"inspect-transitions"}, &output)
	if err == nil || !strings.Contains(err.Error(), "inspect-transitions") {
		t.Fatalf("inspect-transitions invalid arguments = %v", err)
	}
}

func TestClosedProfileCommandsRoundTripWithoutKeyOutput(t *testing.T) {
	now := time.Unix(2_000_401_600, 0).UTC().Truncate(time.Hour)
	network, issuerNode, otherNode := [32]byte{1}, [32]byte{2}, [32]byte{1}
	statePrivate := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{3}, ed25519.SeedSize))
	nodePrivate := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{4}, ed25519.SeedSize))
	issuer, err := credential.InitializeClosedIssuerRoot(credential.ClosedIssuerRootConfig{Root: filepath.Join(t.TempDir(), "issuer"),
		NetworkID: network, NodeID: issuerNode, IdentityKey: nodePrivate, NotBefore: now, NotAfter: now.Add(time.Hour), Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	issuerProfile, err := credential.DecodeClosedIssuerProfile(issuer.Profile, nodePrivate.Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatal(err)
	}
	plan := closedProfilePlan{NetworkID: hex.EncodeToString(network[:]), StateGeneration: hex.EncodeToString(bytes.Repeat([]byte{5}, 32)),
		EpochDigest: hex.EncodeToString(bytes.Repeat([]byte{6}, 32)), Epoch: 7, IssuerNodeID: hex.EncodeToString(issuerNode[:]),
		IssuanceAuthorityKey: hex.EncodeToString(bytes.Repeat([]byte{7}, 32)), NotBefore: now.Format(time.RFC3339), NotAfter: now.Add(time.Hour).Format(time.RFC3339),
		Nodes: []closedProfilePlanNode{{NodeID: hex.EncodeToString(otherNode[:]), RecordDigest: hex.EncodeToString(bytes.Repeat([]byte{8}, 32)), RoleDomain: 1, Subrole: 1, DutyGeneration: 1},
			{NodeID: hex.EncodeToString(issuerNode[:]), RecordDigest: hex.EncodeToString(bytes.Repeat([]byte{9}, 32)), RoleDomain: 2, Subrole: 6, DutyGeneration: 2}}}
	for _, key := range issuerProfile.Keys {
		plan.TokenKeys = append(plan.TokenKeys, closedProfilePlanKey{WindowStart: key.WindowStart.Format(time.RFC3339), Class: uint8(key.Class), SPKI: base64.RawStdEncoding.EncodeToString(key.SPKI)})
	}
	directory := t.TempDir()
	planPath, preparedPath, signedPath, keyPath := filepath.Join(directory, "plan.json"), filepath.Join(directory, "prepared.bin"), filepath.Join(directory, "signed.bin"), filepath.Join(directory, "state.pem")
	planRaw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(planPath, planRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(statePrivate)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	var prepared, signed, inspected bytes.Buffer
	if err := run([]string{"prepare-closed-profile", "--plan", planPath, "--output", preparedPath}, &prepared); err != nil {
		t.Fatalf("prepare closed profile: %v", err)
	}
	if err := run([]string{"sign-closed-profile", "--plan", planPath, "--authority-key", keyPath, "--output", signedPath}, &signed); err != nil {
		t.Fatalf("sign closed profile: %v", err)
	}
	if err := run([]string{"inspect-closed-profile", "--plan", planPath, "--profile", signedPath, "--authority", hex.EncodeToString(statePrivate.Public().(ed25519.PublicKey)), "--at", now.Format(time.RFC3339)}, &inspected); err != nil {
		t.Fatalf("inspect closed profile: %v", err)
	}
	if prepared.Len() == 0 || signed.Len() == 0 || !strings.Contains(inspected.String(), "ardents-closed-profile-inspection-v1") ||
		bytes.Contains(signed.Bytes(), statePrivate) || bytes.Contains(inspected.Bytes(), statePrivate) {
		t.Fatalf("closed profile command outputs leaked key or lacked report: %q / %q / %q", prepared.String(), signed.String(), inspected.String())
	}
}

func TestInspectAlphaCorpusPinsACA2WithoutOpeningEndpointFloor(t *testing.T) {
	now := time.Unix(2_000_400_000, 0).UTC()
	network := [32]byte{1}
	disclosurePublic, disclosurePrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	corpusPublic, corpusPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	link, err := alpha.ParseServiceLink("ardents-alpha://blog.alice")
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := alpha.IssueCorpus(alpha.CorpusInput{Cohort: "alpha-one", Network: network, Serial: 4,
		Bindings: []alpha.BindingInput{{Link: link, Target: [32]byte{9}}}, NotBefore: now.Add(-time.Second), NotAfter: now.Add(time.Minute)}, corpusPrivate)
	if err != nil {
		t.Fatal(err)
	}
	catalog := alphacontrol.CatalogV2{Cohort: "alpha-one", Generation: 1, NotBefore: now.Add(-time.Second), NotAfter: now.Add(time.Minute)}
	for index := range catalog.Components[:3] {
		body := []byte{byte(index + 1)}
		catalog.Components[index] = alphacontrol.Component{Class: alphacontrol.ComponentClass(index + 1), RootID: [32]byte{byte(index + 1)},
			Generation: 1, NotAfter: now.Add(time.Minute), Size: uint32(len(body)), Digest: sha256.Sum256(body)}
	}
	catalog.Components[3] = alphacontrol.Component{Class: alphacontrol.ComponentCorpus, RootID: sha256.Sum256(corpusPublic), Generation: 4,
		NotAfter: now.Add(time.Minute), Size: uint32(len(corpus)), Digest: sha256.Sum256(corpus)}
	catalogRaw, err := signCatalogV2Fixture(catalog, disclosurePrivate)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	catalogPath, corpusPath := filepath.Join(directory, "catalog.ac2"), filepath.Join(directory, "corpus.anc")
	if err := os.WriteFile(catalogPath, catalogRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(corpusPath, corpus, 0o600); err != nil {
		t.Fatal(err)
	}
	floorRoot := filepath.Join(directory, "floor")
	var output bytes.Buffer
	arguments := []string{"inspect-alpha-corpus", "--catalog", catalogPath, "--corpus", corpusPath,
		"--disclosure-key", hex.EncodeToString(disclosurePublic), "--corpus-key", hex.EncodeToString(corpusPublic), "--network", hex.EncodeToString(network[:]), "--at", now.Format(time.RFC3339)}
	if err := run(arguments, &output); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(floorRoot); !os.IsNotExist(err) {
		t.Fatalf("diagnostic command created Endpoint floor: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(output.Bytes(), &fields); err != nil {
		t.Fatalf("alpha corpus report is not JSON: %s, %v", output.String(), err)
	}
	for _, field := range []string{"schema", "cohort", "corpus", "network", "serial"} {
		if _, ok := fields[field]; !ok {
			t.Fatalf("alpha corpus report has no lowercase %q field: %s", field, output.String())
		}
	}
	var report struct {
		Corpus string `json:"corpus"`
		Serial uint64 `json:"serial"`
	}
	if err := json.Unmarshal(output.Bytes(), &report); err != nil || report.Corpus != "accepted" || report.Serial != 4 {
		t.Fatalf("alpha corpus report = %s, %v", output.String(), err)
	}
	if err := run(append(arguments, "--state-root", floorRoot), &bytes.Buffer{}); err == nil {
		t.Fatal("diagnostic command accepted an Endpoint-owned state root")
	}
	if _, err := os.Stat(floorRoot); !os.IsNotExist(err) {
		t.Fatalf("rejected diagnostic command changed Endpoint floor: %v", err)
	}
}
