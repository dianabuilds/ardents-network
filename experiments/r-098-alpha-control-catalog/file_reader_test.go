//go:build ignore

package main

import (
	"crypto/ed25519"
	"encoding/json"
	"testing"
	"time"
)

func TestBoundedCatalogDirectoryAcceptsExactSignedFiles(t *testing.T) {
	directory, config := validFileFixture(t)
	report := readCatalogDirectory(directory, config)
	if report["release"] != "accepted" || report["network-profile"] != "accepted" {
		t.Fatalf("report = %#v, want both components accepted", report)
	}
}

func TestBoundedCatalogDirectoryRejectsChangedComponent(t *testing.T) {
	directory, config := validFileFixture(t)
	if err := writeFixtureFiles(directory, map[string][]byte{"release.json": []byte("changed")}); err != nil {
		t.Fatal(err)
	}
	report := readCatalogDirectory(directory, config)
	if report["release"] != "digest-mismatch" || report["network-profile"] != "accepted" {
		t.Fatalf("report = %#v, want isolated release digest failure", report)
	}
}

func TestBoundedCatalogDirectoryRejectsOversizedCatalogBeforeDecode(t *testing.T) {
	directory, config := validFileFixture(t)
	oversized := make([]byte, maximumCatalogBytes+1)
	if err := writeFixtureFiles(directory, map[string][]byte{"catalog.json": oversized}); err != nil {
		t.Fatal(err)
	}
	report := readCatalogDirectory(directory, config)
	if report["catalog"] != "too-large" {
		t.Fatalf("report = %#v, want oversized catalog rejection", report)
	}
}

func TestDecodeStrictRejectsDuplicateObjectKeys(t *testing.T) {
	var envelope fileEnvelope
	err := decodeStrict([]byte(`{"body_b64":"one","body_b64":"two","signature_b64":"three"}`), &envelope)
	if err == nil {
		t.Fatal("duplicate JSON keys were accepted")
	}
}

func TestBoundedCatalogDirectoryRejectsDuplicateEnvelopeKey(t *testing.T) {
	directory, config := validFileFixture(t)
	valid, err := readBoundedRegularFile(directory+"/catalog.json", maximumCatalogBytes)
	if err != nil {
		t.Fatal(err)
	}
	var envelope fileEnvelope
	if err := json.Unmarshal(valid, &envelope); err != nil {
		t.Fatal(err)
	}
	duplicate := []byte(`{"body_b64":"` + envelope.Body + `","body_b64":"` + envelope.Body + `","signature_b64":"` + envelope.Signature + `"}`)
	if err := writeFixtureFiles(directory, map[string][]byte{"catalog.json": duplicate}); err != nil {
		t.Fatal(err)
	}
	report := readCatalogDirectory(directory, config)
	if report["catalog"] != "invalid" {
		t.Fatalf("report = %#v, want duplicate envelope key rejection", report)
	}
}

func validFileFixture(t *testing.T) (string, fileReaderConfig) {
	t.Helper()
	directory := t.TempDir()
	disclosurePublic, disclosurePrivate := fileKeypair()
	releasePublic, releasePrivate := fileKeypair()
	networkPublic, networkPrivate := fileKeypair()
	config := fileReaderConfig{
		DisclosureKey: disclosurePublic,
		ComponentKeys: map[string]ed25519.PublicKey{
			"release-key": releasePublic,
			"network-key": networkPublic,
		},
		Floors:       map[string]uint64{"release": 3, "network-profile": 5},
		CatalogFloor: 4,
		Now:          time.Unix(1_893_456_000, 0),
	}
	files := map[string][]byte{
		"release.json": makeEnvelope(encode(fileComponent{
			Class: "release", SignerID: "release-key", Version: 3,
			ExpiresAt: config.Now.Add(time.Hour).Unix(), Payload: "release-v3",
		}), releasePrivate),
		"network-profile.json": makeEnvelope(encode(fileComponent{
			Class: "network-profile", SignerID: "network-key", Version: 5,
			ExpiresAt: config.Now.Add(time.Hour).Unix(), Payload: "network-v5",
		}), networkPrivate),
	}
	files["catalog.json"] = makeEnvelope(encode(fileCatalog{
		Cohort: "alpha-1", Revision: 4, ExpiresAt: config.Now.Add(time.Hour).Unix(),
		Entries: []descriptor{
			{Class: "release", SignerID: "release-key", ComponentSHA256: digest(files["release.json"])},
			{Class: "network-profile", SignerID: "network-key", ComponentSHA256: digest(files["network-profile.json"])},
		},
	}), disclosurePrivate)
	if err := writeFixtureFiles(directory, files); err != nil {
		t.Fatal(err)
	}
	return directory, config
}
