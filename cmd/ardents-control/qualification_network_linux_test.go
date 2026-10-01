package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/epoch"
)

func TestQualificationNetworkCommandsProduceOrdinaryVerifiedInitialStateInputs(t *testing.T) {
	root := t.TempDir()
	now := time.Unix(2_000_400_000, 0).UTC()
	network := [32]byte{9}
	_, nodeKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_, authority, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	record := epoch.InitialClosedRecord{NetworkID: network, NodeID: [32]byte{1}, ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(2 * time.Hour), Family: "command-family", Endpoint: "127.0.0.1:4499", Carrier: epoch.CarrierClosedQUIC, Capacity: 1, PublicKey: nodeKey.Public().(ed25519.PublicKey)}
	planPath := filepath.Join(root, "record.json")
	plan, err := json.Marshal(struct {
		Schema string                    `json:"schema"`
		Record epoch.InitialClosedRecord `json:"record"`
	}{"ardents-qualification-node-record-plan-v1", record})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(planPath, plan, 0600); err != nil {
		t.Fatal(err)
	}
	var receipt bytes.Buffer
	recordRoot := filepath.Join(root, "record-output")
	if err := run([]string{"prepare-qualification-node-record", "--plan", planPath, "--output-root", recordRoot}, &receipt); err != nil {
		t.Fatal(err)
	}
	message, err := os.ReadFile(filepath.Join(recordRoot, "node-record.signing-input"))
	if err != nil {
		t.Fatal(err)
	}
	signedRecord := append(append([]byte(nil), message...), ed25519.Sign(nodeKey, message)...)
	public := authority.Public().(ed25519.PublicKey)
	input := epoch.InitialClosedEpoch{NetworkID: network, AssignmentSeed: [32]byte{8}, ValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(time.Hour), Domains: []string{"closed"}, Records: [][]byte{signedRecord}, AuthorityKeys: []ed25519.PublicKey{public}}
	epochPlan := filepath.Join(root, "epoch.json")
	plan, err = json.Marshal(struct {
		Schema string                   `json:"schema"`
		Epoch  epoch.InitialClosedEpoch `json:"epoch"`
	}{"ardents-qualification-epoch-plan-v1", input})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(epochPlan, plan, 0600); err != nil {
		t.Fatal(err)
	}
	epochRoot := filepath.Join(root, "epoch-output")
	receipt.Reset()
	arguments := []string{"prepare-qualification-epoch", "--plan", epochPlan, "--output-root", epochRoot}
	if err := run(arguments, &receipt); err != nil {
		t.Fatal(err)
	}
	message, err = os.ReadFile(filepath.Join(epochRoot, "epoch.unsigned"))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(message)
	id := sha256.Sum256(public)
	wire := append(append([]byte(nil), message...), 1)
	wire = append(wire, id[:]...)
	wire = append(wire, ed25519.Sign(authority, digest[:])...)
	policy := epoch.Policy{NetworkID: network, Authorities: map[[32]byte]ed25519.PublicKey{id: public}, Threshold: 1, Profile: epoch.ProfileClosedRoute, Now: now}
	decision, err := epoch.Verify(policy, wire, input.Records, nil, false)
	if err != nil || len(decision.Candidates) != 1 {
		t.Fatalf("ordinary initial Epoch verification: %v", err)
	}
	material, err := decision.Materialization(0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := epoch.Verify(policy, wire, input.Records, [][]byte{material}, true); err != nil {
		t.Fatalf("ordinary materialized verification: %v", err)
	}
	receipt.Reset()
	if err := run(arguments, &receipt); err == nil {
		t.Fatal("existing Epoch output replaced")
	}
	if receipt.Len() != 0 {
		t.Fatal("replacement refusal acknowledged success")
	}
	after, err := os.ReadFile(filepath.Join(epochRoot, "epoch.unsigned"))
	if err != nil || !bytes.Equal(after, message) {
		t.Fatalf("replacement changed initial Epoch: %v", err)
	}
	signedRecord[len(signedRecord)-1] ^= 1
	plan, err = json.Marshal(struct {
		Schema string                   `json:"schema"`
		Epoch  epoch.InitialClosedEpoch `json:"epoch"`
	}{"ardents-qualification-epoch-plan-v1", input})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(epochPlan, plan, 0600); err != nil {
		t.Fatal(err)
	}
	refusedRoot := filepath.Join(root, "forged-output")
	receipt.Reset()
	if err := run([]string{"prepare-qualification-epoch", "--plan", epochPlan, "--output-root", refusedRoot}, &receipt); err == nil {
		t.Fatal("forged Node Record accepted")
	}
	if _, err := os.Lstat(refusedRoot); !os.IsNotExist(err) || receipt.Len() != 0 {
		t.Fatalf("forged Record refusal changed output: %v", err)
	}
}
