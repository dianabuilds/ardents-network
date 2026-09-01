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
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/alphacontrol"
	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
)

func TestRetiredPlanningCampaignRoutesAreNotCommandSurface(t *testing.T) {
	const usage = "usage: ardents-control inspect-bundle, inspect-transitions, inspect-alpha-corpus, or accept-alpha-corpus"
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
