package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/alphacontrol"
	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
)

func TestInspectReportsSeparateComponentResultsWithoutEndpoint(t *testing.T) {
	now := time.Unix(2_000_400_000, 0).UTC()
	directory, stateRoot := t.TempDir(), filepath.Join(t.TempDir(), "reader")
	disclosurePublic, disclosurePrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	components, roots, catalog := controlFixture(t, now, disclosurePrivate)
	if err := os.WriteFile(filepath.Join(directory, "catalog.ac1"), catalog, 0o600); err != nil {
		t.Fatal(err)
	}
	for index, name := range componentNames {
		if err := os.WriteFile(filepath.Join(directory, name), components[index], 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var output bytes.Buffer
	arguments := []string{"inspect", "--directory", directory, "--state-root", stateRoot, "--disclosure-key", hex.EncodeToString(disclosurePublic),
		"--release-key", hex.EncodeToString(roots[0]), "--network-key", hex.EncodeToString(roots[1]), "--compatibility-key", hex.EncodeToString(roots[2]), "--at", now.Format(time.RFC3339)}
	if err := run(arguments, &output); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Catalog    string `json:"catalog"`
		Components [3]struct {
			Outcome string `json:"outcome"`
		} `json:"components"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil || result.Catalog != "accepted" {
		t.Fatalf("inspection result = %s, %v", output.String(), err)
	}
	for _, component := range result.Components {
		if component.Outcome != "accepted" {
			t.Fatalf("component result = %+v", component)
		}
	}
}

func TestInspectAlphaCorpusPinsACA2AndPersistsItsFloor(t *testing.T) {
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
	catalogRaw, err := alphacontrol.SignV2(catalog, disclosurePrivate)
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
	var output bytes.Buffer
	arguments := []string{"inspect-alpha-corpus", "--catalog", catalogPath, "--corpus", corpusPath, "--state-root", filepath.Join(directory, "floor"),
		"--disclosure-key", hex.EncodeToString(disclosurePublic), "--corpus-key", hex.EncodeToString(corpusPublic), "--network", hex.EncodeToString(network[:]), "--at", now.Format(time.RFC3339)}
	if err := run(arguments, &output); err != nil {
		t.Fatal(err)
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
}

func controlFixture(t *testing.T, now time.Time, catalogSigner ed25519.PrivateKey) ([3][]byte, [3]ed25519.PublicKey, []byte) {
	t.Helper()
	var components [3][]byte
	var roots [3]ed25519.PublicKey
	catalog := alphacontrol.Catalog{Cohort: "alpha-one", Generation: 1, NotBefore: now.Add(-time.Second), NotAfter: now.Add(time.Minute)}
	for index := range components {
		public, private, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		statement := alphacontrol.ComponentStatement{Class: alphacontrol.ComponentClass(index + 1), Generation: 1,
			NotBefore: now.Add(-time.Second), NotAfter: now.Add(time.Minute), Body: []byte{byte(index + 1)}}
		components[index], err = alphacontrol.SignComponent(statement, private)
		if err != nil {
			t.Fatal(err)
		}
		roots[index] = public
		catalog.Components[index] = alphacontrol.Component{Class: statement.Class, RootID: sha256.Sum256(public), Generation: 1,
			NotAfter: statement.NotAfter, Size: uint32(len(components[index])), Digest: sha256.Sum256(components[index])}
	}
	raw, err := alphacontrol.Sign(catalog, catalogSigner)
	if err != nil {
		t.Fatal(err)
	}
	return components, roots, raw
}
