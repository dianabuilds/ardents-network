package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

func TestRetiredPlanningCampaignRoutesAreNotCommandSurface(t *testing.T) {
	const usage = "usage: ardents-control inspect-bundle, inspect-transitions, prepare-closed-profile, sign-closed-profile, inspect-closed-profile, or inspect-closed-issuer-profile"
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

func TestAcceptAlphaCorpusIsRetiredBeforeEffects(t *testing.T) {
	directory := t.TempDir()
	controlRoot := filepath.Join(directory, "control")
	corpusRoot := filepath.Join(directory, "corpus")
	var output bytes.Buffer
	err := run([]string{"accept-alpha-corpus", "--control-state-root", controlRoot, "--corpus-state-root", corpusRoot}, &output)
	if err == nil || err.Error() != "accept-alpha-corpus is retired" {
		t.Fatalf("retired alpha corpus intake error = %v", err)
	}
	if output.Len() != 0 {
		t.Fatalf("retired alpha corpus intake output = %q", output.String())
	}
	for _, root := range []string{controlRoot, corpusRoot} {
		if _, statErr := os.Stat(root); !os.IsNotExist(statErr) {
			t.Fatalf("retired alpha corpus intake created %s: %v", root, statErr)
		}
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
	exportPath := filepath.Join(t.TempDir(), "issuer.json")
	export, err := json.Marshal(map[string]any{"schema": "ardents-closed-issuer-profile-v1", "profile": issuer.Profile,
		"profile_sha256": hex.EncodeToString(issuer.ProfileDigest[:])})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exportPath, export, 0o600); err != nil {
		t.Fatal(err)
	}
	var inventory bytes.Buffer
	if err := run([]string{"inspect-closed-issuer-profile", "--profile", exportPath,
		"--network", hex.EncodeToString(network[:]), "--node", hex.EncodeToString(issuerNode[:]),
		"--node-key", hex.EncodeToString(nodePrivate.Public().(ed25519.PublicKey))}, &inventory); err != nil {
		t.Fatal(err)
	}
	var issuerPlan closedProfilePlan
	if err := json.Unmarshal(inventory.Bytes(), &issuerPlan); err != nil || len(issuerPlan.TokenKeys) != 3 {
		t.Fatalf("issuer inventory = %q / %v", inventory.String(), err)
	}
	plan := closedProfilePlan{NetworkID: hex.EncodeToString(network[:]), StateGeneration: hex.EncodeToString(bytes.Repeat([]byte{5}, 32)),
		EpochDigest: hex.EncodeToString(bytes.Repeat([]byte{6}, 32)), Epoch: 7, IssuerNodeID: hex.EncodeToString(issuerNode[:]),
		IssuanceAuthorityKey: hex.EncodeToString(bytes.Repeat([]byte{7}, 32)), NotBefore: now.Format(time.RFC3339), NotAfter: now.Add(time.Hour).Format(time.RFC3339),
		Nodes: []closedProfilePlanNode{{NodeID: hex.EncodeToString(otherNode[:]), RecordDigest: hex.EncodeToString(bytes.Repeat([]byte{8}, 32)), RoleDomain: 1, Subrole: 1, DutyGeneration: 1},
			{NodeID: hex.EncodeToString(issuerNode[:]), RecordDigest: hex.EncodeToString(bytes.Repeat([]byte{9}, 32)), RoleDomain: 2, Subrole: 6, DutyGeneration: 2}}}
	plan.TokenKeys = issuerPlan.TokenKeys
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

func TestInspectAlphaCorpusIsRetiredBeforeEffects(t *testing.T) {
	directory := t.TempDir()
	// Retained ACA1 inspection and Alpha Corpus floor roots, plus hostile
	// supplied-byte files, must survive the retired route untouched.
	aca1Root := filepath.Join(directory, "aca1-inspection")
	corpusRoot := filepath.Join(directory, "corpus-floor")
	for _, root := range []string{aca1Root, corpusRoot} {
		if err := os.MkdirAll(root, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "floor.bin"), []byte("retained floor"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	hostileCatalog := filepath.Join(directory, "catalog.ac2")
	hostileCorpus := filepath.Join(directory, "corpus.anc")
	if err := os.WriteFile(hostileCatalog, []byte("ACA2 hostile catalog"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hostileCorpus, []byte("hostile corpus"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Nonexistent and hostile path forms reach the identical refusal: the
	// retired route parses no argument and opens no file or root (ADR-0110).
	for _, paths := range [][2]string{
		{filepath.Join(directory, "absent-catalog"), filepath.Join(directory, "absent-corpus")},
		{hostileCatalog, hostileCorpus},
	} {
		var output bytes.Buffer
		err := run([]string{"inspect-alpha-corpus", "--catalog", paths[0], "--corpus", paths[1],
			"--disclosure-key", "00", "--corpus-key", "00", "--network", "00",
			"--state-root", aca1Root, "--at", "2026-09-27T00:00:00Z"}, &output)
		if err == nil || err.Error() != "inspect-alpha-corpus is retired" {
			t.Fatalf("retired alpha corpus inspection error = %v", err)
		}
		if output.Len() != 0 {
			t.Fatalf("retired alpha corpus inspection output = %q", output.String())
		}
	}
	for path, retained := range map[string]string{
		filepath.Join(aca1Root, "floor.bin"):   "retained floor",
		filepath.Join(corpusRoot, "floor.bin"): "retained floor",
		hostileCatalog:                         "ACA2 hostile catalog",
		hostileCorpus:                          "hostile corpus",
	} {
		content, err := os.ReadFile(path)
		if err != nil || string(content) != retained {
			t.Fatalf("retired inspection changed %s: %q, %v", path, content, err)
		}
	}
	for _, root := range []string{aca1Root, corpusRoot} {
		entries, err := os.ReadDir(root)
		if err != nil || len(entries) != 1 {
			t.Fatalf("retired inspection changed %s contents: %+v, %v", root, entries, err)
		}
	}
}
