package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/alphacontrol"
)

func TestQualificationCatalogCommandFeedsOrdinaryVerifierAndRefusesReplacement(t *testing.T) {
	root := t.TempDir()
	now := time.Unix(2_000_400_000, 0).UTC()
	catalog := alphacontrol.Catalog{Cohort: "command-diagnostic", Generation: 1, NotBefore: now.Add(-time.Second), NotAfter: now.Add(time.Hour)}
	for i := range catalog.Components {
		catalog.Components[i] = alphacontrol.Component{Class: alphacontrol.ComponentClass(i + 1), RootID: [32]byte{byte(i + 1)}, Generation: 1, NotAfter: now.Add(time.Minute), Size: 100, Digest: [32]byte{byte(i + 4)}}
	}
	plan, err := json.Marshal(struct {
		Schema  string               `json:"schema"`
		Catalog alphacontrol.Catalog `json:"catalog"`
	}{"ardents-qualification-alpha-catalog-plan-v1", catalog})
	if err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(root, "plan.json")
	if err := os.WriteFile(planPath, plan, 0600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "prepared")
	var receipt bytes.Buffer
	args := []string{"prepare-qualification-catalog", "--plan", planPath, "--output-root", output}
	if err := run(args, &receipt); err != nil {
		t.Fatal(err)
	}
	message, err := os.ReadFile(filepath.Join(output, "catalog.signing-input"))
	if err != nil {
		t.Fatal(err)
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	domain := []byte("ardents-alpha-control-catalog-v1\x00")
	if !bytes.HasPrefix(message, domain) {
		t.Fatal("catalog domain absent")
	}
	raw := append(append([]byte(nil), message[len(domain):]...), ed25519.Sign(private, message)...)
	if verified, _, err := alphacontrol.Verify(raw, public, now); err != nil || verified.Cohort != catalog.Cohort {
		t.Fatalf("ordinary catalog verification: %v", err)
	}
	receipt.Reset()
	if err := run(args, &receipt); err == nil {
		t.Fatal("existing output replaced")
	}
	if receipt.Len() != 0 {
		t.Fatal("replacement refusal acknowledged success")
	}
	after, err := os.ReadFile(filepath.Join(output, "catalog.signing-input"))
	if err != nil || !bytes.Equal(after, message) {
		t.Fatalf("refusal changed bytes: %v", err)
	}
}
