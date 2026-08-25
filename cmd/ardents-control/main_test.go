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
